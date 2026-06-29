package api

import (
	"testing"

	"puppet/internal/config"
	"puppet/internal/confignode"
	"puppet/internal/logstream"
	"puppet/internal/node"
)

func TestNewRouterRegistersProjectArchiveRoutes(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("NewRouter panicked: %v", recovered)
		}
	}()

	cfg := config.Config{SharedFilesDir: t.TempDir()}
	NewRouter(nil, node.NewRegistry(), confignode.NewRegistry(), nil, logstream.NewHub(), cfg)
}
