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

// AdapterOption defines a configuration option for GCP Adapter.
type AdapterOption func(*Adapter)

// WithCategory sets the service category for the GCP adapter.
func WithCategory(category string) AdapterOption {
	return func(a *Adapter) {
		a.category = category
	}
}

// Adapter handles fetching, persisting raw response to GCS, and normalizing GCP pricing.
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

// NewAdapter constructs a new GCP provider adapter.
func NewAdapter(client *Client, st storage.RawStorage, opts ...AdapterOption) *Adapter {
	meter := otel.Meter("cloudvitta.adapter.gcp")
	fetchCounter, _ := meter.Int64Counter("gcp.fetch.count", metric.WithDescription("Number of GCP fetch operations"))
	unmappedCounter, _ := meter.Int64Counter("gcp.normalize.unmapped_count", metric.WithDescription("Number of unmapped GCP taxonomy items"))
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
		tracer:            otel.Tracer("cloudvitta.adapter.gcp"),
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

// Fetch retrieves the GCP price list, concurrently streams raw JSON to storage and normalizes it.
// Returns a domain.FetchResult containing the observations and raw GCS path, and any error.
func (a *Adapter) Fetch(ctx context.Context, limiter *rate.Limiter) (res domain.FetchResult, err error) {
	ctx, span := a.tracer.Start(ctx, "gcp.fetch")
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
				attribute.String("provider", "gcp"),
				attribute.String("category", category),
				attribute.String("status", status),
			))
		}
	}()

	slog.InfoContext(ctx, "starting gcp fetch")

	fetchedAt := time.Now().UTC()
	fetchID := uuid.New().String()
	dateStr := fetchedAt.Format("2006-01-02")
	firstGCSPath := fmt.Sprintf("raw/gcp/%s/%s/%s-page0.json", category, dateStr, fetchID)

	endpoints := a.client.URLs()
	if len(endpoints) == 0 {
		endpoints = []string{DefaultBillingCatalogURL}
	}

	var allObservations []domain.PriceObservation
	var totalIgnoredCount int
	var totalBytes int64
	var totalPages int
	const maxPages = 500

	qSink := quarantine.NewStorageSink(a.storage, "gcp", category, fetchID, fetchedAt)
	normState := NewNormalizationState()

	for epIdx, endpointURL := range endpoints {
		pageToken := ""
		pageIdx := 0

		for {
			if totalPages >= maxPages {
				slog.WarnContext(ctx, "gcp fetch hit max pages limit", "limit", maxPages)
				break
			}

			if limiter != nil {
				if err = limiter.Wait(ctx); err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
					return domain.FetchResult{}, fmt.Errorf("gcp adapter: rate limit wait: %w", err)
				}
			}

			var gcsPath string
			if len(endpoints) == 1 {
				gcsPath = fmt.Sprintf("raw/gcp/%s/%s/%s-page%d.json", category, dateStr, fetchID, pageIdx)
			} else {
				gcsPath = fmt.Sprintf("raw/gcp/%s/%s/%s-ep%d-page%d.json", category, dateStr, fetchID, epIdx, pageIdx)
			}
			if totalPages == 0 {
				firstGCSPath = gcsPath
			}

			var body io.ReadCloser
			body, err = a.client.FetchPriceListURL(ctx, endpointURL, pageToken)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error"), attribute.String("category", category)))
				return domain.FetchResult{}, fmt.Errorf("gcp adapter: fetch price list endpoint %d page %d: %w", epIdx, pageIdx, err)
			}

			cr := provider.NewCountingReader(body)
			pr, pw := io.Pipe()
			tee := io.TeeReader(cr, provider.IgnoreErrorWriter{W: pw})

			var pageResult domain.NormalizationResult
			var pageNextToken string
			g, _ := errgroup.WithContext(ctx)

			// Goroutine 1: Normalize reads from pr
			g.Go(func() error {
				defer func() {
					if r := recover(); r != nil {
						_ = pr.CloseWithError(fmt.Errorf("panic in normalize: %v", r))
					}
				}()
				var normErr error
				pageResult, pageNextToken, normErr = NormalizeForCategoryWithState(pr, fetchedAt, a.category, normState, qSink)
				if normErr != nil {
					_ = pr.CloseWithError(normErr)
					return fmt.Errorf("gcp adapter: normalize endpoint %d page %d: %w", epIdx, pageIdx, normErr)
				}
				_ = pr.Close()
				return nil
			})

			// Goroutine 2: GCS reads from tee
			g.Go(func() error {
				defer func() {
					if r := recover(); r != nil {
						_ = pw.CloseWithError(fmt.Errorf("panic in storage write: %v", r))
					}
				}()
				defer func() { _ = pw.Close() }()
				// Must use parent ctx, not gctx, to ensure durability if normalize fails
				if err := a.storage.WriteStream(ctx, gcsPath, tee); err != nil {
					_ = pw.CloseWithError(err)
					return fmt.Errorf("gcp adapter: storage write endpoint %d page %d: %w", epIdx, pageIdx, err)
				}
				return nil
			})

			if err = g.Wait(); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error"), attribute.String("category", category)))
				slog.ErrorContext(ctx, "gcp fetch failed during stream processing", "error", err, "endpoint", epIdx, "page", pageIdx)
				_ = body.Close()
				return domain.FetchResult{}, err
			}
			_ = body.Close()

			totalBytes += cr.BytesRead()
			allObservations = append(allObservations, pageResult.Observations...)
			totalIgnoredCount += pageResult.IgnoredCount
			totalPages++

			if pageNextToken == "" {
				break
			}
			pageToken = pageNextToken
			pageIdx++
		}
	}

	if a.bytesCounter != nil {
		a.bytesCounter.Add(ctx, totalBytes, metric.WithAttributes(
			attribute.String("provider", "gcp"),
			attribute.String("category", category),
		))
	}

	// Flush quarantined unmapped items to storage
	if qSink.Count() > 0 {
		_ = qSink.Flush(ctx)
		if a.unmappedCounter != nil {
			a.unmappedCounter.Add(ctx, int64(qSink.Count()), metric.WithAttributes(
				attribute.String("provider", "gcp"),
				attribute.String("category", category),
			))
		}
	}

	// Synthesize multi-meter instance observations across all accumulated pages and endpoints
	composedObs, err := normState.FinalizeComposedObservations(fetchedAt, category, qSink)
	if err != nil {
		return domain.FetchResult{}, fmt.Errorf("gcp adapter: finalize composed observations: %w", err)
	}
	allObservations = append(allObservations, composedObs...)

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
	slog.InfoContext(ctx, "gcp fetch completed", "observations", len(obsList), "pages", totalPages, "unmapped", qSink.Count(), "ignored", totalIgnoredCount)

	return domain.FetchResult{
		Observations:  obsList,
		RawGCSPath:    firstGCSPath,
		UnmappedCount: qSink.Count(),
		IgnoredCount:  totalIgnoredCount,
	}, nil
}
