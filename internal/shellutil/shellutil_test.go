package shellutil

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestBuildSilentCommandPowerShellOmitsDebugTrace(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not found")
	}

	out := runSilentShell(t, "Write-Output \"1\"\nWrite-Output \"2\"", "pwsh")
	lines := nonEmptyLines(out)
	if got, want := strings.Join(lines, ","), "1,2"; got != want {
		t.Fatalf("unexpected stdout lines: got %q, want %q; raw output:\n%s", got, want, out)
	}
	for _, line := range lines {
		if strings.Contains(line, "DEBUG:") || strings.Contains(line, "Write-Output") {
			t.Fatalf("stdout contains PowerShell trace line %q; raw output:\n%s", line, out)
		}
	}
}

func TestBuildSilentCommandCmdOmitsCommandEcho(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("cmd is only available on Windows")
	}
	if _, err := exec.LookPath("cmd"); err != nil {
		t.Skip("cmd not found")
	}

	out := runSilentShell(t, "echo 1\necho 2", "cmd")
	lines := nonEmptyLines(out)
	if got, want := strings.Join(lines, ","), "1,2"; got != want {
		t.Fatalf("unexpected stdout lines: got %q, want %q; raw output:\n%s", got, want, out)
	}
	for _, line := range lines {
		if strings.Contains(line, "echo ") {
			t.Fatalf("stdout contains cmd echo line %q; raw output:\n%s", line, out)
		}
	}
}

func TestBuildCommandPowerShellFailsOnMissingCommandWithoutDebugTrace(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not found")
	}

	out, err := runShell("Definitely-Missing-Puppet-Command\nWrite-Output \"after\"", "pwsh")
	if err == nil {
		t.Fatalf("expected pwsh command to fail; output:\n%s", out)
	}
	if strings.Contains(out, "after") {
		t.Fatalf("script continued after missing command; output:\n%s", out)
	}
	if strings.Contains(out, "DEBUG:") {
		t.Fatalf("stdout contains PowerShell debug trace; output:\n%s", out)
	}
}

func TestBuildCommandCmdFailsOnMissingCommandBeforeLaterSuccess(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("cmd is only available on Windows")
	}
	if _, err := exec.LookPath("cmd"); err != nil {
		t.Skip("cmd not found")
	}

	out, err := runShell("definitely_missing_puppet_command_12345\necho after", "cmd")
	if err == nil {
		t.Fatalf("expected cmd command to fail; output:\n%s", out)
	}
	if strings.Contains(out, "after") {
		t.Fatalf("script continued after missing command; output:\n%s", out)
	}
}

func runSilentShell(t *testing.T, script, shell string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd, cleanup, err := BuildSilentCommand(ctx, script, shell)
	if err != nil {
		t.Fatalf("build command: %v", err)
	}
	defer cleanup()

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run command: %v; output:\n%s", err, out)
	}
	return string(out)
}

func runShell(script, shell string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd, cleanup, err := BuildCommand(ctx, script, shell)
	if err != nil {
		return "", err
	}
	defer cleanup()

	out, err := cmd.CombinedOutput()
	return string(out), err
}

func nonEmptyLines(out string) []string {
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
