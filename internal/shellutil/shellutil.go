// Package shellutil builds exec.Cmd values for running shell scripts portably.
//
// Shell selection (the shell parameter):
//
//	""  / "auto"  – auto-detect: pwsh → powershell → cmd on Windows; sh on Unix
//	"pwsh"        – PowerShell 7+  (Windows/Linux/macOS)
//	"powershell"  – Windows PowerShell 5.1
//	"cmd"         – Windows Command Prompt
//	"bat"         – Windows batch file via cmd.exe
//	"sh"          – POSIX sh
//	"bash"        – Bash
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

	"golang.org/x/text/encoding/simplifiedchinese"
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

	case "cmd", "bat":
		bin, err := exec.LookPath("cmd")
		if err != nil {
			return nil, func() {}, fmt.Errorf("cmd not found in PATH")
		}
		extension := ".cmd"
		if shell == "bat" {
			extension = ".bat"
		}
		return cmdCommand(ctx, bin, script, extension)

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

	// UTF-8 BOM tells PowerShell 5.1 to read the file as UTF-8 instead of
	// system ANSI (GBK on Chinese Windows), preventing mojibake in non-ASCII paths.
	const bom = "\xEF\xBB\xBF"
	content := bom +
		"[Console]::OutputEncoding = [Text.Encoding]::UTF8\n" +
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

// cmdCommand runs the script via cmd.exe using a temp .cmd/.bat file.
// @chcp 65001 switches the session to UTF-8 before the user script runs.
func cmdCommand(ctx context.Context, bin string, script string, extension string) (*exec.Cmd, func(), error) {
	tmp, err := os.CreateTemp("", "puppet-shell-*"+extension)
	if err != nil {
		return nil, func() {}, err
	}
	name := tmp.Name()
	clean := func() { _ = os.Remove(name) }

	// cmd.exe reads .cmd/.bat files using the system ANSI code page (GBK on
	// Chinese Windows) regardless of chcp, so encode the file as GBK.
	utf8Content := "@echo off\r\n@chcp 65001 >nul\r\n" +
		strings.ReplaceAll(script, "\n", "\r\n") + "\r\n"
	gbkBytes, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(utf8Content))
	if err != nil {
		// Fall back to UTF-8 if encoding fails (non-Chinese content).
		gbkBytes = []byte(utf8Content)
	}
	if _, err := tmp.Write(gbkBytes); err != nil {
		tmp.Close()
		clean()
		return nil, func() {}, err
	}
	tmp.Close()
	return exec.CommandContext(ctx, bin, "/c", name), clean, nil
}
