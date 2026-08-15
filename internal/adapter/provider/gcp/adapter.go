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

	fetchedAt := time.Now().UTC()
	fetchID := uuid.New().String()
	dateStr := fetchedAt.Format("2006-01-02")
	firstGCSPath := fmt.Sprintf("raw/gcp/compute/%s/%s-page0.json", dateStr, fetchID)

	var allObservations []domain.PriceObservation
	var pageToken string
	pageIdx := 0
	const maxPages = 500

	for {
		if pageIdx >= maxPages {
			slog.WarnContext(ctx, "gcp fetch hit max pages limit", "limit", maxPages)
			break
		}

		if limiter != nil {
			if err := limiter.Wait(ctx); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return domain.FetchResult{}, fmt.Errorf("gcp adapter: rate limit wait: %w", err)
			}
		}

		gcsPath := fmt.Sprintf("raw/gcp/compute/%s/%s-page%d.json", dateStr, fetchID, pageIdx)

		body, err := a.client.FetchPriceList(ctx, pageToken)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error")))
			return domain.FetchResult{}, fmt.Errorf("gcp adapter: fetch price list page %d: %w", pageIdx, err)
		}

		pr, pw := io.Pipe()
		tee := io.TeeReader(body, provider.IgnoreErrorWriter{W: pw})

		var pageObservations []domain.PriceObservation
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
			pageObservations, pageNextToken, normErr = Normalize(pr, fetchedAt)
			if normErr != nil {
				_ = pr.CloseWithError(normErr)
				return fmt.Errorf("gcp adapter: normalize page %d: %w", pageIdx, normErr)
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
				return fmt.Errorf("gcp adapter: storage write page %d: %w", pageIdx, err)
			}
			return nil
		})

		if err := g.Wait(); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error")))
			slog.ErrorContext(ctx, "gcp fetch failed during stream processing", "error", err, "page", pageIdx)
			_ = body.Close()
			return domain.FetchResult{}, err
		}
		_ = body.Close()

		allObservations = append(allObservations, pageObservations...)

		if pageNextToken == "" {
			break
		}
		pageToken = pageNextToken
		pageIdx++
	}

	span.SetStatus(codes.Ok, "")
	span.SetAttributes(attribute.Int("observations.count", len(allObservations)))
	a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "success")))
	slog.InfoContext(ctx, "gcp fetch completed", "observations", len(allObservations), "pages", pageIdx)

	return domain.FetchResult{
		Observations: allObservations,
		RawGCSPath:   firstGCSPath,
	}, nil
}
