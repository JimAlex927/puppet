package pluginhost

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"puppet/internal/node"
)

func TestMain(m *testing.M) {
	switch os.Getenv("PUPPET_PLUGINHOST_TEST_MODE") {
	case "exec":
		runExecTestPlugin()
		return
	case "daemon":
		runDaemonTestPlugin()
		return
	}
	os.Exit(m.Run())
}

func TestExecPluginRegistersAndExecutes(t *testing.T) {
	root := t.TempDir()
	entry, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	writeTestManifest(t, root, "exec-echo", Manifest{
		ID:      "test.exec",
		Name:    "Exec Echo",
		Version: "0.1.0",
		Runtime: RuntimeExec,
		Entry:   entry,
		Env:     map[string]string{"PUPPET_PLUGINHOST_TEST_MODE": "exec"},
		Nodes: []node.NodeMetadata{{
			Type:        "plugin.execEcho",
			Name:        "Exec Echo",
			Description: "Echo through an exec plugin",
			Fields: []node.NodeField{{
				Name:     "message",
				Label:    "Message",
				Type:     "input",
				Required: true,
			}},
		}},
	})

	registry := node.NewRegistry()
	host := New(root)
	if err := host.Register(registry); err != nil {
		t.Fatal(err)
	}
	executor, ok := registry.Get("plugin.execEcho")
	if !ok {
		t.Fatal("exec plugin node was not registered")
	}

	var logs []PluginLog
	result, err := executor.Execute(&node.NodeContext{
		Context:   context.Background(),
		Workspace: "workspace-a",
		Log: func(stream string, content string) {
			logs = append(logs, PluginLog{Stream: stream, Content: content})
		},
	}, map[string]any{"message": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Output["message"]; got != "hello" {
		t.Fatalf("message = %v, want hello", got)
	}
	if got := result.Output["workspace"]; got != "workspace-a" {
		t.Fatalf("workspace = %v, want workspace-a", got)
	}
	if len(logs) != 1 || logs[0].Content != "exec log" {
		t.Fatalf("logs = %#v, want exec log", logs)
	}
}

func TestDaemonPluginRegistersAndExecutes(t *testing.T) {
	root := t.TempDir()
	entry, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	writeTestManifest(t, root, "daemon-echo", Manifest{
		ID:      "test.daemon",
		Name:    "Daemon Echo",
		Version: "0.1.0",
		Runtime: RuntimeDaemon,
		Entry:   entry,
		Env:     map[string]string{"PUPPET_PLUGINHOST_TEST_MODE": "daemon"},
		Nodes: []node.NodeMetadata{{
			Type:        "plugin.daemonEcho",
			Name:        "Daemon Echo",
			Description: "Echo through a daemon plugin",
		}},
	})

	registry := node.NewRegistry()
	host := New(root)
	defer host.Stop()
	if err := host.Register(registry); err != nil {
		t.Fatal(err)
	}
	executor, ok := registry.Get("plugin.daemonEcho")
	if !ok {
		t.Fatal("daemon plugin node was not registered")
	}

	result, err := executor.Execute(&node.NodeContext{
		Context:   context.Background(),
		Workspace: "workspace-b",
		Log:       func(stream string, content string) {},
	}, map[string]any{"message": "from daemon"})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Output["message"]; got != "from daemon" {
		t.Fatalf("message = %v, want from daemon", got)
	}
	if got := result.Output["nodeType"]; got != "plugin.daemonEcho" {
		t.Fatalf("nodeType = %v, want plugin.daemonEcho", got)
	}
}

func writeTestManifest(t *testing.T, root, dirname string, manifest Manifest) {
	t.Helper()
	dir := filepath.Join(root, dirname)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func runExecTestPlugin() {
	content, _ := io.ReadAll(os.Stdin)
	var req ExecuteRequest
	_ = json.Unmarshal(content, &req)
	_ = json.NewEncoder(os.Stdout).Encode(ExecuteResponse{
		Output: map[string]any{
			"message":   req.Params["message"],
			"workspace": req.Workspace,
		},
		Logs: []PluginLog{{Stream: "stdout", Content: "exec log"}},
	})
	os.Exit(0)
}

func runDaemonTestPlugin() {
	addr := ""
	for i := 1; i < len(os.Args)-1; i++ {
		if os.Args[i] == "--addr" {
			addr = os.Args[i+1]
			break
		}
	}
	if addr == "" {
		os.Exit(2)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/execute", func(w http.ResponseWriter, r *http.Request) {
		var req ExecuteRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(ExecuteResponse{
			Output: map[string]any{
				"message":  req.Params["message"],
				"nodeType": req.NodeType,
			},
		})
	})
	_ = http.ListenAndServe(addr, mux)
	os.Exit(1)
}
