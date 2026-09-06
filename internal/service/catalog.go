package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

const (
	// CatalogSummaryCacheTTL defines the cache duration for catalog summary rollups (24h).
	CatalogSummaryCacheTTL = 24 * time.Hour
)

var (
	// ErrCatalogItemNotFound indicates the requested compute instance specification was not found.
	ErrCatalogItemNotFound = errors.New("compute catalog item not found")
)

// CatalogService provides querying capabilities for cloud compute instance specifications.
type CatalogService struct {
	queries     store.Querier
	redisClient redis.Cmdable
}

// NewCatalogService constructs a new CatalogService.
func NewCatalogService(queries store.Querier, redisClient redis.Cmdable) *CatalogService {
	return &CatalogService{
		queries:     queries,
		redisClient: redisClient,
	}
}

// GetCatalogSummary returns the total instances, provider totals, and category breakdowns.
// It uses Redis caching with a 24-hour TTL when available.
func (s *CatalogService) GetCatalogSummary(ctx context.Context) (*domain.ComputeCatalogSummary, error) {
	cacheKey := cache.BuildCatalogSummaryKey(cache.SchemaVersion)

	if s.redisClient != nil {
		val, err := s.redisClient.Get(ctx, cacheKey).Result()
		if err == nil && val != "" {
			var cached domain.ComputeCatalogSummary
			if unmarshalErr := json.Unmarshal([]byte(val), &cached); unmarshalErr == nil {
				return &cached, nil
			}
		}
	}

	providerTotalsRows, err := s.queries.GetTotalComputeInstanceCountByProvider(ctx)
	if err != nil {
		return nil, fmt.Errorf("query total compute instances by provider: %w", err)
	}

	rows, err := s.queries.GetComputeCatalogSummary(ctx)
	if err != nil {
		return nil, fmt.Errorf("query compute catalog summary: %w", err)
	}

	summary := &domain.ComputeCatalogSummary{
		ProviderTotals:    make(map[string]int64),
		CategoryBreakdown: make(map[string]map[string]int64),
	}

	var grandTotal int64
	for _, p := range providerTotalsRows {
		summary.ProviderTotals[p.Provider] = p.TotalInstances
		grandTotal += p.TotalInstances
	}
	summary.TotalInstances = grandTotal

	for _, r := range rows {
		if _, ok := summary.CategoryBreakdown[r.Provider]; !ok {
			summary.CategoryBreakdown[r.Provider] = make(map[string]int64)
		}
		summary.CategoryBreakdown[r.Provider][r.Category] = r.InstanceCount
	}

	if s.redisClient != nil {
		if data, err := json.Marshal(summary); err == nil {
			if setErr := s.redisClient.Set(ctx, cacheKey, data, CatalogSummaryCacheTTL).Err(); setErr != nil {
				slog.WarnContext(ctx, "failed to cache compute catalog summary", "error", setErr)
			}
		}
	}

	return summary, nil
}

// ListCatalogInstances returns a paginated slice of compute instances matching the filter, along with total match count.
func (s *CatalogService) ListCatalogInstances(ctx context.Context, filter domain.CatalogFilter) ([]domain.ComputeCatalogItem, int64, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	countParams, err := store.ToCountComputeCatalogItemsParams(filter)
	if err != nil {
		return nil, 0, fmt.Errorf("build count params: %w", err)
	}

	total, err := s.queries.CountComputeCatalogItems(ctx, countParams)
	if err != nil {
		return nil, 0, fmt.Errorf("count compute catalog items: %w", err)
	}

	params, err := store.ToListComputeCatalogItemsParams(filter)
	if err != nil {
		return nil, 0, fmt.Errorf("build list params: %w", err)
	}

	rows, err := s.queries.ListComputeCatalogItems(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("list compute catalog items: %w", err)
	}

	items := make([]domain.ComputeCatalogItem, 0, len(rows))
	for _, row := range rows {
		item, err := store.ToComputeCatalogItem(row)
		if err != nil {
			return nil, 0, fmt.Errorf("convert catalog item: %w", err)
		}
		items = append(items, item)
	}

	return items, total, nil
}

// GetCatalogInstance retrieves a single compute instance specification by provider and instance type ID.
func (s *CatalogService) GetCatalogInstance(ctx context.Context, provider, instanceTypeID string) (*domain.ComputeCatalogItem, error) {
	row, err := s.queries.GetComputeCatalogItem(ctx, store.GetComputeCatalogItemParams{
		Provider:       provider,
		InstanceTypeID: instanceTypeID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCatalogItemNotFound
		}
		return nil, fmt.Errorf("get compute catalog item: %w", err)
	}

	item, err := store.ToComputeCatalogItem(row)
	if err != nil {
		return nil, fmt.Errorf("convert catalog item: %w", err)
	}
	return &item, nil
}

// UpsertInstance persists or updates a compute instance specification in the catalog.
func (s *CatalogService) UpsertInstance(ctx context.Context, item domain.ComputeCatalogItem) (int64, error) {
	params, err := store.ToUpsertComputeCatalogItemParams(item)
	if err != nil {
		return 0, fmt.Errorf("build upsert params: %w", err)
	}
	id, err := s.queries.UpsertComputeCatalogItem(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("upsert compute catalog item: %w", err)
	}
	return id, nil
}
