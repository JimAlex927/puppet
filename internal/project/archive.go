package project

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"puppet/internal/model"

	"gorm.io/gorm"
)

const archiveFormat = "puppet.project.v1"

type projectArchive struct {
	Format     string         `json:"format"`
	ExportedAt time.Time      `json:"exportedAt"`
	Project    archiveProject `json:"project"`
	Tasks      []archiveTask  `json:"tasks"`
}

type archiveProject struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt,omitempty"`
}

type archiveTask struct {
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	PipelineJSON    string    `json:"pipelineJson"`
	AllowConcurrent bool      `json:"allowConcurrent"`
	TimeoutSeconds  int       `json:"timeoutSeconds"`
	CreatedAt       time.Time `json:"createdAt,omitempty"`
	UpdatedAt       time.Time `json:"updatedAt,omitempty"`
}

func (s *Service) ExportArchive(id uint) ([]byte, string, error) {
	var project model.Project
	if err := s.db.First(&project, id).Error; err != nil {
		return nil, "", err
	}

	var tasks []model.Task
	if err := s.db.Where("project_id = ?", id).Order("id asc").Find(&tasks).Error; err != nil {
		return nil, "", err
	}

	archive := projectArchive{
		Format:     archiveFormat,
		ExportedAt: time.Now().UTC(),
		Project: archiveProject{
			Name:        project.Name,
			Description: project.Description,
			CreatedAt:   project.CreatedAt,
			UpdatedAt:   project.UpdatedAt,
		},
		Tasks: make([]archiveTask, 0, len(tasks)),
	}
	for _, task := range tasks {
		archive.Tasks = append(archive.Tasks, archiveTask{
			Name:            task.Name,
			Description:     task.Description,
			PipelineJSON:    task.PipelineJSON,
			AllowConcurrent: task.AllowConcurrent,
			TimeoutSeconds:  task.TimeoutSeconds,
			CreatedAt:       task.CreatedAt,
			UpdatedAt:       task.UpdatedAt,
		})
	}

	content, err := json.MarshalIndent(archive, "", "  ")
	if err != nil {
		return nil, "", err
	}

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	entry, err := writer.Create("project.json")
	if err != nil {
		return nil, "", err
	}
	if _, err := entry.Write(content); err != nil {
		_ = writer.Close()
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return buf.Bytes(), safeArchiveName(project.Name), nil
}

func (s *Service) ImportArchive(data []byte) (model.Project, error) {
	if len(data) == 0 {
		return model.Project{}, errors.New("empty project archive")
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return model.Project{}, fmt.Errorf("invalid project archive: %w", err)
	}

	var archiveFile *zip.File
	for _, file := range reader.File {
		if filepath.ToSlash(file.Name) == "project.json" {
			archiveFile = file
			break
		}
	}
	if archiveFile == nil {
		return model.Project{}, errors.New("project.json not found in archive")
	}
	if archiveFile.UncompressedSize64 > 10*1024*1024 {
		return model.Project{}, errors.New("project.json is too large")
	}

	content, err := readZipFile(archiveFile)
	if err != nil {
		return model.Project{}, err
	}

	var archive projectArchive
	if err := json.Unmarshal(content, &archive); err != nil {
		return model.Project{}, fmt.Errorf("invalid project.json: %w", err)
	}
	if err := validateArchive(archive); err != nil {
		return model.Project{}, err
	}

	var imported model.Project
	err = s.db.Transaction(func(tx *gorm.DB) error {
		imported = model.Project{
			Name:        archive.Project.Name,
			Description: archive.Project.Description,
		}
		if err := tx.Create(&imported).Error; err != nil {
			return err
		}
		for _, item := range archive.Tasks {
			task := model.Task{
				ProjectID:       imported.ID,
				Name:            item.Name,
				Description:     item.Description,
				PipelineJSON:    item.PipelineJSON,
				AllowConcurrent: item.AllowConcurrent,
				TimeoutSeconds:  item.TimeoutSeconds,
			}
			if task.TimeoutSeconds == 0 {
				task.TimeoutSeconds = 600
			}
			if err := tx.Create(&task).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return imported, err
}

func readZipFile(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(io.LimitReader(reader, 10*1024*1024+1))
}

func validateArchive(archive projectArchive) error {
	if archive.Format != archiveFormat {
		return fmt.Errorf("unsupported project archive format %q", archive.Format)
	}
	if strings.TrimSpace(archive.Project.Name) == "" {
		return errors.New("project name is required")
	}
	for index, task := range archive.Tasks {
		if strings.TrimSpace(task.Name) == "" {
			return fmt.Errorf("task %d name is required", index+1)
		}
		if strings.TrimSpace(task.PipelineJSON) == "" {
			return fmt.Errorf("task %q pipelineJson is required", task.Name)
		}
		var pipeline any
		if err := json.Unmarshal([]byte(task.PipelineJSON), &pipeline); err != nil {
			return fmt.Errorf("task %q pipelineJson is invalid: %w", task.Name, err)
		}
	}
	return nil
}

func safeArchiveName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "project"
	}
	re := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]+`)
	name = re.ReplaceAllString(name, "-")
	name = strings.Trim(name, ". ")
	if name == "" {
		name = "project"
	}
	return name + ".zip"
}
