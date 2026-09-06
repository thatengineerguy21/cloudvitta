package dlq_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/thatengineerguy21/CloudVitta/internal/dlq"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func setupTest(t *testing.T) (*dlq.DLQ, *miniredis.Miniredis, func()) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	d := dlq.New(client)
	return d, mr, func() {
		_ = client.Close()
		mr.Close()
	}
}

func TestRecord_CreatesNewEntry(t *testing.T) {
	d, _, teardown := setupTest(t)
	defer teardown()

	ctx := context.Background()
	err := d.Record(ctx, "aws", "metrics", errors.New("timeout"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry, err := d.Get(ctx, "aws", "metrics")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if entry.ConsecutiveFailures != 1 {
		t.Errorf("expected 1 consecutive failure, got %d", entry.ConsecutiveFailures)
	}
	if entry.Status != "failed" {
		t.Errorf("expected status 'failed', got %s", entry.Status)
	}
	if entry.LastError != "timeout" {
		t.Errorf("expected error 'timeout', got %s", entry.LastError)
	}
}

func TestRecord_IncrementsOnSubsequentFailure(t *testing.T) {
	d, _, teardown := setupTest(t)
	defer teardown()

	ctx := context.Background()
	if err := d.Record(ctx, "aws", "metrics", errors.New("timeout")); err != nil {
		t.Fatalf("first record failed: %v", err)
	}
	time.Sleep(1 * time.Millisecond)
	if err := d.Record(ctx, "aws", "metrics", errors.New("connection reset")); err != nil {
		t.Fatalf("second record failed: %v", err)
	}

	entry, err := d.Get(ctx, "aws", "metrics")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if entry.ConsecutiveFailures != 2 {
		t.Errorf("expected 2 consecutive failures, got %d", entry.ConsecutiveFailures)
	}
	if entry.LastError != "connection reset" {
		t.Errorf("expected error 'connection reset', got %s", entry.LastError)
	}
}

func TestRecord_MarksBlockedForAuthErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		errStr string
	}{
		{"http 401 unauthorized"},
		{"API returned 403 Forbidden"},
		{"authentication failed for user"},
		{"authorization denied"},
		{"invalid credentials provided"},
		{"bad config"},
	}

	for _, tt := range tests {
		t.Run(tt.errStr, func(t *testing.T) {
			mr := miniredis.RunT(t)
			defer mr.Close()
			client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			defer func() { _ = client.Close() }()
			d2 := dlq.New(client)

			err := d2.Record(ctx, "aws", "metrics", errors.New(tt.errStr))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			entry, err := d2.Get(ctx, "aws", "metrics")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if entry.Status != "blocked" {
				t.Errorf("expected status 'blocked' for error %q, got %s", tt.errStr, entry.Status)
			}
		})
	}
}

func TestRecord_MarksFailedForTransientErrors(t *testing.T) {
	d, _, teardown := setupTest(t)
	defer teardown()

	ctx := context.Background()
	err := d.Record(ctx, "gcp", "logs", errors.New("i/o timeout"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry, err := d.Get(ctx, "gcp", "logs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if entry.Status != "failed" {
		t.Errorf("expected status 'failed', got %s", entry.Status)
	}
}

func TestClear_RemovesEntry(t *testing.T) {
	d, _, teardown := setupTest(t)
	defer teardown()

	ctx := context.Background()
	if err := d.Record(ctx, "aws", "metrics", errors.New("error")); err != nil {
		t.Fatalf("record failed: %v", err)
	}

	err := d.Clear(ctx, "aws", "metrics")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = d.Get(ctx, "aws", "metrics")
	if !errors.Is(err, dlq.ErrEntryNotFound) {
		t.Errorf("expected ErrEntryNotFound, got %v", err)
	}
}

func TestClear_NoopIfNotExists(t *testing.T) {
	d, _, teardown := setupTest(t)
	defer teardown()

	ctx := context.Background()
	err := d.Clear(ctx, "aws", "metrics")
	if err != nil {
		t.Errorf("expected nil error on clear missing, got %v", err)
	}
}

func TestGet_ReturnsEntryNotFoundForMissing(t *testing.T) {
	d, _, teardown := setupTest(t)
	defer teardown()

	ctx := context.Background()
	_, err := d.Get(ctx, "aws", "metrics")
	if !errors.Is(err, dlq.ErrEntryNotFound) {
		t.Errorf("expected ErrEntryNotFound, got %v", err)
	}
}

func TestDLQ_Tracing(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() {
		_ = client.Close()
		mr.Close()
	}()

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := tp.Tracer("test-dlq")
	d := dlq.New(client, dlq.WithTracer(tracer))

	ctx := context.Background()

	// 1. Test Record creates dlq.record span with dlq.status
	err := d.Record(ctx, "aws", "compute", errors.New("timeout"))
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	spans := exporter.GetSpans()
	spanNames := make(map[string]bool)
	for _, s := range spans {
		spanNames[s.Name] = true
	}

	if !spanNames["dlq.record"] {
		t.Errorf("expected dlq.record span, got spans: %v", spans)
	}
	if spanNames["dlq.get"] {
		t.Errorf("did not expect nested dlq.get span during Record")
	}

	// 2. Test Get creates dlq.get span
	_, err = d.Get(ctx, "aws", "compute")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	spans = exporter.GetSpans()
	spanNames = make(map[string]bool)
	for _, s := range spans {
		spanNames[s.Name] = true
	}
	if !spanNames["dlq.get"] {
		t.Errorf("expected dlq.get span")
	}

	// 3. Test Clear creates dlq.clear span
	err = d.Clear(ctx, "aws", "compute")
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	spans = exporter.GetSpans()
	foundClear := false
	for _, s := range spans {
		if s.Name == "dlq.clear" {
			foundClear = true
			break
		}
	}
	if !foundClear {
		t.Errorf("expected dlq.clear span")
	}
}

func TestDLQ_Metrics(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() {
		_ = client.Close()
		mr.Close()
	}()

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() { _ = mp.Shutdown(context.Background()) }()

	meter := mp.Meter("test-dlq")
	d := dlq.New(client, dlq.WithMeter(meter))

	ctx := context.Background()

	// Perform Record, Get, Clear operations
	if err := d.Record(ctx, "aws", "compute", errors.New("timeout")); err != nil {
		t.Fatalf("Record failed: %v", err)
	}
	if _, err := d.Get(ctx, "aws", "compute"); err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if err := d.Clear(ctx, "aws", "compute"); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	foundOpsMetric := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "dlq_operations_total" {
				foundOpsMetric = true
				break
			}
		}
	}

	if !foundOpsMetric {
		t.Errorf("expected dlq_operations_total metric to be recorded")
	}
}
