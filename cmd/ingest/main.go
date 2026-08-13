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
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/aws"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/config"
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

	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.Primary.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	slog.Info("starting ingestion job runner...", "environment", cfg.Primary.Environment)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

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

	// --- Cache ---
	var redisClient *redis.Client
	if cfg.Redis.URL != "" {
		rc, err := cache.NewClient(cfg.Redis.URL)
		if err != nil {
			safeURL := "<invalid-url>"
			if parsed, pErr := url.Parse(cfg.Redis.URL); pErr == nil {
				parsed.User = nil
				safeURL = parsed.String()
			}
			slog.Warn("failed to connect to redis, proceeding without cache warming", "url", safeURL, "error", err)
		} else {
			defer func() { _ = rc.Close() }()
			redisClient = rc
		}
	}

	// --- Dependencies & Wiring ---
	queries := store.New(dbPool)
	awsClient := aws.NewClient()
	awsAdapter := aws.NewAdapter(awsClient, rawStorage)
	ingestSvc := service.NewIngestionService(queries, awsAdapter, redisClient)

	// --- Execution ---
	slog.Info("executing AWS compute pricing ingestion...")
	count, err := ingestSvc.RunAWSComputeIngestion(ctx)
	if err != nil {
		return fmt.Errorf("aws compute ingestion failed: %w", err)
	}

	slog.Info("ingestion job completed successfully", "inserted_count", count)
	return nil
}
