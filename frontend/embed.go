package frontend

import (
	"embed"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

//go:embed all:dist
var dist embed.FS

func Dist() (fs.FS, error) {
	if dir := os.Getenv("PUPPET_FRONTEND_DIST_DIR"); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "index.html")); err == nil {
			log.Printf("frontend dist source: %s", dir)
			return os.DirFS(dir), nil
		}
	}

	for _, dir := range []string{"frontend/dist", "dist"} {
		if _, err := os.Stat(filepath.Join(dir, "index.html")); err == nil {
			log.Printf("frontend dist source: %s", dir)
			return os.DirFS(dir), nil
		}
	}

	log.Print("frontend dist source: embedded dist")
	return fs.Sub(dist, "dist")
}
