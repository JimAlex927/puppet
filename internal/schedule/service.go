package schedule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"puppet/internal/engine"
	"puppet/internal/model"
	"puppet/internal/node"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

var ErrInvalidCronSchedule = errors.New("invalid cron schedule")

type Service struct {
	db     *gorm.DB
	engine *engine.Engine
}

type View struct {
	model.TaskSchedule
	ProjectName string `json:"projectName"`
	TaskName    string `json:"taskName"`
	LastStatus  string `json:"lastStatus"`
}

func NewService(db *gorm.DB, engine *engine.Engine) *Service {
	return &Service{db: db, engine: engine}
}

func (s *Service) List() ([]View, error) {
	var schedules []model.TaskSchedule
	if err := s.db.Order("enabled desc, next_run_at asc, id desc").Find(&schedules).Error; err != nil {
		return nil, err
	}
	return s.views(schedules)
}

func (s *Service) Get(id uint) (View, error) {
	var schedule model.TaskSchedule
	if err := s.db.First(&schedule, id).Error; err != nil {
		return View{}, err
	}
	views, err := s.views([]model.TaskSchedule{schedule})
	if err != nil {
		return View{}, err
	}
	return views[0], nil
}

func (s *Service) Create(schedule model.TaskSchedule) (View, error) {
	if err := s.normalize(&schedule, time.Now()); err != nil {
		return View{}, err
	}
	if err := s.db.Create(&schedule).Error; err != nil {
		return View{}, err
	}
	return s.Get(schedule.ID)
}

func (s *Service) Update(id uint, attrs model.TaskSchedule) (View, error) {
	var schedule model.TaskSchedule
	if err := s.db.First(&schedule, id).Error; err != nil {
		return View{}, err
	}
	schedule.ProjectID = attrs.ProjectID
	schedule.TaskID = attrs.TaskID
	schedule.Name = attrs.Name
	schedule.CronExpression = attrs.CronExpression
	schedule.CronTimezone = attrs.CronTimezone
	schedule.Enabled = attrs.Enabled
	schedule.InputJSON = attrs.InputJSON
	if err := s.normalize(&schedule, time.Now()); err != nil {
		return View{}, err
	}
	if err := s.db.Save(&schedule).Error; err != nil {
		return View{}, err
	}
	return s.Get(schedule.ID)
}

func (s *Service) Delete(id uint) error {
	return s.db.Delete(&model.TaskSchedule{}, id).Error
}

func (s *Service) RunNow(ctx context.Context, id uint) (model.TaskRun, error) {
	var schedule model.TaskSchedule
	if err := s.db.First(&schedule, id).Error; err != nil {
		return model.TaskRun{}, err
	}
	run, err := s.startSchedule(ctx, schedule)
	now := time.Now()
	updates := map[string]any{"last_run_at": now}
	if err != nil {
		updates["last_error_message"] = err.Error()
	} else {
		updates["last_run_id"] = run.ID
		updates["last_error_message"] = ""
	}
	_ = s.db.Model(&model.TaskSchedule{}).Where("id = ?", schedule.ID).Updates(updates).Error
	return run, err
}

func (s *Service) normalize(schedule *model.TaskSchedule, from time.Time) error {
	if schedule.TaskID == 0 {
		return fmt.Errorf("taskId is required")
	}
	var task model.Task
	if err := s.db.First(&task, schedule.TaskID).Error; err != nil {
		return err
	}
	if schedule.ProjectID == 0 {
		schedule.ProjectID = task.ProjectID
	}
	if schedule.ProjectID != task.ProjectID {
		return fmt.Errorf("task does not belong to project")
	}
	if strings.TrimSpace(schedule.Name) == "" {
		schedule.Name = task.Name + " schedule"
	}
	return s.normalizeNoTaskLookup(schedule, from)
}

func (s *Service) normalizeNoTaskLookup(schedule *model.TaskSchedule, from time.Time) error {
	schedule.Name = strings.TrimSpace(schedule.Name)
	if schedule.Name == "" {
		schedule.Name = "Schedule"
	}
	schedule.CronExpression = strings.TrimSpace(schedule.CronExpression)
	if schedule.CronExpression == "" {
		return fmt.Errorf("%w: cronExpression is required", ErrInvalidCronSchedule)
	}
	location, err := CronLocation(schedule.CronTimezone)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidCronSchedule, err)
	}
	parsed, err := cron.ParseStandard(schedule.CronExpression)
	if err != nil {
		return fmt.Errorf("%w: invalid cronExpression: %v", ErrInvalidCronSchedule, err)
	}
	if strings.TrimSpace(schedule.CronTimezone) == "" {
		schedule.CronTimezone = "Local"
	}
	if schedule.Enabled {
		next := parsed.Next(from.In(location))
		schedule.NextRunAt = &next
	} else {
		schedule.NextRunAt = nil
	}
	if strings.TrimSpace(schedule.InputJSON) != "" && !json.Valid([]byte(schedule.InputJSON)) {
		return fmt.Errorf("inputJson must be valid JSON")
	}
	return nil
}

func (s *Service) startSchedule(ctx context.Context, schedule model.TaskSchedule) (model.TaskRun, error) {
	var task model.Task
	if err := s.db.First(&task, schedule.TaskID).Error; err != nil {
		return model.TaskRun{}, err
	}
	input, err := RunInputForTask(schedule, task)
	if err != nil {
		return model.TaskRun{}, err
	}
	return s.engine.StartTask(ctx, schedule.TaskID, "schedule", schedule.Name, input)
}

func RunInputForTask(schedule model.TaskSchedule, scheduledTask model.Task) (map[string]any, error) {
	input := map[string]any{}
	if strings.TrimSpace(schedule.InputJSON) != "" {
		if err := json.Unmarshal([]byte(schedule.InputJSON), &input); err != nil {
			return nil, fmt.Errorf("decode inputJson: %w", err)
		}
	}
	var pipeline node.PipelineDefinition
	if err := json.Unmarshal([]byte(scheduledTask.PipelineJSON), &pipeline); err != nil {
		return nil, err
	}
	for _, item := range pipeline.Inputs {
		if _, exists := input[item.Name]; exists {
			continue
		}
		if item.Default != nil {
			input[item.Name] = item.Default
			continue
		}
		if item.Required {
			return nil, fmt.Errorf("required input %q has no default value", item.Name)
		}
	}
	return input, nil
}

func (s *Service) views(schedules []model.TaskSchedule) ([]View, error) {
	views := make([]View, 0, len(schedules))
	for _, item := range schedules {
		view := View{TaskSchedule: item}
		var project model.Project
		if err := s.db.Select("id", "name").First(&project, item.ProjectID).Error; err == nil {
			view.ProjectName = project.Name
		}
		var task model.Task
		if err := s.db.Select("id", "name").First(&task, item.TaskID).Error; err == nil {
			view.TaskName = task.Name
		}
		if item.LastRunID != 0 {
			var run model.TaskRun
			if err := s.db.Select("id", "status").First(&run, item.LastRunID).Error; err == nil {
				view.LastStatus = run.Status
			}
		}
		views = append(views, view)
	}
	return views, nil
}

func CronLocation(name string) (*time.Location, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, "local") {
		return time.Local, nil
	}
	location, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("invalid cronTimezone %q: %w", name, err)
	}
	return location, nil
}
