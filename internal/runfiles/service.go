package runfiles

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"puppet/internal/model"

	"github.com/tus/tusd/v2/pkg/filelocker"
	"github.com/tus/tusd/v2/pkg/filestore"
	tusd "github.com/tus/tusd/v2/pkg/handler"
	"gorm.io/gorm"
)

const defaultMaxUploadSize = 10 * 1024 * 1024 * 1024

type Service struct {
	db           *gorm.DB
	workspaceDir string
	uploadDir    string
	tusHandler   http.Handler
	mountPath    string
	maxFileSize  int64
}

func NewService(db *gorm.DB, workspaceDir string, basePath string) (*Service, error) {
	uploadDir := filepath.Join(workspaceDir, ".run-file-uploads")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return nil, err
	}

	store := filestore.New(uploadDir)
	locker := filelocker.New(uploadDir)
	composer := tusd.NewStoreComposer()
	store.UseIn(composer)
	locker.UseIn(composer)

	service := &Service{
		db:           db,
		workspaceDir: workspaceDir,
		uploadDir:    uploadDir,
		mountPath:    strings.TrimSuffix(basePath, "/"),
		maxFileSize:  defaultMaxUploadSize,
	}
	handler, err := tusd.NewHandler(tusd.Config{
		BasePath:        basePath,
		StoreComposer:   composer,
		MaxSize:         service.maxFileSize,
		DisableDownload: true,
		PreFinishResponseCallback: func(hook tusd.HookEvent) (tusd.HTTPResponse, error) {
			return tusd.HTTPResponse{}, service.saveCompletedUpload(hook)
		},
	})
	if err != nil {
		return nil, err
	}
	service.tusHandler = http.StripPrefix(service.mountPath, handler)
	return service, nil
}

func (s *Service) ServeUpload(w http.ResponseWriter, r *http.Request) {
	s.tusHandler.ServeHTTP(w, r)
}

func (s *Service) saveCompletedUpload(hook tusd.HookEvent) error {
	if hook.Upload.IsPartial {
		return nil
	}

	runID, err := strconv.ParseUint(metadataValue(hook.Upload.MetaData, "runId"), 10, 64)
	if err != nil || runID == 0 {
		return errors.New("completed run file upload has invalid runId")
	}
	inputName := sanitizeInputName(metadataValue(hook.Upload.MetaData, "inputName"))
	if inputName == "" {
		return errors.New("completed run file upload has no inputName")
	}

	var run model.TaskRun
	if err := s.db.First(&run, uint(runID)).Error; err != nil {
		return err
	}
	if run.Status != model.TaskRunPending {
		return fmt.Errorf("task run #%d is %s, files can only be uploaded before it starts", run.ID, run.Status)
	}

	srcPath := hook.Upload.Storage["Path"]
	if srcPath == "" {
		return errors.New("completed run file upload has no storage path")
	}
	absSrc, err := filepath.Abs(srcPath)
	if err != nil {
		return err
	}
	absUploadDir, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(absSrc, absUploadDir+string(os.PathSeparator)) && absSrc != absUploadDir {
		return fmt.Errorf("completed upload path %q is outside run file upload storage", absSrc)
	}

	name := sanitizeFileName(metadataValue(hook.Upload.MetaData, "filename", "name"))
	if name == "" {
		name = hook.Upload.ID
	}
	destDir := filepath.Join(s.workspaceDir, fmt.Sprintf("taskrun-%d", run.ID), "uploaded_files")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	destPath := uniquePath(destDir, name)
	if err := moveFile(absSrc, destPath); err != nil {
		return err
	}
	_ = os.Remove(absSrc + ".info")

	relativePath := filepath.ToSlash(filepath.Join("uploaded_files", filepath.Base(destPath)))
	multiple := strings.EqualFold(metadataValue(hook.Upload.MetaData, "multiple"), "true")
	return s.appendRunInput(run.ID, inputName, relativePath, multiple)
}

func (s *Service) appendRunInput(runID uint, inputName string, relativePath string, multiple bool) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var run model.TaskRun
		if err := tx.First(&run, runID).Error; err != nil {
			return err
		}
		if run.Status != model.TaskRunPending {
			return fmt.Errorf("task run #%d is %s, files can only be attached before it starts", run.ID, run.Status)
		}
		input := map[string]any{}
		if strings.TrimSpace(run.InputJSON) != "" {
			_ = json.Unmarshal([]byte(run.InputJSON), &input)
		}
		if multiple {
			values := []string{}
			switch existing := input[inputName].(type) {
			case []any:
				for _, item := range existing {
					values = append(values, fmt.Sprint(item))
				}
			case []string:
				values = append(values, existing...)
			case string:
				if existing != "" {
					values = append(values, existing)
				}
			}
			values = append(values, relativePath)
			input[inputName] = values
		} else {
			input[inputName] = relativePath
		}
		content, _ := json.Marshal(input)
		run.InputJSON = string(content)
		return tx.Save(&run).Error
	})
}

func metadataValue(meta tusd.MetaData, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(meta[key]); value != "" {
			return value
		}
	}
	return ""
}

func sanitizeInputName(name string) string {
	name = strings.TrimSpace(name)
	if strings.ContainsAny(name, `/\`) {
		return ""
	}
	return name
}

func sanitizeFileName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == string(filepath.Separator) || name == "" {
		return ""
	}
	return name
}

func uniquePath(dir string, name string) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	candidate := filepath.Join(dir, name)
	for index := 1; ; index++ {
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
		candidate = filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, index, ext))
	}
}

func moveFile(src string, dest string) error {
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}
