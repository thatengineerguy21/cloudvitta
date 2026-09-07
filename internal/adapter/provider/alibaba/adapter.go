package alibaba

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

// AdapterOption defines a configuration option for Alibaba Adapter.
type AdapterOption func(*Adapter)

// WithCategory sets the service category for the Alibaba adapter.
func WithCategory(category string) AdapterOption {
	return func(a *Adapter) {
		a.category = category
	}
}

// Adapter handles fetching, persisting raw responses to GCS, and normalizing Alibaba Cloud pricing data.
type Adapter struct {
	client            *Client
	storage           storage.RawStorage
	category          string
	tracer            trace.Tracer
	meter             metric.Meter
	fetchCounter      metric.Int64Counter
	unmappedCounter   metric.Int64Counter
	fetchDurationHist metric.Float64Histogram
	bytesCounter      metric.Int64Counter
}

// NewAdapter constructs a new Alibaba provider adapter.
func NewAdapter(client *Client, st storage.RawStorage, opts ...AdapterOption) *Adapter {
	meter := otel.Meter("cloudvitta.adapter.alibaba")
	fetchCounter, _ := meter.Int64Counter("alibaba.fetch.count", metric.WithDescription("Number of Alibaba fetch operations"))
	unmappedCounter, _ := meter.Int64Counter("alibaba.normalize.unmapped_count", metric.WithDescription("Number of unmapped Alibaba taxonomy items"))
	fetchDurationHist, _ := meter.Float64Histogram(
		"adapter.fetch.duration_seconds",
		metric.WithDescription("Duration of provider fetch operation in seconds"),
		metric.WithUnit("s"),
	)
	bytesCounter, _ := meter.Int64Counter(
		"adapter.fetch.bytes_total",
		metric.WithDescription("Total number of raw response bytes streamed to storage"),
		metric.WithUnit("By"),
	)

	a := &Adapter{
		client:            client,
		storage:           st,
		category:          "",
		tracer:            otel.Tracer("cloudvitta.adapter.alibaba"),
		meter:             meter,
		fetchCounter:      fetchCounter,
		unmappedCounter:   unmappedCounter,
		fetchDurationHist: fetchDurationHist,
		bytesCounter:      bytesCounter,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Fetch retrieves the Alibaba Cloud price list, concurrently streams raw JSON to storage and normalizes it.
// Returns a domain.FetchResult containing the observations, raw GCS path, and any unmapped count.
func (a *Adapter) Fetch(ctx context.Context, limiter *rate.Limiter) (res domain.FetchResult, err error) {
	ctx, span := a.tracer.Start(ctx, "alibaba.fetch")
	defer span.End()

	startTime := time.Now()
	category := a.category
	if category == "" {
		category = "compute"
	}
	defer func() {
		dur := time.Since(startTime).Seconds()
		status := "ok"
		if err != nil {
			status = "error"
		}
		if a.fetchDurationHist != nil {
			a.fetchDurationHist.Record(ctx, dur, metric.WithAttributes(
				attribute.String("provider", "alibaba"),
				attribute.String("category", category),
				attribute.String("status", status),
			))
		}
	}()

	slog.DebugContext(ctx, "starting alibaba fetch")

	if limiter != nil {
		if err = limiter.Wait(ctx); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return domain.FetchResult{}, fmt.Errorf("alibaba adapter: rate limit wait: %w", err)
		}
	}

	fetchedAt := time.Now().UTC()
	fetchID := uuid.New().String()
	dateStr := fetchedAt.Format("2006-01-02")
	gcsPath := fmt.Sprintf("raw/alibaba/%s/%s/%s.json", category, dateStr, fetchID)

	body, err := a.client.FetchPriceList(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if a.fetchCounter != nil {
			a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error"), attribute.String("category", category)))
		}
		return domain.FetchResult{}, fmt.Errorf("alibaba adapter: fetch price list: %w", err)
	}
	defer func() { _ = body.Close() }()

	cr := provider.NewCountingReader(body)
	pr, pw := io.Pipe()
	tee := io.TeeReader(cr, provider.IgnoreErrorWriter{W: pw})

	qSink := quarantine.NewStorageSink(a.storage, "alibaba", category, fetchID, fetchedAt)
	var normResult domain.NormalizationResult
	g, _ := errgroup.WithContext(ctx)

	// Goroutine 1: Normalize reads from pr
	g.Go(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("alibaba adapter: panic in normalize goroutine: %v", r)
				_ = pr.CloseWithError(err)
			}
		}()
		var normErr error
		normResult, normErr = Normalize(pr, fetchedAt, qSink)
		if normErr != nil {
			_ = pr.CloseWithError(normErr)
			return fmt.Errorf("alibaba adapter: normalize: %w", normErr)
		}
		_ = pr.Close()
		return nil
	})

	// Goroutine 2: GCS reads from tee
	g.Go(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("alibaba adapter: panic in storage write goroutine: %v", r)
				_ = pw.CloseWithError(err)
			}
		}()
		defer func() { _ = pw.Close() }()
		if err := a.storage.WriteStream(ctx, gcsPath, tee); err != nil {
			_ = pw.CloseWithError(err)
			return fmt.Errorf("alibaba adapter: storage write: %w", err)
		}
		return nil
	})

	if err = g.Wait(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if a.fetchCounter != nil {
			a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error"), attribute.String("category", category)))
		}
		slog.ErrorContext(ctx, "alibaba fetch failed during stream processing", "error", err)
		return domain.FetchResult{}, err
	}

	if a.bytesCounter != nil {
		a.bytesCounter.Add(ctx, cr.BytesRead(), metric.WithAttributes(
			attribute.String("provider", "alibaba"),
			attribute.String("category", category),
		))
	}

	// Flush quarantined unmapped items to storage
	if qSink.Count() > 0 {
		_ = qSink.Flush(ctx)
		if a.unmappedCounter != nil {
			a.unmappedCounter.Add(ctx, int64(qSink.Count()), metric.WithAttributes(
				attribute.String("provider", "alibaba"),
				attribute.String("category", category),
			))
		}
	}

	obsList := normResult.Observations
	if a.category != "" {
		filtered := make([]domain.PriceObservation, 0, len(obsList))
		for _, obs := range obsList {
			if obs.ServiceCategory == a.category {
				filtered = append(filtered, obs)
			}
		}
		obsList = filtered
	}

	span.SetStatus(codes.Ok, "")
	span.SetAttributes(attribute.Int("observations.count", len(obsList)))
	if a.fetchCounter != nil {
		a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "success"), attribute.String("category", category)))
	}
	slog.InfoContext(ctx, "alibaba fetch completed", "observations", len(obsList), "unmapped", qSink.Count(), "ignored", normResult.IgnoredCount)

	return domain.FetchResult{
		Observations:  obsList,
		RawGCSPath:    gcsPath,
		UnmappedCount: qSink.Count(),
		IgnoredCount:  normResult.IgnoredCount,
	}, nil
}
