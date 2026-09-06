package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

// Fetcher defines the contract for a provider adapter that retrieves pricing data.
type Fetcher interface {
	Fetch(ctx context.Context) (domain.FetchResult, error)
}

// IngestionService orchestrates fetching, normalizing, storage persistence,
// database insertion, and Redis cache warming of provider pricing observations.
type IngestionService struct {
	queries     *store.Queries
	fetcher     Fetcher
	redisClient redis.Cmdable
}

// NewIngestionService constructs a new IngestionService.
func NewIngestionService(queries *store.Queries, fetcher Fetcher, redisClient redis.Cmdable) *IngestionService {
	return &IngestionService{
		queries:     queries,
		fetcher:     fetcher,
		redisClient: redisClient,
	}
}

// RunAWSComputeIngestion triggers the AWS EC2 compute pricing ingestion pipeline:
// fetches AWS price list, streams raw payload to storage, normalizes pricing records,
// upserts each observation into Postgres (inserting only on new/changed price, bumping last_seen_at on unchanged),
// and immediately warms Redis cache keys. Returns the total number of inserted records.
func (s *IngestionService) RunAWSComputeIngestion(ctx context.Context) (int, error) {
	result, err := s.fetcher.Fetch(ctx)
	if err != nil {
		return 0, fmt.Errorf("ingest service: fetch aws compute: %w", err)
	}

	insertedCount := 0
	for _, obs := range result.Observations {
		prev, prevErr := s.queries.GetLatestPriceForSKUAndCategory(ctx, store.GetLatestPriceForSKUAndCategoryParams{
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
			if updateErr := s.queries.UpdatePriceObservationLastSeenAt(ctx, store.UpdatePriceObservationLastSeenAtParams{
				ID:         prev.ID,
				LastSeenAt: store.TimestamptzFromTime(obs.FetchedAt),
			}); updateErr != nil {
				slog.ErrorContext(ctx, "failed to update last_seen_at",
					"provider", obs.Provider,
					"sku", obs.SkuID,
					"error", updateErr,
				)
			}
			continue
		}

		params, paramErr := store.ToInsertPriceObservationParams(obs, result.RawGCSPath, "")
		if paramErr != nil {
			return insertedCount, paramErr
		}

		if _, insertErr := s.queries.InsertPriceObservation(ctx, params); insertErr != nil {
			return insertedCount, fmt.Errorf("ingest service: insert observation for sku %s: %w", obs.SkuID, insertErr)
		}
		insertedCount++
	}

	// Synchronize compute catalog inventory
	if err := SyncComputeCatalog(ctx, s.queries, result.Observations); err != nil {
		slog.WarnContext(ctx, "failed to sync compute catalog inventory", "error", err)
	}

	// Event-driven cache warming

	if s.redisClient != nil && len(result.Observations) > 0 {
		byRegion := make(map[string][]domain.PriceObservation)
		for _, obs := range result.Observations {
			byRegion[obs.Region] = append(byRegion[obs.Region], obs)
		}

		for region, obsList := range byRegion {
			key := cache.BuildKey(cache.SchemaVersion, "aws", "compute", region)
			warmCtx, warmCancel := context.WithTimeout(context.Background(), 5*time.Second)
			if warmErr := cache.Warm(warmCtx, s.redisClient, key, obsList, cache.DefaultTTL); warmErr != nil {
				slog.Warn("failed to warm redis cache after ingestion", "key", key, "error", warmErr)
			}
			warmCancel()
		}
	}

	return insertedCount, nil
}
