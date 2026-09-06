package mcp_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/mcp"
)

func setupTestServices(t *testing.T) (*service.PricingService, *service.FreshnessService, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	freshnessSvc := service.NewFreshnessService(nil, nil)
	pricingSvc := service.NewPricingService(nil, rdb, service.WithFreshnessService(freshnessSvc))

	return pricingSvc, freshnessSvc, mr, rdb
}

func connectTestClient(ctx context.Context, t *testing.T, server *sdk.Server) (*sdk.ClientSession, func()) {
	t.Helper()
	serverTransport, clientTransport := sdk.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect failed: %v", err)
	}

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect failed: %v", err)
	}

	cleanup := func() {
		_ = clientSession.Close()
		_ = serverSession.Close()
	}

	return clientSession, cleanup
}

func seedComputeObservations(ctx context.Context, t *testing.T, rdb *redis.Client) {
	t.Helper()
	awsObs := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "compute",
			SkuID:           "SKU-AWS-T3-MED",
			DisplayName:     "t3.medium",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			Unit:            "hour",
			PriceAmount:     decimal.RequireFromString("0.0416"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "general_purpose"},
			FetchedAt:       time.Now().UTC(),
		},
	}
	azureObs := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "compute",
			SkuID:           "SKU-AZ-D2S-V5",
			DisplayName:     "Standard_D2s_v5",
			Region:          "eastus",
			RegionGroup:     "us-east",
			Unit:            "hour",
			PriceAmount:     decimal.RequireFromString("0.048"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "general_purpose"},
			FetchedAt:       time.Now().UTC(),
		},
	}
	gcpObs := []domain.PriceObservation{
		{
			Provider:        "gcp",
			ServiceCategory: "compute",
			SkuID:           "SKU-GCP-E2-STD-2",
			DisplayName:     "e2-standard-2",
			Region:          "us-east4",
			RegionGroup:     "us-east",
			Unit:            "hour",
			PriceAmount:     decimal.RequireFromString("0.067"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "general_purpose"},
			FetchedAt:       time.Now().UTC(),
		},
	}

	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1"), awsObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "compute", "eastus"), azureObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "compute", "us-east4"), gcpObs, cache.DefaultTTL)
}

func seedStorageObservations(ctx context.Context, t *testing.T, rdb *redis.Client) {
	t.Helper()
	awsObs := []domain.PriceObservation{
		{
			Provider:          "aws",
			ServiceCategory:   "storage",
			SkuID:             "SKU-AWS-S3-STD",
			DisplayName:       "S3 Standard",
			Region:            "us-east-1",
			RegionGroup:       "us-east",
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.023"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 100, StorageClass: "standard"},
			FetchedAt:         time.Now().UTC(),
		},
	}
	azureObs := []domain.PriceObservation{
		{
			Provider:          "azure",
			ServiceCategory:   "storage",
			SkuID:             "SKU-AZ-BLOB-HOT",
			DisplayName:       "Blob Hot",
			Region:            "eastus",
			RegionGroup:       "us-east",
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.020"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 100, StorageClass: "standard"},
			FetchedAt:         time.Now().UTC(),
		},
	}
	gcpObs := []domain.PriceObservation{
		{
			Provider:          "gcp",
			ServiceCategory:   "storage",
			SkuID:             "SKU-GCP-GCS-STD",
			DisplayName:       "Standard Storage",
			Region:            "us-east4",
			RegionGroup:       "us-east",
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.020"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 100, StorageClass: "standard"},
			FetchedAt:         time.Now().UTC(),
		},
	}

	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "storage", "us-east-1"), awsObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "storage", "eastus"), azureObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "storage", "us-east4"), gcpObs, cache.DefaultTTL)
}

func seedNetworkObservations(ctx context.Context, t *testing.T, rdb *redis.Client) {
	t.Helper()
	awsObs := []domain.PriceObservation{
		{
			Provider:          "aws",
			ServiceCategory:   "network",
			SkuID:             "SKU-AWS-NET-EGRESS",
			DisplayName:       "Data Transfer Out",
			Region:            "us-east-1",
			RegionGroup:       "us-east",
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.09"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 50, TransferType: "internet_egress"},
			FetchedAt:         time.Now().UTC(),
		},
	}
	azureObs := []domain.PriceObservation{
		{
			Provider:          "azure",
			ServiceCategory:   "network",
			SkuID:             "SKU-AZ-NET-EGRESS",
			DisplayName:       "Bandwidth Out",
			Region:            "eastus",
			RegionGroup:       "us-east",
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.087"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 50, TransferType: "internet_egress"},
			FetchedAt:         time.Now().UTC(),
		},
	}
	gcpObs := []domain.PriceObservation{
		{
			Provider:          "gcp",
			ServiceCategory:   "network",
			SkuID:             "SKU-GCP-NET-EGRESS",
			DisplayName:       "Internet Egress",
			Region:            "us-east4",
			RegionGroup:       "us-east",
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.085"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 50, TransferType: "internet_egress"},
			FetchedAt:         time.Now().UTC(),
		},
	}

	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "network", "us-east-1"), awsObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "network", "eastus"), azureObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "network", "us-east4"), gcpObs, cache.DefaultTTL)
}

func seedDatabaseObservations(ctx context.Context, t *testing.T, rdb *redis.Client) {
	t.Helper()
	now := time.Now().UTC()
	awsObs := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "database_rdbms",
			SkuID:           "AWS-RDS-PG-4VCORE",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.RequireFromString("0.26"),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:         "postgresql",
				VCPU:           4,
				RAMGB:          16,
				DeploymentTier: "standard",
				ComponentType:  "instance",
			},
			FetchedAt: now,
		},
		{
			Provider:        "aws",
			ServiceCategory: "database_rdbms",
			SkuID:           "AWS-RDS-STORAGE-GP3",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.RequireFromString("0.115"),
			PriceCurrency:   "USD",
			Unit:            "GB-Mo",
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:        "any",
				StorageGB:     1,
				StorageFamily: "gp3",
				ComponentType: "storage",
			},
			FetchedAt: now,
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "database_rdbms", "us-east-1"), awsObs, cache.DefaultTTL)
}

func seedDatabaseNoSQLObservations(ctx context.Context, t *testing.T, rdb *redis.Client) {
	t.Helper()
	now := time.Now().UTC()
	awsObs := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AWS-DDB-READ",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.RequireFromString("0.00013"),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				ReadUnits:     1,
				ComponentType: "throughput",
			},
			FetchedAt: now,
		},
		{
			Provider:        "aws",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AWS-DDB-WRITE",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.RequireFromString("0.00065"),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				WriteUnits:    1,
				ComponentType: "throughput",
			},
			FetchedAt: now,
		},
		{
			Provider:        "aws",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AWS-DDB-STORAGE",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.RequireFromString("0.25"),
			PriceCurrency:   "USD",
			Unit:            "GB-Mo",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				StorageGB:     1,
				StorageClass:  "standard",
				ComponentType: "storage",
			},
			FetchedAt: now,
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "database_nosql", "us-east-1"), awsObs, cache.DefaultTTL)
}

func TestMCP_ToolsList(t *testing.T) {
	pricingSvc, freshnessSvc, mr, rdb := setupTestServices(t)
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	// Include CatalogService to match production wiring (10 tools total).
	mockQ := &mockCatalogQuerierForMCP{
		countCatalogItemsFunc: func(ctx context.Context, arg store.CountComputeCatalogItemsParams) (int64, error) {
			return 0, nil
		},
		listCatalogItemsFunc: func(ctx context.Context, arg store.ListComputeCatalogItemsParams) ([]store.ComputeInstanceCatalog, error) {
			return nil, nil
		},
	}
	catalogSvc := service.NewCatalogService(mockQ, rdb)

	ctx := context.Background()
	server := mcp.NewServer(pricingSvc, freshnessSvc, mcp.WithCatalogService(catalogSvc))
	clientSession, cleanup := connectTestClient(ctx, t, server)
	defer cleanup()

	toolsList, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	expectedTools := map[string]bool{
		"compare_compute":        false,
		"compare_storage":        false,
		"compare_network":        false,
		"compare_database":       false,
		"compare_database_nosql": false,
		"compare_kubernetes":     false,
		"compare_serverless":     false,
		"calculate_workload":     false,
		"get_provider_status":    false,
		"get_compute_catalog":    false,
	}

	for _, tool := range toolsList.Tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("expected tool %q in tools list, but not found", name)
		}
	}
	if len(toolsList.Tools) != 10 {
		t.Errorf("expected exactly 10 tools in tools list, got %d", len(toolsList.Tools))
	}
}

func TestMCP_CompareCompute_HappyPath(t *testing.T) {
	pricingSvc, freshnessSvc, mr, rdb := setupTestServices(t)
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	seedComputeObservations(ctx, t, rdb)

	server := mcp.NewServer(pricingSvc, freshnessSvc)
	clientSession, cleanup := connectTestClient(ctx, t, server)
	defer cleanup()

	vcpu := 2.0
	ramGB := 4.0
	res, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name: "compare_compute",
		Arguments: map[string]any{
			"vcpu":   vcpu,
			"ram_gb": ramGB,
			"region": "us-east",
		},
	})
	if err != nil {
		t.Fatalf("CallTool compare_compute failed: %v", err)
	}

	if res.IsError {
		t.Fatalf("expected CallToolResult success, got error: %+v", res)
	}

	if len(res.Content) == 0 {
		t.Fatal("expected non-empty tool response content")
	}

	text, ok := res.Content[0].(*sdk.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}

	var resp mcp.ComputeComparisonResponse
	if err := json.Unmarshal([]byte(text.Text), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON response: %v", err)
	}

	if len(resp.Results) < 3 {
		t.Errorf("expected at least 3 provider results, got %d", len(resp.Results))
	}
	if len(resp.Warnings) < 4 {
		t.Errorf("expected at least 4 stage-3 warnings, got %d", len(resp.Warnings))
	}
}

func TestMCP_CompareCompute_ValidationErrors(t *testing.T) {
	pricingSvc, freshnessSvc, mr, rdb := setupTestServices(t)
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	server := mcp.NewServer(pricingSvc, freshnessSvc)
	clientSession, cleanup := connectTestClient(ctx, t, server)
	defer cleanup()

	// Test negative vCPU
	res, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name: "compare_compute",
		Arguments: map[string]any{
			"vcpu": -1.0,
		},
	})
	if err == nil && !res.IsError {
		t.Errorf("expected error or isError for negative vCPU, got: %+v", res)
	}

	// Test negative RAM
	res2, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name: "compare_compute",
		Arguments: map[string]any{
			"ram_gb": -5.0,
		},
	})
	if err == nil && !res2.IsError {
		t.Errorf("expected error or isError for negative RAM, got: %+v", res2)
	}
}

func TestMCP_CompareStorage_HappyPath(t *testing.T) {
	pricingSvc, freshnessSvc, mr, rdb := setupTestServices(t)
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	seedStorageObservations(ctx, t, rdb)

	server := mcp.NewServer(pricingSvc, freshnessSvc)
	clientSession, cleanup := connectTestClient(ctx, t, server)
	defer cleanup()

	res, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name: "compare_storage",
		Arguments: map[string]any{
			"size_gb":       100.0,
			"storage_class": "standard",
			"region":        "us-east",
		},
	})
	if err != nil {
		t.Fatalf("CallTool compare_storage failed: %v", err)
	}

	if res.IsError {
		t.Fatalf("expected CallToolResult success, got error: %+v", res)
	}

	text, ok := res.Content[0].(*sdk.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}

	var resp mcp.StorageComparisonResponse
	if err := json.Unmarshal([]byte(text.Text), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if len(resp.Results) < 3 {
		t.Errorf("expected at least 3 storage results, got %d", len(resp.Results))
	}
}

func TestMCP_CompareStorage_Validation(t *testing.T) {
	pricingSvc, freshnessSvc, mr, rdb := setupTestServices(t)
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	server := mcp.NewServer(pricingSvc, freshnessSvc)
	clientSession, cleanup := connectTestClient(ctx, t, server)
	defer cleanup()

	res, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name: "compare_storage",
		Arguments: map[string]any{
			"size_gb": 2_000_000.0,
		},
	})
	if err == nil && !res.IsError {
		t.Errorf("expected error for size_gb > 1,000,000, got: %+v", res)
	}
}

func TestMCP_CompareNetwork_HappyPath(t *testing.T) {
	pricingSvc, freshnessSvc, mr, rdb := setupTestServices(t)
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	seedNetworkObservations(ctx, t, rdb)

	server := mcp.NewServer(pricingSvc, freshnessSvc)
	clientSession, cleanup := connectTestClient(ctx, t, server)
	defer cleanup()

	res, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name: "compare_network",
		Arguments: map[string]any{
			"egress_gb":     50.0,
			"transfer_type": "internet_egress",
			"region":        "us-east",
		},
	})
	if err != nil {
		t.Fatalf("CallTool compare_network failed: %v", err)
	}

	if res.IsError {
		t.Fatalf("expected CallToolResult success, got error: %+v", res)
	}

	text, ok := res.Content[0].(*sdk.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}

	var resp mcp.NetworkComparisonResponse
	if err := json.Unmarshal([]byte(text.Text), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if len(resp.Results) < 3 {
		t.Errorf("expected at least 3 network results, got %d", len(resp.Results))
	}
}

func TestMCP_CalculateWorkload_Complete(t *testing.T) {
	pricingSvc, freshnessSvc, mr, rdb := setupTestServices(t)
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	seedComputeObservations(ctx, t, rdb)
	seedStorageObservations(ctx, t, rdb)
	seedNetworkObservations(ctx, t, rdb)

	server := mcp.NewServer(pricingSvc, freshnessSvc)
	clientSession, cleanup := connectTestClient(ctx, t, server)
	defer cleanup()

	res, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name: "calculate_workload",
		Arguments: map[string]any{
			"region": "us-east",
			"compute": map[string]any{
				"vcpu":   2.0,
				"ram_gb": 4.0,
			},
			"storage": map[string]any{
				"size_gb":       100.0,
				"storage_class": "standard",
			},
			"network": map[string]any{
				"egress_gb":     50.0,
				"transfer_type": "internet_egress",
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool calculate_workload failed: %v", err)
	}

	if res.IsError {
		t.Fatalf("expected CallToolResult success, got error: %+v", res)
	}

	text, ok := res.Content[0].(*sdk.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}

	var resp mcp.CalculateResponse
	if err := json.Unmarshal([]byte(text.Text), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if len(resp.Results) < 3 {
		t.Fatalf("expected 3 provider results, got %d", len(resp.Results))
	}
	for _, pr := range resp.Results {
		if pr.Partial {
			t.Errorf("expected full calculation for %s, got partial", pr.Provider)
		}
		if pr.TotalNormalizedHourlyUSD == nil {
			t.Errorf("expected non-nil total hourly USD for %s", pr.Provider)
		}
	}
}

func TestMCP_CalculateWorkload_PartialHonesty(t *testing.T) {
	pricingSvc, freshnessSvc, mr, rdb := setupTestServices(t)
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	// Only seed compute observations; storage and network are missing -> partial calculation
	seedComputeObservations(ctx, t, rdb)

	server := mcp.NewServer(pricingSvc, freshnessSvc)
	clientSession, cleanup := connectTestClient(ctx, t, server)
	defer cleanup()

	res, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name: "calculate_workload",
		Arguments: map[string]any{
			"region": "us-east",
			"compute": map[string]any{
				"vcpu":   2.0,
				"ram_gb": 4.0,
			},
			"storage": map[string]any{
				"size_gb":       100.0,
				"storage_class": "standard",
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool calculate_workload failed: %v", err)
	}

	if res.IsError {
		t.Fatalf("expected CallToolResult success, got error: %+v", res)
	}

	text, ok := res.Content[0].(*sdk.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}

	var resp mcp.CalculateResponse
	if err := json.Unmarshal([]byte(text.Text), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	for _, pr := range resp.Results {
		if !pr.Partial {
			t.Errorf("expected partial: true for %s", pr.Provider)
		}
		if pr.TotalNormalizedHourlyUSD != nil {
			t.Errorf("expected total_normalized_hourly_usd to be nil for partial result in %s", pr.Provider)
		}
		if pr.PartialTotalNormalizedHourlyUSD == nil {
			t.Errorf("expected partial_total_normalized_hourly_usd to be populated for %s", pr.Provider)
		}
	}
}

func TestMCP_CalculateWorkload_Validation_NoCategories(t *testing.T) {
	pricingSvc, freshnessSvc, mr, rdb := setupTestServices(t)
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	server := mcp.NewServer(pricingSvc, freshnessSvc)
	clientSession, cleanup := connectTestClient(ctx, t, server)
	defer cleanup()

	res, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name:      "calculate_workload",
		Arguments: map[string]any{"region": "us-east"},
	})
	if err == nil && !res.IsError {
		t.Errorf("expected error when no categories specified, got: %+v", res)
	}
}

func TestMCP_GetProviderStatus_Success(t *testing.T) {
	pricingSvc, freshnessSvc, mr, rdb := setupTestServices(t)
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	server := mcp.NewServer(pricingSvc, freshnessSvc)
	clientSession, cleanup := connectTestClient(ctx, t, server)
	defer cleanup()

	// Query AWS (supported)
	res, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name: "get_provider_status",
		Arguments: map[string]any{
			"provider": "aws",
		},
	})
	if err != nil {
		t.Fatalf("CallTool get_provider_status failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error: %+v", res)
	}

	// Query Oracle (stage 3 not yet ingested)
	resOracle, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name: "get_provider_status",
		Arguments: map[string]any{
			"provider": "oracle",
		},
	})
	if err != nil {
		t.Fatalf("CallTool get_provider_status oracle failed: %v", err)
	}
	if resOracle.IsError {
		t.Fatalf("expected success for oracle status, got error: %+v", resOracle)
	}
}

func TestMCP_GetProviderStatus_NotFound(t *testing.T) {
	pricingSvc, freshnessSvc, mr, rdb := setupTestServices(t)
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	server := mcp.NewServer(pricingSvc, freshnessSvc)
	clientSession, cleanup := connectTestClient(ctx, t, server)
	defer cleanup()

	res, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name: "get_provider_status",
		Arguments: map[string]any{
			"provider": "unknown_provider",
		},
	})
	if err == nil && !res.IsError {
		t.Errorf("expected error for unknown provider, got: %+v", res)
	}
}

func TestMCP_CompareDatabase_Success(t *testing.T) {
	pricingSvc, freshnessSvc, mr, rdb := setupTestServices(t)
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	seedDatabaseObservations(ctx, t, rdb)

	server := mcp.NewServer(pricingSvc, freshnessSvc)
	clientSession, cleanup := connectTestClient(ctx, t, server)
	defer cleanup()

	res, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name: "compare_database",
		Arguments: map[string]any{
			"engine":     "postgresql",
			"vcpu":       4,
			"ram_gb":     16,
			"storage_gb": 100,
			"region":     "us-east",
		},
	})
	if err != nil {
		t.Fatalf("CallTool compare_database failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error: %+v", res)
	}

	text, ok := res.Content[0].(*sdk.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}

	var resp mcp.DatabaseComparisonResponse
	if err := json.Unmarshal([]byte(text.Text), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("got %d results, want 1 (aws)", len(resp.Results))
	}
	if resp.Results[0].Provider != "aws" {
		t.Errorf("Provider = %q, want aws", resp.Results[0].Provider)
	}
}

func TestMCP_CompareDatabaseNoSQL_Success(t *testing.T) {
	pricingSvc, freshnessSvc, mr, rdb := setupTestServices(t)
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	seedDatabaseNoSQLObservations(ctx, t, rdb)

	server := mcp.NewServer(pricingSvc, freshnessSvc)
	clientSession, cleanup := connectTestClient(ctx, t, server)
	defer cleanup()

	res, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name: "compare_database_nosql",
		Arguments: map[string]any{
			"data_model":   "document",
			"pricing_mode": "provisioned",
			"read_units":   100,
			"write_units":  20,
			"storage_gb":   50,
			"region":       "us-east",
		},
	})
	if err != nil {
		t.Fatalf("CallTool compare_database_nosql failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error: %+v", res)
	}

	text, ok := res.Content[0].(*sdk.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}

	var resp mcp.DatabaseNoSQLComparisonResponse
	if err := json.Unmarshal([]byte(text.Text), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("got %d results, want 1 (aws)", len(resp.Results))
	}
	if resp.Results[0].Provider != "aws" {
		t.Errorf("Provider = %q, want aws", resp.Results[0].Provider)
	}

	t.Run("OndemandPricingModeRejected", func(t *testing.T) {
		errRes, callErr := clientSession.CallTool(ctx, &sdk.CallToolParams{
			Name: "compare_database_nosql",
			Arguments: map[string]any{
				"data_model":   "document",
				"pricing_mode": "ondemand",
				"region":       "us-east",
			},
		})
		if callErr == nil && !errRes.IsError {
			t.Fatal("expected error for invalid pricing_mode 'ondemand', got success")
		}
	})
}

func TestMCP_CalculateWorkload_AliasConflictValidation(t *testing.T) {
	pricingSvc, freshnessSvc, mr, rdb := setupTestServices(t)
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	seedDatabaseObservations(ctx, t, rdb)
	server := mcp.NewServer(pricingSvc, freshnessSvc)
	clientSession, cleanup := connectTestClient(ctx, t, server)
	defer cleanup()

	t.Run("DifferingValues_FailsWithConflictError", func(t *testing.T) {
		res, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
			Name: "calculate_workload",
			Arguments: map[string]any{
				"region": "us-east",
				"database": map[string]any{
					"engine": "postgresql", "vcpu": 2, "ram_gb": 4, "storage_gb": 100,
				},
				"database_rdbms": map[string]any{
					"engine": "mysql", "vcpu": 4, "ram_gb": 16, "storage_gb": 200,
				},
			},
		})
		if err == nil && !res.IsError {
			t.Fatal("expected error for conflicting database and database_rdbms fields")
		}
	})

	t.Run("IdenticalValues_Succeeds", func(t *testing.T) {
		res, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
			Name: "calculate_workload",
			Arguments: map[string]any{
				"region": "us-east",
				"database": map[string]any{
					"engine": "postgresql", "vcpu": 4, "ram_gb": 16, "storage_gb": 100,
				},
				"database_rdbms": map[string]any{
					"engine": "postgresql", "vcpu": 4, "ram_gb": 16, "storage_gb": 100,
				},
			},
		})
		// Empty miniredis returns empty results without error
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError {
			t.Fatalf("unexpected tool error: %+v", res)
		}
	})
}

func TestMCP_CalculateWorkload_SevenCategories(t *testing.T) {
	pricingSvc, freshnessSvc, mr, rdb := setupTestServices(t)
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	seedComputeObservations(ctx, t, rdb)
	seedStorageObservations(ctx, t, rdb)
	seedNetworkObservations(ctx, t, rdb)
	seedDatabaseObservations(ctx, t, rdb)
	seedDatabaseNoSQLObservations(ctx, t, rdb)

	now := time.Now().UTC()
	// Seed Kubernetes
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "kubernetes", "us-east-1"), []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "kubernetes",
			SkuID:           "SKU-AWS-EKS-STANDARD",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.RequireFromString("0.100"),
			PriceCurrency:   "USD",
			Unit:            "hour",
			KubernetesAttributes: domain.KubernetesAttributes{
				Tier: domain.KubernetesTierStandard,
			},
			FetchedAt: now,
		},
	}, cache.DefaultTTL)

	// Seed Serverless
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "serverless", "us-east-1"), []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AWS-LAMBDA-REQ-X86",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.RequireFromString("0.20"),
			PriceCurrency:   "USD",
			Unit:            "per_million_requests",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  "x86_64",
				Tier:          "consumption",
				ComponentType: "request_fee",
				Unit:          "per_million_requests",
			},
			FetchedAt: now,
		},
		{
			Provider:        "aws",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AWS-LAMBDA-DUR-X86",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.RequireFromString("0.0000166667"),
			PriceCurrency:   "USD",
			Unit:            "per_gb_second",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  "x86_64",
				Tier:          "consumption",
				ComponentType: "duration_fee",
				Unit:          "per_gb_second",
			},
			FetchedAt: now,
		},
	}, cache.DefaultTTL)

	server := mcp.NewServer(pricingSvc, freshnessSvc)
	clientSession, cleanup := connectTestClient(ctx, t, server)
	defer cleanup()

	reqArgs := map[string]any{
		"region": "us-east",
		"compute": map[string]any{
			"vcpu":   2,
			"ram_gb": 4,
			"family": "general_purpose",
		},
		"storage": map[string]any{
			"size_gb":       100,
			"storage_class": "standard",
		},
		"network": map[string]any{
			"egress_gb":     50,
			"transfer_type": "internet_egress",
		},
		"database_rdbms": map[string]any{
			"engine":     "postgresql",
			"vcpu":       4,
			"ram_gb":     16,
			"storage_gb": 100,
		},
		"database_nosql": map[string]any{
			"data_model":   "document",
			"pricing_mode": "provisioned",
			"read_units":   100,
			"write_units":  20,
			"storage_gb":   50,
		},
		"kubernetes": map[string]any{
			"tier": "standard",
		},
		"serverless": map[string]any{
			"architecture":          "x86_64",
			"tier":                  "consumption",
			"requests_per_month":    10000000,
			"memory_mb":             512,
			"execution_duration_ms": 200,
		},
	}

	res, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name:      "calculate_workload",
		Arguments: reqArgs,
	})
	if err != nil {
		t.Fatalf("CallTool calculate_workload failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error: %+v", res)
	}

	text, ok := res.Content[0].(*sdk.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}

	var resp mcp.CalculateResponse
	if err := json.Unmarshal([]byte(text.Text), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	var awsRes *mcp.CalculateProviderResult
	for i := range resp.Results {
		if resp.Results[i].Provider == "aws" {
			awsRes = &resp.Results[i]
			break
		}
	}

	if awsRes == nil {
		t.Fatalf("expected AWS in calculate_workload results: %+v", resp.Results)
	}
	if awsRes.Partial {
		t.Errorf("expected AWS to have partial=false, got partial=true")
	}
	if awsRes.TotalNormalizedHourlyUSD == nil {
		t.Errorf("expected AWS to have total_normalized_hourly_usd populated")
	}
	if len(awsRes.Categories) != 7 {
		t.Errorf("expected 7 categories for AWS, got %d", len(awsRes.Categories))
	}
}
