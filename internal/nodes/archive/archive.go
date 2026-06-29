package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"puppet/internal/node"
)

type CompressExecutor struct{}
type ExtractExecutor struct{}

func NewCompress() *CompressExecutor { return &CompressExecutor{} }
func NewExtract() *ExtractExecutor   { return &ExtractExecutor{} }

func (e *CompressExecutor) Type() string { return "archive_compress" }
func (e *ExtractExecutor) Type() string  { return "archive_extract" }

func (e *CompressExecutor) Metadata() node.NodeMetadata {
	return node.NodeMetadata{
		Type:        e.Type(),
		Name:        "Compress",
		Category:    "file",
		Description: "压缩文件或目录，内置跨平台实现",
		SupportedOS: []string{"linux", "darwin", "windows"},
		Fields: []node.NodeField{
			{Name: "sources", Label: "Source Paths", Type: "textarea", Required: true},
			{Name: "outputPath", Label: "Output Archive", Type: "input", Required: true, Default: "${workspace}/archive.zip"},
			{Name: "workdir", Label: "Workdir", Type: "input", Required: false, Default: "${workspace}"},
			{Name: "format", Label: "Format", Type: "select", Required: true, Default: "auto", Options: []string{"auto", "zip", "tar", "tar.gz", "tgz", "gzip"}},
			{Name: "includeBaseDir", Label: "Include Base Dir", Type: "switch", Required: false, Default: true},
			{Name: "overwrite", Label: "Overwrite", Type: "switch", Required: false, Default: true},
		},
	}
}

func (e *ExtractExecutor) Metadata() node.NodeMetadata {
	return node.NodeMetadata{
		Type:        e.Type(),
		Name:        "Extract",
		Category:    "file",
		Description: "解压压缩包，内置跨平台实现并防止路径逃逸",
		SupportedOS: []string{"linux", "darwin", "windows"},
		Fields: []node.NodeField{
			{Name: "archivePath", Label: "Archive Path", Type: "input", Required: true},
			{Name: "destDir", Label: "Destination Dir", Type: "input", Required: true, Default: "${workspace}"},
			{Name: "workdir", Label: "Workdir", Type: "input", Required: false, Default: "${workspace}"},
			{Name: "format", Label: "Format", Type: "select", Required: true, Default: "auto", Options: []string{"auto", "zip", "tar", "tar.gz", "tgz", "tar.bz2", "tbz2", "gzip"}},
			{Name: "outputName", Label: "Gzip Output Name", Type: "input", Required: false},
			{Name: "overwrite", Label: "Overwrite", Type: "switch", Required: false, Default: true},
		},
	}
}

func (e *CompressExecutor) Validate(params map[string]any) error {
	if len(pathsFrom(params["sources"])) == 0 {
		return fmt.Errorf("sources is required")
	}
	if strings.TrimSpace(stringFrom(params["outputPath"])) == "" {
		return fmt.Errorf("outputPath is required")
	}
	if _, err := normalizeCompressFormat(stringFrom(params["format"]), stringFrom(params["outputPath"])); err != nil {
		return err
	}
	return nil
}

func (e *ExtractExecutor) Validate(params map[string]any) error {
	if strings.TrimSpace(stringFrom(params["archivePath"])) == "" {
		return fmt.Errorf("archivePath is required")
	}
	if strings.TrimSpace(stringFrom(params["destDir"])) == "" {
		return fmt.Errorf("destDir is required")
	}
	if _, err := normalizeExtractFormat(stringFrom(params["format"]), stringFrom(params["archivePath"])); err != nil {
		return err
	}
	return nil
}

func (e *CompressExecutor) Execute(ctx *node.NodeContext, params map[string]any) (*node.NodeResult, error) {
	if err := e.Validate(params); err != nil {
		return nil, err
	}
	workspace, workdir, err := workdirFrom(ctx.Workspace, params["workdir"])
	if err != nil {
		return nil, err
	}
	outputPath, err := resolvePath(workspace, workdir, stringFrom(params["outputPath"]))
	if err != nil {
		return nil, err
	}
	format, err := normalizeCompressFormat(stringFrom(params["format"]), outputPath)
	if err != nil {
		return nil, err
	}
	sources := make([]string, 0)
	for _, source := range pathsFrom(params["sources"]) {
		resolved, err := resolvePath(workspace, workdir, source)
		if err != nil {
			return nil, err
		}
		sources = append(sources, resolved)
	}

	if !boolFrom(params["overwrite"], true) {
		if _, err := os.Stat(outputPath); err == nil {
			return nil, fmt.Errorf("output archive already exists: %s", outputPath)
		}
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, err
	}

	ctx.Log("stdout", fmt.Sprintf("compress format: %s\n", format))
	ctx.Log("stdout", fmt.Sprintf("output archive: %s\n", outputPath))

	var count int
	var bytesWritten int64
	switch format {
	case "zip":
		count, bytesWritten, err = compressZip(ctx, sources, outputPath, boolFrom(params["includeBaseDir"], true))
	case "tar", "tar.gz", "tgz":
		count, bytesWritten, err = compressTar(ctx, sources, outputPath, format != "tar", boolFrom(params["includeBaseDir"], true))
	case "gzip":
		count, bytesWritten, err = compressGzip(ctx, sources, outputPath)
	default:
		err = fmt.Errorf("unsupported compress format %q", format)
	}
	if err != nil {
		return nil, err
	}
	ctx.Log("stdout", fmt.Sprintf("compressed %d file(s), %d bytes\n", count, bytesWritten))
	return &node.NodeResult{Output: map[string]any{
		"format":     format,
		"outputPath": outputPath,
		"files":      count,
		"bytes":      bytesWritten,
	}}, nil
}

func (e *ExtractExecutor) Execute(ctx *node.NodeContext, params map[string]any) (*node.NodeResult, error) {
	if err := e.Validate(params); err != nil {
		return nil, err
	}
	workspace, workdir, err := workdirFrom(ctx.Workspace, params["workdir"])
	if err != nil {
		return nil, err
	}
	archivePath, err := resolvePath(workspace, workdir, stringFrom(params["archivePath"]))
	if err != nil {
		return nil, err
	}
	destDir, err := resolvePath(workspace, workdir, stringFrom(params["destDir"]))
	if err != nil {
		return nil, err
	}
	format, err := normalizeExtractFormat(stringFrom(params["format"]), archivePath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}

	ctx.Log("stdout", fmt.Sprintf("extract format: %s\n", format))
	ctx.Log("stdout", fmt.Sprintf("destination: %s\n", destDir))

	var count int
	var bytesWritten int64
	switch format {
	case "zip":
		count, bytesWritten, err = extractZip(ctx, archivePath, destDir, boolFrom(params["overwrite"], true))
	case "tar", "tar.gz", "tgz", "tar.bz2", "tbz2":
		count, bytesWritten, err = extractTar(ctx, archivePath, destDir, format, boolFrom(params["overwrite"], true))
	case "gzip":
		count, bytesWritten, err = extractGzip(ctx, archivePath, destDir, stringFrom(params["outputName"]), boolFrom(params["overwrite"], true))
	default:
		err = fmt.Errorf("unsupported extract format %q", format)
	}
	if err != nil {
		return nil, err
	}
	ctx.Log("stdout", fmt.Sprintf("extracted %d file(s), %d bytes\n", count, bytesWritten))
	return &node.NodeResult{Output: map[string]any{
		"format":      format,
		"archivePath": archivePath,
		"destDir":     destDir,
		"files":       count,
		"bytes":       bytesWritten,
	}}, nil
}

func compressZip(ctx *node.NodeContext, sources []string, outputPath string, includeBaseDir bool) (int, int64, error) {
	out, err := os.Create(outputPath)
	if err != nil {
		return 0, 0, err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	defer zw.Close()

	var count int
	var total int64
	used := map[string]bool{}
	for _, source := range sources {
		added, bytesWritten, err := walkSource(ctx, source, includeBaseDir, used, func(path string, info os.FileInfo, archiveName string) (int64, error) {
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return 0, err
			}
			header.Name = archiveName
			if info.IsDir() {
				header.Name = strings.TrimSuffix(header.Name, "/") + "/"
			} else {
				header.Method = zip.Deflate
			}
			writer, err := zw.CreateHeader(header)
			if err != nil {
				return 0, err
			}
			if info.IsDir() {
				return 0, nil
			}
			return copyFileTo(path, writer)
		})
		if err != nil {
			return 0, 0, err
		}
		count += added
		total += bytesWritten
	}
	return count, total, nil
}

func compressTar(ctx *node.NodeContext, sources []string, outputPath string, gzipOutput bool, includeBaseDir bool) (int, int64, error) {
	out, err := os.Create(outputPath)
	if err != nil {
		return 0, 0, err
	}
	defer out.Close()

	var writer io.Writer = out
	var gz *gzip.Writer
	if gzipOutput {
		gz = gzip.NewWriter(out)
		defer gz.Close()
		writer = gz
	}
	tw := tar.NewWriter(writer)
	defer tw.Close()

	var count int
	var total int64
	used := map[string]bool{}
	for _, source := range sources {
		added, bytesWritten, err := walkSource(ctx, source, includeBaseDir, used, func(path string, info os.FileInfo, archiveName string) (int64, error) {
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return 0, err
			}
			header.Name = archiveName
			if info.IsDir() {
				header.Name = strings.TrimSuffix(header.Name, "/") + "/"
			}
			if err := tw.WriteHeader(header); err != nil {
				return 0, err
			}
			if info.IsDir() {
				return 0, nil
			}
			return copyFileTo(path, tw)
		})
		if err != nil {
			return 0, 0, err
		}
		count += added
		total += bytesWritten
	}
	return count, total, nil
}

func compressGzip(ctx *node.NodeContext, sources []string, outputPath string) (int, int64, error) {
	if len(sources) != 1 {
		return 0, 0, fmt.Errorf("gzip format supports exactly one source file")
	}
	info, err := os.Stat(sources[0])
	if err != nil {
		return 0, 0, err
	}
	if !info.Mode().IsRegular() {
		return 0, 0, fmt.Errorf("gzip source must be a regular file: %s", sources[0])
	}
	in, err := os.Open(sources[0])
	if err != nil {
		return 0, 0, err
	}
	defer in.Close()
	out, err := os.Create(outputPath)
	if err != nil {
		return 0, 0, err
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	gz.Name = filepath.Base(sources[0])
	gz.ModTime = info.ModTime()
	written, copyErr := io.Copy(gz, in)
	closeErr := gz.Close()
	if copyErr != nil {
		return 0, 0, copyErr
	}
	if closeErr != nil {
		return 0, 0, closeErr
	}
	ctx.Log("stdout", fmt.Sprintf("source: %s\n", sources[0]))
	return 1, written, nil
}

func extractZip(ctx *node.NodeContext, archivePath, destDir string, overwrite bool) (int, int64, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return 0, 0, err
	}
	defer reader.Close()

	var count int
	var total int64
	for _, file := range reader.File {
		if err := ctx.Context.Err(); err != nil {
			return count, total, err
		}
		target, err := archiveTarget(destDir, file.Name)
		if err != nil {
			return count, total, err
		}
		info := file.FileInfo()
		if info.IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return count, total, err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return count, total, fmt.Errorf("unsupported zip entry type: %s", file.Name)
		}
		if err := ensureWritableTarget(target, overwrite); err != nil {
			return count, total, err
		}
		src, err := file.Open()
		if err != nil {
			return count, total, err
		}
		written, err := writeFile(target, info.Mode(), src)
		closeErr := src.Close()
		if err != nil {
			return count, total, err
		}
		if closeErr != nil {
			return count, total, closeErr
		}
		count++
		total += written
	}
	return count, total, nil
}

func extractTar(ctx *node.NodeContext, archivePath, destDir, format string, overwrite bool) (int, int64, error) {
	in, err := os.Open(archivePath)
	if err != nil {
		return 0, 0, err
	}
	defer in.Close()

	var reader io.Reader = in
	var gz *gzip.Reader
	if format == "tar.gz" || format == "tgz" {
		gz, err = gzip.NewReader(in)
		if err != nil {
			return 0, 0, err
		}
		defer gz.Close()
		reader = gz
	} else if format == "tar.bz2" || format == "tbz2" {
		reader = bzip2.NewReader(in)
	}

	tr := tar.NewReader(reader)
	var count int
	var total int64
	for {
		if err := ctx.Context.Err(); err != nil {
			return count, total, err
		}
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, total, err
		}
		target, err := archiveTarget(destDir, header.Name)
		if err != nil {
			return count, total, err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)&0o777); err != nil {
				return count, total, err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := ensureWritableTarget(target, overwrite); err != nil {
				return count, total, err
			}
			written, err := writeFile(target, os.FileMode(header.Mode)&0o777, tr)
			if err != nil {
				return count, total, err
			}
			count++
			total += written
		default:
			return count, total, fmt.Errorf("unsupported tar entry type for %s", header.Name)
		}
	}
	return count, total, nil
}

func extractGzip(ctx *node.NodeContext, archivePath, destDir, outputName string, overwrite bool) (int, int64, error) {
	in, err := os.Open(archivePath)
	if err != nil {
		return 0, 0, err
	}
	defer in.Close()
	gz, err := gzip.NewReader(in)
	if err != nil {
		return 0, 0, err
	}
	defer gz.Close()

	name := strings.TrimSpace(outputName)
	if name == "" {
		name = gz.Name
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(archivePath), ".gz")
	}
	target, err := archiveTarget(destDir, name)
	if err != nil {
		return 0, 0, err
	}
	if err := ensureWritableTarget(target, overwrite); err != nil {
		return 0, 0, err
	}
	written, err := writeFile(target, 0o644, gz)
	if err != nil {
		return 0, 0, err
	}
	ctx.Log("stdout", fmt.Sprintf("output file: %s\n", target))
	return 1, written, nil
}

func walkSource(ctx *node.NodeContext, source string, includeBaseDir bool, used map[string]bool, add func(path string, info os.FileInfo, archiveName string) (int64, error)) (int, int64, error) {
	info, err := os.Stat(source)
	if err != nil {
		return 0, 0, err
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return 0, 0, fmt.Errorf("source must be a regular file or directory: %s", source)
	}
	ctx.Log("stdout", fmt.Sprintf("source: %s\n", source))

	var count int
	var total int64
	rootParent := filepath.Dir(source)
	if info.IsDir() && !includeBaseDir {
		rootParent = source
	}

	err = filepath.WalkDir(source, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Context.Err(); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported source entry type: %s", current)
		}
		if current == source && info.IsDir() && !includeBaseDir {
			return nil
		}
		rel, err := filepath.Rel(rootParent, current)
		if err != nil {
			return err
		}
		archiveName := filepath.ToSlash(rel)
		if archiveName == "." || archiveName == "" {
			archiveName = filepath.Base(current)
		}
		if used[archiveName] {
			return fmt.Errorf("duplicate archive entry name: %s", archiveName)
		}
		used[archiveName] = true
		written, err := add(current, info, archiveName)
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			count++
			total += written
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return count, total, nil
}

func copyFileTo(path string, writer io.Writer) (int64, error) {
	in, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	return io.Copy(writer, in)
}

func writeFile(target string, mode os.FileMode, reader io.Reader) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return 0, err
	}
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return 0, err
	}
	written, copyErr := io.Copy(out, reader)
	closeErr := out.Close()
	if copyErr != nil {
		return written, copyErr
	}
	return written, closeErr
}

func ensureWritableTarget(target string, overwrite bool) error {
	if overwrite {
		return nil
	}
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("target already exists: %s", target)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func archiveTarget(destDir, entryName string) (string, error) {
	if strings.Contains(entryName, "\x00") {
		return "", fmt.Errorf("archive entry contains NUL byte")
	}
	name := strings.ReplaceAll(entryName, "\\", "/")
	clean := path.Clean(name)
	if clean == "." || clean == "/" || clean == "" {
		return "", fmt.Errorf("invalid archive entry name: %q", entryName)
	}
	if path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, ":") {
		return "", fmt.Errorf("unsafe archive entry path: %s", entryName)
	}
	target := filepath.Join(destDir, filepath.FromSlash(clean))
	if !withinDir(destDir, target) {
		return "", fmt.Errorf("archive entry escapes destination: %s", entryName)
	}
	return target, nil
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

func workdirFrom(workspace string, value any) (string, string, error) {
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return "", "", err
	}
	workdir := strings.TrimSpace(stringFrom(value))
	if workdir == "" {
		workdir = absWorkspace
	}
	workdir = strings.ReplaceAll(workdir, "${workspace}", absWorkspace)
	if !filepath.IsAbs(workdir) {
		workdir = filepath.Join(absWorkspace, workdir)
	}
	return absWorkspace, filepath.Clean(workdir), nil
}

func resolvePath(workspace, workdir, value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "${workspace}", workspace))
	if value == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value), nil
	}
	return filepath.Clean(filepath.Join(workdir, value)), nil
}

func normalizeCompressFormat(format, outputPath string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" || format == "auto" {
		format = formatFromPath(outputPath)
	}
	switch format {
	case "zip", "tar", "tar.gz", "tgz", "gzip":
		return format, nil
	case "gz":
		return "gzip", nil
	default:
		return "", fmt.Errorf("unsupported compress format %q", format)
	}
}

func normalizeExtractFormat(format, archivePath string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" || format == "auto" {
		format = formatFromPath(archivePath)
	}
	switch format {
	case "zip", "tar", "tar.gz", "tgz", "tar.bz2", "tbz2", "gzip":
		return format, nil
	case "gz":
		return "gzip", nil
	default:
		return "", fmt.Errorf("unsupported extract format %q", format)
	}
}

func formatFromPath(filePath string) string {
	lower := strings.ToLower(filePath)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		return "tar.gz"
	case strings.HasSuffix(lower, ".tgz"):
		return "tgz"
	case strings.HasSuffix(lower, ".tar.bz2"):
		return "tar.bz2"
	case strings.HasSuffix(lower, ".tbz2"):
		return "tbz2"
	case strings.HasSuffix(lower, ".zip"):
		return "zip"
	case strings.HasSuffix(lower, ".tar"):
		return "tar"
	case strings.HasSuffix(lower, ".gz"):
		return "gzip"
	default:
		return "zip"
	}
}

func pathsFrom(value any) []string {
	raw := stringFrom(value)
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, ",", "\n")
	var items []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			items = append(items, line)
		}
	}
	return items
}

func boolFrom(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "y", "on":
			return true
		case "false", "0", "no", "n", "off":
			return false
		}
	}
	return fallback
}

func stringFrom(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}
