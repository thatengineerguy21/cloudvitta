package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5/pgtype"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/redis/go-redis/v9"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/mcp"
)

type mockCatalogQuerierForMCP struct {
	store.Querier
	listCatalogItemsFunc  func(ctx context.Context, arg store.ListComputeCatalogItemsParams) ([]store.ComputeInstanceCatalog, error)
	countCatalogItemsFunc func(ctx context.Context, arg store.CountComputeCatalogItemsParams) (int64, error)
}

func (m *mockCatalogQuerierForMCP) ListComputeCatalogItems(ctx context.Context, arg store.ListComputeCatalogItemsParams) ([]store.ComputeInstanceCatalog, error) {
	if m.listCatalogItemsFunc != nil {
		return m.listCatalogItemsFunc(ctx, arg)
	}
	return nil, errors.New("ListComputeCatalogItems not implemented")
}

func (m *mockCatalogQuerierForMCP) CountComputeCatalogItems(ctx context.Context, arg store.CountComputeCatalogItemsParams) (int64, error) {
	if m.countCatalogItemsFunc != nil {
		return m.countCatalogItemsFunc(ctx, arg)
	}
	return 0, errors.New("CountComputeCatalogItems not implemented")
}

func TestMCP_GetComputeCatalog(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	freshnessSvc := service.NewFreshnessService(nil, nil)
	pricingSvc := service.NewPricingService(nil, rdb, service.WithFreshnessService(freshnessSvc))

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

	mockQ := &mockCatalogQuerierForMCP{
		countCatalogItemsFunc: func(ctx context.Context, arg store.CountComputeCatalogItemsParams) (int64, error) {
			return 1, nil
		},
		listCatalogItemsFunc: func(ctx context.Context, arg store.ListComputeCatalogItemsParams) ([]store.ComputeInstanceCatalog, error) {
			return mockRows, nil
		},
	}

	catalogSvc := service.NewCatalogService(mockQ, rdb)
	server := mcp.NewServer(pricingSvc, freshnessSvc, mcp.WithCatalogService(catalogSvc))

	ctx := context.Background()
	clientSession, cleanup := connectTestClient(ctx, t, server)
	defer cleanup()

	// 1. List tools and verify get_compute_catalog is registered
	toolsRes, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	found := false
	for _, tool := range toolsRes.Tools {
		if tool.Name == "get_compute_catalog" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected get_compute_catalog tool to be registered")
	}

	// 2. Call tool successfully
	args := map[string]any{
		"provider": "aws",
		"category": "compute_optimized",
		"min_vcpu": 2.0,
		"max_vcpu": 8.0,
	}
	callRes, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name:      "get_compute_catalog",
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	if len(callRes.Content) == 0 {
		t.Fatalf("expected content in call result")
	}

	text := callRes.Content[0].(*sdk.TextContent).Text
	var output mcp.ComputeCatalogOutput
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if output.Count != 1 || output.Total != 1 {
		t.Errorf("expected count=1, total=1, got count=%d, total=%d", output.Count, output.Total)
	}
	if len(output.Instances) != 1 || output.Instances[0].InstanceTypeID != "c7g.xlarge" {
		t.Errorf("unexpected instances in output: %+v", output.Instances)
	}
	if output.Instances[0].CPUArchitecture != "arm64" {
		t.Errorf("expected arm64, got %s", output.Instances[0].CPUArchitecture)
	}

	// 3. Negative validation: invalid negative vcpu
	badArgs := map[string]any{
		"min_vcpu": -1.0,
	}
	badRes, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name:      "get_compute_catalog",
		Arguments: badArgs,
	})
	if err == nil && (badRes == nil || !badRes.IsError) {
		t.Errorf("expected error result for negative min_vcpu")
	}
}
