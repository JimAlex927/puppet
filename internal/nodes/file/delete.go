package file

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"puppet/internal/node"
)

type DeleteExecutor struct{}

func NewDelete() *DeleteExecutor { return &DeleteExecutor{} }

func (e *DeleteExecutor) Type() string { return "file_delete" }

func (e *DeleteExecutor) Metadata() node.NodeMetadata {
	return node.NodeMetadata{
		Type:        e.Type(),
		Name:        "Delete Files",
		Category:    "file",
		Description: "按路径或通配符删除文件、多个文件或目录",
		SupportedOS: []string{"linux", "darwin", "windows"},
		Fields: []node.NodeField{
			{Name: "targets", Label: "Paths / Patterns", Type: "textarea", Required: true},
			{Name: "workdir", Label: "Workdir", Type: "input", Required: false, Default: "${workspace}"},
			{Name: "recursive", Label: "Recursive Directories", Type: "switch", Required: false, Default: true},
			{Name: "allowMissing", Label: "Allow Missing", Type: "switch", Required: false, Default: true},
			{Name: "allowOutsideWorkspace", Label: "Allow Outside Workspace", Type: "switch", Required: false, Default: false},
			{Name: "dryRun", Label: "Dry Run", Type: "switch", Required: false, Default: false},
		},
	}
}

func (e *DeleteExecutor) Validate(params map[string]any) error {
	if len(pathsFrom(params["targets"])) == 0 {
		return fmt.Errorf("targets is required")
	}
	return nil
}

func (e *DeleteExecutor) Execute(ctx *node.NodeContext, params map[string]any) (*node.NodeResult, error) {
	if err := e.Validate(params); err != nil {
		return nil, err
	}
	workspace, workdir, err := workdirFrom(ctx.Workspace, params["workdir"])
	if err != nil {
		return nil, err
	}
	recursive := boolFrom(params["recursive"], true)
	allowMissing := boolFrom(params["allowMissing"], true)
	allowOutsideWorkspace := boolFrom(params["allowOutsideWorkspace"], false)
	dryRun := boolFrom(params["dryRun"], false)

	targets, missing, err := expandTargets(workspace, workdir, pathsFrom(params["targets"]), allowOutsideWorkspace)
	if err != nil {
		return nil, err
	}
	if len(missing) > 0 && !allowMissing {
		return nil, fmt.Errorf("no files matched: %s", strings.Join(missing, ", "))
	}
	sort.Slice(targets, func(i, j int) bool {
		return len(targets[i]) > len(targets[j])
	})

	ctx.Log("stdout", fmt.Sprintf("workdir: %s\n", workdir))
	if dryRun {
		ctx.Log("stdout", "dry run: true\n")
	}

	var files, dirs int
	var bytes int64
	var deleted []string
	for _, target := range targets {
		if err := ctx.Context.Err(); err != nil {
			return nil, err
		}
		info, err := os.Lstat(target)
		if err != nil {
			if os.IsNotExist(err) && allowMissing {
				ctx.Log("stdout", fmt.Sprintf("missing: %s\n", target))
				continue
			}
			return nil, err
		}
		stat, err := measureTarget(target, info)
		if err != nil {
			return nil, err
		}
		ctx.Log("stdout", fmt.Sprintf("delete: %s\n", target))
		if !dryRun {
			if info.IsDir() {
				if recursive {
					err = os.RemoveAll(target)
				} else {
					err = os.Remove(target)
				}
			} else {
				err = os.Remove(target)
			}
			if err != nil {
				return nil, err
			}
		}
		files += stat.files
		dirs += stat.dirs
		bytes += stat.bytes
		deleted = append(deleted, target)
	}
	ctx.Log("stdout", fmt.Sprintf("deleted %d file(s), %d dir(s), %d bytes\n", files, dirs, bytes))
	return &node.NodeResult{Output: map[string]any{
		"files":    files,
		"dirs":     dirs,
		"bytes":    bytes,
		"targets":  deleted,
		"missing":  missing,
		"dryRun":   dryRun,
		"workdir":  workdir,
		"patterns": pathsFrom(params["targets"]),
	}}, nil
}

type targetStat struct {
	files int
	dirs  int
	bytes int64
}

func measureTarget(target string, info os.FileInfo) (targetStat, error) {
	if !info.IsDir() {
		return targetStat{files: 1, bytes: info.Size()}, nil
	}
	var stat targetStat
	err := filepath.WalkDir(target, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			stat.dirs++
		} else {
			stat.files++
			stat.bytes += info.Size()
		}
		return nil
	})
	return stat, err
}

func expandTargets(workspace, workdir string, patterns []string, allowOutsideWorkspace bool) ([]string, []string, error) {
	seen := map[string]bool{}
	var targets []string
	var missing []string
	for _, pattern := range patterns {
		resolvedPattern, err := resolvePath(workspace, workdir, pattern)
		if err != nil {
			return nil, nil, err
		}
		matches := []string{resolvedPattern}
		if hasGlobMeta(resolvedPattern) {
			matches, err = filepath.Glob(resolvedPattern)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
			}
			if len(matches) == 0 {
				missing = append(missing, pattern)
				continue
			}
		}
		if !hasGlobMeta(resolvedPattern) {
			if _, err := os.Lstat(resolvedPattern); os.IsNotExist(err) {
				missing = append(missing, pattern)
				continue
			} else if err != nil {
				return nil, nil, err
			}
		}
		for _, match := range matches {
			clean, err := secureDeleteTarget(workspace, match, allowOutsideWorkspace)
			if err != nil {
				return nil, nil, err
			}
			key, err := filepath.Abs(clean)
			if err != nil {
				return nil, nil, err
			}
			key = filepath.Clean(key)
			if seen[key] {
				continue
			}
			seen[key] = true
			targets = append(targets, key)
		}
	}
	return targets, missing, nil
}

func secureDeleteTarget(workspace, target string, allowOutsideWorkspace bool) (string, error) {
	clean, err := filepath.Abs(filepath.Clean(target))
	if err != nil {
		return "", err
	}
	if isRootPath(clean) {
		return "", fmt.Errorf("refuse to delete filesystem root: %s", clean)
	}
	workspaceAbs, err := filepath.Abs(filepath.Clean(workspace))
	if err != nil {
		return "", err
	}
	if samePath(clean, workspaceAbs) {
		return "", fmt.Errorf("refuse to delete workspace root: %s", clean)
	}
	if !allowOutsideWorkspace && !withinDir(workspaceAbs, clean) {
		return "", fmt.Errorf("target is outside workspace: %s", clean)
	}
	return clean, nil
}

func hasGlobMeta(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

func isRootPath(path string) bool {
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	if volume != "" {
		return samePath(clean, filepath.Clean(volume+string(filepath.Separator)))
	}
	return clean == string(filepath.Separator)
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

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func workdirFrom(workspace string, value any) (string, string, error) {
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return "", "", err
	}
	workdir := cleanPathInput(stringFrom(value))
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
	value = cleanPathInput(strings.ReplaceAll(value, "${workspace}", workspace))
	if value == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value), nil
	}
	return filepath.Clean(filepath.Join(workdir, value)), nil
}

func pathsFrom(value any) []string {
	raw := cleanPathInput(stringFrom(value))
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, ",", "\n")
	var items []string
	for _, line := range strings.Split(raw, "\n") {
		line = cleanPathInput(line)
		if line != "" {
			items = append(items, line)
		}
	}
	return items
}

func cleanPathInput(value string) string {
	value = strings.Map(func(r rune) rune {
		switch r {
		case '\u202A', '\u202B', '\u202C', '\u202D', '\u202E', '\u2066', '\u2067', '\u2068', '\u2069', '\uFEFF':
			return -1
		default:
			return r
		}
	}, value)
	return strings.Trim(strings.TrimSpace(value), `"'`)
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
