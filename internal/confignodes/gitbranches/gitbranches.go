package gitbranches

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"puppet/internal/confignode"
	"puppet/internal/node"
)

type Executor struct{}

func New() *Executor {
	return &Executor{}
}

func (e *Executor) Type() string {
	return "git_branches"
}

func (e *Executor) Metadata() node.NodeMetadata {
	return node.NodeMetadata{
		Type:        e.Type(),
		Name:        "Git Branches",
		Category:    "scm",
		Description: "动态获取 Git 远程分支列表",
		SupportedOS: []string{"linux", "darwin", "windows"},
		Fields: []node.NodeField{
			{Name: "repoUrl", Label: "Repository URL", Type: "input", Required: true},
			{Name: "transport", Label: "Transport", Type: "select", Required: true, Default: "https", Options: []string{"https", "ssh"}},
			{Name: "credentialId", Label: "Credential", Type: "credential", Required: false},
			{Name: "pattern", Label: "Pattern", Type: "input", Required: false, Default: "*"},
		},
	}
}

func (e *Executor) Validate(params map[string]any) error {
	if strings.TrimSpace(stringFrom(params["repoUrl"])) == "" {
		return fmt.Errorf("repoUrl is required")
	}
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git executable was not found in PATH")
	}
	return nil
}

func (e *Executor) Execute(ctx confignode.Context, params map[string]any) (confignode.Result, error) {
	if err := e.Validate(params); err != nil {
		return confignode.Result{}, err
	}
	repoURL := strings.TrimSpace(stringFrom(params["repoUrl"]))
	credentialID := uintFrom(params["credentialId"])
	var credential *node.Credential
	var err error
	if credentialID > 0 {
		if ctx.ResolveCredential == nil {
			return confignode.Result{}, fmt.Errorf("credential resolver is not configured")
		}
		credential, err = ctx.ResolveCredential(credentialID)
		if err != nil {
			return confignode.Result{}, err
		}
	}
	auth, err := prepareAuth(credential)
	if err != nil {
		return confignode.Result{}, err
	}
	defer auth.cleanup()
	commandCtx := ctx.Context
	if commandCtx == nil {
		commandCtx = context.Background()
	}
	cmd := exec.CommandContext(commandCtx, "git", "ls-remote", "--heads", repoURL)
	cmd.Env = append(os.Environ(), auth.env...)
	content, err := cmd.CombinedOutput()
	if err != nil {
		return confignode.Result{}, fmt.Errorf("git ls-remote failed: %s", maskSecrets(string(content), auth.secrets))
	}
	pattern := strings.TrimSpace(stringFrom(params["pattern"]))
	if pattern == "" {
		pattern = "*"
	}
	branches := parseBranches(string(content), pattern)
	return confignode.Result{Output: map[string]any{"options": branches, "branches": branches}}, nil
}

type authContext struct {
	env     []string
	secrets []string
	cleanup func()
}

func prepareAuth(credential *node.Credential) (authContext, error) {
	if credential == nil {
		return authContext{cleanup: func() {}}, nil
	}
	switch credential.Type {
	case "username_password":
		return prepareAskPass(credential.Username, credential.Secrets["password"], []string{credential.Username, credential.Secrets["password"]})
	case "token":
		username := credential.Username
		if username == "" {
			username = "x-access-token"
		}
		return prepareAskPass(username, credential.Secrets["token"], []string{username, credential.Secrets["token"]})
	case "ssh_key":
		return prepareSSHKey(credential.Secrets["privateKey"])
	default:
		return authContext{}, fmt.Errorf("unsupported credential type %q", credential.Type)
	}
}

func prepareAskPass(username string, password string, secrets []string) (authContext, error) {
	if username == "" || password == "" {
		return authContext{}, fmt.Errorf("credential is missing username or password/token")
	}
	dir, err := os.MkdirTemp("", "puppet-config-git-askpass-*")
	if err != nil {
		return authContext{}, err
	}
	userFile := filepath.Join(dir, "username.txt")
	passFile := filepath.Join(dir, "password.txt")
	_ = os.WriteFile(userFile, []byte(username), 0o600)
	_ = os.WriteFile(passFile, []byte(password), 0o600)
	script := filepath.Join(dir, "askpass.sh")
	content := fmt.Sprintf("#!/bin/sh\ncase \"$1\" in\n  *Username*) cat %q ;;\n  *) cat %q ;;\nesac\n", userFile, passFile)
	if runtime.GOOS == "windows" {
		script = filepath.Join(dir, "askpass.cmd")
		content = fmt.Sprintf("@echo off\r\necho %%~1 | findstr /I \"Username\" >nul\r\nif %%errorlevel%%==0 (type \"%s\") else (type \"%s\")\r\n", userFile, passFile)
	}
	if err := os.WriteFile(script, []byte(content), 0o700); err != nil {
		_ = os.RemoveAll(dir)
		return authContext{}, err
	}
	return authContext{env: []string{"GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=" + script}, secrets: secrets, cleanup: func() { _ = os.RemoveAll(dir) }}, nil
}

func prepareSSHKey(privateKey string) (authContext, error) {
	if privateKey == "" {
		return authContext{}, fmt.Errorf("credential is missing privateKey")
	}
	dir, err := os.MkdirTemp("", "puppet-config-git-ssh-*")
	if err != nil {
		return authContext{}, err
	}
	keyFile := filepath.Join(dir, "id_key")
	if err := os.WriteFile(keyFile, []byte(privateKey), 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return authContext{}, err
	}
	return authContext{env: []string{"GIT_TERMINAL_PROMPT=0", fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %q -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new", keyFile)}, secrets: []string{privateKey, keyFile}, cleanup: func() { _ = os.RemoveAll(dir) }}, nil
}

func parseBranches(content string, pattern string) []string {
	branches := []string{}
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) < 2 || !strings.HasPrefix(parts[1], "refs/heads/") {
			continue
		}
		branch := strings.TrimPrefix(parts[1], "refs/heads/")
		if ok, _ := filepath.Match(pattern, branch); pattern == "*" || ok {
			branches = append(branches, branch)
		}
	}
	sort.Strings(branches)
	return branches
}

func maskSecrets(value string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "***")
		}
	}
	return value
}

func stringFrom(value any) string {
	if value == nil {
		return ""
	}
	if typed, ok := value.(string); ok {
		return typed
	}
	return fmt.Sprint(value)
}

func uintFrom(value any) uint {
	switch typed := value.(type) {
	case float64:
		return uint(typed)
	case int:
		return uint(typed)
	case uint:
		return typed
	case string:
		var number uint
		_, _ = fmt.Sscanf(typed, "%d", &number)
		return number
	default:
		return 0
	}
}
