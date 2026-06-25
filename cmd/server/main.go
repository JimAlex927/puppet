package main

import (
	"log"

	"puppet/internal/api"
	"puppet/internal/config"
	"puppet/internal/confignode"
	"puppet/internal/confignodes/gitbranches"
	"puppet/internal/confignodes/script"
	"puppet/internal/db"
	"puppet/internal/engine"
	"puppet/internal/logstream"
	"puppet/internal/node"
	"puppet/internal/nodes/git"
	httpnode "puppet/internal/nodes/http"
	"puppet/internal/nodes/shell"
	"puppet/internal/nodes/sleep"
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
	configRegistry := confignode.NewRegistry()
	configRegistry.Register(gitbranches.New())
	configRegistry.Register(script.New())

	hub := logstream.NewHub()
	runner := engine.New(database, registry, hub, cfg)

	router := api.NewRouter(database, registry, configRegistry, runner, hub)
	log.Printf("puppet server listening on %s", cfg.HTTPAddr)
	if err := router.Run(cfg.HTTPAddr); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
