package config

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr       string
	FrontendAddr   string
	ServerURL      string
	DatabaseDSN    string
	DataDir        string
	WorkspaceDir   string
	SharedFilesDir string

	// Retention: how many completed task runs to keep per task (workspace + logs).
	// 0 means unlimited. Default 30.
	RetainRunsPerTask int
}

func Load() Config {
	dataDir := getenv("PUPPET_DATA_DIR", "data")
	return Config{
		HTTPAddr:          getenv("PUPPET_HTTP_ADDR", ":8080"),
		FrontendAddr:      getenv("PUPPET_FRONTEND_ADDR", ":5173"),
		ServerURL:         getenv("PUPPET_SERVER_URL", "http://localhost:8080"),
		DatabaseDSN:       getenv("PUPPET_DATABASE_DSN", dataDir+"/puppet.db"),
		DataDir:           dataDir,
		WorkspaceDir:      getenv("PUPPET_WORKSPACE_DIR", dataDir+"/workspaces"),
		SharedFilesDir:    getenv("PUPPET_SHARED_FILES_DIR", dataDir+"/shared-files"),
		RetainRunsPerTask: getenvInt("PUPPET_RETAIN_RUNS_PER_TASK", 30),
	}
}

func getenvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
	}
	return fallback
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
