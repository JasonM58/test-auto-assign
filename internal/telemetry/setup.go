package telemetry

import (
	"context"
	"log"

	"github.com/ionextai/git-beacon/pkg/config"
	"github.com/ionextai/git-beacon/pkg/telemetry"
)

func Setup(ctx context.Context, cfg *config.Loader) func(context.Context) error {
	shutdown, err := telemetry.Init(ctx, telemetry.Config{
		ServiceName:          "git-beacon",
		ServiceVer:           "1.0.0",
		Endpoint:             cfg.GetOTLPEndpoint(),
		Insecure:             cfg.GetOTLPInsecure(),
		Environment:          cfg.GetEnvironment(),
		MetricExportInterval: cfg.GetTelemetryMetricExportInterval(),
		Enabled:              cfg.GetTelemetryEnabled(),
	})
	if err != nil {
		log.Fatalf("❌ Failed to initialize telemetry: %v", err)
	}
	return shutdown
}
