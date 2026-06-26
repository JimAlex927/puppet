//go:build windows

package process

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"

	"puppet/internal/node"

	"golang.org/x/sys/windows"
	"golang.org/x/text/encoding/simplifiedchinese"
)

const (
	createNewConsole = 0x00000010
	createNoWindow   = 0x08000000
)

// configureProcessCommand configures background (hidden) process attributes.
// showWindow=true is handled separately via startProcessWithConsole.
func configureProcessCommand(cmd *exec.Cmd, showWindow bool) {
	if !showWindow {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	}
}

func launchProcess(executable string, args []string, workdir string, showWindow bool, stdout, stderr *os.File) (int, error) {
	if showWindow {
		return startProcessWithConsole(executable, args, workdir)
	}
	cmd := exec.Command(executable, args...)
	cmd.Dir = workdir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	configureProcessCommand(cmd, false)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	return pid, nil
}

// startProcessWithConsole launches the process in a new console window using
// CreateProcess directly, without setting STARTF_USESTDHANDLES. This lets the
// process own its new console and write output directly to the window.
func startProcessWithConsole(executable string, args []string, workdir string) (int, error) {
	if resolved, err := exec.LookPath(executable); err == nil {
		executable = resolved
	}
	cmdLine := buildCommandLine(executable, args)
	cmdLinePtr, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		return 0, err
	}
	var workdirPtr *uint16
	if workdir != "" {
		workdirPtr, err = windows.UTF16PtrFromString(workdir)
		if err != nil {
			return 0, err
		}
	}
	si := &windows.StartupInfo{
		Cb: uint32(unsafe.Sizeof(windows.StartupInfo{})),
		// STARTF_USESTDHANDLES is intentionally NOT set so the new console
		// owns stdin/stdout/stderr and the process output appears in the window.
	}
	pi := &windows.ProcessInformation{}
	err = windows.CreateProcess(
		nil,
		cmdLinePtr,
		nil,
		nil,
		false,
		createNewConsole,
		nil,
		workdirPtr,
		si,
		pi,
	)
	if err != nil {
		return 0, fmt.Errorf("CreateProcess: %w", err)
	}
	_ = windows.CloseHandle(pi.Thread)
	_ = windows.CloseHandle(pi.Process)
	return int(pi.ProcessId), nil
}

// buildCommandLine produces a quoted command-line string for CreateProcess.
func buildCommandLine(executable string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, quoteCmdArg(executable))
	for _, arg := range args {
		parts = append(parts, quoteCmdArg(arg))
	}
	return strings.Join(parts, " ")
}

func quoteCmdArg(s string) string {
	if !strings.ContainsAny(s, " \t\"") {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	slashes := 0
	for _, c := range s {
		switch c {
		case '\\':
			slashes++
		case '"':
			// Before a literal quote: double each preceding backslash, then escape the quote.
			for i := 0; i < slashes*2; i++ {
				b.WriteByte('\\')
			}
			slashes = 0
			b.WriteString(`\"`)
		default:
			// Backslashes not followed by a quote are literal — write them as-is.
			for ; slashes > 0; slashes-- {
				b.WriteByte('\\')
			}
			b.WriteRune(c)
		}
	}
	// Before the closing quote: double each trailing backslash.
	for i := 0; i < slashes*2; i++ {
		b.WriteByte('\\')
	}
	b.WriteByte('"')
	return b.String()
}

func processSupported() bool { return true }

func queryProcessInfo(_ *node.NodeContext, pid int) (processInfo, error) {
	handle, err := windows.OpenProcess(
		windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE,
		false,
		uint32(pid),
	)
	if err != nil {
		if errno, ok := err.(syscall.Errno); ok && errno == windows.ERROR_ACCESS_DENIED {
			// Process exists but is not queryable (e.g. runs as SYSTEM); return
			// minimal info so identity verification passes vacuously and kill proceeds.
			return processInfo{PID: pid}, nil
		}
		return processInfo{}, os.ErrNotExist
	}
	defer windows.CloseHandle(handle)

	// WaitForSingleObject(0) tells us whether the process has already exited.
	// A terminated process object remains openable until all handles are closed,
	// so we must actively check rather than relying on OpenProcess failing.
	if s, _ := windows.WaitForSingleObject(handle, 0); s == windows.WAIT_OBJECT_0 {
		return processInfo{}, os.ErrNotExist
	}

	var buf [windows.MAX_PATH]uint16
	size := uint32(len(buf))
	executablePath := ""
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err == nil {
		executablePath = windows.UTF16ToString(buf[:size])
	}

	var creation, exit, kernel, user syscall.Filetime
	creationDate := ""
	if err := syscall.GetProcessTimes(syscall.Handle(handle), &creation, &exit, &kernel, &user); err == nil {
		creationDate = time.Unix(0, creation.Nanoseconds()).UTC().Format(time.RFC3339Nano)
	}

	return processInfo{
		PID:            pid,
		Name:           filepath.Base(executablePath),
		ExecutablePath: executablePath,
		CreationDate:   creationDate,
	}, nil
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
