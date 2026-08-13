package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
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

// IngestionOption allows configuring optional dependencies for IngestionService.
type IngestionOption func(*IngestionService)

// WithRedisClient configures Redis cache warming for the IngestionService.
func WithRedisClient(redisClient redis.Cmdable) IngestionOption {
	return func(s *IngestionService) {
		s.redisClient = redisClient
	}
}

// NewIngestionService constructs a new IngestionService.
func NewIngestionService(queries *store.Queries, fetcher Fetcher, opts ...IngestionOption) *IngestionService {
	s := &IngestionService{
		queries: queries,
		fetcher: fetcher,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// RunAWSComputeIngestion triggers the AWS EC2 compute pricing ingestion pipeline:
// fetches AWS price list, streams raw payload to storage, normalizes pricing records,
// inserts each observation into Postgres via sqlc queries, and immediately warms Redis cache keys.
// Returns the total number of inserted records.
func (s *IngestionService) RunAWSComputeIngestion(ctx context.Context) (int, error) {
	result, err := s.fetcher.Fetch(ctx)
	if err != nil {
		return 0, fmt.Errorf("ingest service: fetch aws compute: %w", err)
	}

	insertedCount := 0
	for _, obs := range result.Observations {
		params, err := toInsertParams(obs, result.RawGCSPath)
		if err != nil {
			return insertedCount, err
		}

		if _, err := s.queries.InsertPriceObservation(ctx, params); err != nil {
			return insertedCount, fmt.Errorf("ingest service: insert observation for sku %s: %w", obs.SkuID, err)
		}
		insertedCount++
	}

	// Event-driven cache warming
	if s.redisClient != nil && len(result.Observations) > 0 {
		byRegion := make(map[string][]domain.PriceObservation)
		for _, obs := range result.Observations {
			byRegion[obs.Region] = append(byRegion[obs.Region], obs)
		}

		for region, obsList := range byRegion {
			key := cache.BuildKey(cache.SchemaVersion, "aws", "compute", region)
			if warmErr := cache.Warm(ctx, s.redisClient, key, obsList, cache.DefaultTTL); warmErr != nil {
				slog.Warn("failed to warm redis cache after ingestion", "key", key, "error", warmErr)
			}
		}
	}

	return insertedCount, nil
}

func toInsertParams(obs domain.PriceObservation, rawGCSPath string) (store.InsertPriceObservationParams, error) {
	attrBytes, err := json.Marshal(obs.Attributes)
	if err != nil {
		return store.InsertPriceObservationParams{}, fmt.Errorf("ingest service: marshal attributes for sku %s: %w", obs.SkuID, err)
	}

	var priceAmt pgtype.Numeric
	if err := priceAmt.Scan(obs.PriceAmount.String()); err != nil {
		return store.InsertPriceObservationParams{}, fmt.Errorf("ingest service: scan price amount for sku %s: %w", obs.SkuID, err)
	}

	return store.InsertPriceObservationParams{
		Provider:        obs.Provider,
		ServiceCategory: obs.ServiceCategory,
		SkuID:           obs.SkuID,
		DisplayName:     obs.DisplayName,
		Region:          obs.Region,
		RegionGroup:     obs.RegionGroup,
		Unit:            obs.Unit,
		PriceAmount:     priceAmt,
		PriceCurrency:   obs.PriceCurrency,
		PricingModel:    obs.PricingModel,
		Attributes:      attrBytes,
		RawResponseRef:  pgtype.Text{String: rawGCSPath, Valid: rawGCSPath != ""},
		FetchedAt:       pgtype.Timestamptz{Time: obs.FetchedAt, Valid: !obs.FetchedAt.IsZero()},
		LastSeenAt:      pgtype.Timestamptz{Time: obs.FetchedAt, Valid: !obs.FetchedAt.IsZero()},
		AnomalyStatus:   pgtype.Text{Valid: false},
	}, nil
}
