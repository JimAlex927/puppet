package sharedfiles

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"puppet/internal/model"

	"github.com/tus/tusd/v2/pkg/filelocker"
	"github.com/tus/tusd/v2/pkg/filestore"
	tusd "github.com/tus/tusd/v2/pkg/handler"
	"gorm.io/gorm"
)

const defaultMaxUploadSize = 10 * 1024 * 1024 * 1024

type contextKey string

const uploadedByKey contextKey = "puppet-shared-files-uploaded-by"

type Service struct {
	db          *gorm.DB
	storageDir  string
	uploadDir   string
	tusHandler  http.Handler
	mountPath   string
	maxFileSize int64
}

func NewService(db *gorm.DB, storageDir string, basePath string) (*Service, error) {
	uploadDir := filepath.Join(storageDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return nil, err
	}

	store := filestore.New(uploadDir)
	locker := filelocker.New(uploadDir)
	composer := tusd.NewStoreComposer()
	store.UseIn(composer)
	locker.UseIn(composer)

	service := &Service{
		db:          db,
		storageDir:  storageDir,
		uploadDir:   uploadDir,
		mountPath:   strings.TrimSuffix(basePath, "/"),
		maxFileSize: defaultMaxUploadSize,
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

func WithUploadedBy(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, uploadedByKey, username)
}

func (s *Service) ServeUpload(w http.ResponseWriter, r *http.Request) {
	s.tusHandler.ServeHTTP(w, r)
}

func (s *Service) List() ([]model.SharedFile, error) {
	var files []model.SharedFile
	err := s.db.Order("id desc").Find(&files).Error
	return files, err
}

func (s *Service) Get(id uint) (model.SharedFile, error) {
	var file model.SharedFile
	err := s.db.First(&file, id).Error
	return file, err
}

func (s *Service) Delete(id uint) error {
	file, err := s.Get(id)
	if err != nil {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.SharedFile{}, id).Error; err != nil {
			return err
		}
		if err := removeUploadFiles(file.StoragePath); err != nil {
			return err
		}
		return nil
	})
}

func (s *Service) saveCompletedUpload(hook tusd.HookEvent) error {
	if hook.Upload.IsPartial {
		return nil
	}

	path := hook.Upload.Storage["Path"]
	if path == "" {
		return errors.New("completed upload has no storage path")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	absUploadDir, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(absPath, absUploadDir+string(os.PathSeparator)) && absPath != absUploadDir {
		return fmt.Errorf("completed upload path %q is outside shared file storage", absPath)
	}

	uploadedBy, _ := hook.Context.Value(uploadedByKey).(string)
	name := metadataValue(hook.Upload.MetaData, "filename", "name")
	if strings.TrimSpace(name) == "" {
		name = hook.Upload.ID
	}
	contentType := metadataValue(hook.Upload.MetaData, "filetype", "type", "contentType")
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	file := model.SharedFile{
		UploadID:    hook.Upload.ID,
		Name:        sanitizeDisplayName(name),
		Size:        hook.Upload.Size,
		ContentType: contentType,
		StoragePath: absPath,
		UploadedBy:  uploadedBy,
	}

	var existing model.SharedFile
	err = s.db.Where("upload_id = ?", hook.Upload.ID).First(&existing).Error
	if err == nil {
		existing.Name = file.Name
		existing.Size = file.Size
		existing.ContentType = file.ContentType
		existing.StoragePath = file.StoragePath
		existing.UploadedBy = file.UploadedBy
		return s.db.Save(&existing).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.db.Create(&file).Error
}

func metadataValue(meta tusd.MetaData, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(meta[key]); value != "" {
			return value
		}
	}
	return ""
}

func sanitizeDisplayName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "download"
	}
	return name
}

func removeUploadFiles(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	infoPath := path + ".info"
	if err := os.Remove(infoPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
