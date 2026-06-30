package pluginhost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"puppet/internal/node"
)

const (
	RuntimeExec   = "exec"
	RuntimeDaemon = "daemon"
)

type Manifest struct {
	ID      string              `json:"id"`
	Name    string              `json:"name"`
	Version string              `json:"version"`
	Runtime string              `json:"runtime"`
	Entry   string              `json:"entry"`
	Args    []string            `json:"args"`
	URL     string              `json:"url"`
	Env     map[string]string   `json:"env"`
	Nodes   []node.NodeMetadata `json:"nodes"`
}

type ExecuteRequest struct {
	NodeType  string         `json:"nodeType"`
	Params    map[string]any `json:"params"`
	Workspace string         `json:"workspace"`
	TaskRunID uint           `json:"taskRunId"`
	NodeRunID uint           `json:"nodeRunId"`
}

type ExecuteResponse struct {
	Output map[string]any `json:"output"`
	Error  string         `json:"error,omitempty"`
	Logs   []PluginLog    `json:"logs,omitempty"`
}

type PluginLog struct {
	Stream  string `json:"stream"`
	Content string `json:"content"`
}

type Host struct {
	dir     string
	daemons []*daemonProcess
}

func New(dir string) *Host {
	return &Host{dir: dir}
}

func (h *Host) Register(registry *node.Registry) error {
	if h == nil || registry == nil || strings.TrimSpace(h.dir) == "" {
		return nil
	}
	entries, err := os.ReadDir(h.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pluginDir := filepath.Join(h.dir, entry.Name())
		manifest, err := readManifest(pluginDir)
		if err != nil {
			log.Printf("pluginhost: skip %s: %v", pluginDir, err)
			continue
		}
		if err := h.registerManifest(registry, pluginDir, manifest); err != nil {
			log.Printf("pluginhost: skip %s: %v", manifest.ID, err)
			continue
		}
	}
	return nil
}

func (h *Host) registerManifest(registry *node.Registry, pluginDir string, manifest Manifest) error {
	if strings.TrimSpace(manifest.ID) == "" {
		return fmt.Errorf("plugin id is required")
	}
	if len(manifest.Nodes) == 0 {
		return fmt.Errorf("plugin nodes is required")
	}
	manifest.Runtime = strings.ToLower(strings.TrimSpace(manifest.Runtime))
	if manifest.Runtime == "" {
		manifest.Runtime = RuntimeExec
	}
	switch manifest.Runtime {
	case RuntimeExec:
		if strings.TrimSpace(manifest.Entry) == "" {
			return fmt.Errorf("exec plugin entry is required")
		}
		for _, meta := range manifest.Nodes {
			registry.Register(&execExecutor{pluginDir: pluginDir, manifest: manifest, metadata: normalizeMetadata(meta)})
		}
	case RuntimeDaemon:
		if strings.TrimSpace(manifest.Entry) == "" && strings.TrimSpace(manifest.URL) == "" {
			return fmt.Errorf("daemon plugin entry or url is required")
		}
		client, err := h.daemonClient(pluginDir, manifest)
		if err != nil {
			return err
		}
		for _, meta := range manifest.Nodes {
			registry.Register(&daemonExecutor{client: client, metadata: normalizeMetadata(meta)})
		}
	default:
		return fmt.Errorf("unsupported plugin runtime %q", manifest.Runtime)
	}
	log.Printf("pluginhost: registered plugin %s (%s) with %d node(s)", manifest.ID, manifest.Runtime, len(manifest.Nodes))
	return nil
}

func readManifest(pluginDir string) (Manifest, error) {
	content, err := os.ReadFile(filepath.Join(pluginDir, "plugin.json"))
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func normalizeMetadata(meta node.NodeMetadata) node.NodeMetadata {
	if meta.Category == "" {
		meta.Category = "plugin"
	}
	if meta.SupportedOS == nil {
		meta.SupportedOS = []string{"linux", "darwin", "windows"}
	}
	if meta.Fields == nil {
		meta.Fields = []node.NodeField{}
	}
	return meta
}

type execExecutor struct {
	pluginDir string
	manifest  Manifest
	metadata  node.NodeMetadata
}

func (e *execExecutor) Type() string { return e.metadata.Type }
func (e *execExecutor) Metadata() node.NodeMetadata {
	return e.metadata
}
func (e *execExecutor) Validate(params map[string]any) error {
	return validateFields(e.metadata, params)
}

func (e *execExecutor) Execute(ctx *node.NodeContext, params map[string]any) (*node.NodeResult, error) {
	req := ExecuteRequest{
		NodeType:  e.metadata.Type,
		Params:    params,
		Workspace: ctx.Workspace,
		TaskRunID: ctx.TaskRunID,
		NodeRunID: ctx.NodeRunID,
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	entry := resolveEntry(e.pluginDir, e.manifest.Entry)
	args := append([]string{}, e.manifest.Args...)
	args = append(args, "execute")
	cmd := exec.CommandContext(ctx.Context, entry, args...)
	cmd.Dir = e.pluginDir
	cmd.Env = pluginEnv(os.Environ(), e.manifest.Env)
	cmd.Stdin = bytes.NewReader(payload)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if stderr.Len() > 0 && ctx.Log != nil {
		ctx.Log("stderr", stderr.String())
	}
	if err != nil {
		return nil, fmt.Errorf("plugin %s execute failed: %w", e.manifest.ID, err)
	}
	return decodeResponse(ctx, out)
}

type daemonProcess struct {
	cmd *exec.Cmd
}

type daemonClient struct {
	manifest Manifest
	baseURL  string
	client   *http.Client
}

type daemonExecutor struct {
	client   *daemonClient
	metadata node.NodeMetadata
}

func (e *daemonExecutor) Type() string { return e.metadata.Type }
func (e *daemonExecutor) Metadata() node.NodeMetadata {
	return e.metadata
}
func (e *daemonExecutor) Validate(params map[string]any) error {
	return validateFields(e.metadata, params)
}

func (e *daemonExecutor) Execute(ctx *node.NodeContext, params map[string]any) (*node.NodeResult, error) {
	req := ExecuteRequest{
		NodeType:  e.metadata.Type,
		Params:    params,
		Workspace: ctx.Workspace,
		TaskRunID: ctx.TaskRunID,
		NodeRunID: ctx.NodeRunID,
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx.Context, http.MethodPost, strings.TrimRight(e.client.baseURL, "/")+"/execute", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := e.client.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("plugin daemon %s returned HTTP %d: %s", e.client.manifest.ID, resp.StatusCode, strings.TrimSpace(string(content)))
	}
	return decodeResponse(ctx, content)
}

func (h *Host) daemonClient(pluginDir string, manifest Manifest) (*daemonClient, error) {
	if strings.TrimSpace(manifest.URL) != "" {
		return &daemonClient{manifest: manifest, baseURL: manifest.URL, client: &http.Client{Timeout: 0}}, nil
	}
	addr, err := reserveAddr()
	if err != nil {
		return nil, err
	}

	entry := resolveEntry(pluginDir, manifest.Entry)
	args := append([]string{}, manifest.Args...)
	args = append(args, "serve", "--addr", addr)
	cmd := exec.Command(entry, args...)
	cmd.Dir = pluginDir
	cmd.Env = pluginEnv(os.Environ(), manifest.Env)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	h.daemons = append(h.daemons, &daemonProcess{cmd: cmd})
	baseURL := "http://" + addr
	if err := waitHealthy(baseURL, 10*time.Second); err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}
	return &daemonClient{manifest: manifest, baseURL: baseURL, client: &http.Client{Timeout: 0}}, nil
}

func reserveAddr() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	return addr, nil
}

func waitHealthy(baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	for time.Now().Before(deadline) {
		resp, err := client.Get(strings.TrimRight(baseURL, "/") + "/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("plugin daemon did not become healthy at %s", baseURL)
}

func decodeResponse(ctx *node.NodeContext, content []byte) (*node.NodeResult, error) {
	var resp ExecuteResponse
	if err := json.Unmarshal(content, &resp); err != nil {
		return nil, fmt.Errorf("decode plugin response: %w; raw=%s", err, strings.TrimSpace(string(content)))
	}
	for _, item := range resp.Logs {
		stream := item.Stream
		if stream == "" {
			stream = "stdout"
		}
		if ctx.Log != nil {
			ctx.Log(stream, item.Content)
		}
	}
	if resp.Error != "" {
		return &node.NodeResult{Output: resp.Output}, fmt.Errorf("%s", resp.Error)
	}
	if resp.Output == nil {
		resp.Output = map[string]any{}
	}
	return &node.NodeResult{Output: resp.Output}, nil
}

func validateFields(meta node.NodeMetadata, params map[string]any) error {
	for _, field := range meta.Fields {
		if !field.Required {
			continue
		}
		value, ok := params[field.Name]
		if !ok || value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" {
			return fmt.Errorf("%s is required", field.Name)
		}
	}
	return nil
}

func resolveEntry(pluginDir, entry string) string {
	entry = strings.TrimSpace(entry)
	if filepath.IsAbs(entry) {
		return entry
	}
	if strings.ContainsAny(entry, `/\`) {
		return filepath.Join(pluginDir, entry)
	}
	if runtime.GOOS == "windows" && entry != "" && filepath.Ext(entry) == "" {
		if _, err := os.Stat(filepath.Join(pluginDir, entry+".exe")); err == nil {
			return filepath.Join(pluginDir, entry+".exe")
		}
	}
	if _, err := os.Stat(filepath.Join(pluginDir, entry)); err == nil {
		return filepath.Join(pluginDir, entry)
	}
	return entry
}

func pluginEnv(base []string, extra map[string]string) []string {
	env := append([]string{}, base...)
	for key, value := range extra {
		env = append(env, key+"="+value)
	}
	return env
}

func (h *Host) Stop() {
	for _, daemon := range h.daemons {
		if daemon.cmd != nil && daemon.cmd.Process != nil {
			_ = daemon.cmd.Process.Kill()
		}
	}
}
