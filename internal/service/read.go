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
	"github.com/thatengineerguy21/CloudVitta/internal/observability"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/singleflight"
)

// PricingService handles reading cloud pricing data using a singleflight-wrapped
// cache-aside pattern (Redis cache first, Postgres query miss fallback).
type PricingService struct {
	queries      *store.Queries
	redisClient  redis.Cmdable
	tracer       trace.Tracer
	cacheMetrics *observability.CacheMetrics
	sfGroup      singleflight.Group
}

// PricingOption allows configuring optional dependencies for PricingService.
type PricingOption func(*PricingService)

// WithTracer attaches an OpenTelemetry Tracer to PricingService.
func WithTracer(tracer trace.Tracer) PricingOption {
	return func(s *PricingService) {
		s.tracer = tracer
	}
}

// WithCacheMetrics attaches CacheMetrics to PricingService for hit/miss recording.
func WithCacheMetrics(metrics *observability.CacheMetrics) PricingOption {
	return func(s *PricingService) {
		s.cacheMetrics = metrics
	}
}

// NewPricingService constructs a new PricingService.
func NewPricingService(queries *store.Queries, redisClient redis.Cmdable, opts ...PricingOption) *PricingService {
	s := &PricingService{
		queries:     queries,
		redisClient: redisClient,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// GetComputePrices retrieves compute pricing observations for provider, category, and regionGroup.
// It attempts a Redis read first; on miss, it uses singleflight to collapse concurrent DB queries,
// queries Postgres by regionGroup, and warms the cache before returning.
func (s *PricingService) GetComputePrices(ctx context.Context, provider, category, regionGroup string) ([]domain.PriceObservation, error) {
	cacheKey := cache.BuildKey(cache.SchemaVersion, provider, category, regionGroup)

	// 1. Try Cache-Aside Read from Redis
	if s.redisClient != nil {
		cacheCtx := ctx
		var cacheSpan trace.Span
		if s.tracer != nil {
			cacheCtx, cacheSpan = s.tracer.Start(ctx, "Cache Read")
		}

		cachedObs, err := cache.Get(cacheCtx, s.redisClient, cacheKey)

		if cacheSpan != nil {
			cacheSpan.End()
		}

		if err == nil {
			if s.cacheMetrics != nil {
				s.cacheMetrics.RecordHit(ctx)
			}
			return cachedObs, nil
		}
		if !errors.Is(err, cache.ErrCacheMiss) {
			slog.WarnContext(ctx, "cache read error, proceeding to DB fallback", "key", cacheKey, "error", err)
		}
	}

	if s.cacheMetrics != nil {
		s.cacheMetrics.RecordMiss(ctx)
	}

	// 2. Singleflight Collapse on Cache Miss
	ch := s.sfGroup.DoChan(cacheKey, func() (interface{}, error) {
		// Detach DB context from caller's context so single request cancellation
		// doesn't fail the underlying shared query for all waiting requests.
		dbCtx, dbCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer dbCancel()

		if s.tracer != nil {
			var dbSpan trace.Span
			dbCtx, dbSpan = s.tracer.Start(dbCtx, "Postgres Fallback Query")
			defer dbSpan.End()
		}

		dbRows, dbErr := s.queries.GetPriceObservations(dbCtx, store.GetPriceObservationsParams{
			Provider:        provider,
			ServiceCategory: category,
			RegionGroup:     regionGroup,
		})
		if dbErr != nil {
			return nil, fmt.Errorf("pricing service: db query failed: %w", dbErr)
		}

		var regionObs []domain.PriceObservation
		for _, row := range dbRows {
			obs, mapErr := mapStoreToDomain(row)
			if mapErr != nil {
				return nil, fmt.Errorf("pricing service: map store row: %w", mapErr)
			}
			regionObs = append(regionObs, obs)
		}

		// 3. Event-driven cache warming after DB fetch
		if s.redisClient != nil && len(regionObs) > 0 {
			warmCtx, warmCancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = cache.Warm(warmCtx, s.redisClient, cacheKey, regionObs, cache.DefaultTTL)
			warmCancel()
		}

		return regionObs, nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}
		obs, ok := res.Val.([]domain.PriceObservation)
		if !ok {
			return nil, fmt.Errorf("pricing service: unexpected singleflight return type %T", res.Val)
		}
		return obs, nil
	}
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
