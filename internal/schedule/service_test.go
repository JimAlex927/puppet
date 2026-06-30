package schedule

import (
	"testing"
	"time"

	"puppet/internal/model"
)

func TestNormalizeScheduleComputesNextRun(t *testing.T) {
	from := time.Date(2026, 6, 29, 10, 3, 0, 0, time.UTC)
	svc := &Service{}
	item := model.TaskSchedule{
		TaskID:         1,
		ProjectID:      1,
		Name:           "test",
		CronExpression: "*/5 * * * *",
		CronTimezone:   "UTC",
		Enabled:        true,
	}

	if err := svc.normalizeNoTaskLookup(&item, from); err != nil {
		t.Fatalf("normalize returned error: %v", err)
	}
	if item.NextRunAt == nil {
		t.Fatal("NextRunAt was nil")
	}
	want := time.Date(2026, 6, 29, 10, 5, 0, 0, time.UTC)
	if !item.NextRunAt.Equal(want) {
		t.Fatalf("next run mismatch: got %s, want %s", item.NextRunAt, want)
	}
}

func TestNormalizeScheduleRejectsInvalidExpression(t *testing.T) {
	svc := &Service{}
	item := model.TaskSchedule{
		TaskID:         1,
		ProjectID:      1,
		Name:           "test",
		CronExpression: "bad cron",
		CronTimezone:   "UTC",
		Enabled:        true,
	}
	if err := svc.normalizeNoTaskLookup(&item, time.Now()); err == nil {
		t.Fatal("expected invalid cron expression to fail")
	}
}

func TestNormalizeScheduleDisabledClearsNextRun(t *testing.T) {
	next := time.Now()
	svc := &Service{}
	item := model.TaskSchedule{
		TaskID:         1,
		ProjectID:      1,
		Name:           "test",
		CronExpression: "*/5 * * * *",
		CronTimezone:   "UTC",
		Enabled:        false,
		NextRunAt:      &next,
	}
	if err := svc.normalizeNoTaskLookup(&item, time.Now()); err != nil {
		t.Fatalf("normalize returned error: %v", err)
	}
	if item.NextRunAt != nil {
		t.Fatalf("expected NextRunAt to be cleared, got %s", item.NextRunAt)
	}
}
