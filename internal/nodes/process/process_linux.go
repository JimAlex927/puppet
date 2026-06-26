//go:build linux

package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"puppet/internal/node"
)

func configureProcessCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func processSupported() bool { return true }

func queryProcessInfo(ctx *node.NodeContext, pid int) (processInfo, error) {
	procDir := filepath.Join("/proc", strconv.Itoa(pid))
	if _, err := os.Stat(procDir); err != nil {
		if os.IsNotExist(err) {
			return processInfo{}, os.ErrNotExist
		}
		return processInfo{}, err
	}
	name := readTrim(filepath.Join(procDir, "comm"))
	executablePath, _ := os.Readlink(filepath.Join(procDir, "exe"))
	cmdlineBytes, _ := os.ReadFile(filepath.Join(procDir, "cmdline"))
	commandLine := strings.TrimSpace(strings.ReplaceAll(string(cmdlineBytes), "\x00", " "))
	creationDate := linuxProcessStartID(filepath.Join(procDir, "stat"))
	return processInfo{
		PID:            pid,
		Name:           name,
		ExecutablePath: executablePath,
		CommandLine:    commandLine,
		CreationDate:   creationDate,
	}, nil
}

func pidsByProcessName(ctx *node.NodeContext, name string) ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	pids := []int{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		info, err := queryProcessInfo(ctx, pid)
		if err != nil {
			continue
		}
		if strings.EqualFold(info.Name, name) || strings.EqualFold(filepath.Base(info.ExecutablePath), name) {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

func pidsByPort(ctx *node.NodeContext, port int) ([]int, error) {
	inodes := map[string]bool{}
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		items, err := listeningSocketInodes(path, port)
		if err != nil {
			continue
		}
		for _, inode := range items {
			inodes[inode] = true
		}
	}
	if len(inodes) == 0 {
		return nil, nil
	}
	return pidsBySocketInode(inodes)
}

func killPID(ctx *node.NodeContext, pid int, force bool) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if force {
		ctx.Log("system", fmt.Sprintf("sending SIGKILL to pid %d\n", pid))
		return proc.Signal(syscall.SIGKILL)
	}
	ctx.Log("system", fmt.Sprintf("sending SIGTERM to pid %d\n", pid))
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := queryProcessInfo(ctx, pid); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	ctx.Log("system", fmt.Sprintf("pid %d did not exit after SIGTERM, sending SIGKILL\n", pid))
	return proc.Signal(syscall.SIGKILL)
}

func killManagedPID(ctx *node.NodeContext, pid int, force bool) error {
	signal := syscall.SIGTERM
	if force {
		signal = syscall.SIGKILL
	}
	ctx.Log("system", fmt.Sprintf("sending %s to process group %d\n", signalName(signal), pid))
	if err := syscall.Kill(-pid, signal); err != nil {
		return err
	}
	if force {
		return nil
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := queryProcessInfo(ctx, pid); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	ctx.Log("system", fmt.Sprintf("process group %d did not exit after SIGTERM, sending SIGKILL\n", pid))
	return syscall.Kill(-pid, syscall.SIGKILL)
}

func signalName(signal syscall.Signal) string {
	switch signal {
	case syscall.SIGTERM:
		return "SIGTERM"
	case syscall.SIGKILL:
		return "SIGKILL"
	default:
		return signal.String()
	}
}

func listeningSocketInodes(path string, port int) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	targetPort := fmt.Sprintf("%04X", port)
	inodes := []string{}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		local := fields[1]
		state := fields[3]
		if state != "0A" {
			continue
		}
		parts := strings.Split(local, ":")
		if len(parts) != 2 || !strings.EqualFold(parts[1], targetPort) {
			continue
		}
		inodes = append(inodes, fields[9])
	}
	return inodes, nil
}

func pidsBySocketInode(inodes map[string]bool) ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	set := map[int]bool{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		fdDir := filepath.Join("/proc", entry.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			if strings.HasPrefix(link, "socket:[") && strings.HasSuffix(link, "]") {
				inode := strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]")
				if inodes[inode] {
					set[pid] = true
				}
			}
		}
	}
	pids := make([]int, 0, len(set))
	for pid := range set {
		pids = append(pids, pid)
	}
	return pids, nil
}

func linuxProcessStartID(statPath string) string {
	content, err := os.ReadFile(statPath)
	if err != nil {
		return ""
	}
	text := string(content)
	rightParen := strings.LastIndex(text, ")")
	if rightParen < 0 || rightParen+2 >= len(text) {
		return ""
	}
	fields := strings.Fields(text[rightParen+2:])
	if len(fields) <= 19 {
		return ""
	}
	bootID := readTrim("/proc/sys/kernel/random/boot_id")
	if bootID == "" {
		return fields[19]
	}
	return bootID + ":" + fields[19]
}

func readTrim(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}
