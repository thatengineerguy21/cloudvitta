package azure

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

// AdapterOption defines a configuration option for Azure Adapter.
type AdapterOption func(*Adapter)

// WithCategory sets the service category for the Azure adapter.
func WithCategory(category string) AdapterOption {
	return func(a *Adapter) {
		a.category = category
	}
}

// Adapter handles fetching, persisting raw response to GCS, and normalizing Azure pricing.
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

// NewAdapter constructs a new Azure provider adapter.
func NewAdapter(client *Client, st storage.RawStorage, opts ...AdapterOption) *Adapter {
	meter := otel.Meter("cloudvitta.adapter.azure")
	fetchCounter, _ := meter.Int64Counter("azure.fetch.count", metric.WithDescription("Number of Azure fetch operations"))
	unmappedCounter, _ := meter.Int64Counter("azure.normalize.unmapped_count", metric.WithDescription("Number of unmapped Azure taxonomy items"))
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
		tracer:            otel.Tracer("cloudvitta.adapter.azure"),
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

// Fetch retrieves the Azure price list, concurrently streams raw JSON to storage and normalizes it.
// Returns a domain.FetchResult containing the observations and raw GCS path, and any error.
func (a *Adapter) Fetch(ctx context.Context, limiter *rate.Limiter) (res domain.FetchResult, err error) {
	ctx, span := a.tracer.Start(ctx, "azure.fetch")
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
				attribute.String("provider", "azure"),
				attribute.String("category", category),
				attribute.String("status", status),
			))
		}
	}()

	slog.InfoContext(ctx, "starting azure fetch")

	fetchedAt := time.Now().UTC()
	fetchID := uuid.New().String()
	dateStr := fetchedAt.Format("2006-01-02")

	var allObservations []domain.PriceObservation
	var totalIgnoredCount int
	var totalBytes int64
	var firstGCSPath string

	qSink := quarantine.NewStorageSink(a.storage, "azure", category, fetchID, fetchedAt)
	nextLink := ""
	pageIdx := 1

	for {
		if limiter != nil {
			if err = limiter.Wait(ctx); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return domain.FetchResult{}, fmt.Errorf("azure adapter: rate limit wait page %d: %w", pageIdx, err)
			}
		}

		gcsPath := fmt.Sprintf("raw/azure/%s/%s/%s-page%d.json", category, dateStr, fetchID, pageIdx)
		if firstGCSPath == "" {
			firstGCSPath = gcsPath
		}

		var body io.ReadCloser
		body, err = a.client.FetchPriceList(ctx, nextLink)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error"), attribute.String("category", category)))
			return domain.FetchResult{}, fmt.Errorf("azure adapter: fetch price list page %d: %w", pageIdx, err)
		}

		cr := provider.NewCountingReader(body)
		pr, pw := io.Pipe()
		tee := io.TeeReader(cr, provider.IgnoreErrorWriter{W: pw})

		var pageResult domain.NormalizationResult
		var pageNextLink string
		g, _ := errgroup.WithContext(ctx)

		// Goroutine 1: Normalize reads from pr
		g.Go(func() error {
			var normErr error
			pageResult, pageNextLink, normErr = Normalize(pr, fetchedAt, qSink)
			if normErr != nil {
				_ = pr.CloseWithError(normErr)
				return fmt.Errorf("azure adapter: normalize page %d: %w", pageIdx, normErr)
			}
			_ = pr.Close()
			return nil
		})

		// Goroutine 2: GCS reads from tee
		g.Go(func() error {
			defer func() { _ = pw.Close() }()
			if err := a.storage.WriteStream(ctx, gcsPath, tee); err != nil {
				_ = pw.CloseWithError(err)
				return fmt.Errorf("azure adapter: storage write page %d: %w", pageIdx, err)
			}
			return nil
		})

		if err = g.Wait(); err != nil {
			_ = body.Close()
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error"), attribute.String("category", category)))
			slog.ErrorContext(ctx, "azure fetch failed during stream processing", "error", err, "page", pageIdx)
			return domain.FetchResult{}, err
		}
		_ = body.Close()

		totalBytes += cr.BytesRead()
		allObservations = append(allObservations, pageResult.Observations...)
		totalIgnoredCount += pageResult.IgnoredCount

		if pageNextLink == "" {
			break
		}
		nextLink = pageNextLink
		pageIdx++
	}

	if a.bytesCounter != nil {
		a.bytesCounter.Add(ctx, totalBytes, metric.WithAttributes(
			attribute.String("provider", "azure"),
			attribute.String("category", category),
		))
	}

	// Flush quarantined unmapped items to storage
	if qSink.Count() > 0 {
		_ = qSink.Flush(ctx)
		if a.unmappedCounter != nil {
			a.unmappedCounter.Add(ctx, int64(qSink.Count()), metric.WithAttributes(
				attribute.String("provider", "azure"),
				attribute.String("category", category),
			))
		}
	}

	obsList := allObservations
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
	a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "success"), attribute.String("category", category)))
	slog.InfoContext(ctx, "azure fetch completed", "observations", len(obsList), "pages", pageIdx, "unmapped", qSink.Count(), "ignored", totalIgnoredCount)

	return domain.FetchResult{
		Observations:  obsList,
		RawGCSPath:    firstGCSPath,
		UnmappedCount: qSink.Count(),
		IgnoredCount:  totalIgnoredCount,
	}, nil
}
