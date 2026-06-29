// Package cleanup periodically removes old task run workspaces and log records
// to prevent unbounded disk and database growth.
package cleanup

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"

	"puppet/internal/model"
)

type Cleaner struct {
	db           *gorm.DB
	workspaceDir string
	retainCount  int // per task; 0 = unlimited
}

func New(db *gorm.DB, workspaceDir string, retainCount int) *Cleaner {
	return &Cleaner{db: db, workspaceDir: workspaceDir, retainCount: retainCount}
}

// Start runs the cleanup loop in a background goroutine until ctx is cancelled.
func (c *Cleaner) Start(ctx context.Context) {
	go func() {
		// Run once at startup, then every hour.
		c.run()
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.run()
			}
		}
	}()
}

func (c *Cleaner) run() {
	if c.retainCount <= 0 {
		return
	}
	var taskIDs []uint
	if err := c.db.Model(&model.Task{}).Pluck("id", &taskIDs).Error; err != nil {
		log.Printf("[cleanup] list tasks: %v", err)
		return
	}
	for _, taskID := range taskIDs {
		if err := c.cleanTask(taskID); err != nil {
			log.Printf("[cleanup] task %d: %v", taskID, err)
		}
	}
}

func (c *Cleaner) cleanTask(taskID uint) error {
	// Find completed runs for this task ordered newest first.
	var runs []model.TaskRun
	if err := c.db.
		Where("task_id = ? AND status NOT IN ?", taskID, []string{model.TaskRunPending, model.TaskRunRunning}).
		Order("id DESC").
		Find(&runs).Error; err != nil {
		return err
	}
	if len(runs) <= c.retainCount {
		return nil
	}
	toDelete := runs[c.retainCount:]
	for _, run := range toDelete {
		if err := c.deleteRun(run); err != nil {
			log.Printf("[cleanup] delete run %d: %v", run.ID, err)
		}
	}
	return nil
}

func (c *Cleaner) deleteRun(run model.TaskRun) error {
	// Delete workspace directory.
	wsDir := filepath.Join(c.workspaceDir, fmt.Sprintf("taskrun-%d", run.ID))
	if err := os.RemoveAll(wsDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove workspace: %w", err)
	}

	// Delete logs and node runs in a single transaction.
	return c.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_run_id = ?", run.ID).Delete(&model.RunLog{}).Error; err != nil {
			return err
		}
		if err := tx.Where("task_run_id = ?", run.ID).Delete(&model.NodeRun{}).Error; err != nil {
			return err
		}
		return tx.Delete(&run).Error
	})
}
