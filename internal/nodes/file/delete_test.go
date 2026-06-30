package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"puppet/internal/node"
)

func TestDeleteGlobFiles(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "a.txt"), "alpha")
	mustWriteFile(t, filepath.Join(workspace, "b.txt"), "bravo")
	mustWriteFile(t, filepath.Join(workspace, "keep.log"), "keep")

	result, err := NewDelete().Execute(testContext(workspace), map[string]any{
		"targets":   "*.txt",
		"workdir":   "${workspace}",
		"recursive": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Output["files"]; got != 2 {
		t.Fatalf("files = %v, want 2", got)
	}
	assertMissing(t, filepath.Join(workspace, "a.txt"))
	assertMissing(t, filepath.Join(workspace, "b.txt"))
	assertExists(t, filepath.Join(workspace, "keep.log"))
}

func TestDeleteDirectoryRecursively(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "dir", "nested", "a.txt"), "alpha")
	mustWriteFile(t, filepath.Join(workspace, "dir", "b.txt"), "bravo")

	result, err := NewDelete().Execute(testContext(workspace), map[string]any{
		"targets":   "dir",
		"workdir":   "${workspace}",
		"recursive": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Output["files"]; got != 2 {
		t.Fatalf("files = %v, want 2", got)
	}
	if got := result.Output["dirs"]; got != 2 {
		t.Fatalf("dirs = %v, want 2", got)
	}
	assertMissing(t, filepath.Join(workspace, "dir"))
}

func TestDeleteDryRunDoesNotRemove(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "a.txt"), "alpha")

	result, err := NewDelete().Execute(testContext(workspace), map[string]any{
		"targets": "*.txt",
		"workdir": "${workspace}",
		"dryRun":  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Output["dryRun"]; got != true {
		t.Fatalf("dryRun = %v, want true", got)
	}
	assertExists(t, filepath.Join(workspace, "a.txt"))
}

func TestDeleteRejectsWorkspaceRoot(t *testing.T) {
	workspace := t.TempDir()
	_, err := NewDelete().Execute(testContext(workspace), map[string]any{
		"targets": "${workspace}",
		"workdir": "${workspace}",
	})
	if err == nil {
		t.Fatal("expected deleting workspace root to fail")
	}
}

func TestDeleteRejectsOutsideWorkspaceByDefault(t *testing.T) {
	workspace := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	mustWriteFile(t, outside, "outside")

	_, err := NewDelete().Execute(testContext(workspace), map[string]any{
		"targets": outside,
		"workdir": "${workspace}",
	})
	if err == nil {
		t.Fatal("expected deleting outside workspace to fail")
	}
	assertExists(t, outside)
}

func testContext(workspace string) *node.NodeContext {
	return &node.NodeContext{
		Context:   context.Background(),
		Workspace: workspace,
		Log:       func(string, string) {},
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be missing, stat err=%v", path, err)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}
