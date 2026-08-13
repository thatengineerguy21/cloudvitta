package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	gcsstorage "cloud.google.com/go/storage"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/aws"
	"github.com/thatengineerguy21/CloudVitta/internal/config"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
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
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	// --- Storage ---
	var rawStorage storage.RawStorage
	if cfg.Primary.Environment == "production" || os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" {
		gcsClient, err := gcsstorage.NewClient(ctx)
		if err != nil {
			slog.Error("failed to create GCS client, falling back to memory storage", "error", err)
			rawStorage = storage.NewMemoryRawStorage()
		} else {
			defer func() { _ = gcsClient.Close() }()
			rawStorage = storage.NewGCSStorage(gcsClient, cfg.Storage.GCSBucketName)
		}
	} else {
		slog.Info("using in-memory raw storage for local development")
		rawStorage = storage.NewMemoryRawStorage()
	}

	// --- Dependencies & Wiring ---
	queries := store.New(dbPool)
	awsClient := aws.NewClient()
	awsAdapter := aws.NewAdapter(awsClient, rawStorage)
	ingestSvc := service.NewIngestionService(queries, awsAdapter)

	// --- Execution ---
	slog.Info("executing AWS compute pricing ingestion...")
	count, err := ingestSvc.RunAWSComputeIngestion(ctx)
	if err != nil {
		slog.Error("AWS compute ingestion failed", "error", err)
		os.Exit(1)
	}

	slog.Info("ingestion job completed successfully", "inserted_count", count)
}
