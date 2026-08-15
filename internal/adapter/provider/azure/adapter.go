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
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

// Adapter handles fetching, persisting raw response to GCS, and normalizing Azure compute pricing.
type Adapter struct {
	client       *Client
	storage      storage.RawStorage
	tracer       trace.Tracer
	meter        metric.Meter
	fetchCounter metric.Int64Counter
}

// NewAdapter constructs a new Azure provider adapter.
func NewAdapter(client *Client, st storage.RawStorage) *Adapter {
	meter := otel.Meter("cloudvitta.adapter.azure")
	fetchCounter, _ := meter.Int64Counter("azure.fetch.count", metric.WithDescription("Number of Azure fetch operations"))
	return &Adapter{
		client:       client,
		storage:      st,
		tracer:       otel.Tracer("cloudvitta.adapter.azure"),
		meter:        meter,
		fetchCounter: fetchCounter,
	}
}

// Fetch retrieves the Azure price list, concurrently streams raw JSON to storage and normalizes it.
// Returns a domain.FetchResult containing the observations and raw GCS path, and any error.
func (a *Adapter) Fetch(ctx context.Context, limiter *rate.Limiter) (domain.FetchResult, error) {
	ctx, span := a.tracer.Start(ctx, "azure.fetch")
	defer span.End()

	slog.InfoContext(ctx, "starting azure fetch")

	fetchedAt := time.Now().UTC()
	fetchID := uuid.New().String()
	dateStr := fetchedAt.Format("2006-01-02")

	var allObservations []domain.PriceObservation
	var firstGCSPath string

	nextLink := ""
	pageIdx := 1

	for {
		if limiter != nil {
			if err := limiter.Wait(ctx); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return domain.FetchResult{}, fmt.Errorf("azure adapter: rate limit wait page %d: %w", pageIdx, err)
			}
		}

		gcsPath := fmt.Sprintf("raw/azure/compute/%s/%s-page%d.json", dateStr, fetchID, pageIdx)
		if firstGCSPath == "" {
			firstGCSPath = gcsPath
		}

		body, err := a.client.FetchPriceList(ctx, nextLink)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error")))
			return domain.FetchResult{}, fmt.Errorf("azure adapter: fetch price list page %d: %w", pageIdx, err)
		}

		pr, pw := io.Pipe()
		tee := io.TeeReader(body, provider.IgnoreErrorWriter{W: pw})

		var pageObservations []domain.PriceObservation
		var pageNextLink string
		g, _ := errgroup.WithContext(ctx)

		// Goroutine 1: Normalize reads from pr
		g.Go(func() error {
			var normErr error
			pageObservations, pageNextLink, normErr = Normalize(pr, fetchedAt)
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

		if err := g.Wait(); err != nil {
			_ = body.Close()
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error")))
			slog.ErrorContext(ctx, "azure fetch failed during stream processing", "error", err, "page", pageIdx)
			return domain.FetchResult{}, err
		}
		_ = body.Close()

		allObservations = append(allObservations, pageObservations...)

		if pageNextLink == "" {
			break
		}
		nextLink = pageNextLink
		pageIdx++
	}

	span.SetStatus(codes.Ok, "")
	span.SetAttributes(attribute.Int("observations.count", len(allObservations)))
	a.fetchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "success")))
	slog.InfoContext(ctx, "azure fetch completed", "observations", len(allObservations), "pages", pageIdx)

	return domain.FetchResult{
		Observations: allObservations,
		RawGCSPath:   firstGCSPath,
	}, nil
}
