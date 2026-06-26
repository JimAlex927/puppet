package db

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"puppet/internal/config"
	"puppet/internal/model"
	"puppet/internal/task"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.WorkspaceDir, 0o755); err != nil {
		return nil, err
	}

	database, err := gorm.Open(sqlite.Open(cfg.DatabaseDSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := database.AutoMigrate(
		&model.Project{},
		&model.Task{},
		&model.TaskRun{},
		&model.NodeRun{},
		&model.RunLog{},
		&model.Agent{},
		&model.Credential{},
		&model.User{},
		&model.Session{},
	); err != nil {
		return nil, err
	}
	if err := reconcileInterruptedRuns(database); err != nil {
		return nil, err
	}
	if err := seedDefaultUser(database); err != nil {
		return nil, err
	}
	if err := seedLocalAgent(database); err != nil {
		return nil, err
	}
	if err := seedDemo(database); err != nil {
		return nil, err
	}
	return database, nil
}

func reconcileInterruptedRuns(database *gorm.DB) error {
	now := time.Now()
	if err := database.Model(&model.TaskRun{}).
		Where("status IN ?", []string{model.TaskRunPending, model.TaskRunRunning}).
		Updates(map[string]any{
			"status":        model.TaskRunCanceled,
			"finished_at":   now,
			"error_message": "canceled because server restarted while run was active",
		}).Error; err != nil {
		return err
	}
	return database.Model(&model.NodeRun{}).
		Where("status IN ?", []string{model.NodeRunPending, model.NodeRunRunning}).
		Updates(map[string]any{
			"status":        model.NodeRunCanceled,
			"finished_at":   now,
			"error_message": "canceled because server restarted while run was active",
		}).Error
}

func seedDefaultUser(database *gorm.DB) error {
	var count int64
	if err := database.Model(&model.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := HashPassword("puppetadmin")
	if err != nil {
		return err
	}
	return database.Create(&model.User{
		Username:     "puppetadmin",
		DisplayName:  "Puppet Admin",
		Role:         "admin",
		PasswordHash: hash,
		Status:       "active",
	}).Error
}

func HashPassword(password string) (string, error) {
	content, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(content), err
}

func seedLocalAgent(database *gorm.DB) error {
	hostname, _ := os.Hostname()
	labels, _ := json.Marshal([]string{"local"})
	now := time.Now()
	agent := model.Agent{
		Name:            "local-agent",
		OS:              runtime.GOOS,
		Arch:            runtime.GOARCH,
		Hostname:        hostname,
		LabelsJSON:      string(labels),
		Status:          "online",
		LastHeartbeatAt: &now,
	}
	var existing model.Agent
	err := database.Where("name = ?", agent.Name).First(&existing).Error
	if err == nil {
		existing.OS = agent.OS
		existing.Arch = agent.Arch
		existing.Hostname = agent.Hostname
		existing.LabelsJSON = agent.LabelsJSON
		existing.Status = "online"
		existing.LastHeartbeatAt = &now
		return database.Save(&existing).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return database.Create(&agent).Error
}

func seedDemo(database *gorm.DB) error {
	var count int64
	if err := database.Model(&model.Project{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	project := model.Project{Name: "Demo Project", Description: "内置示例项目"}
	if err := database.Create(&project).Error; err != nil {
		return err
	}
	pipeline := task.DefaultPipelineJSON("Demo Pipeline")
	demoTask := model.Task{
		ProjectID:       project.ID,
		Name:            "Demo Task",
		Description:     "包含 shell、sleep、http 的示例任务",
		PipelineJSON:    pipeline,
		AllowConcurrent: false,
		TimeoutSeconds:  600,
	}
	if err := database.Create(&demoTask).Error; err != nil {
		return fmt.Errorf("seed demo task: %w", err)
	}
	return nil
}
