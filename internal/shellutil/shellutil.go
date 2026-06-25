// Package shellutil builds exec.Cmd values for running shell scripts portably.
//
// Shell selection (the shell parameter):
//   ""  / "auto"  – auto-detect: pwsh → powershell → cmd on Windows; sh on Unix
//   "pwsh"        – PowerShell 7+  (Windows/Linux/macOS)
//   "powershell"  – Windows PowerShell 5.1
//   "cmd"         – Windows Command Prompt
//   "sh"          – POSIX sh
//   "bash"        – Bash
//
// On Windows, scripts are always written to a temp file so that multi-line
// scripts, special characters, and non-ASCII text work correctly.
// UTF-8 output encoding is forced in every Windows path.
package shellutil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// BuildCommand returns a ready-to-start Cmd. The shell parameter selects which
// shell to use; pass "" or "auto" for automatic selection. Callers must call
// cleanup() after the command finishes to remove any temp files.
func BuildCommand(ctx context.Context, script, shell string) (cmd *exec.Cmd, cleanup func(), err error) {
	if shell == "" || shell == "auto" {
		return buildAuto(ctx, script)
	}
	return buildExplicit(ctx, script, shell)
}

// ShellName returns the name of the shell that BuildCommand("auto") would pick.
func ShellName() string {
	if runtime.GOOS != "windows" {
		return "sh"
	}
	for _, name := range []string{"pwsh", "powershell", "cmd"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return "unknown"
}

func buildAuto(ctx context.Context, script string) (*exec.Cmd, func(), error) {
	if runtime.GOOS != "windows" {
		return buildExplicit(ctx, script, "sh")
	}
	for _, name := range []string{"pwsh", "powershell", "cmd"} {
		if _, err := exec.LookPath(name); err == nil {
			return buildExplicit(ctx, script, name)
		}
	}
	return nil, func() {}, fmt.Errorf("no usable shell found on PATH (tried pwsh, powershell, cmd)")
}

func buildExplicit(ctx context.Context, script, shell string) (*exec.Cmd, func(), error) {
	switch shell {
	case "pwsh":
		bin, err := exec.LookPath("pwsh")
		if err != nil {
			return nil, func() {}, fmt.Errorf("pwsh not found in PATH")
		}
		return psCommand(ctx, bin, false, script)

	case "powershell":
		bin, err := exec.LookPath("powershell")
		if err != nil {
			return nil, func() {}, fmt.Errorf("powershell not found in PATH")
		}
		return psCommand(ctx, bin, true, script)

	case "cmd":
		bin, err := exec.LookPath("cmd")
		if err != nil {
			return nil, func() {}, fmt.Errorf("cmd not found in PATH")
		}
		return cmdCommand(ctx, bin, script)

	case "sh", "bash", "zsh", "fish":
		bin, err := exec.LookPath(shell)
		if err != nil {
			return nil, func() {}, fmt.Errorf("%s not found in PATH", shell)
		}
		return exec.CommandContext(ctx, bin, "-c", script), func() {}, nil

	default:
		// Treat the value as a raw executable path/name.
		bin, err := exec.LookPath(shell)
		if err != nil {
			return nil, func() {}, fmt.Errorf("shell %q not found in PATH", shell)
		}
		return exec.CommandContext(ctx, bin, "-c", script), func() {}, nil
	}
}

// psCommand runs the script via PowerShell using a temp .ps1 file.
// bypassPolicy adds -ExecutionPolicy Bypass (needed for Windows PowerShell 5.1).
func psCommand(ctx context.Context, bin string, bypassPolicy bool, script string) (*exec.Cmd, func(), error) {
	tmp, err := os.CreateTemp("", "puppet-shell-*.ps1")
	if err != nil {
		return nil, func() {}, err
	}
	name := tmp.Name()
	clean := func() { _ = os.Remove(name) }

	// Force UTF-8 on both Console and pipeline output.
	content := "[Console]::OutputEncoding = [Text.Encoding]::UTF8\n" +
		"$OutputEncoding = [Text.Encoding]::UTF8\n" +
		script
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		clean()
		return nil, func() {}, err
	}
	tmp.Close()

	args := []string{"-NoLogo", "-NoProfile", "-NonInteractive"}
	if bypassPolicy {
		args = append(args, "-ExecutionPolicy", "Bypass")
	}
	args = append(args, "-File", name)
	return exec.CommandContext(ctx, bin, args...), clean, nil
}

// cmdCommand runs the script via cmd.exe using a temp .cmd file.
// @chcp 65001 switches the session to UTF-8 before the user script runs.
func cmdCommand(ctx context.Context, bin string, script string) (*exec.Cmd, func(), error) {
	tmp, err := os.CreateTemp("", "puppet-shell-*.cmd")
	if err != nil {
		return nil, func() {}, err
	}
	name := tmp.Name()
	clean := func() { _ = os.Remove(name) }

	content := "@echo off\r\n@chcp 65001 >nul\r\n" +
		strings.ReplaceAll(script, "\n", "\r\n") + "\r\n"
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		clean()
		return nil, func() {}, err
	}
	tmp.Close()
	return exec.CommandContext(ctx, bin, "/c", name), clean, nil
}
