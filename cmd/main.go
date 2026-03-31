package main

import (
	"context"
	"flag"
	"log"

	"github.com/ionextai/git-beacon/internal/app"
	"github.com/ionextai/git-beacon/pkg/config"
)

func main() {
	cfgPath := flag.String("config", "env.yaml", "Path to configuration file")
	mode := flag.String("mode", "autoassign", "Mode: autoassign | metrics")
	flag.Parse()

	if *cfgPath == "" {
		log.Fatal("❌ Config file path is required")
	}

	cfg := config.NewLoader(*cfgPath)
	ctx := context.Background()

	app.Run(ctx, cfg, *mode)
}
