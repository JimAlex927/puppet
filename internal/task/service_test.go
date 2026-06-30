package task

import (
	"testing"

	"puppet/internal/model"
	"puppet/internal/node"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestPipelineVersionsSaveAndRestore(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Task{}, &model.PipelineVersion{}); err != nil {
		t.Fatal(err)
	}
	service := NewService(db)

	created, err := service.Create(model.Task{Name: "Deploy", ProjectID: 1})
	if err != nil {
		t.Fatal(err)
	}
	versions, err := service.PipelineVersions(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 || versions[0].Version != 1 || versions[0].Message != "initial" {
		t.Fatalf("unexpected initial versions: %#v", versions)
	}

	pipeline := node.PipelineDefinition{
		Name:          "Deploy v2",
		AgentSelector: node.AgentSelector{Labels: []string{"local"}},
		StartNodeID:   "shell-1",
		Nodes: []node.PipelineNode{{
			ID:     "shell-1",
			Name:   "Shell",
			Type:   "shell",
			Params: map[string]any{"script": "echo v2"},
		}},
	}
	if _, err := service.UpdatePipeline(created.ID, pipeline, "alice"); err != nil {
		t.Fatal(err)
	}
	versions, err = service.PipelineVersions(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 || versions[0].Version != 2 || versions[0].CreatedBy != "alice" {
		t.Fatalf("unexpected saved versions: %#v", versions)
	}

	restored, restoreVersion, err := service.RestorePipelineVersion(created.ID, versions[1].ID, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if restored.Name == pipeline.Name {
		t.Fatalf("expected restore to initial pipeline, got %#v", restored)
	}
	if restoreVersion.Version != 3 || restoreVersion.Message != "restore v1" || restoreVersion.CreatedBy != "bob" {
		t.Fatalf("unexpected restore version: %#v", restoreVersion)
	}
}
