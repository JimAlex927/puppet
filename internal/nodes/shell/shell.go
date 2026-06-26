package shell

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"puppet/internal/node"
	"puppet/internal/shellutil"
)

type Executor struct{}

func New() *Executor {
	return &Executor{}
}

func (e *Executor) Type() string {
	return "shell"
}

func (e *Executor) Metadata() node.NodeMetadata {
	return node.NodeMetadata{
		Type:        e.Type(),
		Name:        "Shell",
		Category:    "build",
		Description: "执行本机 shell 脚本并捕获 stdout/stderr",
		SupportedOS: []string{"linux", "darwin", "windows"},
		Fields: []node.NodeField{
			{Name: "script", Label: "Script", Type: "textarea", Required: true},
			{Name: "workdir", Label: "Workdir", Type: "input", Required: false, Default: "${workspace}"},
			{
				Name:    "shell",
				Label:   "Shell",
				Type:    "select",
				Default: "auto",
				Options: []string{"auto", "pwsh", "powershell", "cmd", "bat", "sh", "bash"},
			},
		},
	}
}

func (e *Executor) Validate(params map[string]any) error {
	if strings.TrimSpace(stringFrom(params["script"])) == "" {
		return fmt.Errorf("script is required")
	}
	return nil
}

func (e *Executor) Execute(ctx *node.NodeContext, params map[string]any) (*node.NodeResult, error) {
	script := stringFrom(params["script"])
	workdir := strings.TrimSpace(stringFrom(params["workdir"]))
	if workdir == "" || workdir == "${workspace}" {
		workdir = ctx.Workspace
	}
	workdir = strings.ReplaceAll(workdir, "${workspace}", ctx.Workspace)
	if !filepath.IsAbs(workdir) {
		workdir = filepath.Join(ctx.Workspace, workdir)
	}
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return nil, err
	}

	shell := stringFrom(params["shell"])
	cmd, cleanup, err := shellutil.BuildCommand(ctx.Context, script, shell)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	cmd.Dir = workdir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	effectiveShell := shell
	if effectiveShell == "" || effectiveShell == "auto" {
		effectiveShell = shellutil.ShellName()
	}
	ctx.Log("stdout", fmt.Sprintf("working directory: %s\n", workdir))
	ctx.Log("stdout", fmt.Sprintf("shell: %s\n", effectiveShell))
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go scan(stdout, "stdout", ctx.Log, &wg)
	go scan(stderr, "stderr", ctx.Log, &wg)

	err = cmd.Wait()
	wg.Wait()
	if err != nil {
		return nil, err
	}
	return &node.NodeResult{Output: map[string]any{"workdir": workdir}}, nil
}

func scan(pipe any, stream string, log func(string, string), wg *sync.WaitGroup) {
	defer wg.Done()
	reader, ok := pipe.(interface{ Read([]byte) (int, error) })
	if !ok {
		return
	}
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		log(stream, scanner.Text()+"\n")
	}
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
