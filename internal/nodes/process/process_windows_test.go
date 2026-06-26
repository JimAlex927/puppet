package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"puppet/internal/node"
)

func TestProcessStartAndStopWithMetadata(t *testing.T) {
	workspace := t.TempDir()
	startExecutor := NewStart()
	stopExecutor := NewStop()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	startResult, err := startExecutor.Execute(&node.NodeContext{
		Context:   ctx,
		Workspace: workspace,
		Log:       func(stream string, content string) { t.Logf("%s: %s", stream, content) },
	}, map[string]any{
		"name":             "process-node-test",
		"executable":       "ping",
		"args":             "-t 127.0.0.1",
		"workdir":          "${workspace}",
		"ifAlreadyRunning": "allow",
	})
	if err != nil {
		t.Fatalf("start process: %v", err)
	}
	metadataPath := fmt.Sprint(startResult.Output["metadataPath"])
	pid := intFrom(startResult.Output["pid"])
	defer func() {
		if pid > 0 {
			_ = killPID(&node.NodeContext{Context: context.Background(), Log: func(string, string) {}}, pid, true)
		}
	}()

	if _, err := stopExecutor.Execute(&node.NodeContext{
		Context:   ctx,
		Workspace: workspace,
		Log:       func(stream string, content string) { t.Logf("%s: %s", stream, content) },
	}, map[string]any{
		"metadataPath": metadataPath,
		"force":        true,
	}); err != nil {
		t.Fatalf("stop process: %v", err)
	}

	if _, err := queryProcessInfo(&node.NodeContext{Context: ctx}, pid); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected process to be stopped, query err=%v", err)
	}
}

func TestProcessWorkdirDoesNotChangeDefaultMetadataPath(t *testing.T) {
	workspace := t.TempDir()
	workdir := t.TempDir()
	startExecutor := NewStart()
	stopExecutor := NewStop()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	startResult, err := startExecutor.Execute(&node.NodeContext{
		Context:   ctx,
		Workspace: workspace,
		Log:       func(stream string, content string) { t.Logf("%s: %s", stream, content) },
	}, map[string]any{
		"name":             "process-node-workdir-test",
		"executable":       "ping",
		"args":             "-t 127.0.0.1",
		"workdir":          workdir,
		"ifAlreadyRunning": "allow",
	})
	if err != nil {
		t.Fatalf("start process: %v", err)
	}
	pid := intFrom(startResult.Output["pid"])
	defer func() {
		if pid > 0 {
			_ = killPID(&node.NodeContext{Context: context.Background(), Log: func(string, string) {}}, pid, true)
		}
	}()

	if _, err := os.Stat(filepath.Join(workspace, "processes", "process-node-workdir-test.json")); err != nil {
		t.Fatalf("expected metadata under workspace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workdir, "processes", "process-node-workdir-test.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("did not expect metadata under workdir, stat err=%v", err)
	}

	if _, err := stopExecutor.Execute(&node.NodeContext{
		Context:   ctx,
		Workspace: workspace,
		Log:       func(stream string, content string) { t.Logf("%s: %s", stream, content) },
	}, map[string]any{
		"name":    "process-node-workdir-test",
		"workdir": workdir,
		"force":   true,
	}); err != nil {
		t.Fatalf("stop process with workdir default metadata: %v", err)
	}
}

func TestProcessMetadataPathCanBeDirectory(t *testing.T) {
	workspace := t.TempDir()
	metadataDir := filepath.Join(t.TempDir(), "metadata")
	startExecutor := NewStart()
	stopExecutor := NewStop()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	startResult, err := startExecutor.Execute(&node.NodeContext{
		Context:   ctx,
		Workspace: workspace,
		Log:       func(stream string, content string) { t.Logf("%s: %s", stream, content) },
	}, map[string]any{
		"name":             "process-node-dir-test",
		"executable":       "ping",
		"args":             "-t 127.0.0.1",
		"metadataPath":     metadataDir,
		"ifAlreadyRunning": "allow",
	})
	if err != nil {
		t.Fatalf("start process: %v", err)
	}
	pid := intFrom(startResult.Output["pid"])
	defer func() {
		if pid > 0 {
			_ = killPID(&node.NodeContext{Context: context.Background(), Log: func(string, string) {}}, pid, true)
		}
	}()

	expected := filepath.Join(metadataDir, "process-node-dir-test.json")
	if fmt.Sprint(startResult.Output["metadataPath"]) != expected {
		t.Fatalf("metadata path mismatch: got %s want %s", startResult.Output["metadataPath"], expected)
	}
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected metadata file under directory: %v", err)
	}
	if _, err := stopExecutor.Execute(&node.NodeContext{
		Context:   ctx,
		Workspace: workspace,
		Log:       func(stream string, content string) { t.Logf("%s: %s", stream, content) },
	}, map[string]any{
		"metadataPath": metadataDir,
		"name":         "process-node-dir-test",
		"force":        true,
	}); err != nil {
		t.Fatalf("stop process with metadata directory: %v", err)
	}
}

func TestDecodeWindowsCommandOutput(t *testing.T) {
	gbkSuccess := []byte{0xb3, 0xc9, 0xb9, 0xa6}
	if got := decodeWindowsCommandOutput(gbkSuccess); got != "成功" {
		t.Fatalf("decode GBK output mismatch: got %q", got)
	}
	if got := decodeWindowsCommandOutput([]byte("utf-8 ok")); got != "utf-8 ok" {
		t.Fatalf("decode UTF-8 output mismatch: got %q", got)
	}
}
