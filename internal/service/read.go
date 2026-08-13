package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
	"golang.org/x/sync/singleflight"
)

// PricingService handles reading cloud pricing data using a singleflight-wrapped
// cache-aside pattern (Redis cache first, Postgres query miss fallback).
type PricingService struct {
	queries     *store.Queries
	redisClient redis.Cmdable
	sfGroup     singleflight.Group
}

// NewPricingService constructs a new PricingService.
func NewPricingService(queries *store.Queries, redisClient redis.Cmdable) *PricingService {
	return &PricingService{
		queries:     queries,
		redisClient: redisClient,
	}
}

// GetComputePrices retrieves compute pricing observations for provider, category, and region.
// It attempts a Redis read first; on miss, it uses singleflight to collapse concurrent DB queries,
// queries Postgres by region_group, filters for the region, and warms the cache before returning.
func (s *PricingService) GetComputePrices(ctx context.Context, provider, category, region string) ([]domain.PriceObservation, error) {
	cacheKey := cache.BuildKey(cache.SchemaVersion, provider, category, region)

	// 1. Try Cache-Aside Read from Redis
	if s.redisClient != nil {
		cachedObs, err := cache.Get(ctx, s.redisClient, cacheKey)
		if err == nil {
			return cachedObs, nil
		}
		if !errors.Is(err, cache.ErrCacheMiss) {
			slog.Warn("cache read error, proceeding to DB fallback", "key", cacheKey, "error", err)
		}
	}

	// 2. Singleflight Collapse on Cache Miss
	val, err, _ := s.sfGroup.Do(cacheKey, func() (interface{}, error) {
		// Resolve region to region_group for DB query (cache is per-region, DB index is per-region-group)
		var regionGroup string
		var regErr error
		switch provider {
		case "aws":
			regionGroup, regErr = regionmap.MapAWSRegion(region)
		default:
			regionGroup = region
		}
		if regErr != nil {
			return nil, fmt.Errorf("pricing service: resolve region group: %w", regErr)
		}

		dbRows, dbErr := s.queries.GetPriceObservations(ctx, store.GetPriceObservationsParams{
			Provider:        provider,
			ServiceCategory: category,
			RegionGroup:     regionGroup,
		})
		if dbErr != nil {
			return nil, fmt.Errorf("pricing service: db query failed: %w", dbErr)
		}

		var regionObs []domain.PriceObservation
		for _, row := range dbRows {
			// DB queries by region_group, so filter for exact requested region
			if row.Region != region {
				continue
			}

			obs, mapErr := mapStoreToDomain(row)
			if mapErr != nil {
				return nil, fmt.Errorf("pricing service: map store row: %w", mapErr)
			}
			regionObs = append(regionObs, obs)
		}

		// 3. Event-driven cache warming after DB fetch
		if s.redisClient != nil && len(regionObs) > 0 {
			_ = cache.Warm(ctx, s.redisClient, cacheKey, regionObs, cache.DefaultTTL)
		}

		return regionObs, nil
	})

	if err != nil {
		return nil, err
	}

	obs, ok := val.([]domain.PriceObservation)
	if !ok {
		return nil, fmt.Errorf("pricing service: unexpected singleflight return type %T", val)
	}

	return obs, nil
}

func mapStoreToDomain(row store.PriceObservation) (domain.PriceObservation, error) {
	var attrs domain.ComputeAttributes
	if len(row.Attributes) > 0 {
		if err := json.Unmarshal(row.Attributes, &attrs); err != nil {
			return domain.PriceObservation{}, fmt.Errorf("unmarshal attributes: %w", err)
		}
	}

	priceDec, err := numericToDecimal(row.PriceAmount)
	if err != nil {
		return domain.PriceObservation{}, fmt.Errorf("convert numeric price: %w", err)
	}

	var fetchedAtTime time.Time
	if row.FetchedAt.Valid {
		fetchedAtTime = row.FetchedAt.Time
	}

	return domain.PriceObservation{
		Provider:        row.Provider,
		ServiceCategory: row.ServiceCategory,
		SkuID:           row.SkuID,
		DisplayName:     row.DisplayName,
		Region:          row.Region,
		RegionGroup:     row.RegionGroup,
		Unit:            row.Unit,
		PriceAmount:     priceDec,
		PriceCurrency:   row.PriceCurrency,
		PricingModel:    row.PricingModel,
		Attributes:      attrs,
		FetchedAt:       fetchedAtTime,
	}, nil
}

func numericToDecimal(n pgtype.Numeric) (decimal.Decimal, error) {
	if !n.Valid {
		return decimal.Zero, nil
	}
	val, err := n.Value()
	if err != nil || val == nil {
		return decimal.Zero, nil
	}
	str, ok := val.(string)
	if !ok {
		str = fmt.Sprintf("%v", val)
	}
	return decimal.NewFromString(str)
}
