package observability

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds configuration parameters for OpenTelemetry initialization.
type Config struct {
	ServiceName string
	Endpoint    string
	Headers     string
}

// Providers bundles initialized OpenTelemetry providers and instances.
type Providers struct {
	Tracer         trace.Tracer
	Meter          metric.Meter
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	LoggerProvider *sdklog.LoggerProvider
	Shutdown       func(context.Context) error
}

// InitOTel initializes OpenTelemetry tracing, metrics, and log providers.
// If endpoint is empty, exporters fall back safely to in-memory/no-op implementations for local development.
func InitOTel(ctx context.Context, cfg Config) (*Providers, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "cloudvitta"
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("observability: create resource: %w", err)
	}

	// 1. Tracer Provider Setup
	var tp *sdktrace.TracerProvider
	if cfg.Endpoint != "" {
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpointURL(cfg.Endpoint),
		}
		traceExporter, err := otlptracehttp.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("observability: create OTLP trace exporter: %w", err)
		}
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExporter),
			sdktrace.WithResource(res),
		)
	} else {
		tp = sdktrace.NewTracerProvider(sdktrace.WithResource(res))
	}

	// 2. Meter Provider & Prometheus Exporter Setup
	promExporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("observability: create prometheus exporter: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(promExporter),
		sdkmetric.WithResource(res),
	)

	// 3. Logger Provider Setup
	var lp *sdklog.LoggerProvider
	if cfg.Endpoint != "" {
		logExporter, err := otlploghttp.New(ctx, otlploghttp.WithEndpointURL(cfg.Endpoint))
		if err != nil {
			return nil, fmt.Errorf("observability: create OTLP log exporter: %w", err)
		}
		lp = sdklog.NewLoggerProvider(
			sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
			sdklog.WithResource(res),
		)
	} else {
		lp = sdklog.NewLoggerProvider(sdklog.WithResource(res))
	}

	tracer := tp.Tracer(cfg.ServiceName)
	meter := mp.Meter(cfg.ServiceName)

	shutdown := func(shutdownCtx context.Context) error {
		var errs []error
		if err := tp.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("trace shutdown: %w", err))
		}
		if err := mp.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("metric shutdown: %w", err))
		}
		if err := lp.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("log shutdown: %w", err))
		}
		return errors.Join(errs...)
	}

	return &Providers{
		Tracer:         tracer,
		Meter:          meter,
		TracerProvider: tp,
		MeterProvider:  mp,
		LoggerProvider: lp,
		Shutdown:       shutdown,
	}, nil
}
