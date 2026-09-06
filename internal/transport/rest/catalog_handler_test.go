package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest"
)

type mockCatalogQuerierForREST struct {
	store.Querier
	getCatalogSummaryFunc func(ctx context.Context) ([]store.GetComputeCatalogSummaryRow, error)
	getTotalInstancesFunc func(ctx context.Context) ([]store.GetTotalComputeInstanceCountByProviderRow, error)
	listCatalogItemsFunc  func(ctx context.Context, arg store.ListComputeCatalogItemsParams) ([]store.ComputeInstanceCatalog, error)
	countCatalogItemsFunc func(ctx context.Context, arg store.CountComputeCatalogItemsParams) (int64, error)
}

func (m *mockCatalogQuerierForREST) GetComputeCatalogSummary(ctx context.Context) ([]store.GetComputeCatalogSummaryRow, error) {
	if m.getCatalogSummaryFunc != nil {
		return m.getCatalogSummaryFunc(ctx)
	}
	return nil, errors.New("GetComputeCatalogSummary not implemented")
}

func (m *mockCatalogQuerierForREST) GetTotalComputeInstanceCountByProvider(ctx context.Context) ([]store.GetTotalComputeInstanceCountByProviderRow, error) {
	if m.getTotalInstancesFunc != nil {
		return m.getTotalInstancesFunc(ctx)
	}
	return nil, errors.New("GetTotalComputeInstanceCountByProvider not implemented")
}

func (m *mockCatalogQuerierForREST) ListComputeCatalogItems(ctx context.Context, arg store.ListComputeCatalogItemsParams) ([]store.ComputeInstanceCatalog, error) {
	if m.listCatalogItemsFunc != nil {
		return m.listCatalogItemsFunc(ctx, arg)
	}
	return nil, errors.New("ListComputeCatalogItems not implemented")
}

func (m *mockCatalogQuerierForREST) CountComputeCatalogItems(ctx context.Context, arg store.CountComputeCatalogItemsParams) (int64, error) {
	if m.countCatalogItemsFunc != nil {
		return m.countCatalogItemsFunc(ctx, arg)
	}
	return 0, errors.New("CountComputeCatalogItems not implemented")
}

func TestCatalogSummaryHandler_Success(t *testing.T) {
	mockQ := &mockCatalogQuerierForREST{
		getTotalInstancesFunc: func(ctx context.Context) ([]store.GetTotalComputeInstanceCountByProviderRow, error) {
			return []store.GetTotalComputeInstanceCountByProviderRow{
				{Provider: "aws", TotalInstances: 10},
				{Provider: "azure", TotalInstances: 20},
			}, nil
		},
		getCatalogSummaryFunc: func(ctx context.Context) ([]store.GetComputeCatalogSummaryRow, error) {
			return []store.GetComputeCatalogSummaryRow{
				{Provider: "aws", Category: "general_purpose", InstanceCount: 6},
				{Provider: "aws", Category: "compute_optimized", InstanceCount: 4},
				{Provider: "azure", Category: "general_purpose", InstanceCount: 20},
			}, nil
		},
	}

	catalogSvc := service.NewCatalogService(mockQ, nil)
	handler := rest.NewCatalogSummaryHandler(catalogSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/compute/summary", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp rest.CatalogSummaryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.TotalInstances != 30 {
		t.Errorf("expected 30 total instances, got %d", resp.TotalInstances)
	}
	if resp.ProviderTotals["aws"] != 10 || resp.ProviderTotals["azure"] != 20 {
		t.Errorf("unexpected provider totals: %+v", resp.ProviderTotals)
	}
	if resp.CategoryBreakdown["aws"]["general_purpose"] != 6 {
		t.Errorf("unexpected category breakdown: %+v", resp.CategoryBreakdown)
	}
}

func TestCatalogSummaryHandler_MethodNotAllowed(t *testing.T) {
	catalogSvc := service.NewCatalogService(&mockCatalogQuerierForREST{}, nil)
	handler := rest.NewCatalogSummaryHandler(catalogSvc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/compute/summary", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestCatalogInstancesHandler_Success(t *testing.T) {
	now := time.Now().UTC()
	var numVcpu, numMem pgtype.Numeric
	_ = numVcpu.Scan("4.00")
	_ = numMem.Scan("16.00")

	mockRows := []store.ComputeInstanceCatalog{
		{
			ID:              101,
			Provider:        "aws",
			InstanceTypeID:  "c7g.xlarge",
			DisplayName:     "Compute Optimized c7g.xlarge",
			InstanceFamily:  "c7g",
			Category:        "compute_optimized",
			Vcpu:            numVcpu,
			MemoryGib:       numMem,
			CpuArchitecture: "arm64",
			GpuCount:        0,
			IsBurstable:     false,
			IsCurrentGen:    true,
			FirstSeenAt:     store.TimestamptzFromTime(now),
			LastSeenAt:      store.TimestamptzFromTime(now),
			Attributes:      []byte(`{"vcpu":4,"ram_gb":16,"family":"c7g"}`),
		},
	}

	mockQ := &mockCatalogQuerierForREST{
		countCatalogItemsFunc: func(ctx context.Context, arg store.CountComputeCatalogItemsParams) (int64, error) {
			return 1, nil
		},
		listCatalogItemsFunc: func(ctx context.Context, arg store.ListComputeCatalogItemsParams) ([]store.ComputeInstanceCatalog, error) {
			return mockRows, nil
		},
	}

	catalogSvc := service.NewCatalogService(mockQ, nil)
	handler := rest.NewCatalogInstancesHandler(catalogSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/compute/instances?provider=aws&category=compute_optimized&min_vcpu=2&max_vcpu=8&limit=10&offset=0", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp rest.CatalogInstancesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Count != 1 || resp.Total != 1 {
		t.Errorf("expected count=1, total=1, got count=%d, total=%d", resp.Count, resp.Total)
	}
	if len(resp.Instances) != 1 || resp.Instances[0].InstanceTypeID != "c7g.xlarge" {
		t.Errorf("unexpected instances: %+v", resp.Instances)
	}
	if resp.Instances[0].CPUArchitecture != "arm64" {
		t.Errorf("expected arm64, got %s", resp.Instances[0].CPUArchitecture)
	}
}

func TestCatalogInstancesHandler_InvalidParams(t *testing.T) {
	catalogSvc := service.NewCatalogService(&mockCatalogQuerierForREST{}, nil)
	handler := rest.NewCatalogInstancesHandler(catalogSvc)

	badRequests := []string{
		"/api/v1/catalog/compute/instances?min_vcpu=-1",
		"/api/v1/catalog/compute/instances?max_vcpu=abc",
		"/api/v1/catalog/compute/instances?min_memory_gib=-5",
		"/api/v1/catalog/compute/instances?max_memory_gib=invalid",
		"/api/v1/catalog/compute/instances?limit=0",
		"/api/v1/catalog/compute/instances?limit=-10",
		"/api/v1/catalog/compute/instances?offset=-1",
	}

	for _, path := range badRequests {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for path %s, got %d", path, rec.Code)
		}
	}
}
