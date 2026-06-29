package project

import (
	"testing"

	"puppet/internal/model"
	"puppet/internal/task"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestProjectArchiveRoundTrip(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Project{}, &model.Task{}); err != nil {
		t.Fatal(err)
	}

	service := NewService(db)
	project, err := service.Create(model.Project{Name: "Release Flow", Description: "Deploy pipeline"})
	if err != nil {
		t.Fatal(err)
	}
	originalTask := model.Task{
		ProjectID:       project.ID,
		Name:            "Deploy",
		Description:     "Ship it",
		PipelineJSON:    task.DefaultPipelineJSON("Deploy"),
		AllowConcurrent: true,
		TimeoutSeconds:  900,
	}
	if err := db.Create(&originalTask).Error; err != nil {
		t.Fatal(err)
	}

	content, filename, err := service.ExportArchive(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if filename != "Release Flow.zip" {
		t.Fatalf("filename = %q, want %q", filename, "Release Flow.zip")
	}

	imported, err := service.ImportArchive(content)
	if err != nil {
		t.Fatal(err)
	}
	if imported.ID == project.ID {
		t.Fatalf("imported project reused source id %d", project.ID)
	}
	if imported.Name != project.Name || imported.Description != project.Description {
		t.Fatalf("imported project = %#v, want name/description from %#v", imported, project)
	}

	var importedTasks []model.Task
	if err := db.Where("project_id = ?", imported.ID).Find(&importedTasks).Error; err != nil {
		t.Fatal(err)
	}
	if len(importedTasks) != 1 {
		t.Fatalf("imported task count = %d, want 1", len(importedTasks))
	}
	if importedTasks[0].Name != originalTask.Name ||
		importedTasks[0].PipelineJSON != originalTask.PipelineJSON ||
		importedTasks[0].AllowConcurrent != originalTask.AllowConcurrent ||
		importedTasks[0].TimeoutSeconds != originalTask.TimeoutSeconds {
		t.Fatalf("imported task = %#v, want %#v", importedTasks[0], originalTask)
	}
}
