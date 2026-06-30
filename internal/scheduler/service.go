package scheduler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"puppet/internal/engine"
	"puppet/internal/model"
	"puppet/internal/schedule"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type Service struct {
	db       *gorm.DB
	engine   *engine.Engine
	interval time.Duration
}

func New(db *gorm.DB, engine *engine.Engine) *Service {
	return &Service{db: db, engine: engine, interval: 30 * time.Second}
}

func (s *Service) Start(ctx context.Context) {
	if s == nil || s.db == nil || s.engine == nil {
		return
	}
	go s.loop(ctx)
}

func (s *Service) loop(ctx context.Context) {
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.tick(ctx, time.Now())
			timer.Reset(s.interval)
		}
	}
}

func (s *Service) tick(ctx context.Context, now time.Time) {
	var schedules []model.TaskSchedule
	err := s.db.
		Where("enabled = ? AND next_run_at IS NOT NULL AND next_run_at <= ?", true, now).
		Order("next_run_at asc, id asc").
		Find(&schedules).Error
	if err != nil {
		log.Printf("scheduler: list due schedules: %v", err)
		return
	}

	for _, scheduledItem := range schedules {
		if err := ctx.Err(); err != nil {
			return
		}
		if err := s.runDueSchedule(ctx, scheduledItem, now); err != nil {
			log.Printf("scheduler: schedule #%d %q: %v", scheduledItem.ID, scheduledItem.Name, err)
		}
	}
}

func (s *Service) runDueSchedule(ctx context.Context, scheduledItem model.TaskSchedule, now time.Time) error {
	if !scheduledItem.Enabled {
		return nil
	}
	parsed, err := cron.ParseStandard(strings.TrimSpace(scheduledItem.CronExpression))
	if err != nil {
		return fmt.Errorf("parse cron expression: %w", err)
	}
	location, err := schedule.CronLocation(scheduledItem.CronTimezone)
	if err != nil {
		return err
	}
	next := parsed.Next(now.In(location))
	updates := map[string]any{
		"next_run_at": next,
		"last_run_at": now,
	}
	result := s.db.Model(&model.TaskSchedule{}).
		Where("id = ? AND enabled = ? AND next_run_at <= ?", scheduledItem.ID, true, now).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}

	var task model.Task
	if err := s.db.First(&task, scheduledItem.TaskID).Error; err != nil {
		return err
	}
	input, err := runInput(scheduledItem, task)
	if err != nil {
		_ = s.db.Model(&model.TaskSchedule{}).Where("id = ?", scheduledItem.ID).
			Update("last_error_message", err.Error()).Error
		return err
	}
	run, err := s.engine.StartTask(context.Background(), scheduledItem.TaskID, "schedule", scheduledItem.Name, input)
	if err != nil {
		_ = s.db.Model(&model.TaskSchedule{}).Where("id = ?", scheduledItem.ID).
			Update("last_error_message", err.Error()).Error
		return fmt.Errorf("start task: %w", err)
	}
	_ = s.db.Model(&model.TaskSchedule{}).Where("id = ?", scheduledItem.ID).
		Updates(map[string]any{"last_run_id": run.ID, "last_error_message": ""}).Error
	log.Printf("scheduler: started schedule #%d %q task #%d", scheduledItem.ID, scheduledItem.Name, scheduledItem.TaskID)
	return nil
}

func runInput(scheduledItem model.TaskSchedule, task model.Task) (map[string]any, error) {
	return schedule.RunInputForTask(scheduledItem, task)
}
