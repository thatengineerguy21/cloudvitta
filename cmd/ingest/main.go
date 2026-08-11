package main

import (
	"github.com/thatengineerguy21/CloudVitta/internal/config"
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
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

	_ = ctx
	slog.Info("ingestion job completed successfully")
}
