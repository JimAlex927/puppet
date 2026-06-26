//go:build windows

package process

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unicode/utf8"

	"puppet/internal/node"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func configureProcessCommand(cmd *exec.Cmd) {}

func processSupported() bool { return true }

func queryProcessInfo(ctx *node.NodeContext, pid int) (processInfo, error) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		return processInfo{}, fmt.Errorf("pwsh is required to verify process identity")
	}
	script := fmt.Sprintf(`$p = Get-CimInstance Win32_Process -Filter "ProcessId = %d"; if ($null -eq $p) { exit 3 }; $created = ""; if ($null -ne $p.CreationDate) { if ($p.CreationDate -is [datetime]) { $created = $p.CreationDate.ToUniversalTime().ToString("o") } else { $created = [Management.ManagementDateTimeConverter]::ToDateTime($p.CreationDate).ToUniversalTime().ToString("o") } }; [pscustomobject]@{ pid = [int]$p.ProcessId; name = [string]$p.Name; executablePath = [string]$p.ExecutablePath; commandLine = [string]$p.CommandLine; creationDate = $created } | ConvertTo-Json -Compress`, pid)
	cmd := exec.CommandContext(ctx.Context, pwsh, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 3 {
			return processInfo{}, os.ErrNotExist
		}
		return processInfo{}, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	var info processInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return processInfo{}, err
	}
	return info, nil
}

func pidsByProcessName(ctx *node.NodeContext, name string) ([]int, error) {
	cmd := exec.CommandContext(ctx.Context, "tasklist", "/FI", "IMAGENAME eq "+name, "/FO", "CSV", "/NH")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(strings.NewReader(string(out)))
	records, _ := reader.ReadAll()
	pids := []int{}
	for _, record := range records {
		if len(record) < 2 || !strings.EqualFold(record[0], name) {
			continue
		}
		pid, _ := strconv.Atoi(strings.TrimSpace(record[1]))
		if pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

func pidsByPort(ctx *node.NodeContext, port int) ([]int, error) {
	cmd := exec.CommandContext(ctx.Context, "netstat", "-ano", "-p", "tcp")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	suffix := ":" + strconv.Itoa(port)
	pids := []int{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || !strings.EqualFold(fields[3], "LISTENING") {
			continue
		}
		if !strings.HasSuffix(fields[1], suffix) {
			continue
		}
		pid, _ := strconv.Atoi(fields[len(fields)-1])
		if pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

func killPID(ctx *node.NodeContext, pid int, force bool) error {
	args := []string{"/PID", strconv.Itoa(pid), "/T"}
	if force {
		args = append(args, "/F")
	}
	cmd := exec.CommandContext(ctx.Context, "taskkill", args...)
	content, err := cmd.CombinedOutput()
	if len(content) > 0 {
		ctx.Log("system", decodeWindowsCommandOutput(content))
	}
	return err
}

func killManagedPID(ctx *node.NodeContext, pid int, force bool) error {
	return killPID(ctx, pid, force)
}

func decodeWindowsCommandOutput(content []byte) string {
	if utf8.Valid(content) {
		return string(content)
	}
	decoded, err := simplifiedchinese.GBK.NewDecoder().String(string(content))
	if err != nil {
		return string(content)
	}
	return decoded
}
