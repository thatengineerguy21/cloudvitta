package gcp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

// Adapter handles fetching, persisting raw response to GCS, and normalizing GCP compute pricing.
type Adapter struct {
	client       *Client
	storage      storage.RawStorage
	tracer       trace.Tracer
	meter        metric.Meter
	fetchCounter metric.Int64Counter
}

// NewAdapter constructs a new GCP provider adapter.
func NewAdapter(client *Client, st storage.RawStorage) *Adapter {
	meter := otel.Meter("cloudvitta.adapter.gcp")
	fetchCounter, _ := meter.Int64Counter("gcp.fetch.count", metric.WithDescription("Number of GCP fetch operations"))
	return &Adapter{
		client:       client,
		storage:      st,
		tracer:       otel.Tracer("cloudvitta.adapter.gcp"),
		meter:        meter,
		fetchCounter: fetchCounter,
	}
}

// Fetch retrieves the GCP price list, concurrently streams raw JSON to storage and normalizes it.
// Returns a domain.FetchResult containing the observations and raw GCS path, and any error.
func (a *Adapter) Fetch(ctx context.Context, limiter *rate.Limiter) (domain.FetchResult, error) {
	ctx, span := a.tracer.Start(ctx, "gcp.fetch")
	defer span.End()

	slog.InfoContext(ctx, "starting gcp fetch")

	if limiter != nil {
		if err := limiter.Wait(ctx); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return domain.FetchResult{}, fmt.Errorf("gcp adapter: rate limit wait: %w", err)
		}
	}

	fetchedAt := time.Now().UTC()
	fetchID := uuid.New().String()
	dateStr := fetchedAt.Format("2006-01-02")
	gcsPath := fmt.Sprintf("raw/gcp/compute/%s/%s.json", dateStr, fetchID)

	body, err := a.client.FetchPriceList(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error")))
		return domain.FetchResult{}, fmt.Errorf("gcp adapter: fetch price list: %w", err)
	}
	defer func() { _ = body.Close() }()

	pr, pw := io.Pipe()
	tee := io.TeeReader(body, provider.IgnoreErrorWriter{W: pw})

	var observations []domain.PriceObservation
	g, _ := errgroup.WithContext(ctx)

	// Goroutine 1: Normalize reads from pr
	g.Go(func() error {
		var normErr error
		observations, normErr = Normalize(pr, fetchedAt)
		if normErr != nil {
			_ = pr.CloseWithError(normErr)
			return fmt.Errorf("gcp adapter: normalize: %w", normErr)
		}
		_ = pr.Close()
		return nil
	})

	// Goroutine 2: GCS reads from tee
	g.Go(func() error {
		defer func() { _ = pw.Close() }()
		if err := a.storage.WriteStream(ctx, gcsPath, tee); err != nil {
			_ = pw.CloseWithError(err)
			return fmt.Errorf("gcp adapter: storage write: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error")))
		slog.ErrorContext(ctx, "gcp fetch failed during stream processing", "error", err)
		return domain.FetchResult{}, err
	}

	span.SetStatus(codes.Ok, "")
	span.SetAttributes(attribute.Int("observations.count", len(observations)))
	a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "success")))
	slog.InfoContext(ctx, "gcp fetch completed", "observations", len(observations))

	return domain.FetchResult{
		Observations: observations,
		RawGCSPath:   gcsPath,
	}, nil
}
