package script

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"puppet/internal/confignode"
	"puppet/internal/node"
	"puppet/internal/shellutil"
)

type Executor struct{}

func New() *Executor { return &Executor{} }

func (e *Executor) Type() string { return "script" }

func (e *Executor) Metadata() node.NodeMetadata {
	return node.NodeMetadata{
		Type:        "script",
		Name:        "脚本",
		Category:    "script",
		Description: "执行 Shell 脚本，标准输出每行作为一个选项",
		SupportedOS: []string{"linux", "darwin", "windows"},
		Fields: []node.NodeField{
			{
				Name:     "script",
				Label:    "脚本",
				Type:     "textarea",
				Required: true,
			},
			{
				Name:    "shell",
				Label:   "Shell",
				Type:    "select",
				Default: "auto",
				Options: []string{"auto", "pwsh", "powershell", "cmd", "bat", "sh", "bash"},
			},
			{
				Name:    "timeoutSeconds",
				Label:   "超时（秒）",
				Type:    "number",
				Default: float64(30),
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

func (e *Executor) Execute(ctx confignode.Context, params map[string]any) (confignode.Result, error) {
	if err := e.Validate(params); err != nil {
		return confignode.Result{}, err
	}

	script := strings.TrimSpace(stringFrom(params["script"]))
	timeout := 30
	if t, ok := params["timeoutSeconds"].(float64); ok && t > 0 {
		timeout = int(t)
	}

	execCtx, cancel := context.WithTimeout(ctx.Context, time.Duration(timeout)*time.Second)
	defer cancel()

	shell := stringFrom(params["shell"])
	cmd, cleanup, err := shellutil.BuildSilentCommand(execCtx, script, shell)
	if err != nil {
		return confignode.Result{}, err
	}
	defer cleanup()

	out, err := cmd.Output()
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return confignode.Result{}, fmt.Errorf("script timed out after %ds", timeout)
		}
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return confignode.Result{}, fmt.Errorf("script failed (exit %d): %s", exitErr.ExitCode(), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return confignode.Result{}, fmt.Errorf("script failed: %w", err)
	}

	options := []string{}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			options = append(options, line)
		}
	}

	return confignode.Result{Output: map[string]any{"options": options}}, nil
}

func stringFrom(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprint(value)
}
