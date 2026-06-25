package config

import "os"

type Config struct {
	HTTPAddr     string
	ServerURL    string
	DatabaseDSN  string
	DataDir      string
	WorkspaceDir string
}

func Load() Config {
	dataDir := getenv("PUPPET_DATA_DIR", "data")
	return Config{
		HTTPAddr:     getenv("PUPPET_HTTP_ADDR", ":8080"),
		ServerURL:    getenv("PUPPET_SERVER_URL", "http://localhost:8080"),
		DatabaseDSN:  getenv("PUPPET_DATABASE_DSN", dataDir+"/puppet.db"),
		DataDir:      dataDir,
		WorkspaceDir: getenv("PUPPET_WORKSPACE_DIR", dataDir+"/workspaces"),
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
