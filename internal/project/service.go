package project

import (
	"puppet/internal/model"

	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List() ([]model.Project, error) {
	var projects []model.Project
	err := s.db.Order("id desc").Find(&projects).Error
	return projects, err
}

func (s *Service) Get(id uint) (model.Project, error) {
	var project model.Project
	err := s.db.First(&project, id).Error
	return project, err
}

func (s *Service) Create(project model.Project) (model.Project, error) {
	err := s.db.Create(&project).Error
	return project, err
}

func (s *Service) Update(id uint, attrs model.Project) (model.Project, error) {
	project, err := s.Get(id)
	if err != nil {
		return project, err
	}
	project.Name = attrs.Name
	project.Description = attrs.Description
	err = s.db.Save(&project).Error
	return project, err
}

func (s *Service) Delete(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project_id = ?", id).Delete(&model.Task{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Project{}, id).Error
	})
}
