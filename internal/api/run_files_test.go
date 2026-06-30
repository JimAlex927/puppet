package api

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestCleanRunFilePathRejectsUnsafePaths(t *testing.T) {
	unsafe := []string{
		"../secret.txt",
		"/etc/passwd",
		`C:\Windows\system.ini`,
		"nested/../../secret.txt",
	}
	for _, item := range unsafe {
		if _, err := cleanRunFilePath(item); err == nil {
			t.Fatalf("cleanRunFilePath(%q) expected error", item)
		}
	}
}

func TestCleanRunFilePathNormalizesRelativePath(t *testing.T) {
	got, err := cleanRunFilePath(`logs\app.log`)
	if err != nil {
		t.Fatal(err)
	}
	if got != "logs/app.log" {
		t.Fatalf("path = %q, want logs/app.log", got)
	}
}

func TestZipTaskRunTargetsIncludesFilesAndSkipsDownloads(t *testing.T) {
	root := t.TempDir()
	mustWriteAPITestFile(t, filepath.Join(root, "a.txt"), "alpha")
	mustWriteAPITestFile(t, filepath.Join(root, "dir", "b.txt"), "bravo")
	mustWriteAPITestFile(t, filepath.Join(root, runDownloadsDirName, "old.zip"), "skip")

	output := filepath.Join(root, runDownloadsDirName, "bundle.zip")
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := zipTaskRunTargets(root, output, []string{
		filepath.Join(root, "a.txt"),
		filepath.Join(root, "dir"),
	}); err != nil {
		t.Fatal(err)
	}

	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	names := map[string]bool{}
	for _, file := range reader.File {
		names[file.Name] = true
	}
	for _, want := range []string{"a.txt", "dir/", "dir/b.txt"} {
		if !names[want] {
			t.Fatalf("zip missing %s; got %#v", want, names)
		}
	}
	if names[runDownloadsDirName+"/old.zip"] {
		t.Fatalf("zip should not include %s", runDownloadsDirName)
	}
}

func mustWriteAPITestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
