package telemetry

import (
	"context"
	"crypto/tls"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"

	"google.golang.org/grpc/credentials"
)

type Config struct {
	ServiceName          string
	ServiceVer           string
	Endpoint             string
	Insecure             bool
	Environment          string
	MetricExportInterval int
	Enabled              bool
}
func Init(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if !cfg.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	// Resource (service identity)
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVer),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}

	mo := []otlpmetricgrpc.Option{}
	if cfg.Endpoint != "" {
		mo = append(mo, otlpmetricgrpc.WithEndpoint(cfg.Endpoint))
	}
	if cfg.Insecure {
		mo = append(mo, otlpmetricgrpc.WithInsecure())
	} else {
		mo = append(mo, otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(&tls.Config{})))
	}

	me, err := otlpmetricgrpc.New(ctx, mo...)
	if err != nil {
		return nil, err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(me, sdkmetric.WithInterval(time.Duration(cfg.MetricExportInterval)*time.Second))),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	// Propagation
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return func(ctx context.Context) error {
		return mp.Shutdown(ctx)
	}, nil
}