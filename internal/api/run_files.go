package api

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"puppet/internal/logstream"
	"puppet/internal/model"

	"github.com/gin-gonic/gin"
)

const runDownloadsDirName = ".downloads"

type taskRunFileEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"isDir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

type taskRunFileList struct {
	Path    string             `json:"path"`
	Parent  string             `json:"parent"`
	Entries []taskRunFileEntry `json:"entries"`
}

type createTaskRunFileBundleRequest struct {
	Paths []string `json:"paths"`
}

type taskRunFileBundleEvent struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
	DownloadURL string `json:"downloadUrl,omitempty"`
}

func (h *Handler) listTaskRunFiles(c *gin.Context) {
	root, target, relPath, err := h.resolveTaskRunPath(c, c.Query("path"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		respond(c, nil, err)
		return
	}
	if !info.IsDir() {
		respond(c, nil, fmt.Errorf("path is not a directory: %s", relPath))
		return
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		respond(c, nil, err)
		return
	}
	items := make([]taskRunFileEntry, 0, len(entries))
	for _, entry := range entries {
		if relPath == "" && entry.Name() == runDownloadsDirName {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			respond(c, nil, err)
			return
		}
		fullPath := filepath.Join(target, entry.Name())
		rel, err := filepath.Rel(root, fullPath)
		if err != nil {
			respond(c, nil, err)
			return
		}
		items = append(items, taskRunFileEntry{
			Name:    entry.Name(),
			Path:    filepath.ToSlash(rel),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	ok(c, taskRunFileList{Path: relPath, Parent: parentRunPath(relPath), Entries: items})
}

func (h *Handler) downloadTaskRunFile(c *gin.Context) {
	_, target, _, err := h.resolveTaskRunPath(c, c.Query("path"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		respond(c, nil, err)
		return
	}
	if info.IsDir() {
		respond(c, nil, fmt.Errorf("cannot download a directory directly"))
		return
	}
	c.FileAttachment(target, info.Name())
}

func (h *Handler) createTaskRunFileBundle(c *gin.Context) {
	var req createTaskRunFileBundleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond(c, nil, err)
		return
	}
	if len(req.Paths) == 0 {
		respond(c, nil, fmt.Errorf("paths is required"))
		return
	}
	taskRunID := paramID(c, "id")
	root, err := h.taskRunWorkspace(taskRunID)
	if err != nil {
		respond(c, nil, err)
		return
	}
	targets := make([]string, 0, len(req.Paths))
	for _, item := range req.Paths {
		_, target, _, err := h.resolveTaskRunPath(c, item)
		if err != nil {
			respond(c, nil, err)
			return
		}
		if !withinDir(root, target) || strings.Contains(filepath.ToSlash(target), "/"+runDownloadsDirName+"/") {
			respond(c, nil, fmt.Errorf("invalid bundle path: %s", item))
			return
		}
		targets = append(targets, target)
	}
	bundleID := randomHex(12)
	event := taskRunFileBundleEvent{ID: bundleID, Status: "running", Message: "正在打包文件"}
	h.hub.Publish(taskRunID, logstream.Event{Type: "file_bundle", Data: event})

	go h.buildTaskRunFileBundle(taskRunID, root, bundleID, targets)
	ok(c, event)
}

func (h *Handler) downloadTaskRunFileBundle(c *gin.Context) {
	taskRunID := paramID(c, "id")
	root, err := h.taskRunWorkspace(taskRunID)
	if err != nil {
		respond(c, nil, err)
		return
	}
	bundleID := cleanBundleID(c.Param("bundle"))
	if bundleID == "" {
		respond(c, nil, fmt.Errorf("invalid bundle id"))
		return
	}
	target := filepath.Join(root, runDownloadsDirName, bundleID+".zip")
	if !withinDir(filepath.Join(root, runDownloadsDirName), target) {
		respond(c, nil, fmt.Errorf("invalid bundle path"))
		return
	}
	if _, err := os.Stat(target); err != nil {
		respond(c, nil, err)
		return
	}
	c.FileAttachment(target, bundleID+".zip")
}

func (h *Handler) buildTaskRunFileBundle(taskRunID uint, root string, bundleID string, targets []string) {
	bundleDir := filepath.Join(root, runDownloadsDirName)
	outputPath := filepath.Join(bundleDir, bundleID+".zip")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		h.publishBundleFailed(taskRunID, bundleID, err)
		return
	}
	if err := zipTaskRunTargets(root, outputPath, targets); err != nil {
		h.publishBundleFailed(taskRunID, bundleID, err)
		return
	}
	h.hub.Publish(taskRunID, logstream.Event{Type: "file_bundle", Data: taskRunFileBundleEvent{
		ID:          bundleID,
		Status:      "ready",
		Message:     "打包完成",
		DownloadURL: fmt.Sprintf("/api/task-runs/%d/file-bundles/%s/download", taskRunID, bundleID),
	}})
}

func (h *Handler) publishBundleFailed(taskRunID uint, bundleID string, err error) {
	h.hub.Publish(taskRunID, logstream.Event{Type: "file_bundle", Data: taskRunFileBundleEvent{
		ID:      bundleID,
		Status:  "failed",
		Message: err.Error(),
	}})
}

func (h *Handler) resolveTaskRunPath(c *gin.Context, rawPath string) (string, string, string, error) {
	root, err := h.taskRunWorkspace(paramID(c, "id"))
	if err != nil {
		return "", "", "", err
	}
	relPath, err := cleanRunFilePath(rawPath)
	if err != nil {
		return "", "", "", err
	}
	target := filepath.Join(root, filepath.FromSlash(relPath))
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", "", "", err
	}
	if !withinDir(root, targetAbs) {
		return "", "", "", fmt.Errorf("path escapes task run workspace")
	}
	return root, targetAbs, relPath, nil
}

func (h *Handler) taskRunWorkspace(taskRunID uint) (string, error) {
	var run model.TaskRun
	if err := h.db.First(&run, taskRunID).Error; err != nil {
		return "", err
	}
	root, err := filepath.Abs(filepath.Join(h.cfg.WorkspaceDir, fmt.Sprintf("taskrun-%d", run.ID)))
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(root); err != nil {
		return "", err
	}
	return root, nil
}

func cleanRunFilePath(rawPath string) (string, error) {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" || rawPath == "." || rawPath == "/" {
		return "", nil
	}
	if strings.Contains(rawPath, "\x00") {
		return "", fmt.Errorf("path contains NUL byte")
	}
	rawPath = strings.ReplaceAll(rawPath, "\\", "/")
	if pathVolumeName(rawPath) != "" || strings.HasPrefix(rawPath, "/") {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(rawPath)))
	if clean == "." {
		return "", nil
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("parent paths are not allowed")
	}
	return clean, nil
}

func parentRunPath(relPath string) string {
	if relPath == "" {
		return ""
	}
	parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(relPath)))
	if parent == "." {
		return ""
	}
	return parent
}

func withinDir(parent, child string) bool {
	parentAbs, err := filepath.Abs(parent)
	if err != nil {
		return false
	}
	childAbs, err := filepath.Abs(child)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(parentAbs, childAbs)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

func zipTaskRunTargets(root string, outputPath string, targets []string) error {
	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	defer zw.Close()
	used := map[string]bool{}
	for _, target := range targets {
		if err := addZipTarget(root, zw, target, used); err != nil {
			return err
		}
	}
	return nil
}

func addZipTarget(root string, zw *zip.Writer, target string, used map[string]bool) error {
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return filepath.WalkDir(target, func(current string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Name() == runDownloadsDirName {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			return addZipEntry(root, zw, current, info, used)
		})
	}
	return addZipEntry(root, zw, target, info, used)
}

func addZipEntry(root string, zw *zip.Writer, current string, info os.FileInfo, used map[string]bool) error {
	if !withinDir(root, current) {
		return fmt.Errorf("zip entry escapes workspace: %s", current)
	}
	rel, err := filepath.Rel(root, current)
	if err != nil {
		return err
	}
	name := filepath.ToSlash(rel)
	if name == "." || name == "" || strings.HasPrefix(name, runDownloadsDirName+"/") {
		return nil
	}
	if used[name] {
		return nil
	}
	used[name] = true
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = name
	if info.IsDir() {
		header.Name = strings.TrimSuffix(header.Name, "/") + "/"
		_, err = zw.CreateHeader(header)
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	header.Method = zip.Deflate
	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	in, err := os.Open(current)
	if err != nil {
		return err
	}
	defer in.Close()
	_, err = io.Copy(writer, in)
	return err
}

func randomHex(bytesLen int) string {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func cleanBundleID(value string) string {
	value = strings.TrimSuffix(strings.TrimSpace(value), ".zip")
	for _, r := range value {
		if (r < 'a' || r > 'f') && (r < '0' || r > '9') {
			return ""
		}
	}
	return value
}

func pathVolumeName(path string) string {
	if len(path) >= 2 && path[1] == ':' {
		return path[:2]
	}
	return ""
}
