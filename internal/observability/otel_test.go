package observability_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/observability"
)

func TestInitOTel_LocalFallback(t *testing.T) {
	ctx := context.Background()

	providers, err := observability.InitOTel(ctx, observability.Config{
		ServiceName: "cloudvitta-test",
		Endpoint:    "", // empty endpoint triggers local fallback
	})
	if err != nil {
		t.Fatalf("InitOTel failed: %v", err)
	}
	if providers == nil {
		t.Fatal("expected non-nil providers")
	}

	if providers.Tracer == nil {
		t.Error("expected non-nil Tracer")
	}
	if providers.Meter == nil {
		t.Error("expected non-nil Meter")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := providers.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}
}

func TestSetupLogger(t *testing.T) {
	ctx := context.Background()
	providers, err := observability.InitOTel(ctx, observability.Config{ServiceName: "test"})
	if err != nil {
		t.Fatalf("InitOTel failed: %v", err)
	}
	defer func() { _ = providers.Shutdown(ctx) }()

	var buf bytes.Buffer
	logger := observability.SetupLogger(slog.LevelInfo, providers.LoggerProvider, &buf)

	logger.Info("test log message", "key", "value")

	output := buf.String()
	if output == "" {
		t.Error("expected log output in buffer")
	}
}

func TestCacheMetrics(t *testing.T) {
	ctx := context.Background()
	providers, err := observability.InitOTel(ctx, observability.Config{ServiceName: "test"})
	if err != nil {
		t.Fatalf("InitOTel failed: %v", err)
	}
	defer func() { _ = providers.Shutdown(ctx) }()

	cacheMetrics, err := observability.NewCacheMetrics(providers.Meter)
	if err != nil {
		t.Fatalf("NewCacheMetrics failed: %v", err)
	}

	// Record hits and misses without panic
	cacheMetrics.RecordHit(ctx)
	cacheMetrics.RecordMiss(ctx)
}
