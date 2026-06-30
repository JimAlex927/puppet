package task

import (
	"encoding/json"
	"fmt"
	"strings"

	"puppet/internal/model"
	"puppet/internal/node"

	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) ListByProject(projectID uint) ([]model.Task, error) {
	var tasks []model.Task
	err := s.db.Where("project_id = ?", projectID).Order("id desc").Find(&tasks).Error
	return tasks, err
}

func (s *Service) Get(id uint) (model.Task, error) {
	var task model.Task
	err := s.db.First(&task, id).Error
	return task, err
}

func (s *Service) Create(task model.Task) (model.Task, error) {
	if task.PipelineJSON == "" {
		task.PipelineJSON = DefaultPipelineJSON(task.Name)
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		return createPipelineVersion(tx, task.ID, task.PipelineJSON, "", "initial")
	})
	return task, err
}

func (s *Service) Update(id uint, attrs model.Task) (model.Task, error) {
	task, err := s.Get(id)
	if err != nil {
		return task, err
	}
	task.Name = attrs.Name
	task.Description = attrs.Description
	task.AllowConcurrent = attrs.AllowConcurrent
	task.TimeoutSeconds = attrs.TimeoutSeconds
	if attrs.PipelineJSON != "" {
		task.PipelineJSON = attrs.PipelineJSON
	}
	err = s.db.Save(&task).Error
	return task, err
}

func (s *Service) Delete(id uint) error {
	return s.db.Delete(&model.Task{}, id).Error
}

func (s *Service) Pipeline(id uint) (node.PipelineDefinition, error) {
	task, err := s.Get(id)
	if err != nil {
		return node.PipelineDefinition{}, err
	}
	var pipeline node.PipelineDefinition
	err = json.Unmarshal([]byte(task.PipelineJSON), &pipeline)
	return pipeline, err
}

func (s *Service) UpdatePipeline(id uint, pipeline node.PipelineDefinition, createdBy string) (node.PipelineDefinition, error) {
	content, err := json.MarshalIndent(pipeline, "", "  ")
	if err != nil {
		return pipeline, err
	}
	task, err := s.Get(id)
	if err != nil {
		return pipeline, err
	}
	next := string(content)
	if normalizePipelineJSON(task.PipelineJSON) == normalizePipelineJSON(next) {
		return pipeline, nil
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := ensurePipelineHistoryTx(tx, task.ID, task.PipelineJSON, ""); err != nil {
			return err
		}
		task.PipelineJSON = next
		if err := tx.Save(&task).Error; err != nil {
			return err
		}
		return createPipelineVersion(tx, task.ID, next, createdBy, "save")
	})
	return pipeline, err
}

func (s *Service) PipelineVersions(taskID uint) ([]model.PipelineVersion, error) {
	task, err := s.Get(taskID)
	if err != nil {
		return nil, err
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		return ensurePipelineHistoryTx(tx, task.ID, task.PipelineJSON, "")
	}); err != nil {
		return nil, err
	}
	var versions []model.PipelineVersion
	err = s.db.Where("task_id = ?", taskID).Order("version desc").Find(&versions).Error
	return versions, err
}

func (s *Service) PipelineVersion(taskID uint, versionID uint) (model.PipelineVersion, error) {
	if err := s.EnsurePipelineHistory(taskID); err != nil {
		return model.PipelineVersion{}, err
	}
	var version model.PipelineVersion
	err := s.db.Where("task_id = ? AND id = ?", taskID, versionID).First(&version).Error
	return version, err
}

func (s *Service) RestorePipelineVersion(taskID uint, versionID uint, createdBy string) (node.PipelineDefinition, model.PipelineVersion, error) {
	var restored node.PipelineDefinition
	var created model.PipelineVersion
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var task model.Task
		if err := tx.First(&task, taskID).Error; err != nil {
			return err
		}
		if err := ensurePipelineHistoryTx(tx, task.ID, task.PipelineJSON, ""); err != nil {
			return err
		}
		var target model.PipelineVersion
		if err := tx.Where("task_id = ? AND id = ?", taskID, versionID).First(&target).Error; err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(target.PipelineJSON), &restored); err != nil {
			return err
		}
		if normalizePipelineJSON(task.PipelineJSON) == normalizePipelineJSON(target.PipelineJSON) {
			created = target
			return nil
		}
		task.PipelineJSON = target.PipelineJSON
		if err := tx.Save(&task).Error; err != nil {
			return err
		}
		message := fmt.Sprintf("restore v%d", target.Version)
		createdVersion, err := createPipelineVersionRecord(tx, task.ID, target.PipelineJSON, createdBy, message)
		if err != nil {
			return err
		}
		created = createdVersion
		return nil
	})
	return restored, created, err
}

func (s *Service) EnsurePipelineHistory(taskID uint) error {
	task, err := s.Get(taskID)
	if err != nil {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		return ensurePipelineHistoryTx(tx, task.ID, task.PipelineJSON, "")
	})
}

func ensurePipelineHistoryTx(tx *gorm.DB, taskID uint, pipelineJSON string, createdBy string) error {
	var count int64
	if err := tx.Model(&model.PipelineVersion{}).Where("task_id = ?", taskID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return createPipelineVersion(tx, taskID, pipelineJSON, createdBy, "initial")
}

func createPipelineVersion(tx *gorm.DB, taskID uint, pipelineJSON string, createdBy string, message string) error {
	_, err := createPipelineVersionRecord(tx, taskID, pipelineJSON, createdBy, message)
	return err
}

func createPipelineVersionRecord(tx *gorm.DB, taskID uint, pipelineJSON string, createdBy string, message string) (model.PipelineVersion, error) {
	var latest model.PipelineVersion
	version := 1
	err := tx.Where("task_id = ?", taskID).Order("version desc").First(&latest).Error
	if err == nil {
		version = latest.Version + 1
	} else if err != gorm.ErrRecordNotFound {
		return model.PipelineVersion{}, err
	}
	item := model.PipelineVersion{
		TaskID:       taskID,
		Version:      version,
		PipelineJSON: pipelineJSON,
		CreatedBy:    createdBy,
		Message:      message,
	}
	err = tx.Create(&item).Error
	return item, err
}

func normalizePipelineJSON(content string) string {
	var value any
	if err := json.Unmarshal([]byte(content), &value); err != nil {
		return strings.TrimSpace(content)
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return strings.TrimSpace(content)
	}
	return string(normalized)
}
