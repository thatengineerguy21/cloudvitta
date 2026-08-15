package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
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
	queries     *store.Queries
	redisClient redis.Cmdable
	dlq         *dlq.DLQ
	factory     *provider.Factory
	config      OrchestratorConfig
}

// NewOrchestrator constructs a new ingestion orchestrator.
func NewOrchestrator(
	queries *store.Queries,
	redisClient redis.Cmdable,
	dlqSvc *dlq.DLQ,
	factory *provider.Factory,
	cfg OrchestratorConfig,
) *Orchestrator {
	return &Orchestrator{
		queries:     queries,
		redisClient: redisClient,
		dlq:         dlqSvc,
		factory:     factory,
		config:      cfg,
	}
}

// JobResult captures the outcome of a single (provider, category) ingestion job.
type JobResult struct {
	Provider      string
	Category      string
	InsertedCount int
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
				result.Skipped = true
				return result
			}
			slog.ErrorContext(ctx, "failed to acquire ingestion lock",
				"provider", job.Provider,
				"category", job.Category,
				"error", err,
			)
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
		return provider.RateLimitedFetch(ctx, job)
	})
	if err != nil {
		result.Err = fmt.Errorf("fetch %s/%s: %w", job.Provider, job.Category, err)
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

	// Step 3: Insert observations with anomaly detection
	for _, obs := range fetchResult.Observations {
		anomalyStatus := o.checkAnomaly(ctx, obs)

		params, marshalErr := toInsertParams(obs, fetchResult.RawGCSPath)
		if marshalErr != nil {
			slog.ErrorContext(ctx, "failed to build insert params",
				"provider", job.Provider,
				"sku", obs.SkuID,
				"error", marshalErr,
			)
			continue
		}

		// Override anomaly status
		if anomalyStatus != "" {
			params.AnomalyStatus = pgtype.Text{String: anomalyStatus, Valid: true}
			result.AnomalyCount++
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

	slog.InfoContext(ctx, "ingestion job completed",
		"provider", job.Provider,
		"category", job.Category,
		"inserted", result.InsertedCount,
		"anomalies", result.AnomalyCount,
		"total_observations", len(fetchResult.Observations),
	)
	return result
}

// checkAnomaly compares the incoming price against the latest recorded price
// for the same SKU/region. Returns "pending_review" if the price differs by
// more than AnomalyThreshold, otherwise returns empty string.
func (o *Orchestrator) checkAnomaly(ctx context.Context, obs domain.PriceObservation) string {
	prev, err := o.queries.GetLatestPriceForSKU(ctx, store.GetLatestPriceForSKUParams{
		Provider: obs.Provider,
		SkuID:    obs.SkuID,
		Region:   obs.Region,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No prior observation: first time seeing this SKU, no anomaly
			return ""
		}
		slog.WarnContext(ctx, "anomaly check: failed to query previous price",
			"provider", obs.Provider,
			"sku", obs.SkuID,
			"region", obs.Region,
			"error", err,
		)
		return ""
	}

	// Convert pgtype.Numeric to big.Float for comparison
	oldPrice := numericToBigFloat(prev.PriceAmount)
	if oldPrice == nil || oldPrice.Sign() == 0 {
		return ""
	}

	newPrice := new(big.Float).SetPrec(128)
	newPrice.SetString(obs.PriceAmount.String())
	if newPrice.Sign() == 0 {
		return ""
	}

	// Calculate ratio = max(new/old, old/new)
	ratio := new(big.Float).SetPrec(128)
	ratio.Quo(newPrice, oldPrice)

	one := new(big.Float).SetFloat64(1.0)
	if ratio.Cmp(one) < 0 {
		// ratio < 1, invert it
		ratio.Quo(one, ratio)
	}

	threshold := new(big.Float).SetFloat64(float64(AnomalyThreshold))
	if ratio.Cmp(threshold) >= 0 {
		slog.WarnContext(ctx, "price anomaly detected",
			"provider", obs.Provider,
			"sku", obs.SkuID,
			"region", obs.Region,
			"ratio", ratio.Text('f', 2),
		)
		return "pending_review"
	}
	return ""
}

// numericToBigFloat converts a pgtype.Numeric to *big.Float.
func numericToBigFloat(n pgtype.Numeric) *big.Float {
	if !n.Valid || n.Int == nil {
		return nil
	}

	f := new(big.Float).SetPrec(128).SetInt(n.Int)
	if n.Exp != 0 {
		// Numeric stores as Int * 10^Exp
		exp := new(big.Float).SetPrec(128)
		ten := big.NewInt(10)
		if n.Exp > 0 {
			pow := new(big.Int).Exp(ten, big.NewInt(int64(n.Exp)), nil)
			exp.SetInt(pow)
			f.Mul(f, exp)
		} else {
			pow := new(big.Int).Exp(ten, big.NewInt(int64(-n.Exp)), nil)
			exp.SetInt(pow)
			f.Quo(f, exp)
		}
	}
	return f
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
