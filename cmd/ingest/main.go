package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"time"

	gcsstorage "cloud.google.com/go/storage"
	"github.com/redis/go-redis/v9"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/aws"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/azure"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/gcp"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/config"
	"github.com/thatengineerguy21/CloudVitta/internal/dlq"
	"github.com/thatengineerguy21/CloudVitta/internal/observability"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("ingestion job failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	serviceName := "cloudvitta-ingest"

	// --- Observability & OpenTelemetry Setup ---
	otelProviders, err := observability.InitOTel(ctx, observability.Config{
		ServiceName: serviceName,
		Endpoint:    cfg.Observability.OTLPEndpoint,
		Headers:     cfg.Observability.OTLPHeaders,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "OpenTelemetry initialization error: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = otelProviders.Shutdown(shutdownCtx)
	}()

	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.Primary.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	logger := observability.SetupLogger(serviceName, level, otelProviders.LoggerProvider, os.Stdout)
	slog.SetDefault(logger)

	slog.InfoContext(ctx, "starting ingestion job runner...", "environment", cfg.Primary.Environment)

	// --- Database ---
	dbPool, err := store.NewPool(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer dbPool.Close()

	// --- Storage ---
	gcsClient, err := gcsstorage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create GCS client: %w", err)
	}
	defer func() { _ = gcsClient.Close() }()
	rawStorage := storage.NewGCSStorage(gcsClient, cfg.Storage.GCSBucketName)

	// --- Redis ---
	var redisClient redis.Cmdable
	if cfg.Redis.URL != "" {
		rc, err := cache.NewClient(cfg.Redis.URL)
		if err != nil {
			safeURL := "<invalid-url>"
			if parsed, pErr := url.Parse(cfg.Redis.URL); pErr == nil {
				parsed.User = nil
				safeURL = parsed.String()
			}
			slog.WarnContext(ctx, "failed to connect to redis, proceeding without DLQ/Cache", "url", safeURL, "error", err)
		} else {
			defer func() { _ = rc.Close() }()
			redisClient = rc
		}
	}

	// --- Provider Factory ---
	factory := provider.NewFactory()

	// Register AWS compute adapter with rate limiting and retry config
	awsClient := aws.NewClient()
	awsAdapter := aws.NewAdapter(awsClient, rawStorage)
	factory.Register(provider.ProviderConfig{
		Provider:       "aws",
		Category:       "compute",
		RateLimitRPS:   10, // AWS bulk API is generous; 10 req/s is safe
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, awsAdapter)

	// Register Azure compute adapter with rate limiting and retry config
	azureClient := azure.NewClient()
	azureAdapter := azure.NewAdapter(azureClient, rawStorage)
	factory.Register(provider.ProviderConfig{
		Provider:       "azure",
		Category:       "compute",
		RateLimitRPS:   10, // Azure Retail Prices API is public; 10 req/s is safe
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, azureAdapter)

	// Register GCP compute adapter with rate limiting and retry config
	var gcpOpts []gcp.Option
	if cfg.GCP.APIKey != "" {
		gcpOpts = append(gcpOpts, gcp.WithAPIKey(cfg.GCP.APIKey))
	}
	gcpClient := gcp.NewClient(gcpOpts...)
	gcpAdapter := gcp.NewAdapter(gcpClient, rawStorage)
	factory.Register(provider.ProviderConfig{
		Provider:       "gcp",
		Category:       "compute",
		RateLimitRPS:   10, // GCP Cloud Billing API standard rate limit
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, gcpAdapter)

	// --- DLQ ---
	var dlqSvc *dlq.DLQ
	if redisClient != nil {
		dlqSvc = dlq.New(redisClient)
	}

	// --- Orchestrator ---
	queries := store.New(dbPool)
	orchConfig := service.DefaultOrchestratorConfig()
	orchConfig.Tracer = otelProviders.Tracer
	orchConfig.Meter = otelProviders.Meter

	orchestrator := service.NewOrchestrator(
		queries,
		redisClient,
		dlqSvc,
		factory,
		orchConfig,
	)

	// --- Execution ---
	slog.InfoContext(ctx, "starting orchestrated ingestion run...")
	results := orchestrator.RunAll(ctx)

	// Report results
	var hasErrors bool
	for _, r := range results {
		if r.Skipped {
			slog.InfoContext(ctx, "ingestion job skipped (lock held)",
				"provider", r.Provider,
				"category", r.Category,
			)
			continue
		}
		if r.Err != nil {
			slog.ErrorContext(ctx, "ingestion job failed",
				"provider", r.Provider,
				"category", r.Category,
				"error", r.Err,
			)
			hasErrors = true
			continue
		}
		slog.InfoContext(ctx, "ingestion job completed",
			"provider", r.Provider,
			"category", r.Category,
			"inserted", r.InsertedCount,
			"anomalies", r.AnomalyCount,
		)
	}

	if hasErrors {
		return fmt.Errorf("one or more ingestion jobs failed")
	}

	slog.InfoContext(ctx, "all ingestion jobs completed successfully")
	return nil
}
