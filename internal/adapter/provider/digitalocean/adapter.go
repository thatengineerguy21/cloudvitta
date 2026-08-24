package digitalocean

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

// AdapterOption defines a configuration option for DigitalOcean Adapter.
type AdapterOption func(*Adapter)

// WithCategory sets the service category for the DigitalOcean adapter.
func WithCategory(category string) AdapterOption {
	return func(a *Adapter) {
		a.category = category
	}
}

// Adapter handles fetching, persisting raw response to GCS, and normalizing DigitalOcean pricing.
type Adapter struct {
	client          *Client
	storage         storage.RawStorage
	category        string
	tracer          trace.Tracer
	meter           metric.Meter
	fetchCounter    metric.Int64Counter
	unmappedCounter metric.Int64Counter
}

// NewAdapter constructs a new DigitalOcean provider adapter.
func NewAdapter(client *Client, st storage.RawStorage, opts ...AdapterOption) *Adapter {
	meter := otel.Meter("cloudvitta.adapter.digitalocean")
	fetchCounter, _ := meter.Int64Counter("digitalocean.fetch.count", metric.WithDescription("Number of DigitalOcean fetch operations"))
	unmappedCounter, _ := meter.Int64Counter("digitalocean.normalize.unmapped_count", metric.WithDescription("Number of unmapped DigitalOcean taxonomy items"))

	a := &Adapter{
		client:          client,
		storage:         st,
		category:        "compute",
		tracer:          otel.Tracer("cloudvitta.adapter.digitalocean"),
		meter:           meter,
		fetchCounter:    fetchCounter,
		unmappedCounter: unmappedCounter,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Fetch retrieves the DigitalOcean Droplet sizes, concurrently streams raw JSON to storage and normalizes it.
// Returns a domain.FetchResult containing the observations and raw GCS path, and any error.
func (a *Adapter) Fetch(ctx context.Context, limiter *rate.Limiter) (domain.FetchResult, error) {
	ctx, span := a.tracer.Start(ctx, "digitalocean.fetch")
	defer span.End()

	slog.DebugContext(ctx, "starting digitalocean fetch")

	if limiter != nil {
		if err := limiter.Wait(ctx); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return domain.FetchResult{}, fmt.Errorf("digitalocean adapter: rate limit wait: %w", err)
		}
	}

	fetchedAt := time.Now().UTC()
	fetchID := uuid.New().String()
	dateStr := fetchedAt.Format("2006-01-02")
	category := a.category
	if category == "" {
		category = "compute"
	}
	gcsPath := fmt.Sprintf("raw/digitalocean/%s/%s/%s.json", category, dateStr, fetchID)

	body, err := a.client.FetchPriceList(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if a.fetchCounter != nil {
			a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error"), attribute.String("category", category)))
		}
		return domain.FetchResult{}, fmt.Errorf("digitalocean adapter: fetch price list: %w", err)
	}
	defer func() { _ = body.Close() }()

	pr, pw := io.Pipe()
	tee := io.TeeReader(body, provider.IgnoreErrorWriter{W: pw})

	qSink := quarantine.NewStorageSink(a.storage, "digitalocean", category, fetchID, fetchedAt)
	var observations []domain.PriceObservation
	g, _ := errgroup.WithContext(ctx)

	// Goroutine 1: Normalize reads from pr
	g.Go(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("digitalocean adapter: panic in normalize goroutine: %v", r)
				_ = pr.CloseWithError(err)
			}
		}()
		var normErr error
		observations, normErr = Normalize(pr, fetchedAt, qSink)
		if normErr != nil {
			_ = pr.CloseWithError(normErr)
			return fmt.Errorf("digitalocean adapter: normalize: %w", normErr)
		}
		_ = pr.Close()
		return nil
	})

	// Goroutine 2: GCS reads from tee
	g.Go(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("digitalocean adapter: panic in storage write goroutine: %v", r)
				_ = pw.CloseWithError(err)
			}
		}()
		defer func() { _ = pw.Close() }()
		if err := a.storage.WriteStream(ctx, gcsPath, tee); err != nil {
			_ = pw.CloseWithError(err)
			return fmt.Errorf("digitalocean adapter: storage write: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if a.fetchCounter != nil {
			a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error"), attribute.String("category", category)))
		}
		slog.ErrorContext(ctx, "digitalocean fetch failed during stream processing", "error", err)
		return domain.FetchResult{}, err
	}

	// Flush quarantined unmapped items to storage
	if qSink.Count() > 0 {
		_ = qSink.Flush(ctx)
		if a.unmappedCounter != nil {
			a.unmappedCounter.Add(ctx, int64(qSink.Count()), metric.WithAttributes(
				attribute.String("provider", "digitalocean"),
				attribute.String("category", category),
			))
		}
	}

	span.SetStatus(codes.Ok, "")
	span.SetAttributes(attribute.Int("observations.count", len(observations)))
	if a.fetchCounter != nil {
		a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "success"), attribute.String("category", category)))
	}
	slog.InfoContext(ctx, "digitalocean fetch completed", "observations", len(observations), "unmapped", qSink.Count())

	return domain.FetchResult{
		Observations:  observations,
		RawGCSPath:    gcsPath,
		UnmappedCount: qSink.Count(),
	}, nil
}
