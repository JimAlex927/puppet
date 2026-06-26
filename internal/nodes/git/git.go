package git

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"puppet/internal/node"
)

type Executor struct{}

func New() *Executor {
	return &Executor{}
}

func (e *Executor) Type() string {
	return "git"
}

func (e *Executor) Metadata() node.NodeMetadata {
	return node.NodeMetadata{
		Type:        e.Type(),
		Name:        "Git Checkout",
		Category:    "scm",
		Description: "按指定 transport 和工作区策略检出 Git 仓库代码",
		SupportedOS: []string{"linux", "darwin", "windows"},
		Fields: []node.NodeField{
			{Name: "repoUrl", Label: "Repository URL", Type: "input", Required: true},
			{Name: "transport", Label: "Transport", Type: "select", Required: true, Default: "https", Options: []string{"https", "ssh"}},
			{Name: "refType", Label: "Ref Type", Type: "select", Required: true, Default: "branch", Options: []string{"branch", "tag", "commit"}},
			{Name: "ref", Label: "Ref", Type: "input", Required: true, Default: "main"},
			{Name: "credentialId", Label: "Credential", Type: "credential", Required: false},
			{Name: "checkoutDir", Label: "Checkout Dir", Type: "input", Required: false, Default: "${workspace}/source"},
			{Name: "workspacePolicy", Label: "Workspace Policy", Type: "select", Required: true, Default: "fail_if_dirty", Options: []string{"fail_if_dirty", "reset_and_clean", "wipe_and_clone", "reuse"}},
			{Name: "depth", Label: "Depth", Type: "number", Required: false, Default: 1},
			{Name: "submodules", Label: "Submodules", Type: "switch", Required: false, Default: false},
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
	transport := normalizedTransport(params)
	if transport != "https" && transport != "ssh" && transport != "local" {
		return fmt.Errorf("transport must be https or ssh")
	}
	if err := validateTransportURL(transport, stringFrom(params["repoUrl"])); err != nil {
		return err
	}
	refType := stringFrom(params["refType"])
	if refType == "" {
		refType = "branch"
	}
	if refType != "branch" && refType != "tag" && refType != "commit" {
		return fmt.Errorf("refType must be branch, tag or commit")
	}
	if strings.TrimSpace(stringFrom(params["ref"])) == "" {
		return fmt.Errorf("ref is required")
	}
	if err := validateWorkspacePolicy(normalizedWorkspacePolicy(params)); err != nil {
		return err
	}
	return nil
}

func (e *Executor) Execute(ctx *node.NodeContext, params map[string]any) (*node.NodeResult, error) {
	repoURL := strings.TrimSpace(stringFrom(params["repoUrl"]))
	transport := normalizedTransport(params)
	refType := strings.TrimSpace(stringFrom(params["refType"]))
	if refType == "" {
		refType = "branch"
	}
	ref := strings.TrimSpace(stringFrom(params["ref"]))
	checkoutDir, err := resolveTargetDir(ctx.Workspace, stringFrom(params["checkoutDir"]))
	if err != nil {
		return nil, err
	}
	depth := intFrom(params["depth"])
	workspacePolicy := normalizedWorkspacePolicy(params)
	submodules := boolFrom(params["submodules"])

	credentialID := uintFrom(params["credentialId"])
	credential, err := resolveCredential(ctx, credentialID)
	if err != nil {
		return nil, err
	}
	if err := validateCredentialForTransport(transport, credential); err != nil {
		return nil, err
	}
	auth, err := prepareAuth(ctx.Workspace, transport, credential)
	if err != nil {
		return nil, err
	}
	defer auth.cleanup()

	ctx.Log("system", "git checkout plan\n")
	ctx.Log("system", fmt.Sprintf("- transport: %s\n", transport))
	ctx.Log("system", fmt.Sprintf("- repository: %s\n", maskURL(repoURL)))
	ctx.Log("system", fmt.Sprintf("- ref: %s:%s\n", refType, ref))
	ctx.Log("system", fmt.Sprintf("- checkoutDir: %s\n", checkoutDir))
	ctx.Log("system", fmt.Sprintf("- credential: %s\n", credentialLabel(credential)))
	ctx.Log("system", fmt.Sprintf("- workspacePolicy: %s\n", workspacePolicy))
	ctx.Log("system", fmt.Sprintf("- depth: %d, submodules: %t\n", depth, submodules))

	action, err := e.prepareWorkspace(ctx, checkoutDir, repoURL, workspacePolicy, auth)
	if err != nil {
		return nil, err
	}

	if action == "update" {
		if err := e.updateRepo(ctx, checkoutDir, repoURL, refType, ref, auth); err != nil {
			return nil, err
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(checkoutDir), 0o755); err != nil {
			return nil, err
		}
		if err := e.cloneRepo(ctx, repoURL, refType, ref, checkoutDir, depth, auth); err != nil {
			return nil, err
		}
	}

	if submodules {
		if err := e.runGit(ctx, checkoutDir, auth, "submodule", "update", "--init", "--recursive"); err != nil {
			return nil, err
		}
	}

	commit, _ := e.gitOutput(ctx, checkoutDir, auth, "rev-parse", "HEAD")
	branch, _ := e.gitOutput(ctx, checkoutDir, auth, "rev-parse", "--abbrev-ref", "HEAD")
	message, _ := e.gitOutput(ctx, checkoutDir, auth, "log", "-1", "--pretty=%s")
	author, _ := e.gitOutput(ctx, checkoutDir, auth, "log", "-1", "--pretty=%an <%ae>")
	commit = strings.TrimSpace(commit)
	branch = strings.TrimSpace(branch)
	message = strings.TrimSpace(message)
	author = strings.TrimSpace(author)

	ctx.Log("system", "git checkout result\n")
	ctx.Log("system", fmt.Sprintf("- commit: %s\n", commit))
	ctx.Log("system", fmt.Sprintf("- branch: %s\n", branch))
	ctx.Log("system", fmt.Sprintf("- author: %s\n", author))
	ctx.Log("system", fmt.Sprintf("- message: %s\n", message))

	return &node.NodeResult{Output: map[string]any{
		"repoUrl":      maskURL(repoURL),
		"transport":    transport,
		"refType":      refType,
		"ref":          ref,
		"branch":       branch,
		"commit":       commit,
		"message":      message,
		"author":       author,
		"checkoutDir":  checkoutDir,
		"credentialId": credentialID,
	}}, nil
}

func (e *Executor) prepareWorkspace(ctx *node.NodeContext, checkoutDir string, repoURL string, policy string, auth authContext) (string, error) {
	ctx.Log("system", "workspace check\n")
	exists, isDir, err := pathState(checkoutDir)
	if err != nil {
		return "", err
	}
	if !exists {
		ctx.Log("system", "- checkoutDir does not exist, will clone\n")
		return "clone", nil
	}
	if !isDir {
		return "", fmt.Errorf("checkoutDir exists but is not a directory")
	}
	if isEmptyDir(checkoutDir) {
		ctx.Log("system", "- checkoutDir exists and is empty, will clone into it\n")
		return "clone", nil
	}
	if !isGitRepo(checkoutDir) {
		ctx.Log("system", "- checkoutDir exists but is not a git repository\n")
		if policy == "wipe_and_clone" {
			ctx.Log("system", "- policy wipe_and_clone selected, removing checkoutDir\n")
			return "clone", os.RemoveAll(checkoutDir)
		}
		return "", fmt.Errorf("checkoutDir exists and is not a git repository; choose wipe_and_clone to replace it")
	}

	remote, _ := e.gitOutput(ctx, checkoutDir, auth, "remote", "get-url", "origin")
	remote = strings.TrimSpace(remote)
	ctx.Log("system", "- repository detected\n")
	if remote != "" {
		ctx.Log("system", fmt.Sprintf("- origin: %s\n", maskURL(remote)))
	}

	dirty, statusOutput, err := e.isDirty(ctx, checkoutDir, auth)
	if err != nil {
		return "", err
	}
	if dirty {
		ctx.Log("system", "- worktree status: dirty\n")
		ctx.Log("system", "- dirty summary:\n")
		for _, line := range strings.Split(strings.TrimSpace(statusOutput), "\n") {
			if strings.TrimSpace(line) != "" {
				ctx.Log("system", fmt.Sprintf("  %s\n", line))
			}
		}
	} else {
		ctx.Log("system", "- worktree status: clean\n")
	}

	switch policy {
	case "fail_if_dirty":
		if dirty {
			return "", fmt.Errorf("checkoutDir has uncommitted or untracked changes; choose reset_and_clean, wipe_and_clone or reuse")
		}
		return "update", nil
	case "reset_and_clean":
		ctx.Log("system", "- policy reset_and_clean selected\n")
		if err := e.runGit(ctx, checkoutDir, auth, "reset", "--hard"); err != nil {
			return "", err
		}
		if err := e.runGit(ctx, checkoutDir, auth, "clean", "-fdx"); err != nil {
			return "", err
		}
		return "update", nil
	case "wipe_and_clone":
		ctx.Log("system", "- policy wipe_and_clone selected, removing checkoutDir\n")
		if err := os.RemoveAll(checkoutDir); err != nil {
			return "", err
		}
		return "clone", nil
	case "reuse":
		ctx.Log("system", "- policy reuse selected, will not clean dirty files\n")
		return "update", nil
	default:
		return "", fmt.Errorf("unsupported workspacePolicy %q", policy)
	}
}

func (e *Executor) cloneRepo(ctx *node.NodeContext, repoURL, refType, ref, checkoutDir string, depth int, auth authContext) error {
	args := []string{"clone"}
	if refType == "branch" || refType == "tag" {
		args = append(args, "--branch", ref)
	}
	if depth > 0 && refType != "commit" {
		args = append(args, "--depth", strconv.Itoa(depth))
	}
	args = append(args, repoURL, checkoutDir)
	if err := e.runGit(ctx, ctx.Workspace, auth, args...); err != nil {
		return err
	}
	if refType == "commit" {
		return e.runGit(ctx, checkoutDir, auth, "checkout", ref)
	}
	return nil
}

func (e *Executor) updateRepo(ctx *node.NodeContext, checkoutDir, repoURL, refType, ref string, auth authContext) error {
	if err := e.runGit(ctx, checkoutDir, auth, "remote", "set-url", "origin", repoURL); err != nil {
		return err
	}
	if err := e.runGit(ctx, checkoutDir, auth, "fetch", "--tags", "--prune", "origin"); err != nil {
		return err
	}
	switch refType {
	case "branch":
		if err := e.runGit(ctx, checkoutDir, auth, "checkout", ref); err != nil {
			return err
		}
		return e.runGit(ctx, checkoutDir, auth, "pull", "--ff-only", "origin", ref)
	case "tag":
		return e.runGit(ctx, checkoutDir, auth, "checkout", "tags/"+ref)
	case "commit":
		return e.runGit(ctx, checkoutDir, auth, "checkout", ref)
	default:
		return fmt.Errorf("unsupported refType %q", refType)
	}
}

func (e *Executor) isDirty(ctx *node.NodeContext, checkoutDir string, auth authContext) (bool, string, error) {
	content, err := e.gitOutput(ctx, checkoutDir, auth, "status", "--porcelain")
	if err != nil {
		return false, "", err
	}
	return strings.TrimSpace(content) != "", content, nil
}

func (e *Executor) runGit(ctx *node.NodeContext, dir string, auth authContext, args ...string) error {
	ctx.Log("system", fmt.Sprintf("$ git %s\n", maskArgs(args, auth.secrets)))
	cmd := exec.CommandContext(ctx.Context, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), auth.env...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go scan(stdout, "stdout", auth.secrets, ctx.Log, &wg)
	go scan(stderr, "stderr", auth.secrets, ctx.Log, &wg)
	wg.Wait()
	return cmd.Wait()
}

func (e *Executor) gitOutput(ctx *node.NodeContext, dir string, auth authContext, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx.Context, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), auth.env...)
	content, err := cmd.CombinedOutput()
	return maskSecrets(string(content), auth.secrets), err
}

type authContext struct {
	env     []string
	secrets []string
	cleanup func()
}

func noAuth() authContext {
	return authContext{cleanup: func() {}}
}

func resolveCredential(ctx *node.NodeContext, id uint) (*node.Credential, error) {
	if id == 0 {
		return nil, nil
	}
	if ctx.ResolveCredential == nil {
		return nil, fmt.Errorf("credential resolver is not configured")
	}
	return ctx.ResolveCredential(id)
}

func prepareAuth(workspace string, transport string, credential *node.Credential) (authContext, error) {
	if credential == nil {
		return noAuth(), nil
	}
	switch credential.Type {
	case "username_password":
		password := credential.Secrets["password"]
		return prepareAskPass(workspace, credential.Username, password, []string{credential.Username, password})
	case "token":
		token := credential.Secrets["token"]
		username := credential.Username
		if username == "" {
			username = "x-access-token"
		}
		return prepareAskPass(workspace, username, token, []string{username, token})
	case "ssh_key":
		if transport != "ssh" {
			return authContext{}, fmt.Errorf("ssh_key credential requires transport=ssh")
		}
		return prepareSSHKey(workspace, credential.Secrets["privateKey"])
	default:
		return authContext{}, fmt.Errorf("unsupported credential type %q", credential.Type)
	}
}

func prepareAskPass(workspace string, username string, password string, secrets []string) (authContext, error) {
	if username == "" || password == "" {
		return authContext{}, fmt.Errorf("credential is missing username or password/token")
	}
	dir, err := os.MkdirTemp(workspace, ".git-askpass-*")
	if err != nil {
		return authContext{}, err
	}
	userFile := filepath.Join(dir, "username.txt")
	passFile := filepath.Join(dir, "password.txt")
	if err := os.WriteFile(userFile, []byte(username), 0o600); err != nil {
		os.RemoveAll(dir)
		return authContext{}, err
	}
	if err := os.WriteFile(passFile, []byte(password), 0o600); err != nil {
		os.RemoveAll(dir)
		return authContext{}, err
	}
	script, err := writeAskPassScript(dir, userFile, passFile)
	if err != nil {
		os.RemoveAll(dir)
		return authContext{}, err
	}
	return authContext{
		env: []string{
			"GIT_TERMINAL_PROMPT=0",
			"GIT_ASKPASS=" + script,
		},
		secrets: secrets,
		cleanup: func() { _ = os.RemoveAll(dir) },
	}, nil
}

func writeAskPassScript(dir string, userFile string, passFile string) (string, error) {
	if runtime.GOOS == "windows" {
		script := filepath.Join(dir, "askpass.cmd")
		content := fmt.Sprintf("@echo off\r\necho %%~1 | findstr /I \"Username\" >nul\r\nif %%errorlevel%%==0 (type \"%s\") else (type \"%s\")\r\n", userFile, passFile)
		return script, os.WriteFile(script, []byte(content), 0o700)
	}
	script := filepath.Join(dir, "askpass.sh")
	content := fmt.Sprintf("#!/bin/sh\ncase \"$1\" in\n  *Username*) cat %q ;;\n  *) cat %q ;;\nesac\n", userFile, passFile)
	return script, os.WriteFile(script, []byte(content), 0o700)
}

func prepareSSHKey(workspace string, privateKey string) (authContext, error) {
	if privateKey == "" {
		return authContext{}, fmt.Errorf("credential is missing privateKey")
	}
	// Normalize line endings (CRLF → LF) and ensure trailing newline.
	// OpenSSH rejects PEM keys that have \r\n endings or no final newline.
	privateKey = strings.ReplaceAll(privateKey, "\r\n", "\n")
	privateKey = strings.ReplaceAll(privateKey, "\r", "\n")
	if !strings.HasSuffix(privateKey, "\n") {
		privateKey += "\n"
	}
	dir, err := os.MkdirTemp(workspace, ".git-ssh-*")
	if err != nil {
		return authContext{}, err
	}
	keyFile := filepath.Join(dir, "id_key")
	if err := os.WriteFile(keyFile, []byte(privateKey), 0o600); err != nil {
		os.RemoveAll(dir)
		return authContext{}, err
	}
	sshCommand := fmt.Sprintf("ssh -i %q -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new", keyFile)
	return authContext{
		env:     []string{"GIT_TERMINAL_PROMPT=0", "GIT_SSH_COMMAND=" + sshCommand},
		secrets: []string{privateKey, keyFile},
		cleanup: func() { _ = os.RemoveAll(dir) },
	}, nil
}

func resolveTargetDir(workspace string, raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		raw = "${workspace}/source"
	}
	raw = strings.ReplaceAll(raw, "${workspace}", workspace)
	raw = filepath.Clean(raw)
	workspace = filepath.Clean(workspace)
	if !filepath.IsAbs(raw) && !strings.HasPrefix(raw, workspace+string(filepath.Separator)) {
		raw = filepath.Join(workspace, raw)
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(workspaceAbs, abs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("checkoutDir must stay inside workspace")
	}
	return abs, nil
}

func isGitRepo(targetDir string) bool {
	info, err := os.Stat(filepath.Join(targetDir, ".git"))
	return err == nil && info.IsDir()
}

func pathState(path string) (bool, bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return true, info.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, false, nil
	}
	return false, false, err
}

func isEmptyDir(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) == 0
}

func scan(reader io.Reader, stream string, secrets []string, log func(string, string), wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		log(stream, maskSecrets(scanner.Text(), secrets)+"\n")
	}
}

func maskArgs(args []string, secrets []string) string {
	return maskSecrets(strings.Join(args, " "), secrets)
}

func maskURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.User == nil {
		return value
	}
	parsed.User = url.User("***")
	return parsed.String()
}

func maskSecrets(value string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "***")
		}
	}
	return value
}

func credentialLabel(credential *node.Credential) string {
	if credential == nil {
		return "none"
	}
	return fmt.Sprintf("#%d %s (%s)", credential.ID, credential.Name, credential.Type)
}

func looksLikeSSHURL(repoURL string) bool {
	return strings.HasPrefix(repoURL, "git@") || strings.HasPrefix(repoURL, "ssh://")
}

func looksLikeHTTPSURL(repoURL string) bool {
	return strings.HasPrefix(repoURL, "https://")
}

func looksLikeLocalRepo(repoURL string) bool {
	if strings.HasPrefix(repoURL, "file://") {
		return true
	}
	return filepath.IsAbs(repoURL) || strings.HasPrefix(repoURL, ".")
}

func normalizedTransport(params map[string]any) string {
	transport := strings.ToLower(strings.TrimSpace(stringFrom(params["transport"])))
	if transport != "" {
		return transport
	}
	repoURL := strings.TrimSpace(stringFrom(params["repoUrl"]))
	if looksLikeSSHURL(repoURL) {
		return "ssh"
	}
	if looksLikeLocalRepo(repoURL) {
		return "local"
	}
	return "https"
}

func normalizedWorkspacePolicy(params map[string]any) string {
	policy := strings.ToLower(strings.TrimSpace(stringFrom(params["workspacePolicy"])))
	if policy != "" {
		return policy
	}
	if boolFrom(params["clean"]) {
		return "wipe_and_clone"
	}
	return "fail_if_dirty"
}

func validateWorkspacePolicy(policy string) error {
	switch policy {
	case "fail_if_dirty", "reset_and_clean", "wipe_and_clone", "reuse":
		return nil
	default:
		return fmt.Errorf("workspacePolicy must be fail_if_dirty, reset_and_clean, wipe_and_clone or reuse")
	}
}

func validateTransportURL(transport string, repoURL string) error {
	switch transport {
	case "https":
		if !looksLikeHTTPSURL(repoURL) {
			return fmt.Errorf("transport=https requires an https:// repository URL")
		}
	case "ssh":
		if !looksLikeSSHURL(repoURL) {
			return fmt.Errorf("transport=ssh requires git@host:path or ssh:// repository URL")
		}
	case "local":
		if !looksLikeLocalRepo(repoURL) {
			return fmt.Errorf("local repository URL must be an absolute path, relative path or file:// URL")
		}
	}
	return nil
}

func validateCredentialForTransport(transport string, credential *node.Credential) error {
	if credential == nil {
		return nil
	}
	switch transport {
	case "https":
		if credential.Type != "username_password" && credential.Type != "token" {
			return fmt.Errorf("transport=https requires token or username_password credential")
		}
	case "ssh":
		if credential.Type != "ssh_key" {
			return fmt.Errorf("transport=ssh requires ssh_key credential")
		}
	case "local":
		return fmt.Errorf("local repository checkout does not use credentials")
	}
	return nil
}

func stringFrom(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func intFrom(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		var number int
		_, _ = fmt.Sscanf(typed, "%d", &number)
		return number
	default:
		return 0
	}
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
		var number uint64
		_, _ = fmt.Sscanf(typed, "%d", &number)
		return uint(number)
	default:
		return 0
	}
}

func boolFrom(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed == "true" || typed == "1"
	default:
		return false
	}
}
