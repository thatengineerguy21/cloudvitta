package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

type mockCatalogQuerier struct {
	store.Querier
	getCatalogSummaryFunc        func(ctx context.Context) ([]store.GetComputeCatalogSummaryRow, error)
	getTotalInstancesFunc        func(ctx context.Context) ([]store.GetTotalComputeInstanceCountByProviderRow, error)
	listCatalogItemsFunc         func(ctx context.Context, arg store.ListComputeCatalogItemsParams) ([]store.ComputeInstanceCatalog, error)
	countCatalogItemsFunc        func(ctx context.Context, arg store.CountComputeCatalogItemsParams) (int64, error)
	getCatalogItemFunc           func(ctx context.Context, arg store.GetComputeCatalogItemParams) (store.ComputeInstanceCatalog, error)
	upsertComputeCatalogItemFunc func(ctx context.Context, arg store.UpsertComputeCatalogItemParams) (int64, error)
}

func (m *mockCatalogQuerier) GetComputeCatalogSummary(ctx context.Context) ([]store.GetComputeCatalogSummaryRow, error) {
	if m.getCatalogSummaryFunc != nil {
		return m.getCatalogSummaryFunc(ctx)
	}
	return nil, errors.New("GetComputeCatalogSummary not implemented")
}

func (m *mockCatalogQuerier) GetTotalComputeInstanceCountByProvider(ctx context.Context) ([]store.GetTotalComputeInstanceCountByProviderRow, error) {
	if m.getTotalInstancesFunc != nil {
		return m.getTotalInstancesFunc(ctx)
	}
	return nil, errors.New("GetTotalComputeInstanceCountByProvider not implemented")
}

func (m *mockCatalogQuerier) ListComputeCatalogItems(ctx context.Context, arg store.ListComputeCatalogItemsParams) ([]store.ComputeInstanceCatalog, error) {
	if m.listCatalogItemsFunc != nil {
		return m.listCatalogItemsFunc(ctx, arg)
	}
	return nil, errors.New("ListComputeCatalogItems not implemented")
}

func (m *mockCatalogQuerier) CountComputeCatalogItems(ctx context.Context, arg store.CountComputeCatalogItemsParams) (int64, error) {
	if m.countCatalogItemsFunc != nil {
		return m.countCatalogItemsFunc(ctx, arg)
	}
	return 0, errors.New("CountComputeCatalogItems not implemented")
}

func (m *mockCatalogQuerier) GetComputeCatalogItem(ctx context.Context, arg store.GetComputeCatalogItemParams) (store.ComputeInstanceCatalog, error) {
	if m.getCatalogItemFunc != nil {
		return m.getCatalogItemFunc(ctx, arg)
	}
	return store.ComputeInstanceCatalog{}, errors.New("GetComputeCatalogItem not implemented")
}

func (m *mockCatalogQuerier) UpsertComputeCatalogItem(ctx context.Context, arg store.UpsertComputeCatalogItemParams) (int64, error) {
	if m.upsertComputeCatalogItemFunc != nil {
		return m.upsertComputeCatalogItemFunc(ctx, arg)
	}
	return 0, errors.New("UpsertComputeCatalogItem not implemented")
}

func TestCatalogService_GetCatalogSummary_CacheMissAndHit(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	dbCallCount := 0
	mockQ := &mockCatalogQuerier{
		getTotalInstancesFunc: func(ctx context.Context) ([]store.GetTotalComputeInstanceCountByProviderRow, error) {
			dbCallCount++
			return []store.GetTotalComputeInstanceCountByProviderRow{
				{Provider: "aws", TotalInstances: 100},
				{Provider: "azure", TotalInstances: 150},
			}, nil
		},
		getCatalogSummaryFunc: func(ctx context.Context) ([]store.GetComputeCatalogSummaryRow, error) {
			return []store.GetComputeCatalogSummaryRow{
				{Provider: "aws", Category: "general_purpose", InstanceCount: 60},
				{Provider: "aws", Category: "compute_optimized", InstanceCount: 40},
				{Provider: "azure", Category: "general_purpose", InstanceCount: 150},
			}, nil
		},
	}

	svc := service.NewCatalogService(mockQ, rdb)
	ctx := context.Background()

	// First call: cache miss, should hit DB
	summary, err := svc.GetCatalogSummary(ctx)
	if err != nil {
		t.Fatalf("unexpected error on cache miss: %v", err)
	}
	if summary.TotalInstances != 250 {
		t.Errorf("expected 250 total instances, got %d", summary.TotalInstances)
	}
	if summary.ProviderTotals["aws"] != 100 || summary.ProviderTotals["azure"] != 150 {
		t.Errorf("unexpected provider totals: %+v", summary.ProviderTotals)
	}
	if summary.CategoryBreakdown["aws"]["general_purpose"] != 60 {
		t.Errorf("unexpected aws breakdown: %+v", summary.CategoryBreakdown["aws"])
	}
	if dbCallCount != 1 {
		t.Errorf("expected 1 DB call, got %d", dbCallCount)
	}

	// Verify Redis has the key
	key := cache.BuildCatalogSummaryKey(cache.SchemaVersion)
	val, err := rdb.Get(ctx, key).Result()
	if err != nil || val == "" {
		t.Fatalf("expected key %s to be set in Redis, err: %v", key, err)
	}

	// Second call: should hit Redis cache, no DB call
	summary2, err := svc.GetCatalogSummary(ctx)
	if err != nil {
		t.Fatalf("unexpected error on cache hit: %v", err)
	}
	if summary2.TotalInstances != 250 {
		t.Errorf("expected 250 total instances on cache hit, got %d", summary2.TotalInstances)
	}
	if dbCallCount != 1 {
		t.Errorf("expected still 1 DB call due to cache, got %d", dbCallCount)
	}
}

func TestCatalogService_GetCatalogSummary_NilRedis(t *testing.T) {
	mockQ := &mockCatalogQuerier{
		getTotalInstancesFunc: func(ctx context.Context) ([]store.GetTotalComputeInstanceCountByProviderRow, error) {
			return []store.GetTotalComputeInstanceCountByProviderRow{
				{Provider: "gcp", TotalInstances: 50},
			}, nil
		},
		getCatalogSummaryFunc: func(ctx context.Context) ([]store.GetComputeCatalogSummaryRow, error) {
			return []store.GetComputeCatalogSummaryRow{
				{Provider: "gcp", Category: "compute_optimized", InstanceCount: 50},
			}, nil
		},
	}

	svc := service.NewCatalogService(mockQ, nil)
	summary, err := svc.GetCatalogSummary(context.Background())
	if err != nil {
		t.Fatalf("unexpected error with nil redis: %v", err)
	}
	if summary.TotalInstances != 50 {
		t.Errorf("expected 50 total instances, got %d", summary.TotalInstances)
	}
}

func TestCatalogService_ListCatalogInstances(t *testing.T) {
	now := time.Now().UTC()
	var numVcpu, numMem pgtype.Numeric
	_ = numVcpu.Scan("4.00")
	_ = numMem.Scan("16.00")

	mockRows := []store.ComputeInstanceCatalog{
		{
			ID:              1,
			Provider:        "aws",
			InstanceTypeID:  "m6i.xlarge",
			DisplayName:     "General Purpose m6i.xlarge",
			InstanceFamily:  "m6i",
			Category:        "general_purpose",
			Vcpu:            numVcpu,
			MemoryGib:       numMem,
			CpuArchitecture: "x86_64",
			GpuCount:        0,
			IsBurstable:     false,
			IsCurrentGen:    true,
			FirstSeenAt:     store.TimestamptzFromTime(now),
			LastSeenAt:      store.TimestamptzFromTime(now),
			Attributes:      []byte(`{"vcpu":4,"ram_gb":16,"family":"m6i"}`),
		},
	}

	mockQ := &mockCatalogQuerier{
		countCatalogItemsFunc: func(ctx context.Context, arg store.CountComputeCatalogItemsParams) (int64, error) {
			return 1, nil
		},
		listCatalogItemsFunc: func(ctx context.Context, arg store.ListComputeCatalogItemsParams) ([]store.ComputeInstanceCatalog, error) {
			if arg.Limit != 50 {
				t.Errorf("expected default limit 50, got %d", arg.Limit)
			}
			return mockRows, nil
		},
	}

	svc := service.NewCatalogService(mockQ, nil)
	provider := "aws"
	items, total, err := svc.ListCatalogInstances(context.Background(), domain.CatalogFilter{
		Provider: &provider,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].InstanceTypeID != "m6i.xlarge" || items[0].VCPU != 4 || items[0].MemoryGiB != 16 {
		t.Errorf("unexpected item fields: %+v", items[0])
	}
}

func TestCatalogService_GetCatalogInstance(t *testing.T) {
	now := time.Now().UTC()
	var numVcpu, numMem pgtype.Numeric
	_ = numVcpu.Scan("8.00")
	_ = numMem.Scan("32.00")

	mockQ := &mockCatalogQuerier{
		getCatalogItemFunc: func(ctx context.Context, arg store.GetComputeCatalogItemParams) (store.ComputeInstanceCatalog, error) {
			if arg.Provider == "aws" && arg.InstanceTypeID == "c6i.2xlarge" {
				return store.ComputeInstanceCatalog{
					ID:              42,
					Provider:        "aws",
					InstanceTypeID:  "c6i.2xlarge",
					DisplayName:     "Compute Optimized c6i.2xlarge",
					InstanceFamily:  "c6i",
					Category:        "compute_optimized",
					Vcpu:            numVcpu,
					MemoryGib:       numMem,
					CpuArchitecture: "x86_64",
					FirstSeenAt:     store.TimestamptzFromTime(now),
					LastSeenAt:      store.TimestamptzFromTime(now),
				}, nil
			}
			return store.ComputeInstanceCatalog{}, pgx.ErrNoRows
		},
	}

	svc := service.NewCatalogService(mockQ, nil)
	ctx := context.Background()

	item, err := svc.GetCatalogInstance(ctx, "aws", "c6i.2xlarge")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ID != 42 || item.VCPU != 8 || item.MemoryGiB != 32 {
		t.Errorf("unexpected item: %+v", item)
	}

	// Not found check
	_, err = svc.GetCatalogInstance(ctx, "aws", "nonexistent")
	if !errors.Is(err, service.ErrCatalogItemNotFound) {
		t.Errorf("expected ErrCatalogItemNotFound, got %v", err)
	}
}

func TestCatalogService_UpsertInstance(t *testing.T) {
	mockQ := &mockCatalogQuerier{
		upsertComputeCatalogItemFunc: func(ctx context.Context, arg store.UpsertComputeCatalogItemParams) (int64, error) {
			if arg.InstanceTypeID == "t4g.small" {
				return 99, nil
			}
			return 0, errors.New("unexpected SKU")
		},
	}

	svc := service.NewCatalogService(mockQ, nil)
	id, err := svc.UpsertInstance(context.Background(), domain.ComputeCatalogItem{
		Provider:        "aws",
		InstanceTypeID:  "t4g.small",
		DisplayName:     "Burstable t4g.small",
		InstanceFamily:  "t4g",
		Category:        "general_purpose",
		VCPU:            2,
		MemoryGiB:       2,
		CPUArchitecture: "arm64",
		IsBurstable:     true,
		IsCurrentGen:    true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 99 {
		t.Errorf("expected id 99, got %d", id)
	}
}
