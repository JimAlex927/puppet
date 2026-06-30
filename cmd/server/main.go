package main

import (
	"context"
	"log"
	"net/http"

	embeddedFrontend "puppet/frontend"
	"puppet/internal/api"
	"puppet/internal/cleanup"
	"puppet/internal/config"
	"puppet/internal/confignode"
	"puppet/internal/confignodes/gitbranches"
	"puppet/internal/confignodes/script"
	"puppet/internal/db"
	"puppet/internal/engine"
	"puppet/internal/logstream"
	"puppet/internal/node"
	archivenode "puppet/internal/nodes/archive"
	filenode "puppet/internal/nodes/file"
	"puppet/internal/nodes/git"
	httpnode "puppet/internal/nodes/http"
	processnode "puppet/internal/nodes/process"
	"puppet/internal/nodes/shell"
	"puppet/internal/nodes/sleep"
	"puppet/internal/pluginhost"
	"puppet/internal/scheduler"
	"puppet/internal/web"
)

func main() {
	cfg := config.Load()

	database, err := db.Open(cfg)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	registry := node.NewRegistry()
	registry.Register(shell.New())
	registry.Register(sleep.New())
	registry.Register(httpnode.New())
	registry.Register(git.New())
	registry.Register(processnode.NewStart())
	registry.Register(processnode.NewStop())
	registry.Register(archivenode.NewCompress())
	registry.Register(archivenode.NewExtract())
	registry.Register(filenode.NewDelete())
	plugins := pluginhost.New(cfg.PluginDir)
	defer plugins.Stop()
	if err := plugins.Register(registry); err != nil {
		log.Printf("load plugins: %v", err)
	}
	configRegistry := confignode.NewRegistry()
	configRegistry.Register(gitbranches.New())
	configRegistry.Register(script.New())

	hub := logstream.NewHub()
	runner := engine.New(database, registry, hub, cfg)

	cleaner := cleanup.New(database, cfg.WorkspaceDir, cfg.RetainRunsPerTask)
	cleaner.Start(context.Background())
	scheduler.New(database, runner).Start(context.Background())

	router := api.NewRouter(database, registry, configRegistry, runner, hub, cfg)
	go func() {
		log.Printf("puppet api listening on %s", cfg.HTTPAddr)
		if err := router.Run(cfg.HTTPAddr); err != nil {
			log.Fatalf("run api server: %v", err)
		}
	}()

	dist, err := embeddedFrontend.Dist()
	if err != nil {
		log.Fatalf("load embedded frontend: %v", err)
	}
	frontendHandler, err := web.NewHandler(dist, cfg.ServerURL)
	if err != nil {
		log.Fatalf("create frontend server: %v", err)
	}
	log.Printf("puppet frontend listening on %s, api proxy target %s", cfg.FrontendAddr, cfg.ServerURL)
	if err := http.ListenAndServe(cfg.FrontendAddr, frontendHandler); err != nil {
		log.Fatalf("run frontend server: %v", err)
	}
}
