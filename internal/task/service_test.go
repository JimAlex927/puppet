package task

import (
	"strings"
	"testing"

	"puppet/internal/model"
	"puppet/internal/node"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestPipelineVersionsComeFromTaskRuns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Task{}, &model.TaskRun{}); err != nil {
		t.Fatal(err)
	}
	service := NewService(db)

	created, err := service.Create(model.Task{Name: "Deploy", ProjectID: 1})
	if err != nil {
		t.Fatal(err)
	}
	updatedPipeline := node.PipelineDefinition{
		Name:          "Edited but not run",
		AgentSelector: node.AgentSelector{Labels: []string{"local"}},
		Nodes:         []node.PipelineNode{},
	}
	if _, err := service.UpdatePipeline(created.ID, updatedPipeline, "alice"); err != nil {
		t.Fatal(err)
	}
	versions, err := service.PipelineVersions(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 0 {
		t.Fatalf("saving pipeline should not create versions, got %#v", versions)
	}

	run := model.TaskRun{
		ProjectID:            created.ProjectID,
		TaskID:               created.ID,
		Status:               model.TaskRunSuccess,
		TriggerType:          "manual",
		TriggeredBy:          "alice",
		PipelineSnapshotJSON: created.PipelineJSON,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	versions, err = service.PipelineVersions(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 || versions[0].TaskRunID != run.ID || versions[0].Version != int(run.ID) {
		t.Fatalf("unexpected run versions: %#v", versions)
	}

	copied, err := service.CreateFromPipelineVersion(created.ID, run.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if copied.PipelineJSON != created.PipelineJSON {
		t.Fatalf("copied task should use run snapshot")
	}
	if !strings.Contains(copied.Name, "run #") {
		t.Fatalf("unexpected copied task name: %q", copied.Name)
	}
}
