package dlq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var (
	ErrEntryNotFound = errors.New("dlq: entry not found")
)

// Entry represents a single DLQ record for a failed ingestion job.
type Entry struct {
	Provider            string    `json:"provider"`
	Category            string    `json:"category"`
	Timestamp           time.Time `json:"timestamp"`
	LastError           string    `json:"last_error"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	Status              string    `json:"status"` // "failed" or "blocked"
}

// Option configures DLQ options.
type Option func(*DLQ)

// WithTracer specifies a custom trace.Tracer for the DLQ.
func WithTracer(t trace.Tracer) Option {
	return func(d *DLQ) {
		if t != nil {
			d.tracer = t
		}
	}
}

// WithMeter specifies a custom metric.Meter for the DLQ.
func WithMeter(m metric.Meter) Option {
	return func(d *DLQ) {
		if m != nil {
			counter, err := m.Int64Counter(
				"dlq_operations_total",
				metric.WithDescription("Total number of DLQ operations"),
				metric.WithUnit("1"),
			)
			if err == nil {
				d.opsCounter = counter
			}
		}
	}
}

// DLQ manages dead-letter queue records for failed ingestion attempts.
type DLQ struct {
	client     redis.Cmdable
	keyPrefix  string // default "ingestion:dlq"
	tracer     trace.Tracer
	opsCounter metric.Int64Counter
}

// New constructs a new DLQ instance with OpenTelemetry instrumentation.
func New(client redis.Cmdable, opts ...Option) *DLQ {
	tracer := otel.Tracer("cloudvitta.dlq")
	meter := otel.Meter("cloudvitta.dlq")
	counter, _ := meter.Int64Counter(
		"dlq_operations_total",
		metric.WithDescription("Total number of DLQ operations"),
		metric.WithUnit("1"),
	)

	d := &DLQ{
		client:     client,
		keyPrefix:  "ingestion:dlq",
		tracer:     tracer,
		opsCounter: counter,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

func (d *DLQ) key(provider, category string) string {
	return fmt.Sprintf("%s:%s:%s", d.keyPrefix, provider, category)
}

// Record persists or increments a DLQ failure entry with tracing, logging, and metrics.
func (d *DLQ) Record(ctx context.Context, provider, category string, jobErr error) (err error) {
	// Classify status
	entryStatus := "failed"
	if jobErr != nil {
		msg := strings.ToLower(jobErr.Error())
		if strings.Contains(msg, "401") || strings.Contains(msg, "403") ||
			strings.Contains(msg, "authentication") || strings.Contains(msg, "authorization") ||
			strings.Contains(msg, "invalid credentials") || strings.Contains(msg, "config") {
			entryStatus = "blocked"
		}
	}

	ctx, span := d.tracer.Start(ctx, "dlq.record",
		trace.WithAttributes(
			attribute.String("dlq.provider", provider),
			attribute.String("dlq.category", category),
			attribute.String("dlq.status", entryStatus),
		),
	)
	defer func() {
		opStatus := "ok"
		if err != nil {
			opStatus = "error"
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		if d.opsCounter != nil {
			d.opsCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("operation", "record"),
				attribute.String("status", opStatus),
			))
		}
		span.End()
	}()

	k := d.key(provider, category)

	entry, err := d.getEntry(ctx, provider, category)
	if err != nil && !errors.Is(err, ErrEntryNotFound) {
		return err
	}

	if errors.Is(err, ErrEntryNotFound) {
		entry = Entry{
			Provider: provider,
			Category: category,
		}
	}

	entry.ConsecutiveFailures++
	entry.Timestamp = time.Now().UTC()
	entry.LastError = jobErr.Error()
	entry.Status = entryStatus

	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	if err = d.client.Set(ctx, k, b, 0).Err(); err != nil {
		return err
	}

	slog.InfoContext(ctx, "dlq entry recorded",
		"provider", provider,
		"category", category,
		"status", entry.Status,
		"consecutive_failures", entry.ConsecutiveFailures,
	)

	return nil
}

// Clear removes a DLQ record when an ingestion job succeeds.
func (d *DLQ) Clear(ctx context.Context, provider, category string) (err error) {
	ctx, span := d.tracer.Start(ctx, "dlq.clear",
		trace.WithAttributes(
			attribute.String("dlq.provider", provider),
			attribute.String("dlq.category", category),
		),
	)
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		if d.opsCounter != nil {
			d.opsCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("operation", "clear"),
				attribute.String("status", status),
			))
		}
		span.End()
	}()

	k := d.key(provider, category)
	if err = d.client.Del(ctx, k).Err(); err != nil {
		return err
	}

	slog.InfoContext(ctx, "dlq entry cleared",
		"provider", provider,
		"category", category,
	)

	return nil
}

// Get fetches the DLQ entry for a provider and category.
func (d *DLQ) Get(ctx context.Context, provider, category string) (entry Entry, err error) {
	ctx, span := d.tracer.Start(ctx, "dlq.get",
		trace.WithAttributes(
			attribute.String("dlq.provider", provider),
			attribute.String("dlq.category", category),
		),
	)
	defer func() {
		status := "ok"
		if err != nil && !errors.Is(err, ErrEntryNotFound) {
			status = "error"
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			slog.WarnContext(ctx, "dlq get failed",
				"provider", provider,
				"category", category,
				"error", err,
			)
		}
		if d.opsCounter != nil {
			d.opsCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("operation", "get"),
				attribute.String("status", status),
			))
		}
		span.End()
	}()

	return d.getEntry(ctx, provider, category)
}

func (d *DLQ) getEntry(ctx context.Context, provider, category string) (Entry, error) {
	k := d.key(provider, category)
	val, err := d.client.Get(ctx, k).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return Entry{}, ErrEntryNotFound
		}
		return Entry{}, err
	}

	var entry Entry
	if err := json.Unmarshal([]byte(val), &entry); err != nil {
		return Entry{}, err
	}

	return entry, nil
}
