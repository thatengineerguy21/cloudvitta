package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	gcsstorage "cloud.google.com/go/storage"
	"github.com/thatengineerguy21/CloudVitta/internal/config"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
)

func main() {
	prefix := flag.String("prefix", "quarantine/", "Storage prefix to scan for quarantined jsonl files")
	format := flag.String("format", "markdown", "Output format: 'markdown', 'snippets', or 'json'")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		slog.Warn("could not load env config, falling back to local/memory storage", "error", err)
	}

	var rawStorage storage.RawStorage
	if cfg != nil && cfg.Storage.GCSBucketName != "" {
		gcsClient, err := gcsstorage.NewClient(ctx)
		if err != nil {
			slog.Error("failed to create GCS client", "error", err)
			os.Exit(1)
		}
		defer func() { _ = gcsClient.Close() }()
		rawStorage = storage.NewGCSStorage(gcsClient, cfg.Storage.GCSBucketName)
	} else {
		rawStorage = storage.NewMemoryRawStorage()
	}

	report, err := quarantine.GenerateDigestFromStorage(ctx, rawStorage, *prefix)
	if err != nil {
		slog.Error("failed to generate quarantine digest", "error", err)
		os.Exit(1)
	}

	switch *format {
	case "snippets":
		fmt.Println(report.GoSnippets())
	default:
		fmt.Println(report.Markdown())
	}
}
