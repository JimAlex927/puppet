package task

import (
	"encoding/json"

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
	err := s.db.Create(&task).Error
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

func (s *Service) UpdatePipeline(id uint, pipeline node.PipelineDefinition) (node.PipelineDefinition, error) {
	content, err := json.MarshalIndent(pipeline, "", "  ")
	if err != nil {
		return pipeline, err
	}
	task, err := s.Get(id)
	if err != nil {
		return pipeline, err
	}
	task.PipelineJSON = string(content)
	return pipeline, s.db.Save(&task).Error
}
