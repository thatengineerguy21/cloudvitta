package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
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

// InitOTel initializes OpenTelemetry tracing, dual metrics (Prometheus local + OTLP remote), and log providers.
// If endpoint is empty, OTLP exporters fall back safely to in-memory/no-op implementations for local development.
func InitOTel(ctx context.Context, cfg Config) (*Providers, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "cloudvitta"
	}

	// Set global OpenTelemetry error handler so OTLP background export failures (e.g. auth errors) get logged
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		slog.Error("opentelemetry runtime error", "error", err)
	}))

	headers := parseHeaders(cfg.Headers)

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
		traceEndpoint := strings.TrimRight(cfg.Endpoint, "/")
		if !strings.HasSuffix(traceEndpoint, "/v1/traces") {
			traceEndpoint += "/v1/traces"
		}
		traceOpts := []otlptracehttp.Option{
			otlptracehttp.WithEndpointURL(traceEndpoint),
			otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
		}
		if len(headers) > 0 {
			traceOpts = append(traceOpts, otlptracehttp.WithHeaders(headers))
		}
		traceExporter, err := otlptracehttp.New(ctx, traceOpts...)
		if err != nil {
			return nil, fmt.Errorf("observability: create OTLP trace exporter: %w", err)
		}
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(
				traceExporter,
				sdktrace.WithMaxExportBatchSize(512),
				sdktrace.WithMaxQueueSize(2048),
			),
			sdktrace.WithResource(res),
		)
	} else {
		tp = sdktrace.NewTracerProvider(sdktrace.WithResource(res))
	}

	// 2. Dual Meter Provider Setup (Prometheus for local /metrics + OTLP PeriodicReader when endpoint is configured)
	promExporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("observability: create prometheus exporter: %w", err)
	}

	meterReaders := []sdkmetric.Reader{promExporter}
	if cfg.Endpoint != "" {
		metricEndpoint := strings.TrimRight(cfg.Endpoint, "/")
		if !strings.HasSuffix(metricEndpoint, "/v1/metrics") {
			metricEndpoint += "/v1/metrics"
		}
		metricOpts := []otlpmetrichttp.Option{
			otlpmetrichttp.WithEndpointURL(metricEndpoint),
			otlpmetrichttp.WithCompression(otlpmetrichttp.GzipCompression),
		}
		if len(headers) > 0 {
			metricOpts = append(metricOpts, otlpmetrichttp.WithHeaders(headers))
		}
		otlpMetricExporter, err := otlpmetrichttp.New(ctx, metricOpts...)
		if err != nil {
			return nil, fmt.Errorf("observability: create OTLP metric exporter: %w", err)
		}
		meterReaders = append(meterReaders, sdkmetric.NewPeriodicReader(otlpMetricExporter))
	}

	meterOpts := make([]sdkmetric.Option, 0, len(meterReaders)+1)
	for _, r := range meterReaders {
		meterOpts = append(meterOpts, sdkmetric.WithReader(r))
	}
	meterOpts = append(meterOpts, sdkmetric.WithResource(res))
	mp := sdkmetric.NewMeterProvider(meterOpts...)

	// 3. Logger Provider Setup
	var lp *sdklog.LoggerProvider
	if cfg.Endpoint != "" {
		logEndpoint := strings.TrimRight(cfg.Endpoint, "/")
		if !strings.HasSuffix(logEndpoint, "/v1/logs") {
			logEndpoint += "/v1/logs"
		}
		logOpts := []otlploghttp.Option{
			otlploghttp.WithEndpointURL(logEndpoint),
			otlploghttp.WithCompression(otlploghttp.GzipCompression),
		}
		if len(headers) > 0 {
			logOpts = append(logOpts, otlploghttp.WithHeaders(headers))
		}
		logExporter, err := otlploghttp.New(ctx, logOpts...)
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

	// Set global providers so instrumentation libraries like otelhttp can use them
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)

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

func parseHeaders(raw string) map[string]string {
	headers := make(map[string]string)
	if raw == "" {
		return headers
	}
	pairs := strings.Split(raw, ",")
	for _, pair := range pairs {
		k, v, found := strings.Cut(strings.TrimSpace(pair), "=")
		if found {
			headers[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return headers
}
