package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"golang.org/x/sync/errgroup"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/dlq"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

// AnomalyThreshold defines the maximum ratio between new and old prices
// before flagging as anomalous. A value of 10 means prices differing by
// more than 10x in either direction are flagged.
const AnomalyThreshold = 10

// OrchestratorConfig holds configuration for the ingestion orchestrator.
type OrchestratorConfig struct {
	// MaxConcurrency caps the total number of in-flight provider fetch operations.
	// This protects the Cloud Run instance from resource exhaustion.
	MaxConcurrency int
	// LockTTL is the duration for which an ingestion lock is held.
	LockTTL time.Duration
	// Tracer is the OpenTelemetry tracer instance for tracing ingestion jobs.
	Tracer trace.Tracer
	// Meter is the OpenTelemetry meter instance for recording job and anomaly metrics.
	Meter metric.Meter
}

// DefaultOrchestratorConfig returns sensible defaults.
func DefaultOrchestratorConfig() OrchestratorConfig {
	return OrchestratorConfig{
		MaxConcurrency: 5,
		LockTTL:        15 * time.Minute,
	}
}

// Orchestrator coordinates concurrent ingestion jobs across all registered
// provider/category pairs. It handles locking, DLQ recording, anomaly
// detection, and cache warming.
type Orchestrator struct {
	queries        *store.Queries
	redisClient    redis.Cmdable
	dlq            *dlq.DLQ
	factory        *provider.Factory
	config         OrchestratorConfig
	tracer         trace.Tracer
	jobsTotal      metric.Int64Counter
	anomaliesTotal metric.Int64Counter
}

// NewOrchestrator constructs a new ingestion orchestrator.
func NewOrchestrator(
	queries *store.Queries,
	redisClient redis.Cmdable,
	dlqSvc *dlq.DLQ,
	factory *provider.Factory,
	cfg OrchestratorConfig,
) *Orchestrator {
	tracer := cfg.Tracer
	if tracer == nil {
		tracer = noop.NewTracerProvider().Tracer("cloudvitta-orchestrator")
	}

	var jobsTotal metric.Int64Counter
	var anomaliesTotal metric.Int64Counter
	if cfg.Meter != nil {
		var err error
		jobsTotal, err = cfg.Meter.Int64Counter(
			"ingest_jobs_total",
			metric.WithDescription("Total number of provider ingestion jobs executed"),
			metric.WithUnit("1"),
		)
		if err != nil {
			slog.Warn("failed to initialize ingest_jobs_total metric", "error", err)
		}
		anomaliesTotal, err = cfg.Meter.Int64Counter(
			"anomalies_detected_total",
			metric.WithDescription("Total number of pricing anomalies detected during ingestion"),
			metric.WithUnit("1"),
		)
		if err != nil {
			slog.Warn("failed to initialize anomalies_detected_total metric", "error", err)
		}
	}

	return &Orchestrator{
		queries:        queries,
		redisClient:    redisClient,
		dlq:            dlqSvc,
		factory:        factory,
		config:         cfg,
		tracer:         tracer,
		jobsTotal:      jobsTotal,
		anomaliesTotal: anomaliesTotal,
	}
}

// JobResult captures the outcome of a single (provider, category) ingestion job.
type JobResult struct {
	Provider      string
	Category      string
	InsertedCount int
	UpdatedCount  int
	AnomalyCount  int
	Err           error
	Skipped       bool // true if lock was already held
}

// RunAll executes all registered provider/category ingestion jobs concurrently,
// bounded by the global concurrency semaphore. One failing job does not abort others.
// Returns a slice of results for each job.
func (o *Orchestrator) RunAll(ctx context.Context) []JobResult {
	jobs := o.factory.BuildJobs()
	results := make([]JobResult, len(jobs))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(o.config.MaxConcurrency)

	for i, job := range jobs {
		i, job := i, job // capture loop variables
		g.Go(func() error {
			results[i] = o.runJob(gctx, job)
			// Always return nil: one job failure must not cancel others.
			return nil
		})
	}

	// Wait for all jobs. Errors are captured in results, not returned here.
	_ = g.Wait()
	return results
}

// runJob executes a single (provider, category) ingestion job with locking,
// retry, DLQ recording, and anomaly detection.
func (o *Orchestrator) runJob(ctx context.Context, job provider.Job) JobResult {
	ctx, span := o.tracer.Start(ctx, "ingest.job",
		trace.WithAttributes(
			attribute.String("provider", job.Provider),
			attribute.String("category", job.Category),
		),
	)
	defer span.End()

	result := JobResult{
		Provider: job.Provider,
		Category: job.Category,
	}

	// Step 1: Acquire idempotency lock
	if o.redisClient != nil {
		lock, err := cache.AcquireIngestionLock(ctx, o.redisClient, job.Provider, job.Category, o.config.LockTTL)
		if err != nil {
			if errors.Is(err, cache.ErrLockHeld) {
				slog.WarnContext(ctx, "ingestion already in progress, skipping",
					"provider", job.Provider,
					"category", job.Category,
				)
				span.SetAttributes(attribute.String("status", "skipped"))
				o.recordJobMetric(ctx, job.Provider, job.Category, "skipped")
				result.Skipped = true
				return result
			}
			slog.ErrorContext(ctx, "failed to acquire ingestion lock",
				"provider", job.Provider,
				"category", job.Category,
				"error", err,
			)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			o.recordJobMetric(ctx, job.Provider, job.Category, "failed")
			result.Err = fmt.Errorf("acquire lock for %s/%s: %w", job.Provider, job.Category, err)
			return result
		}
		defer func() {
			if releaseErr := lock.Release(ctx); releaseErr != nil {
				slog.WarnContext(ctx, "failed to release ingestion lock",
					"provider", job.Provider,
					"category", job.Category,
					"error", releaseErr,
				)
			}
		}()
	}

	// Step 2: Fetch with retry and rate limiting
	fetchResult, err := provider.Do(ctx, job.Retry, func(ctx context.Context) (domain.FetchResult, error) {
		return job.Fetch(ctx)
	})
	if err != nil {
		result.Err = fmt.Errorf("fetch %s/%s: %w", job.Provider, job.Category, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		o.recordJobMetric(ctx, job.Provider, job.Category, "failed")

		// Step 2b: Record to DLQ on failure
		if o.dlq != nil {
			if dlqErr := o.dlq.Record(ctx, job.Provider, job.Category, err); dlqErr != nil {
				slog.ErrorContext(ctx, "failed to record DLQ entry",
					"provider", job.Provider,
					"category", job.Category,
					"error", dlqErr,
				)
			}
		}
		return result
	}

	// Step 3: Upsert-with-history observations with anomaly detection
	for _, obs := range fetchResult.Observations {
		prev, prevErr := o.queries.GetLatestPriceForSKUAndCategory(ctx, store.GetLatestPriceForSKUAndCategoryParams{
			Provider:        obs.Provider,
			ServiceCategory: obs.ServiceCategory,
			SkuID:           obs.SkuID,
			Region:          obs.Region,
		})

		var oldPrice decimal.Decimal
		hasPrior := false
		if prevErr == nil {
			hasPrior = true
			if dec, decErr := store.NumericToDecimal(prev.PriceAmount); decErr == nil {
				oldPrice = dec
			}
		} else if !store.IsNotFound(prevErr) {
			slog.WarnContext(ctx, "failed to query previous price observation",
				"provider", obs.Provider,
				"sku", obs.SkuID,
				"region", obs.Region,
				"error", prevErr,
			)
		}

		// Check if price and billing dimensions are identical
		if hasPrior && !oldPrice.IsZero() && oldPrice.Equal(obs.PriceAmount) &&
			prev.Unit == obs.Unit && prev.PricingModel == obs.PricingModel && prev.PriceCurrency == obs.PriceCurrency {
			// Unchanged observation: bump last_seen_at timestamp on existing row
			if err := o.queries.UpdatePriceObservationLastSeenAt(ctx, store.UpdatePriceObservationLastSeenAtParams{
				ID:         prev.ID,
				LastSeenAt: store.TimestamptzFromTime(obs.FetchedAt),
			}); err != nil {
				slog.ErrorContext(ctx, "failed to update last_seen_at",
					"provider", job.Provider,
					"sku", obs.SkuID,
					"error", err,
				)
				continue
			}
			result.UpdatedCount++
			continue
		}

		// New SKU or price change: check for anomaly against prior price
		anomalyStatus := ""
		if hasPrior && !oldPrice.IsZero() && !obs.PriceAmount.IsZero() {
			ratio := obs.PriceAmount.Div(oldPrice)
			if ratio.LessThan(decimal.NewFromInt(1)) {
				ratio = oldPrice.Div(obs.PriceAmount)
			}
			threshold := decimal.NewFromInt(AnomalyThreshold)
			if ratio.GreaterThanOrEqual(threshold) {
				slog.WarnContext(ctx, "price anomaly detected",
					"provider", obs.Provider,
					"sku", obs.SkuID,
					"region", obs.Region,
					"ratio", ratio.StringFixed(2),
				)
				anomalyStatus = "pending_review"
				result.AnomalyCount++
				o.recordAnomalyMetric(ctx, job.Provider, job.Category, obs.SkuID)
			}
		}

		params, marshalErr := store.ToInsertPriceObservationParams(obs, fetchResult.RawGCSPath, anomalyStatus)
		if marshalErr != nil {
			slog.ErrorContext(ctx, "failed to build insert params",
				"provider", job.Provider,
				"sku", obs.SkuID,
				"error", marshalErr,
			)
			continue
		}

		if _, insertErr := o.queries.InsertPriceObservation(ctx, params); insertErr != nil {
			slog.ErrorContext(ctx, "failed to insert observation",
				"provider", job.Provider,
				"sku", obs.SkuID,
				"error", insertErr,
			)
			continue
		}
		result.InsertedCount++
	}

	// Step 4: Clear DLQ entry on success
	if o.dlq != nil {
		if clearErr := o.dlq.Clear(ctx, job.Provider, job.Category); clearErr != nil {
			slog.WarnContext(ctx, "failed to clear DLQ entry on success",
				"provider", job.Provider,
				"category", job.Category,
				"error", clearErr,
			)
		}
	}

	// Step 5: Event-driven cache warming
	if o.redisClient != nil && len(fetchResult.Observations) > 0 {
		o.warmCache(ctx, job.Provider, job.Category, fetchResult.Observations)
	}

	span.SetStatus(codes.Ok, "")
	span.SetAttributes(
		attribute.String("status", "success"),
		attribute.Int("inserted_count", result.InsertedCount),
		attribute.Int("updated_count", result.UpdatedCount),
		attribute.Int("anomalies_count", result.AnomalyCount),
	)
	o.recordJobMetric(ctx, job.Provider, job.Category, "success")

	slog.InfoContext(ctx, "ingestion job completed",
		"provider", job.Provider,
		"category", job.Category,
		"inserted", result.InsertedCount,
		"updated", result.UpdatedCount,
		"anomalies", result.AnomalyCount,
		"total_observations", len(fetchResult.Observations),
	)
	return result
}

func (o *Orchestrator) recordJobMetric(ctx context.Context, provider, category, status string) {
	if o.jobsTotal != nil {
		o.jobsTotal.Add(ctx, 1, metric.WithAttributes(
			attribute.String("provider", provider),
			attribute.String("category", category),
			attribute.String("status", status),
		))
	}
}

func (o *Orchestrator) recordAnomalyMetric(ctx context.Context, provider, category, sku string) {
	if o.anomaliesTotal != nil {
		o.anomaliesTotal.Add(ctx, 1, metric.WithAttributes(
			attribute.String("provider", provider),
			attribute.String("category", category),
			attribute.String("sku", sku),
		))
	}
}

// warmCache writes observation data to Redis grouped by region.
func (o *Orchestrator) warmCache(ctx context.Context, prov, category string, observations []domain.PriceObservation) {
	byRegion := make(map[string][]domain.PriceObservation)
	for _, obs := range observations {
		byRegion[obs.Region] = append(byRegion[obs.Region], obs)
	}

	for region, obsList := range byRegion {
		key := cache.BuildKey(cache.SchemaVersion, prov, category, region)
		warmCtx, warmCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if warmErr := cache.Warm(warmCtx, o.redisClient, key, obsList, cache.DefaultTTL); warmErr != nil {
			slog.Warn("failed to warm redis cache after ingestion", "key", key, "error", warmErr)
		}
		warmCancel()
	}
}
