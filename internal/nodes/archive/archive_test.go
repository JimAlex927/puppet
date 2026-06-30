package archive

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"puppet/internal/node"
)

func TestZipRoundTrip(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "src", "a.txt"), "alpha")
	mustWriteFile(t, filepath.Join(workspace, "src", "nested", "b.txt"), "bravo")
	if err := os.MkdirAll(filepath.Join(workspace, "src", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx := testContext(workspace)
	compress := NewCompress()
	_, err := compress.Execute(ctx, map[string]any{
		"sources":        "src",
		"outputPath":     "out.zip",
		"workdir":        "${workspace}",
		"format":         "zip",
		"includeBaseDir": true,
		"overwrite":      true,
	})
	if err != nil {
		t.Fatalf("compress zip: %v", err)
	}

	extract := NewExtract()
	_, err = extract.Execute(ctx, map[string]any{
		"archivePath": "out.zip",
		"destDir":     "dest",
		"workdir":     "${workspace}",
		"format":      "zip",
		"overwrite":   true,
	})
	if err != nil {
		t.Fatalf("extract zip: %v", err)
	}

	assertFile(t, filepath.Join(workspace, "dest", "src", "a.txt"), "alpha")
	assertFile(t, filepath.Join(workspace, "dest", "src", "nested", "b.txt"), "bravo")
	if info, err := os.Stat(filepath.Join(workspace, "dest", "src", "empty")); err != nil || !info.IsDir() {
		t.Fatalf("empty directory was not restored, info=%v err=%v", info, err)
	}
}

func TestTarGzipRoundTripWithoutBaseDir(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "src", "a.txt"), "alpha")
	mustWriteFile(t, filepath.Join(workspace, "src", "nested", "b.txt"), "bravo")

	ctx := testContext(workspace)
	_, err := NewCompress().Execute(ctx, map[string]any{
		"sources":        "src",
		"outputPath":     "out.tar.gz",
		"workdir":        "${workspace}",
		"format":         "tar.gz",
		"includeBaseDir": false,
		"overwrite":      true,
	})
	if err != nil {
		t.Fatalf("compress tar.gz: %v", err)
	}

	_, err = NewExtract().Execute(ctx, map[string]any{
		"archivePath": "out.tar.gz",
		"destDir":     "dest",
		"workdir":     "${workspace}",
		"format":      "tar.gz",
		"overwrite":   true,
	})
	if err != nil {
		t.Fatalf("extract tar.gz: %v", err)
	}

	assertFile(t, filepath.Join(workspace, "dest", "a.txt"), "alpha")
	assertFile(t, filepath.Join(workspace, "dest", "nested", "b.txt"), "bravo")
}

func TestCompressUsesOutputDirAndArchiveName(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "src", "a.txt"), "alpha")

	_, err := NewCompress().Execute(testContext(workspace), map[string]any{
		"sources":        "src",
		"outputDir":      "archives",
		"archiveName":    "selected.zip",
		"workdir":        "${workspace}",
		"format":         "auto",
		"includeBaseDir": true,
		"overwrite":      true,
	})
	if err != nil {
		t.Fatalf("compress with outputDir/archiveName: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "archives", "selected.zip")); err != nil {
		t.Fatalf("archive was not created at outputDir/archiveName: %v", err)
	}
}

func TestExtractZipRejectsPathTraversal(t *testing.T) {
	workspace := t.TempDir()
	archivePath := filepath.Join(workspace, "evil.zip")
	out, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(out)
	writer, err := zw.Create("../evil.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("nope")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = NewExtract().Execute(testContext(workspace), map[string]any{
		"archivePath": "evil.zip",
		"destDir":     "dest",
		"workdir":     "${workspace}",
		"format":      "zip",
		"overwrite":   true,
	})
	if err == nil {
		t.Fatal("expected path traversal archive to fail")
	}
	if _, err := os.Stat(filepath.Join(workspace, "evil.txt")); !os.IsNotExist(err) {
		t.Fatalf("path traversal wrote outside destination, stat err=%v", err)
	}
}

func TestCleanPathInputRemovesBidiCharacters(t *testing.T) {
	const hiddenLeftToRightEmbedding = "\u202a"
	got := cleanPathInput(hiddenLeftToRightEmbedding + `C:\Users\1\archive.zip`)
	want := `C:\Users\1\archive.zip`
	if got != want {
		t.Fatalf("cleanPathInput mismatch: got %q, want %q", got, want)
	}
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

func assertFile(t *testing.T, path string, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(content) != expected {
		t.Fatalf("content mismatch for %s: got %q, want %q", path, content, expected)
	}
}
