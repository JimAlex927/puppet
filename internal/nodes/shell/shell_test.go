package shell

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"puppet/internal/node"
)

func TestExecuteLogsCommandsWithoutPowerShellDebugTrace(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not found")
	}

	var logs []string
	_, err := New().Execute(&node.NodeContext{
		Context:   context.Background(),
		Workspace: t.TempDir(),
		Log: func(stream string, content string) {
			logs = append(logs, content)
		},
	}, map[string]any{
		"shell":  "pwsh",
		"script": "Write-Output \"ok\"",
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(logs, "")
	if !strings.Contains(joined, "+ Write-Output \"ok\"") {
		t.Fatalf("expected command preview in logs, got:\n%s", joined)
	}
	if strings.Contains(joined, "DEBUG:") {
		t.Fatalf("unexpected PowerShell debug trace in logs:\n%s", joined)
	}
}

func TestExecuteFailsOnMissingPowerShellCommand(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not found")
	}

	_, err := New().Execute(&node.NodeContext{
		Context:   context.Background(),
		Workspace: t.TempDir(),
		Log:       func(stream string, content string) {},
	}, map[string]any{
		"shell":  "pwsh",
		"script": "Definitely-Missing-Puppet-Command\nWrite-Output \"after\"",
	})
	if err == nil {
		t.Fatal("expected missing PowerShell command to fail the node")
	}
}
