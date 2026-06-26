package process

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"puppet/internal/node"
)

type Executor struct{}
type StartExecutor struct{}
type StopExecutor struct{}

type processMetadata struct {
	Name             string    `json:"name"`
	PID              int       `json:"pid"`
	ProcessName      string    `json:"processName"`
	Executable       string    `json:"executable"`
	ExecutablePath   string    `json:"executablePath"`
	Args             []string  `json:"args"`
	CommandLine      string    `json:"commandLine"`
	Workdir          string    `json:"workdir"`
	Port             int       `json:"port"`
	StdoutLogPath    string    `json:"stdoutLogPath,omitempty"`
	StderrLogPath    string    `json:"stderrLogPath,omitempty"`
	ShowWindow       bool      `json:"showWindow"`
	ProcessStartedAt string    `json:"processStartedAt"`
	StartedAt        time.Time `json:"startedAt"`
	StoppedAt        time.Time `json:"stoppedAt,omitempty"`
	Status           string    `json:"status"`
}

type processInfo struct {
	PID            int    `json:"pid"`
	Name           string `json:"name"`
	ExecutablePath string `json:"executablePath"`
	CommandLine    string `json:"commandLine"`
	CreationDate   string `json:"creationDate"`
}

func New() *Executor {
	return &Executor{}
}

func NewStart() *StartExecutor {
	return &StartExecutor{}
}

func NewStop() *StopExecutor {
	return &StopExecutor{}
}

func (e *Executor) Type() string {
	return "process"
}

func (e *StartExecutor) Type() string {
	return "process_start"
}

func (e *StopExecutor) Type() string {
	return "process_stop"
}

func (e *Executor) Metadata() node.NodeMetadata {
	return node.NodeMetadata{
		Type:        e.Type(),
		Name:        "Process",
		Category:    "runtime",
		Description: "启动或停止本机进程，并用 metadata 校验 PID 身份",
		SupportedOS: []string{"windows", "linux"},
		Fields: []node.NodeField{
			{Name: "action", Label: "Action", Type: "select", Required: true, Default: "start", Options: []string{"start", "stop"}},
			{Name: "name", Label: "Name", Type: "input", Required: false, Default: "app"},
			{Name: "executable", Label: "Executable", Type: "input", Required: false},
			{Name: "args", Label: "Arguments", Type: "input", Required: false},
			{Name: "workdir", Label: "Workdir", Type: "input", Required: false, Default: "${workspace}"},
			{Name: "metadataPath", Label: "Metadata Path", Type: "input", Required: false},
			{Name: "port", Label: "Port", Type: "number", Required: false, Default: 0},
			{Name: "showWindow", Label: "Show Console Window", Type: "switch", Required: false, Default: false},
			{Name: "ifAlreadyRunning", Label: "If Already Running", Type: "select", Required: false, Default: "fail", Options: []string{"fail", "stop", "allow"}},
			{Name: "force", Label: "Force Stop", Type: "switch", Required: false, Default: true},
		},
	}
}

func (e *StartExecutor) Metadata() node.NodeMetadata {
	return node.NodeMetadata{
		Type:        e.Type(),
		Name:        "Process Start",
		Category:    "runtime",
		Description: "启动本机进程，记录 PID、日志和 metadata",
		SupportedOS: []string{"windows", "linux"},
		Fields: []node.NodeField{
			{Name: "name", Label: "Name", Type: "input", Required: false, Default: "app"},
			{Name: "executable", Label: "Executable", Type: "input", Required: true},
			{Name: "args", Label: "Arguments", Type: "input", Required: false},
			{Name: "workdir", Label: "Workdir", Type: "input", Required: false, Default: "${workspace}"},
			{Name: "metadataPath", Label: "Metadata Path", Type: "input", Required: false},
			{Name: "port", Label: "Port", Type: "number", Required: false, Default: 0},
			{Name: "showWindow", Label: "Show Console Window", Type: "switch", Required: false, Default: false},
			{Name: "ifAlreadyRunning", Label: "If Already Running", Type: "select", Required: false, Default: "fail", Options: []string{"fail", "stop", "allow"}},
			{Name: "force", Label: "Force Stop Existing", Type: "switch", Required: false, Default: true},
		},
	}
}

func (e *StopExecutor) Metadata() node.NodeMetadata {
	return node.NodeMetadata{
		Type:        e.Type(),
		Name:        "Process Stop",
		Category:    "runtime",
		Description: "按 metadata 或 port 停止进程",
		SupportedOS: []string{"windows", "linux"},
		Fields: []node.NodeField{
			{Name: "stopBy", Label: "Stop By", Type: "select", Required: true, Default: "metadata", Options: []string{"metadata", "port"}},
			{Name: "name", Label: "Name", Type: "input", Required: false, Default: "app", ShowWhen: &node.NodeFieldCondition{Field: "stopBy", Equals: "metadata"}},
			{Name: "metadataPath", Label: "Metadata Path", Type: "input", Required: false, ShowWhen: &node.NodeFieldCondition{Field: "stopBy", Equals: "metadata"}},
			{Name: "port", Label: "Port", Type: "number", Required: true, Default: 0, ShowWhen: &node.NodeFieldCondition{Field: "stopBy", Equals: "port"}},
			{Name: "force", Label: "Force Stop", Type: "switch", Required: false, Default: true},
		},
	}
}

func (e *Executor) Validate(params map[string]any) error {
	action := strings.TrimSpace(stringFrom(params["action"]))
	if action == "" {
		action = "start"
	}
	switch action {
	case "start":
		if strings.TrimSpace(stringFrom(params["executable"])) == "" {
			return fmt.Errorf("executable is required when action=start")
		}
	case "stop":
	default:
		return fmt.Errorf("unsupported process action %q", action)
	}
	return nil
}

func (e *StartExecutor) Validate(params map[string]any) error {
	if strings.TrimSpace(stringFrom(params["executable"])) == "" {
		return fmt.Errorf("executable is required")
	}
	return nil
}

func (e *StopExecutor) Validate(params map[string]any) error {
	return nil
}

func (e *Executor) Execute(ctx *node.NodeContext, params map[string]any) (*node.NodeResult, error) {
	if !processSupported() {
		return nil, fmt.Errorf("process node currently supports windows and linux")
	}
	if err := e.Validate(params); err != nil {
		return nil, err
	}
	action := strings.TrimSpace(stringFrom(params["action"]))
	if action == "" {
		action = "start"
	}
	if action == "stop" {
		return e.stop(ctx, params)
	}
	return e.start(ctx, params)
}

func (e *StartExecutor) Execute(ctx *node.NodeContext, params map[string]any) (*node.NodeResult, error) {
	if !processSupported() {
		return nil, fmt.Errorf("process node currently supports windows and linux")
	}
	if err := e.Validate(params); err != nil {
		return nil, err
	}
	return start(ctx, params)
}

func (e *StopExecutor) Execute(ctx *node.NodeContext, params map[string]any) (*node.NodeResult, error) {
	if !processSupported() {
		return nil, fmt.Errorf("process node currently supports windows and linux")
	}
	if err := e.Validate(params); err != nil {
		return nil, err
	}
	return stop(ctx, params)
}

func (e *Executor) start(ctx *node.NodeContext, params map[string]any) (*node.NodeResult, error) {
	return start(ctx, params)
}

func (e *Executor) stop(ctx *node.NodeContext, params map[string]any) (*node.NodeResult, error) {
	return stop(ctx, params)
}

func start(ctx *node.NodeContext, params map[string]any) (*node.NodeResult, error) {
	name := valueOrDefault(stringFrom(params["name"]), "app")
	executable := strings.TrimSpace(stringFrom(params["executable"]))
	args, err := splitArgs(stringFrom(params["args"]))
	if err != nil {
		return nil, err
	}
	workdir := resolvePath(ctx.Workspace, stringFrom(params["workdir"]))
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return nil, err
	}
	processName := filepath.Base(executable)
	port := intFrom(params["port"])
	showWindow := boolFrom(params["showWindow"], false)
	policy := valueOrDefault(stringFrom(params["ifAlreadyRunning"]), "fail")

	existing, err := existingPIDs(ctx, processName, port)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		ctx.Log("system", fmt.Sprintf("existing process check matched pids=%v policy=%s\n", existing, policy))
		switch policy {
		case "fail":
			return nil, fmt.Errorf("process already running: %v", existing)
		case "stop":
			for _, pid := range existing {
				if err := killPID(ctx, pid, true); err != nil {
					return nil, err
				}
			}
		case "allow":
		default:
			return nil, fmt.Errorf("unsupported ifAlreadyRunning policy %q", policy)
		}
	}

	metadataPath := resolveMetadataPath(ctx.Workspace, stringFrom(params["metadataPath"]), name)
	processDir := filepath.Dir(metadataPath)
	if err := os.MkdirAll(processDir, 0o755); err != nil {
		return nil, err
	}

	var stdoutPath, stderrPath string
	var stdoutFile, stderrFile *os.File
	if !showWindow {
		stamp := time.Now().Format("20060102150405")
		stdoutPath = filepath.Join(processDir, name+"-"+stamp+".out.log")
		stderrPath = filepath.Join(processDir, name+"-"+stamp+".err.log")
		stdoutFile, err = os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, err
		}
		defer stdoutFile.Close()
		stderrFile, err = os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, err
		}
		defer stderrFile.Close()
	}

	ctx.Log("system", fmt.Sprintf("process start plan: executable=%s args=%q workdir=%s showWindow=%t\n", executable, strings.Join(args, " "), workdir, showWindow))
	pid, err := launchProcess(executable, args, workdir, showWindow, stdoutFile, stderrFile)
	if err != nil {
		return nil, err
	}

	info, err := queryProcessInfo(ctx, pid)
	if err != nil {
		ctx.Log("system", fmt.Sprintf("process identity query warning: %v\n", err))
	}
	meta := processMetadata{
		Name:             name,
		PID:              pid,
		ProcessName:      firstNonEmpty(info.Name, processName),
		Executable:       executable,
		ExecutablePath:   info.ExecutablePath,
		Args:             args,
		CommandLine:      info.CommandLine,
		Workdir:          workdir,
		Port:             port,
		StdoutLogPath:    stdoutPath,
		StderrLogPath:    stderrPath,
		ShowWindow:       showWindow,
		ProcessStartedAt: info.CreationDate,
		StartedAt:        time.Now(),
		Status:           "running",
	}
	if err := writeMetadata(metadataPath, meta); err != nil {
		return nil, err
	}
	ctx.Log("system", fmt.Sprintf("process started: pid=%d metadata=%s showWindow=%t stdout=%s stderr=%s\n", pid, metadataPath, showWindow, stdoutPath, stderrPath))
	result := map[string]any{
		"pid":          pid,
		"name":         name,
		"processName":  meta.ProcessName,
		"metadataPath": metadataPath,
	}
	if stdoutPath != "" {
		result["stdoutLog"] = stdoutPath
		result["stderrLog"] = stderrPath
	}
	return &node.NodeResult{Output: result}, nil
}

func stop(ctx *node.NodeContext, params map[string]any) (*node.NodeResult, error) {
	stopBy := strings.TrimSpace(stringFrom(params["stopBy"]))
	name := valueOrDefault(stringFrom(params["name"]), "app")
	metadataPath := strings.TrimSpace(stringFrom(params["metadataPath"]))
	force := boolFrom(params["force"], true)

	// Old process_stop JSON had no stopBy and could rely on port fallback.
	if stopBy == "" {
		if metadataPath == "" && intFrom(params["port"]) > 0 {
			stopBy = "port"
		} else {
			stopBy = "metadata"
		}
	}

	switch stopBy {
	case "metadata":
		path := resolveMetadataPath(ctx.Workspace, metadataPath, name)
		if metadataPath == "" {
			ctx.Log("system", fmt.Sprintf("metadataPath is empty, using default metadata %s\n", path))
		}
		return stopFromMetadata(ctx, path, force)
	case "port":
		port := intFrom(params["port"])
		if port <= 0 {
			return nil, fmt.Errorf("port is required when stopBy=port")
		}
		processName := strings.TrimSpace(stringFrom(params["processName"])) // Backward compatibility for old pipeline JSON.
		return stopFromPort(ctx, processName, port, force)
	default:
		return nil, fmt.Errorf("unsupported stopBy %q", stopBy)
	}
}

func stopFromPort(ctx *node.NodeContext, processName string, port int, force bool) (*node.NodeResult, error) {
	pids, err := existingPIDs(ctx, processName, port)
	if err != nil {
		return nil, err
	}
	if len(pids) == 0 {
		ctx.Log("system", fmt.Sprintf("no process listening on port %d\n", port))
		return &node.NodeResult{Output: map[string]any{"stopped": 0}}, nil
	}
	for _, pid := range pids {
		if err := killPID(ctx, pid, force); err != nil {
			return nil, err
		}
	}
	ctx.Log("system", fmt.Sprintf("stopped pids=%v by port=%d\n", pids, port))
	return &node.NodeResult{Output: map[string]any{"stopped": len(pids), "pids": pids, "port": port}}, nil
}

func stopFromMetadata(ctx *node.NodeContext, metadataPath string, force bool) (*node.NodeResult, error) {
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("metadata not found at %s", metadataPath)
		}
		return nil, err
	}
	var meta processMetadata
	if err := json.Unmarshal(content, &meta); err != nil {
		return nil, err
	}
	info, err := queryProcessInfo(ctx, meta.PID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			meta.Status = "not_running"
			_ = writeMetadata(metadataPath, meta)
			ctx.Log("system", fmt.Sprintf("process pid=%d is not running\n", meta.PID))
			return &node.NodeResult{Output: map[string]any{"stopped": 0, "pid": meta.PID}}, nil
		}
		return nil, err
	}
	if err := verifyIdentity(meta, info); err != nil {
		return nil, err
	}
	if err := killManagedPID(ctx, meta.PID, force); err != nil {
		return nil, err
	}
	meta.Status = "stopped"
	meta.StoppedAt = time.Now()
	_ = writeMetadata(metadataPath, meta)
	ctx.Log("system", fmt.Sprintf("stopped process pid=%d metadata=%s\n", meta.PID, metadataPath))
	return &node.NodeResult{Output: map[string]any{"stopped": 1, "pid": meta.PID, "metadataPath": metadataPath}}, nil
}

func verifyIdentity(meta processMetadata, info processInfo) error {
	if meta.ProcessName != "" && !strings.EqualFold(meta.ProcessName, info.Name) {
		return fmt.Errorf("refuse to stop pid %d: process name changed from %q to %q", meta.PID, meta.ProcessName, info.Name)
	}
	if meta.ExecutablePath != "" && info.ExecutablePath != "" && !samePath(meta.ExecutablePath, info.ExecutablePath) {
		return fmt.Errorf("refuse to stop pid %d: executable path changed", meta.PID)
	}
	if meta.ProcessStartedAt != "" && info.CreationDate != "" && meta.ProcessStartedAt != info.CreationDate {
		return fmt.Errorf("refuse to stop pid %d: pid appears to have been reused", meta.PID)
	}
	return nil
}

func existingPIDs(ctx *node.NodeContext, processName string, port int) ([]int, error) {
	set := map[int]bool{}
	if port > 0 {
		pids, err := pidsByPort(ctx, port)
		if err != nil {
			return nil, err
		}
		for _, pid := range pids {
			set[pid] = true
		}
	}
	if processName != "" {
		pids, err := pidsByProcessName(ctx, processName)
		if err != nil {
			return nil, err
		}
		if port > 0 {
			filtered := map[int]bool{}
			for _, pid := range pids {
				if set[pid] {
					filtered[pid] = true
				}
			}
			set = filtered
		} else {
			for _, pid := range pids {
				set[pid] = true
			}
		}
	}
	result := make([]int, 0, len(set))
	for pid := range set {
		result = append(result, pid)
	}
	return result, nil
}

func writeMetadata(path string, meta processMetadata) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func resolveMetadataPath(baseDir string, value string, name string) string {
	if strings.TrimSpace(value) == "" {
		return filepath.Join(baseDir, "processes", name+".json")
	}
	path := resolvePath(baseDir, value)
	if isMetadataDirectory(path) {
		return filepath.Join(path, name+".json")
	}
	return path
}

func isMetadataDirectory(path string) bool {
	if info, err := os.Stat(path); err == nil {
		return info.IsDir()
	}
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ""
}

func resolvePath(workspace string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "${workspace}" {
		return workspace
	}
	value = strings.ReplaceAll(value, "${workspace}", workspace)
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(workspace, value)
}

func splitArgs(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	args := []string{}
	var current strings.Builder
	inQuote := false
	escaped := false
	for _, r := range value {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			inQuote = !inQuote
			continue
		}
		if r == ' ' || r == '\t' {
			if inQuote {
				current.WriteRune(r)
				continue
			}
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if inQuote {
		return nil, fmt.Errorf("unterminated quote in arguments")
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args, nil
}

func samePath(a string, b string) bool {
	cleanA, errA := filepath.Abs(a)
	cleanB, errB := filepath.Abs(b)
	if errA == nil {
		a = cleanA
	}
	if errB == nil {
		b = cleanB
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func valueOrDefault(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func stringFrom(value any) string {
	if value == nil {
		return ""
	}
	if typed, ok := value.(string); ok {
		return typed
	}
	return fmt.Sprint(value)
}

func intFrom(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		number, _ := strconv.Atoi(strings.TrimSpace(typed))
		return number
	default:
		return 0
	}
}

func boolFrom(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		if typed == "" {
			return fallback
		}
		parsed, err := strconv.ParseBool(typed)
		if err == nil {
			return parsed
		}
	}
	return fallback
}
