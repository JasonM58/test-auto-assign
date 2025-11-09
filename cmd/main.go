package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/ionextai/git-beacon/internal/app"
	"github.com/ionextai/git-beacon/pkg/config"
)

func main() {
	fmt.Println("ini terbaru 2")
	cfgPath := flag.String("config", "env.yaml", "Path to configuration file")
	flag.Parse()

	if *cfgPath == "" {
		log.Fatal("❌ Config file path is required")
	}

	cfg := config.NewLoader(*cfgPath)
	ctx := context.Background()

	app.Run(ctx, cfg)
}
