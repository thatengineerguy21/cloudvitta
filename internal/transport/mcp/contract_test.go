package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/auth"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/config"
	"github.com/thatengineerguy21/CloudVitta/internal/dlq"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/middleware/authmw"
	"github.com/thatengineerguy21/CloudVitta/internal/middleware/ratelimit"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/mcp"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest"
)

func ptrDecimal(d decimal.Decimal) *decimal.Decimal {
	return &d
}

// --- Mock Querier & DLQ Harness ---

type contractMockQuerier struct {
	store.Querier
	getProviderCategoryStatusFunc func(ctx context.Context, provider string) ([]store.GetProviderCategoryStatusRow, error)
}

func (m *contractMockQuerier) GetProviderCategoryStatus(ctx context.Context, provider string) ([]store.GetProviderCategoryStatusRow, error) {
	if m.getProviderCategoryStatusFunc != nil {
		return m.getProviderCategoryStatusFunc(ctx, provider)
	}
	return nil, nil
}

type contractMockDLQ struct {
	getFunc func(ctx context.Context, provider, category string) (dlq.Entry, error)
}

func (m *contractMockDLQ) Get(ctx context.Context, provider, category string) (dlq.Entry, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, provider, category)
	}
	return dlq.Entry{}, dlq.ErrEntryNotFound
}

// --- Test Harness Construction ---

type contractTestHarness struct {
	mr           *miniredis.Miniredis
	rdb          *redis.Client
	pricingSvc   *service.PricingService
	freshnessSvc *service.FreshnessService
	restRouter   http.Handler
	mcpServer    *sdk.Server
	mcpClient    *sdk.ClientSession
	cleanupMCP   func()
	bearerToken  string
	fixedNow     time.Time
}

func setupContractParityTest(t *testing.T) *contractTestHarness {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	fixedNow := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

	mockQuerier := &contractMockQuerier{
		getProviderCategoryStatusFunc: func(ctx context.Context, provider string) ([]store.GetProviderCategoryStatusRow, error) {
			switch provider {
			case "aws":
				return []store.GetProviderCategoryStatusRow{
					{
						ServiceCategory:  "compute",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						ObservationCount: 450,
					},
					{
						ServiceCategory:  "storage",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						ObservationCount: 120,
					},
					{
						ServiceCategory:  "network",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						ObservationCount: 35,
					},
					{
						ServiceCategory:  "database_rdbms",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						ObservationCount: 80,
					},
					{
						ServiceCategory:  "database_nosql",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						ObservationCount: 60,
					},
					{
						ServiceCategory:  "kubernetes",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						ObservationCount: 20,
					},
					{
						ServiceCategory:  "serverless",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						ObservationCount: 40,
					},
				}, nil
			case "azure":
				// Stale observations (> 168 hours ago)
				return []store.GetProviderCategoryStatusRow{
					{
						ServiceCategory:  "compute",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-200 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-200 * time.Hour), Valid: true},
						ObservationCount: 300,
					},
					{
						ServiceCategory:  "storage",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-200 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-200 * time.Hour), Valid: true},
						ObservationCount: 90,
					},
					{
						ServiceCategory:  "network",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-200 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-200 * time.Hour), Valid: true},
						ObservationCount: 20,
					},
					{
						ServiceCategory:  "database_rdbms",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-200 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-200 * time.Hour), Valid: true},
						ObservationCount: 50,
					},
					{
						ServiceCategory:  "database_nosql",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-200 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-200 * time.Hour), Valid: true},
						ObservationCount: 40,
					},
					{
						ServiceCategory:  "kubernetes",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-200 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-200 * time.Hour), Valid: true},
						ObservationCount: 15,
					},
					{
						ServiceCategory:  "serverless",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-200 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-200 * time.Hour), Valid: true},
						ObservationCount: 30,
					},
				}, nil
			case "gcp":
				return []store.GetProviderCategoryStatusRow{
					{
						ServiceCategory:  "compute",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						ObservationCount: 400,
					},
					{
						ServiceCategory:  "storage",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						ObservationCount: 110,
					},
					{
						ServiceCategory:  "network",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						ObservationCount: 30,
					},
					{
						ServiceCategory:  "database_rdbms",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						ObservationCount: 60,
					},
					{
						ServiceCategory:  "database_nosql",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						ObservationCount: 50,
					},
					{
						ServiceCategory:  "kubernetes",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						ObservationCount: 10,
					},
					{
						ServiceCategory:  "serverless",
						LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-1 * time.Hour), Valid: true},
						ObservationCount: 25,
					},
				}, nil
			default:
				return nil, nil
			}
		},
	}

	mockDLQ := &contractMockDLQ{
		getFunc: func(ctx context.Context, provider, category string) (dlq.Entry, error) {
			if provider == "gcp" && category == "compute" {
				return dlq.Entry{
					Provider:            "gcp",
					Category:            "compute",
					Status:              "failed",
					LastError:           "503 Service Unavailable",
					ConsecutiveFailures: 3,
					Timestamp:           fixedNow.Add(-10 * time.Minute),
				}, nil
			}
			return dlq.Entry{}, dlq.ErrEntryNotFound
		},
	}

	freshnessSvc := service.NewFreshnessService(
		mockQuerier,
		mockDLQ,
		service.WithNowFunc(func() time.Time { return fixedNow }),
		service.WithDefaultThreshold(168*time.Hour),
	)

	pricingSvc := service.NewPricingService(
		nil,
		rdb,
		service.WithFreshnessService(freshnessSvc),
	)

	jwtSecretStr := "super-secret-jwt-signing-key-32b-length!"
	jwtSecret := []byte(jwtSecretStr)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:        jwtSecretStr,
			AnonCookieSecret: "super-secret-cookie-signing-key-32b!",
		},
		RateLimit: config.RateLimitConfig{
			StandardTierRate: 120,
			FreeTierRate:     20,
			IPCeilingRate:    60,
			LoginRate:        10,
		},
		CORS: config.CORSConfig{
			AllowedOrigins:   []string{"https://cloudvitta.dev"},
			AllowCredentials: true,
		},
	}

	restRouter := rest.NewRouter(pricingSvc, nil, freshnessSvc, nil, rdb, cfg)
	mcpServer := mcp.NewServer(pricingSvc, freshnessSvc)

	ctx := context.Background()
	clientSession, cleanupMCP := connectTestClient(ctx, t, mcpServer)

	userID := uuid.New()
	token, err := auth.GenerateAccessToken(userID, "standard", jwtSecret, fixedNow, 15*time.Minute)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	return &contractTestHarness{
		mr:           mr,
		rdb:          rdb,
		pricingSvc:   pricingSvc,
		freshnessSvc: freshnessSvc,
		restRouter:   restRouter,
		mcpServer:    mcpServer,
		mcpClient:    clientSession,
		cleanupMCP:   cleanupMCP,
		bearerToken:  token,
		fixedNow:     fixedNow,
	}
}

func (h *contractTestHarness) Close() {
	if h.cleanupMCP != nil {
		h.cleanupMCP()
	}
	if h.rdb != nil {
		_ = h.rdb.Close()
	}
	if h.mr != nil {
		h.mr.Close()
	}
}

// --- Fixture Seeders ---

func seedContractComputeObservations(ctx context.Context, t *testing.T, rdb *redis.Client, baseTime time.Time) {
	t.Helper()

	awsRegionGroup, err := regionmap.MapRegion("aws", "us-east-1")
	if err != nil {
		t.Fatalf("MapRegion(aws, us-east-1) failed: %v", err)
	}
	azureRegionGroup, err := regionmap.MapRegion("azure", "eastus")
	if err != nil {
		t.Fatalf("MapRegion(azure, eastus) failed: %v", err)
	}
	gcpRegionGroup, err := regionmap.MapRegion("gcp", "us-east4")
	if err != nil {
		t.Fatalf("MapRegion(gcp, us-east4) failed: %v", err)
	}

	awsObs := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "compute",
			SkuID:           "SKU-AWS-T3-MED",
			DisplayName:     "t3.medium",
			Region:          "us-east-1",
			RegionGroup:     awsRegionGroup,
			Unit:            "hour",
			PriceAmount:     decimal.RequireFromString("0.0416"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "general_purpose"},
			FetchedAt:       baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:        "aws",
			ServiceCategory: "compute",
			SkuID:           "SKU-AWS-C5-LRG",
			DisplayName:     "c5.large",
			Region:          "us-east-1",
			RegionGroup:     awsRegionGroup,
			Unit:            "hour",
			PriceAmount:     decimal.RequireFromString("0.0850"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "compute_optimized"},
			FetchedAt:       baseTime.Add(-1 * time.Hour),
		},
	}
	azureObs := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "compute",
			SkuID:           "SKU-AZ-D2S-V5",
			DisplayName:     "Standard_D2s_v5",
			Region:          "eastus",
			RegionGroup:     azureRegionGroup,
			Unit:            "hour",
			PriceAmount:     decimal.RequireFromString("0.0480"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "general_purpose"},
			FetchedAt:       baseTime.Add(-1 * time.Hour),
		},
	}
	gcpObs := []domain.PriceObservation{
		{
			Provider:        "gcp",
			ServiceCategory: "compute",
			SkuID:           "SKU-GCP-E2-STD-2",
			DisplayName:     "e2-standard-2",
			Region:          "us-east4",
			RegionGroup:     gcpRegionGroup,
			Unit:            "hour",
			PriceAmount:     decimal.RequireFromString("0.0670"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "general_purpose"},
			FetchedAt:       baseTime.Add(-1 * time.Hour),
		},
	}

	oracleRegionGroup, err := regionmap.MapRegion("oracle", "us-ashburn-1")
	if err != nil {
		t.Fatalf("MapRegion(oracle, us-ashburn-1) failed: %v", err)
	}
	ibmRegionGroup, err := regionmap.MapRegion("ibm", "us-east")
	if err != nil {
		t.Fatalf("MapRegion(ibm, us-east) failed: %v", err)
	}
	aliRegionGroup, err := regionmap.MapRegion("alibaba", "us-east-1")
	if err != nil {
		t.Fatalf("MapRegion(alibaba, us-east-1) failed: %v", err)
	}
	doRegionGroup, err := regionmap.MapRegion("digitalocean", "nyc3")
	if err != nil {
		t.Fatalf("MapRegion(digitalocean, nyc3) failed: %v", err)
	}

	oracleObs := []domain.PriceObservation{
		{
			Provider:        "oracle",
			ServiceCategory: "compute",
			SkuID:           "SKU-OCI-E4-FLEX-2-4",
			DisplayName:     "VM.Standard.E4.Flex (1 OCPU, 4 GB)",
			Region:          "us-ashburn-1",
			RegionGroup:     oracleRegionGroup,
			Unit:            "hour",
			PriceAmount:     decimal.RequireFromString("0.0250"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "general_purpose"},
			FetchedAt:       baseTime.Add(-1 * time.Hour),
		},
	}
	ibmObs := []domain.PriceObservation{
		{
			Provider:        "ibm",
			ServiceCategory: "compute",
			SkuID:           "SKU-IBM-BX2-2X4",
			DisplayName:     "bx2-2x4",
			Region:          "us-east",
			RegionGroup:     ibmRegionGroup,
			Unit:            "hour",
			PriceAmount:     decimal.RequireFromString("0.0480"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "general_purpose"},
			FetchedAt:       baseTime.Add(-1 * time.Hour),
		},
	}
	aliObs := []domain.PriceObservation{
		{
			Provider:        "alibaba",
			ServiceCategory: "compute",
			SkuID:           "SKU-ALI-ECS-G7-2X4",
			DisplayName:     "ecs.g7.large",
			Region:          "us-east-1",
			RegionGroup:     aliRegionGroup,
			Unit:            "hour",
			PriceAmount:     decimal.RequireFromString("0.0450"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "general_purpose"},
			FetchedAt:       baseTime.Add(-1 * time.Hour),
		},
	}
	doObs := []domain.PriceObservation{
		{
			Provider:        "digitalocean",
			ServiceCategory: "compute",
			SkuID:           "SKU-DO-S-2VCPU-4GB",
			DisplayName:     "s-2vcpu-4gb",
			Region:          "nyc3",
			RegionGroup:     doRegionGroup,
			Unit:            "hour",
			PriceAmount:     decimal.RequireFromString("0.0357"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "general_purpose"},
			FetchedAt:       baseTime.Add(-1 * time.Hour),
		},
	}

	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1"), awsObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "compute", "eastus"), azureObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "compute", "us-east4"), gcpObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "oracle", "compute", "us-ashburn-1"), oracleObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "ibm", "compute", "us-east"), ibmObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "alibaba", "compute", "us-east-1"), aliObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "digitalocean", "compute", "nyc3"), doObs, cache.DefaultTTL)
}

func seedContractStorageObservations(ctx context.Context, t *testing.T, rdb *redis.Client, baseTime time.Time) {
	t.Helper()

	awsRegionGroup, err := regionmap.MapRegion("aws", "us-east-1")
	if err != nil {
		t.Fatalf("MapRegion(aws, us-east-1) failed: %v", err)
	}
	azureRegionGroup, err := regionmap.MapRegion("azure", "eastus")
	if err != nil {
		t.Fatalf("MapRegion(azure, eastus) failed: %v", err)
	}
	gcpRegionGroup, err := regionmap.MapRegion("gcp", "us-east4")
	if err != nil {
		t.Fatalf("MapRegion(gcp, us-east4) failed: %v", err)
	}
	oracleRegionGroup, err := regionmap.MapRegion("oracle", "us-ashburn-1")
	if err != nil {
		t.Fatalf("MapRegion(oracle, us-ashburn-1) failed: %v", err)
	}
	ibmRegionGroup, err := regionmap.MapRegion("ibm", "us-east")
	if err != nil {
		t.Fatalf("MapRegion(ibm, us-east) failed: %v", err)
	}
	aliRegionGroup, err := regionmap.MapRegion("alibaba", "us-east-1")
	if err != nil {
		t.Fatalf("MapRegion(alibaba, us-east-1) failed: %v", err)
	}
	doRegionGroup, err := regionmap.MapRegion("digitalocean", "nyc3")
	if err != nil {
		t.Fatalf("MapRegion(digitalocean, nyc3) failed: %v", err)
	}

	awsObs := []domain.PriceObservation{
		{
			Provider:          "aws",
			ServiceCategory:   "storage",
			SkuID:             "SKU-AWS-S3-STD",
			DisplayName:       "S3 Standard",
			Region:            "us-east-1",
			RegionGroup:       awsRegionGroup,
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.0230"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 100, StorageClass: "standard"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:          "aws",
			ServiceCategory:   "storage",
			SkuID:             "SKU-AWS-S3-IA",
			DisplayName:       "S3 Standard-IA",
			Region:            "us-east-1",
			RegionGroup:       awsRegionGroup,
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.0125"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 500, StorageClass: "infrequent_access"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
	}
	azureObs := []domain.PriceObservation{
		{
			Provider:          "azure",
			ServiceCategory:   "storage",
			SkuID:             "SKU-AZ-BLOB-HOT",
			DisplayName:       "Blob Hot",
			Region:            "eastus",
			RegionGroup:       azureRegionGroup,
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.0200"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 100, StorageClass: "standard"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:          "azure",
			ServiceCategory:   "storage",
			SkuID:             "SKU-AZ-BLOB-COOL",
			DisplayName:       "Blob Cool",
			Region:            "eastus",
			RegionGroup:       azureRegionGroup,
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.0100"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 500, StorageClass: "infrequent_access"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
	}
	gcpObs := []domain.PriceObservation{
		{
			Provider:          "gcp",
			ServiceCategory:   "storage",
			SkuID:             "SKU-GCP-GCS-STD",
			DisplayName:       "Standard Storage",
			Region:            "us-east4",
			RegionGroup:       gcpRegionGroup,
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.0200"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 100, StorageClass: "standard"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:          "gcp",
			ServiceCategory:   "storage",
			SkuID:             "SKU-GCP-GCS-NEAR",
			DisplayName:       "Nearline Storage",
			Region:            "us-east4",
			RegionGroup:       gcpRegionGroup,
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.0100"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 500, StorageClass: "infrequent_access"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
	}
	oracleStorageObs := []domain.PriceObservation{
		{
			Provider:          "oracle",
			ServiceCategory:   "storage",
			SkuID:             "SKU-OCI-OBJ-STD",
			DisplayName:       "Object Storage Standard",
			Region:            "us-ashburn-1",
			RegionGroup:       oracleRegionGroup,
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.0255"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 100, StorageClass: "standard"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
	}
	ibmStorageObs := []domain.PriceObservation{
		{
			Provider:          "ibm",
			ServiceCategory:   "storage",
			SkuID:             "SKU-IBM-COS-STD",
			DisplayName:       "Cloud Object Storage Standard",
			Region:            "us-east",
			RegionGroup:       ibmRegionGroup,
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.0220"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 100, StorageClass: "standard"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
	}
	aliStorageObs := []domain.PriceObservation{
		{
			Provider:          "alibaba",
			ServiceCategory:   "storage",
			SkuID:             "SKU-ALI-OSS-STD",
			DisplayName:       "Object Storage Standard",
			Region:            "us-east-1",
			RegionGroup:       aliRegionGroup,
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.0190"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 100, StorageClass: "standard"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
	}
	doStorageObs := []domain.PriceObservation{
		{
			Provider:          "digitalocean",
			ServiceCategory:   "storage",
			SkuID:             "SKU-DO-SPACES-STD",
			DisplayName:       "Spaces Standard Storage",
			Region:            "nyc3",
			RegionGroup:       doRegionGroup,
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.0200"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 100, StorageClass: "standard"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
	}

	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "storage", "us-east-1"), awsObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "storage", "eastus"), azureObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "storage", "us-east4"), gcpObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "oracle", "storage", "us-ashburn-1"), oracleStorageObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "ibm", "storage", "us-east"), ibmStorageObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "alibaba", "storage", "us-east-1"), aliStorageObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "digitalocean", "storage", "nyc3"), doStorageObs, cache.DefaultTTL)
}

func seedContractNetworkObservations(ctx context.Context, t *testing.T, rdb *redis.Client, baseTime time.Time) {
	t.Helper()

	awsRegionGroup, err := regionmap.MapRegion("aws", "us-east-1")
	if err != nil {
		t.Fatalf("MapRegion(aws, us-east-1) failed: %v", err)
	}
	azureRegionGroup, err := regionmap.MapRegion("azure", "eastus")
	if err != nil {
		t.Fatalf("MapRegion(azure, eastus) failed: %v", err)
	}
	gcpRegionGroup, err := regionmap.MapRegion("gcp", "us-east4")
	if err != nil {
		t.Fatalf("MapRegion(gcp, us-east4) failed: %v", err)
	}
	oracleRegionGroup, err := regionmap.MapRegion("oracle", "us-ashburn-1")
	if err != nil {
		t.Fatalf("MapRegion(oracle, us-ashburn-1) failed: %v", err)
	}
	ibmRegionGroup, err := regionmap.MapRegion("ibm", "us-east")
	if err != nil {
		t.Fatalf("MapRegion(ibm, us-east) failed: %v", err)
	}
	aliRegionGroup, err := regionmap.MapRegion("alibaba", "us-east-1")
	if err != nil {
		t.Fatalf("MapRegion(alibaba, us-east-1) failed: %v", err)
	}
	doRegionGroup, err := regionmap.MapRegion("digitalocean", "nyc3")
	if err != nil {
		t.Fatalf("MapRegion(digitalocean, nyc3) failed: %v", err)
	}

	awsObs := []domain.PriceObservation{
		{
			Provider:          "aws",
			ServiceCategory:   "network",
			SkuID:             "SKU-AWS-NET-EGRESS",
			DisplayName:       "Data Transfer Out",
			Region:            "us-east-1",
			RegionGroup:       awsRegionGroup,
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.0900"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 50, TransferType: "internet_egress"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:          "aws",
			ServiceCategory:   "network",
			SkuID:             "SKU-AWS-NET-INTRA",
			DisplayName:       "Intra-Region Transfer",
			Region:            "us-east-1",
			RegionGroup:       awsRegionGroup,
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.0100"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 100, TransferType: "intra_region"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
	}
	azureObs := []domain.PriceObservation{
		{
			Provider:          "azure",
			ServiceCategory:   "network",
			SkuID:             "SKU-AZ-NET-EGRESS",
			DisplayName:       "Bandwidth Out",
			Region:            "eastus",
			RegionGroup:       azureRegionGroup,
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.0870"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 50, TransferType: "internet_egress"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:          "azure",
			ServiceCategory:   "network",
			SkuID:             "SKU-AZ-NET-INTRA",
			DisplayName:       "Intra-Region VNet Transfer",
			Region:            "eastus",
			RegionGroup:       azureRegionGroup,
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.0100"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 100, TransferType: "intra_region"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
	}
	gcpObs := []domain.PriceObservation{
		{
			Provider:          "gcp",
			ServiceCategory:   "network",
			SkuID:             "SKU-GCP-NET-EGRESS",
			DisplayName:       "Internet Egress",
			Region:            "us-east4",
			RegionGroup:       gcpRegionGroup,
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.0850"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 50, TransferType: "internet_egress"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:          "gcp",
			ServiceCategory:   "network",
			SkuID:             "SKU-GCP-NET-INTRA",
			DisplayName:       "Intra-Region Egress",
			Region:            "us-east4",
			RegionGroup:       gcpRegionGroup,
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.0100"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 100, TransferType: "intra_region"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
	}
	oracleNetworkObs := []domain.PriceObservation{
		{
			Provider:          "oracle",
			ServiceCategory:   "network",
			SkuID:             "SKU-OCI-NET-EGRESS",
			DisplayName:       "Outbound Data Transfer",
			Region:            "us-ashburn-1",
			RegionGroup:       oracleRegionGroup,
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.0085"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 50, TransferType: "internet_egress"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
	}
	ibmNetworkObs := []domain.PriceObservation{
		{
			Provider:          "ibm",
			ServiceCategory:   "network",
			SkuID:             "SKU-IBM-NET-EGRESS",
			DisplayName:       "Public Egress",
			Region:            "us-east",
			RegionGroup:       ibmRegionGroup,
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.0900"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 50, TransferType: "internet_egress"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
	}
	aliNetworkObs := []domain.PriceObservation{
		{
			Provider:          "alibaba",
			ServiceCategory:   "network",
			SkuID:             "SKU-ALI-NET-EGRESS",
			DisplayName:       "PayByTraffic Internet Egress",
			Region:            "us-east-1",
			RegionGroup:       aliRegionGroup,
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.0800"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 50, TransferType: "internet_egress"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
	}
	doNetworkObs := []domain.PriceObservation{
		{
			Provider:          "digitalocean",
			ServiceCategory:   "network",
			SkuID:             "SKU-DO-NET-EGRESS",
			DisplayName:       "Additional Bandwidth Transfer Out",
			Region:            "nyc3",
			RegionGroup:       doRegionGroup,
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.0100"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 50, TransferType: "internet_egress"},
			FetchedAt:         baseTime.Add(-1 * time.Hour),
		},
	}

	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "network", "us-east-1"), awsObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "network", "eastus"), azureObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "network", "us-east4"), gcpObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "oracle", "network", "us-ashburn-1"), oracleNetworkObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "ibm", "network", "us-east"), ibmNetworkObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "alibaba", "network", "us-east-1"), aliNetworkObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "digitalocean", "network", "nyc3"), doNetworkObs, cache.DefaultTTL)
}

func seedContractKubernetesObservations(ctx context.Context, t *testing.T, rdb *redis.Client, baseTime time.Time) {
	t.Helper()

	awsRegionGroup, err := regionmap.MapRegion("aws", "us-east-1")
	if err != nil {
		t.Fatalf("MapRegion(aws, us-east-1) failed: %v", err)
	}
	azureRegionGroup, err := regionmap.MapRegion("azure", "eastus")
	if err != nil {
		t.Fatalf("MapRegion(azure, eastus) failed: %v", err)
	}
	gcpRegionGroup, err := regionmap.MapRegion("gcp", "us-east4")
	if err != nil {
		t.Fatalf("MapRegion(gcp, us-east4) failed: %v", err)
	}

	awsObs := []domain.PriceObservation{
		{
			Provider:             "aws",
			ServiceCategory:      "kubernetes",
			SkuID:                "SKU-AWS-EKS-STANDARD",
			DisplayName:          "Amazon EKS Standard",
			Region:               "us-east-1",
			RegionGroup:          awsRegionGroup,
			Unit:                 "Hrs",
			PriceAmount:          decimal.RequireFromString("0.1000"),
			PriceCurrency:        "USD",
			PricingModel:         "OnDemand",
			KubernetesAttributes: domain.KubernetesAttributes{Tier: domain.KubernetesTierStandard},
			FetchedAt:            baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:             "aws",
			ServiceCategory:      "kubernetes",
			SkuID:                "SKU-AWS-EKS-EXTENDED",
			DisplayName:          "Amazon EKS Extended Support",
			Region:               "us-east-1",
			RegionGroup:          awsRegionGroup,
			Unit:                 "Hrs",
			PriceAmount:          decimal.RequireFromString("0.6000"),
			PriceCurrency:        "USD",
			PricingModel:         "OnDemand",
			KubernetesAttributes: domain.KubernetesAttributes{Tier: domain.KubernetesTierExtendedSupport},
			FetchedAt:            baseTime.Add(-1 * time.Hour),
		},
	}

	azureObs := []domain.PriceObservation{
		{
			Provider:             "azure",
			ServiceCategory:      "kubernetes",
			SkuID:                "SKU-AZURE-AKS-FREE",
			DisplayName:          "Azure Kubernetes Service Free",
			Region:               "eastus",
			RegionGroup:          azureRegionGroup,
			Unit:                 "Hrs",
			PriceAmount:          decimal.Zero,
			PriceCurrency:        "USD",
			PricingModel:         "OnDemand",
			KubernetesAttributes: domain.KubernetesAttributes{Tier: domain.KubernetesTierFree},
			FetchedAt:            baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:             "azure",
			ServiceCategory:      "kubernetes",
			SkuID:                "SKU-AZURE-AKS-STANDARD",
			DisplayName:          "Azure Kubernetes Service Standard",
			Region:               "eastus",
			RegionGroup:          azureRegionGroup,
			Unit:                 "Hrs",
			PriceAmount:          decimal.RequireFromString("0.1000"),
			PriceCurrency:        "USD",
			PricingModel:         "OnDemand",
			KubernetesAttributes: domain.KubernetesAttributes{Tier: domain.KubernetesTierStandard},
			FetchedAt:            baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:             "azure",
			ServiceCategory:      "kubernetes",
			SkuID:                "SKU-AZURE-AKS-EXTENDED",
			DisplayName:          "Azure Kubernetes Service Extended Support",
			Region:               "eastus",
			RegionGroup:          azureRegionGroup,
			Unit:                 "Hrs",
			PriceAmount:          decimal.RequireFromString("0.6000"),
			PriceCurrency:        "USD",
			PricingModel:         "OnDemand",
			KubernetesAttributes: domain.KubernetesAttributes{Tier: domain.KubernetesTierExtendedSupport},
			FetchedAt:            baseTime.Add(-1 * time.Hour),
		},
	}

	gcpObs := []domain.PriceObservation{
		{
			Provider:             "gcp",
			ServiceCategory:      "kubernetes",
			SkuID:                "SKU-GCP-GKE-STANDARD",
			DisplayName:          "GKE Standard Cluster Management Fee",
			Region:               "us-east4",
			RegionGroup:          gcpRegionGroup,
			Unit:                 "hour",
			PriceAmount:          decimal.RequireFromString("0.1000"),
			PriceCurrency:        "USD",
			PricingModel:         "OnDemand",
			KubernetesAttributes: domain.KubernetesAttributes{Tier: domain.KubernetesTierStandard},
			FetchedAt:            baseTime.Add(-1 * time.Hour),
		},
	}

	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "kubernetes", "us-east-1"), awsObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "kubernetes", "eastus"), azureObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "kubernetes", "us-east4"), gcpObs, cache.DefaultTTL)
}

func seedContractServerlessObservations(ctx context.Context, t *testing.T, rdb *redis.Client, baseTime time.Time) {
	t.Helper()

	awsRegionGroup, err := regionmap.MapRegion("aws", "us-east-1")
	if err != nil {
		t.Fatalf("MapRegion(aws, us-east-1) failed: %v", err)
	}
	azureRegionGroup, err := regionmap.MapRegion("azure", "eastus")
	if err != nil {
		t.Fatalf("MapRegion(azure, eastus) failed: %v", err)
	}
	gcpRegionGroup, err := regionmap.MapRegion("gcp", "us-east4")
	if err != nil {
		t.Fatalf("MapRegion(gcp, us-east4) failed: %v", err)
	}

	awsObs := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AWS-REQ-X86",
			DisplayName:     "AWS Lambda Invocation Requests (x86_64)",
			Region:          "us-east-1",
			RegionGroup:     awsRegionGroup,
			Unit:            "Requests",
			PriceAmount:     decimal.RequireFromString("0.2000"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeRequestFee,
				Unit:          domain.UnitPerMillionRequests,
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:        "aws",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AWS-DUR-X86",
			DisplayName:     "AWS Lambda Compute Duration (x86_64)",
			Region:          "us-east-1",
			RegionGroup:     awsRegionGroup,
			Unit:            "Seconds",
			PriceAmount:     decimal.RequireFromString("0.0000166667"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeDurationFee,
				Unit:          domain.UnitPerGBSecond,
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:        "aws",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AWS-REQ-ARM",
			DisplayName:     "AWS Lambda Invocation Requests (arm64)",
			Region:          "us-east-1",
			RegionGroup:     awsRegionGroup,
			Unit:            "Requests",
			PriceAmount:     decimal.RequireFromString("0.2000"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureARM64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeRequestFee,
				Unit:          domain.UnitPerMillionRequests,
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:        "aws",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AWS-DUR-ARM",
			DisplayName:     "AWS Lambda Compute Duration (arm64)",
			Region:          "us-east-1",
			RegionGroup:     awsRegionGroup,
			Unit:            "Seconds",
			PriceAmount:     decimal.RequireFromString("0.0000133334"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureARM64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeDurationFee,
				Unit:          domain.UnitPerGBSecond,
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
	}

	azureObs := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AZURE-FUNCTIONS-REQ",
			DisplayName:     "Azure Functions Standard Total Executions",
			Region:          "eastus",
			RegionGroup:     azureRegionGroup,
			Unit:            "10",
			PriceAmount:     decimal.RequireFromString("0.000002"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeRequestFee,
				Unit:          domain.UnitPer10Requests,
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:        "azure",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AZURE-FUNCTIONS-DUR",
			DisplayName:     "Azure Functions Standard Execution Time",
			Region:          "eastus",
			RegionGroup:     azureRegionGroup,
			Unit:            "1 GB Second",
			PriceAmount:     decimal.RequireFromString("0.000016"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeDurationFee,
				Unit:          domain.UnitPerGBSecond,
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
	}

	gcpObs := []domain.PriceObservation{
		{
			Provider:        "gcp",
			ServiceCategory: "serverless",
			SkuID:           "SKU-GCP-CF-INVOCATIONS",
			DisplayName:     "Cloud Functions Invocations",
			Region:          "us-east4",
			RegionGroup:     gcpRegionGroup,
			Unit:            "Calls",
			PriceAmount:     decimal.RequireFromString("0.0000004"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeRequestFee,
				Unit:          domain.UnitPerRequest,
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:        "gcp",
			ServiceCategory: "serverless",
			SkuID:           "SKU-GCP-CF-CPU-TIME",
			DisplayName:     "Cloud Functions CPU Time",
			Region:          "us-east4",
			RegionGroup:     gcpRegionGroup,
			Unit:            "s",
			PriceAmount:     decimal.RequireFromString("0.0000100"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeDurationFeeCPU,
				Unit:          domain.UnitPerGHzSecond,
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:        "gcp",
			ServiceCategory: "serverless",
			SkuID:           "SKU-GCP-CF-MEM-TIME",
			DisplayName:     "Cloud Functions Memory Time",
			Region:          "us-east4",
			RegionGroup:     gcpRegionGroup,
			Unit:            "GiBy.s",
			PriceAmount:     decimal.RequireFromString("0.0000025"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeDurationFeeMemory,
				Unit:          domain.UnitPerGBSecond,
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
	}

	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "serverless", "us-east-1"), awsObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "serverless", "eastus"), azureObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "serverless", "us-east4"), gcpObs, cache.DefaultTTL)
}

func seedContractDatabaseObservations(ctx context.Context, t *testing.T, rdb *redis.Client, baseTime time.Time) {
	t.Helper()

	awsRegionGroup, err := regionmap.MapRegion("aws", "us-east-1")
	if err != nil {
		t.Fatalf("MapRegion(aws, us-east-1) failed: %v", err)
	}
	azureRegionGroup, err := regionmap.MapRegion("azure", "eastus")
	if err != nil {
		t.Fatalf("MapRegion(azure, eastus) failed: %v", err)
	}
	gcpRegionGroup, err := regionmap.MapRegion("gcp", "us-east4")
	if err != nil {
		t.Fatalf("MapRegion(gcp, us-east4) failed: %v", err)
	}

	awsObs := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "database_rdbms",
			SkuID:           "SKU-AWS-RDS-PG-4VCORE",
			DisplayName:     "db.m5.xlarge PostgreSQL",
			Region:          "us-east-1",
			RegionGroup:     awsRegionGroup,
			PriceAmount:     decimal.RequireFromString("0.2600"),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			PricingModel:    "OnDemand",
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:         "postgresql",
				VCPU:           4,
				RAMGB:          16,
				DeploymentTier: "standard",
				ComponentType:  "instance",
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:        "aws",
			ServiceCategory: "database_rdbms",
			SkuID:           "SKU-AWS-RDS-STORAGE-GP3",
			DisplayName:     "General Purpose SSD (gp3)",
			Region:          "us-east-1",
			RegionGroup:     awsRegionGroup,
			PriceAmount:     decimal.RequireFromString("0.1150"),
			PriceCurrency:   "USD",
			Unit:            "GB-Mo",
			PricingModel:    "OnDemand",
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:        "any",
				StorageGB:     1,
				StorageFamily: "gp3",
				ComponentType: "storage",
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
	}

	azureObs := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "database_rdbms",
			SkuID:           "SKU-AZ-PG-FLEX-4VCORE",
			DisplayName:     "Standard_D4ds_v5 PostgreSQL",
			Region:          "eastus",
			RegionGroup:     azureRegionGroup,
			PriceAmount:     decimal.RequireFromString("0.2800"),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			PricingModel:    "OnDemand",
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:         "postgresql",
				VCPU:           4,
				RAMGB:          16,
				DeploymentTier: "standard",
				ComponentType:  "instance",
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:        "azure",
			ServiceCategory: "database_rdbms",
			SkuID:           "SKU-AZ-PG-STORAGE-SSD",
			DisplayName:     "Managed Disk SSD",
			Region:          "eastus",
			RegionGroup:     azureRegionGroup,
			PriceAmount:     decimal.RequireFromString("0.1150"),
			PriceCurrency:   "USD",
			Unit:            "GB-Mo",
			PricingModel:    "OnDemand",
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:        "any",
				StorageGB:     1,
				StorageFamily: "ssd",
				ComponentType: "storage",
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
	}

	gcpObs := []domain.PriceObservation{
		{
			Provider:        "gcp",
			ServiceCategory: "database_rdbms",
			SkuID:           "SKU-GCP-SQL-PG-4VCORE",
			DisplayName:     "Cloud SQL PostgreSQL Custom 4 vCPU 16GB",
			Region:          "us-east4",
			RegionGroup:     gcpRegionGroup,
			PriceAmount:     decimal.RequireFromString("0.3000"),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			PricingModel:    "OnDemand",
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:         "postgresql",
				VCPU:           4,
				RAMGB:          16,
				DeploymentTier: "standard",
				ComponentType:  "instance",
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:        "gcp",
			ServiceCategory: "database_rdbms",
			SkuID:           "SKU-GCP-SQL-STORAGE-SSD",
			DisplayName:     "Cloud SQL Storage SSD",
			Region:          "us-east4",
			RegionGroup:     gcpRegionGroup,
			PriceAmount:     decimal.RequireFromString("0.1700"),
			PriceCurrency:   "USD",
			Unit:            "GB-Mo",
			PricingModel:    "OnDemand",
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:        "any",
				StorageGB:     1,
				StorageFamily: "ssd",
				ComponentType: "storage",
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
	}

	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "database_rdbms", "us-east-1"), awsObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "database_rdbms", "eastus"), azureObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "database_rdbms", "us-east4"), gcpObs, cache.DefaultTTL)
}

func seedContractDatabaseNoSQLObservations(ctx context.Context, t *testing.T, rdb *redis.Client, baseTime time.Time) {
	t.Helper()

	awsRegionGroup, err := regionmap.MapRegion("aws", "us-east-1")
	if err != nil {
		t.Fatalf("MapRegion(aws, us-east-1) failed: %v", err)
	}
	azureRegionGroup, err := regionmap.MapRegion("azure", "eastus")
	if err != nil {
		t.Fatalf("MapRegion(azure, eastus) failed: %v", err)
	}
	gcpRegionGroup, err := regionmap.MapRegion("gcp", "us-east4")
	if err != nil {
		t.Fatalf("MapRegion(gcp, us-east4) failed: %v", err)
	}

	awsObs := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AWS-DDB-READ",
			DisplayName:     "DynamoDB Read Capacity Unit",
			Region:          "us-east-1",
			RegionGroup:     awsRegionGroup,
			PriceAmount:     decimal.RequireFromString("0.00013"),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			PricingModel:    "OnDemand",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				ReadUnits:     1,
				ComponentType: "throughput",
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:        "aws",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AWS-DDB-WRITE",
			DisplayName:     "DynamoDB Write Capacity Unit",
			Region:          "us-east-1",
			RegionGroup:     awsRegionGroup,
			PriceAmount:     decimal.RequireFromString("0.00065"),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			PricingModel:    "OnDemand",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				WriteUnits:    1,
				ComponentType: "throughput",
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:        "aws",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AWS-DDB-STORAGE",
			DisplayName:     "DynamoDB Standard Storage",
			Region:          "us-east-1",
			RegionGroup:     awsRegionGroup,
			PriceAmount:     decimal.RequireFromString("0.2500"),
			PriceCurrency:   "USD",
			Unit:            "GB-Mo",
			PricingModel:    "OnDemand",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				StorageGB:     1,
				StorageClass:  "standard",
				ComponentType: "storage",
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
	}

	azureObs := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AZ-COSMOS-RU",
			DisplayName:     "Cosmos DB 100 RU/s",
			Region:          "eastus",
			RegionGroup:     azureRegionGroup,
			PriceAmount:     decimal.RequireFromString("0.0080"),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			PricingModel:    "OnDemand",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				ReadUnits:     100,
				WriteUnits:    100,
				ComponentType: "throughput",
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:        "azure",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AZ-COSMOS-STORAGE",
			DisplayName:     "Cosmos DB Standard Storage",
			Region:          "eastus",
			RegionGroup:     azureRegionGroup,
			PriceAmount:     decimal.RequireFromString("0.2500"),
			PriceCurrency:   "USD",
			Unit:            "GB-Mo",
			PricingModel:    "OnDemand",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				StorageGB:     1,
				StorageClass:  "standard",
				ComponentType: "storage",
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
	}

	gcpObs := []domain.PriceObservation{
		{
			Provider:        "gcp",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-GCP-FIRESTORE-READ",
			DisplayName:     "Firestore Document Reads",
			Region:          "us-east4",
			RegionGroup:     gcpRegionGroup,
			PriceAmount:     decimal.RequireFromString("0.0300"),
			PriceCurrency:   "USD",
			Unit:            "100k-ops",
			PricingModel:    "OnDemand",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "on_demand",
				ReadUnits:     100000,
				ComponentType: "throughput",
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:        "gcp",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-GCP-FIRESTORE-WRITE",
			DisplayName:     "Firestore Document Writes",
			Region:          "us-east4",
			RegionGroup:     gcpRegionGroup,
			PriceAmount:     decimal.RequireFromString("0.0900"),
			PriceCurrency:   "USD",
			Unit:            "100k-ops",
			PricingModel:    "OnDemand",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "on_demand",
				WriteUnits:    100000,
				ComponentType: "throughput",
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
		{
			Provider:        "gcp",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-GCP-FIRESTORE-STORAGE",
			DisplayName:     "Firestore Storage",
			Region:          "us-east4",
			RegionGroup:     gcpRegionGroup,
			PriceAmount:     decimal.RequireFromString("0.1800"),
			PriceCurrency:   "USD",
			Unit:            "GB-Mo",
			PricingModel:    "OnDemand",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "on_demand",
				StorageGB:     1,
				StorageClass:  "standard",
				ComponentType: "storage",
			},
			FetchedAt: baseTime.Add(-1 * time.Hour),
		},
	}

	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "database_nosql", "us-east-1"), awsObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "database_nosql", "eastus"), azureObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "database_nosql", "us-east4"), gcpObs, cache.DefaultTTL)
}

// --- Asymmetric Invocation Helpers ---

func invokeREST[T any](t *testing.T, handler http.Handler, method, target string, body any, bearerToken string) (T, int, string) {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("invokeREST: failed to marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req := httptest.NewRequest(method, target, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var res T
	if rr.Code >= 200 && rr.Code < 300 {
		if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
			t.Fatalf("invokeREST: failed to decode response body (status %d): %v\nBody: %s", rr.Code, err, rr.Body.String())
		}
	}

	return res, rr.Code, rr.Body.String()
}

func invokeMCP[T any](ctx context.Context, t *testing.T, client *sdk.ClientSession, toolName string, args any) (T, error) {
	t.Helper()

	var argsMap map[string]any
	if args != nil {
		argsBytes, err := json.Marshal(args)
		if err != nil {
			t.Fatalf("invokeMCP: failed to marshal args: %v", err)
		}
		if err := json.Unmarshal(argsBytes, &argsMap); err != nil {
			t.Fatalf("invokeMCP: failed to unmarshal args into map: %v", err)
		}
	}

	callRes, err := client.CallTool(ctx, &sdk.CallToolParams{
		Name:      toolName,
		Arguments: argsMap,
	})
	if err != nil {
		var zero T
		return zero, err
	}
	if callRes.IsError {
		var zero T
		var errMsg string
		if len(callRes.Content) > 0 {
			if text, ok := callRes.Content[0].(*sdk.TextContent); ok {
				errMsg = text.Text
			}
		}
		if errMsg == "" {
			errMsg = "tool execution indicated failure (isError=true)"
		}
		return zero, errors.New(errMsg)
	}

	if len(callRes.Content) == 0 {
		t.Fatalf("invokeMCP: empty response content for tool %s", toolName)
	}

	text, ok := callRes.Content[0].(*sdk.TextContent)
	if !ok {
		t.Fatalf("invokeMCP: expected TextContent, got %T", callRes.Content[0])
	}

	var res T
	if err := json.Unmarshal([]byte(text.Text), &res); err != nil {
		t.Fatalf("invokeMCP: failed to unmarshal response: %v\nContent: %s", err, text.Text)
	}

	return res, nil
}

// --- Deep Parity Comparators ---

func assertWarningsParity(t *testing.T, contextName string, restWarnings []rest.ProviderWarning, mcpWarnings []mcp.ProviderWarning) {
	t.Helper()

	if len(restWarnings) != len(mcpWarnings) {
		t.Fatalf("%s: warnings count mismatch: REST=%d, MCP=%d", contextName, len(restWarnings), len(mcpWarnings))
	}
	for j := range restWarnings {
		restWarn := restWarnings[j]
		mcpWarn := mcpWarnings[j]
		if restWarn.Provider != mcpWarn.Provider || restWarn.Code != mcpWarn.Code || restWarn.Message != mcpWarn.Message {
			t.Errorf("%s: warning[%d] mismatch: REST=%+v, MCP=%+v", contextName, j, restWarn, mcpWarn)
		}
	}
}

func assertComputeParity(t *testing.T, restResp rest.ComputeComparisonResponse, mcpResp mcp.ComputeComparisonResponse) {
	t.Helper()

	if len(restResp.Results) != len(mcpResp.Results) {
		t.Fatalf("result count mismatch: REST=%d, MCP=%d", len(restResp.Results), len(mcpResp.Results))
	}

	for i := range restResp.Results {
		restItem := restResp.Results[i]
		mcpItem := mcpResp.Results[i]

		if restItem.Provider != mcpItem.Provider {
			t.Errorf("item[%d] provider mismatch: REST=%s, MCP=%s", i, restItem.Provider, mcpItem.Provider)
		}
		if restItem.SkuID != mcpItem.SkuID {
			t.Errorf("item[%d] sku_id mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.SkuID, mcpItem.SkuID)
		}
		if restItem.MatchedSpec != mcpItem.MatchedSpec {
			t.Errorf("item[%d] matched_spec mismatch for %s: REST=%+v, MCP=%+v", i, restItem.Provider, restItem.MatchedSpec, mcpItem.MatchedSpec)
		}
		if restItem.MatchQuality != mcpItem.MatchQuality {
			t.Errorf("item[%d] match_quality mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.MatchQuality, mcpItem.MatchQuality)
		}
		if math.Abs(restItem.MatchDeltaPct-mcpItem.MatchDeltaPct) > 0.0001 {
			t.Errorf("item[%d] match_delta_pct mismatch for %s: REST=%f, MCP=%f", i, restItem.Provider, restItem.MatchDeltaPct, mcpItem.MatchDeltaPct)
		}
		if !reflect.DeepEqual(restItem.MissingAttributes, mcpItem.MissingAttributes) {
			t.Errorf("item[%d] missing_attributes mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.MissingAttributes, mcpItem.MissingAttributes)
		}
		if !restItem.Price.Amount.Equal(mcpItem.Price.Amount) {
			t.Errorf("item[%d] price.amount mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Amount, mcpItem.Price.Amount)
		}
		if restItem.Price.Unit != mcpItem.Price.Unit {
			t.Errorf("item[%d] price.unit mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Unit, mcpItem.Price.Unit)
		}
		if restItem.Price.Currency != mcpItem.Price.Currency {
			t.Errorf("item[%d] price.currency mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Currency, mcpItem.Price.Currency)
		}
		if !restItem.NormalizedHourlyUSD.Equal(mcpItem.NormalizedHourlyUSD) {
			t.Errorf("item[%d] normalized_hourly_usd mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.NormalizedHourlyUSD, mcpItem.NormalizedHourlyUSD)
		}
		if restItem.NormalizedHourlyUSD.IsZero() {
			t.Errorf("item[%d] normalized_hourly_usd is zero for %s (loud typo guard)", i, restItem.Provider)
		}
		if restItem.Stale != mcpItem.Stale {
			t.Errorf("item[%d] stale flag mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.Stale, mcpItem.Stale)
		}
		if !restItem.FetchedAt.Equal(mcpItem.FetchedAt) {
			t.Errorf("item[%d] fetched_at mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.FetchedAt, mcpItem.FetchedAt)
		}
	}

	assertWarningsParity(t, "compute", restResp.Warnings, mcpResp.Warnings)
}

func assertStorageParity(t *testing.T, restResp rest.StorageComparisonResponse, mcpResp mcp.StorageComparisonResponse) {
	t.Helper()

	if len(restResp.Results) != len(mcpResp.Results) {
		t.Fatalf("result count mismatch: REST=%d, MCP=%d", len(restResp.Results), len(mcpResp.Results))
	}

	for i := range restResp.Results {
		restItem := restResp.Results[i]
		mcpItem := mcpResp.Results[i]

		if restItem.Provider != mcpItem.Provider {
			t.Errorf("item[%d] provider mismatch: REST=%s, MCP=%s", i, restItem.Provider, mcpItem.Provider)
		}
		if restItem.SkuID != mcpItem.SkuID {
			t.Errorf("item[%d] sku_id mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.SkuID, mcpItem.SkuID)
		}
		if restItem.MatchedSpec != mcpItem.MatchedSpec {
			t.Errorf("item[%d] matched_spec mismatch for %s: REST=%+v, MCP=%+v", i, restItem.Provider, restItem.MatchedSpec, mcpItem.MatchedSpec)
		}
		if restItem.MatchQuality != mcpItem.MatchQuality {
			t.Errorf("item[%d] match_quality mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.MatchQuality, mcpItem.MatchQuality)
		}
		if math.Abs(restItem.MatchDeltaPct-mcpItem.MatchDeltaPct) > 0.0001 {
			t.Errorf("item[%d] match_delta_pct mismatch for %s: REST=%f, MCP=%f", i, restItem.Provider, restItem.MatchDeltaPct, mcpItem.MatchDeltaPct)
		}
		if !reflect.DeepEqual(restItem.MissingAttributes, mcpItem.MissingAttributes) {
			t.Errorf("item[%d] missing_attributes mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.MissingAttributes, mcpItem.MissingAttributes)
		}
		if !restItem.Price.Amount.Equal(mcpItem.Price.Amount) {
			t.Errorf("item[%d] price.amount mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Amount, mcpItem.Price.Amount)
		}
		if restItem.Price.Unit != mcpItem.Price.Unit {
			t.Errorf("item[%d] price.unit mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Unit, mcpItem.Price.Unit)
		}
		if restItem.Price.Currency != mcpItem.Price.Currency {
			t.Errorf("item[%d] price.currency mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Currency, mcpItem.Price.Currency)
		}
		if !restItem.MonthlyCostUSD.Equal(mcpItem.MonthlyCostUSD) {
			t.Errorf("item[%d] monthly_cost_usd mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.MonthlyCostUSD, mcpItem.MonthlyCostUSD)
		}
		if restItem.MonthlyCostUSD.IsZero() {
			t.Errorf("item[%d] monthly_cost_usd is zero for %s (loud typo guard)", i, restItem.Provider)
		}
		if restItem.Stale != mcpItem.Stale {
			t.Errorf("item[%d] stale flag mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.Stale, mcpItem.Stale)
		}
		if !restItem.FetchedAt.Equal(mcpItem.FetchedAt) {
			t.Errorf("item[%d] fetched_at mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.FetchedAt, mcpItem.FetchedAt)
		}
	}

	assertWarningsParity(t, "storage", restResp.Warnings, mcpResp.Warnings)
}

func assertNetworkParity(t *testing.T, restResp rest.NetworkComparisonResponse, mcpResp mcp.NetworkComparisonResponse) {
	t.Helper()

	if len(restResp.Results) != len(mcpResp.Results) {
		t.Fatalf("result count mismatch: REST=%d, MCP=%d", len(restResp.Results), len(mcpResp.Results))
	}

	for i := range restResp.Results {
		restItem := restResp.Results[i]
		mcpItem := mcpResp.Results[i]

		if restItem.Provider != mcpItem.Provider {
			t.Errorf("item[%d] provider mismatch: REST=%s, MCP=%s", i, restItem.Provider, mcpItem.Provider)
		}
		if restItem.SkuID != mcpItem.SkuID {
			t.Errorf("item[%d] sku_id mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.SkuID, mcpItem.SkuID)
		}
		if restItem.MatchedSpec != mcpItem.MatchedSpec {
			t.Errorf("item[%d] matched_spec mismatch for %s: REST=%+v, MCP=%+v", i, restItem.Provider, restItem.MatchedSpec, mcpItem.MatchedSpec)
		}
		if restItem.MatchQuality != mcpItem.MatchQuality {
			t.Errorf("item[%d] match_quality mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.MatchQuality, mcpItem.MatchQuality)
		}
		if math.Abs(restItem.MatchDeltaPct-mcpItem.MatchDeltaPct) > 0.0001 {
			t.Errorf("item[%d] match_delta_pct mismatch for %s: REST=%f, MCP=%f", i, restItem.Provider, restItem.MatchDeltaPct, mcpItem.MatchDeltaPct)
		}
		if !reflect.DeepEqual(restItem.MissingAttributes, mcpItem.MissingAttributes) {
			t.Errorf("item[%d] missing_attributes mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.MissingAttributes, mcpItem.MissingAttributes)
		}
		if !restItem.Price.Amount.Equal(mcpItem.Price.Amount) {
			t.Errorf("item[%d] price.amount mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Amount, mcpItem.Price.Amount)
		}
		if restItem.Price.Unit != mcpItem.Price.Unit {
			t.Errorf("item[%d] price.unit mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Unit, mcpItem.Price.Unit)
		}
		if restItem.Price.Currency != mcpItem.Price.Currency {
			t.Errorf("item[%d] price.currency mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Currency, mcpItem.Price.Currency)
		}
		if !restItem.MonthlyCostUSD.Equal(mcpItem.MonthlyCostUSD) {
			t.Errorf("item[%d] monthly_cost_usd mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.MonthlyCostUSD, mcpItem.MonthlyCostUSD)
		}
		if restItem.MonthlyCostUSD.IsZero() {
			t.Errorf("item[%d] monthly_cost_usd is zero for %s (loud typo guard)", i, restItem.Provider)
		}
		if restItem.Stale != mcpItem.Stale {
			t.Errorf("item[%d] stale flag mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.Stale, mcpItem.Stale)
		}
		if !restItem.FetchedAt.Equal(mcpItem.FetchedAt) {
			t.Errorf("item[%d] fetched_at mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.FetchedAt, mcpItem.FetchedAt)
		}
	}

	assertWarningsParity(t, "network", restResp.Warnings, mcpResp.Warnings)
}

func assertDatabaseParity(t *testing.T, restResp rest.DatabaseComparisonResponse, mcpResp mcp.DatabaseComparisonResponse) {
	t.Helper()

	if len(restResp.Results) != len(mcpResp.Results) {
		t.Fatalf("result count mismatch: REST=%d, MCP=%d", len(restResp.Results), len(mcpResp.Results))
	}

	for i := range restResp.Results {
		restItem := restResp.Results[i]
		mcpItem := mcpResp.Results[i]

		if restItem.Provider != mcpItem.Provider {
			t.Errorf("item[%d] provider mismatch: REST=%s, MCP=%s", i, restItem.Provider, mcpItem.Provider)
		}
		if restItem.SkuID != mcpItem.SkuID {
			t.Errorf("item[%d] sku_id mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.SkuID, mcpItem.SkuID)
		}
		if restItem.MatchedSpec != mcpItem.MatchedSpec {
			t.Errorf("item[%d] matched_spec mismatch for %s: REST=%+v, MCP=%+v", i, restItem.Provider, restItem.MatchedSpec, mcpItem.MatchedSpec)
		}
		if restItem.MatchQuality != mcpItem.MatchQuality {
			t.Errorf("item[%d] match_quality mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.MatchQuality, mcpItem.MatchQuality)
		}
		if math.Abs(restItem.MatchDeltaPct-mcpItem.MatchDeltaPct) > 0.0001 {
			t.Errorf("item[%d] match_delta_pct mismatch for %s: REST=%f, MCP=%f", i, restItem.Provider, restItem.MatchDeltaPct, mcpItem.MatchDeltaPct)
		}
		if !reflect.DeepEqual(restItem.MissingAttributes, mcpItem.MissingAttributes) {
			t.Errorf("item[%d] missing_attributes mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.MissingAttributes, mcpItem.MissingAttributes)
		}
		if !restItem.Price.Amount.Equal(mcpItem.Price.Amount) {
			t.Errorf("item[%d] price.amount mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Amount, mcpItem.Price.Amount)
		}
		if restItem.Price.Unit != mcpItem.Price.Unit {
			t.Errorf("item[%d] price.unit mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Unit, mcpItem.Price.Unit)
		}
		if restItem.Price.Currency != mcpItem.Price.Currency {
			t.Errorf("item[%d] price.currency mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Currency, mcpItem.Price.Currency)
		}
		if !restItem.NormalizedHourlyUSD.Equal(mcpItem.NormalizedHourlyUSD) {
			t.Errorf("item[%d] normalized_hourly_usd mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.NormalizedHourlyUSD, mcpItem.NormalizedHourlyUSD)
		}
		if restItem.NormalizedHourlyUSD.IsZero() {
			t.Errorf("item[%d] normalized_hourly_usd is zero for %s (loud typo guard)", i, restItem.Provider)
		}
		if restItem.Stale != mcpItem.Stale {
			t.Errorf("item[%d] stale flag mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.Stale, mcpItem.Stale)
		}
		if !restItem.FetchedAt.Equal(mcpItem.FetchedAt) {
			t.Errorf("item[%d] fetched_at mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.FetchedAt, mcpItem.FetchedAt)
		}
	}

	assertWarningsParity(t, "database_rdbms", restResp.Warnings, mcpResp.Warnings)
}

func assertDatabaseNoSQLParity(t *testing.T, restResp rest.DatabaseNoSQLComparisonResponse, mcpResp mcp.DatabaseNoSQLComparisonResponse) {
	t.Helper()

	if len(restResp.Results) != len(mcpResp.Results) {
		t.Fatalf("result count mismatch: REST=%d, MCP=%d", len(restResp.Results), len(mcpResp.Results))
	}

	for i := range restResp.Results {
		restItem := restResp.Results[i]
		mcpItem := mcpResp.Results[i]

		if restItem.Provider != mcpItem.Provider {
			t.Errorf("item[%d] provider mismatch: REST=%s, MCP=%s", i, restItem.Provider, mcpItem.Provider)
		}
		if restItem.SkuID != mcpItem.SkuID {
			t.Errorf("item[%d] sku_id mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.SkuID, mcpItem.SkuID)
		}
		if restItem.MatchedSpec != mcpItem.MatchedSpec {
			t.Errorf("item[%d] matched_spec mismatch for %s: REST=%+v, MCP=%+v", i, restItem.Provider, restItem.MatchedSpec, mcpItem.MatchedSpec)
		}
		if restItem.MatchQuality != mcpItem.MatchQuality {
			t.Errorf("item[%d] match_quality mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.MatchQuality, mcpItem.MatchQuality)
		}
		if math.Abs(restItem.MatchDeltaPct-mcpItem.MatchDeltaPct) > 0.0001 {
			t.Errorf("item[%d] match_delta_pct mismatch for %s: REST=%f, MCP=%f", i, restItem.Provider, restItem.MatchDeltaPct, mcpItem.MatchDeltaPct)
		}
		if !reflect.DeepEqual(restItem.MissingAttributes, mcpItem.MissingAttributes) {
			t.Errorf("item[%d] missing_attributes mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.MissingAttributes, mcpItem.MissingAttributes)
		}
		if !restItem.Price.Amount.Equal(mcpItem.Price.Amount) {
			t.Errorf("item[%d] price.amount mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Amount, mcpItem.Price.Amount)
		}
		if restItem.Price.Unit != mcpItem.Price.Unit {
			t.Errorf("item[%d] price.unit mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Unit, mcpItem.Price.Unit)
		}
		if restItem.Price.Currency != mcpItem.Price.Currency {
			t.Errorf("item[%d] price.currency mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Currency, mcpItem.Price.Currency)
		}
		if !restItem.NormalizedHourlyUSD.Equal(mcpItem.NormalizedHourlyUSD) {
			t.Errorf("item[%d] normalized_hourly_usd mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.NormalizedHourlyUSD, mcpItem.NormalizedHourlyUSD)
		}
		if restItem.NormalizedHourlyUSD.IsZero() {
			t.Errorf("item[%d] normalized_hourly_usd is zero for %s (loud typo guard)", i, restItem.Provider)
		}
		if restItem.Stale != mcpItem.Stale {
			t.Errorf("item[%d] stale flag mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.Stale, mcpItem.Stale)
		}
		if !restItem.FetchedAt.Equal(mcpItem.FetchedAt) {
			t.Errorf("item[%d] fetched_at mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.FetchedAt, mcpItem.FetchedAt)
		}
	}

	assertWarningsParity(t, "database_nosql", restResp.Warnings, mcpResp.Warnings)
}

func assertKubernetesParity(t *testing.T, restResp rest.KubernetesComparisonResponse, mcpResp mcp.KubernetesComparisonResponse) {
	t.Helper()

	if len(restResp.Results) != len(mcpResp.Results) {
		t.Fatalf("result count mismatch: REST=%d, MCP=%d", len(restResp.Results), len(mcpResp.Results))
	}

	for i := range restResp.Results {
		restItem := restResp.Results[i]
		mcpItem := mcpResp.Results[i]

		if restItem.Provider != mcpItem.Provider {
			t.Errorf("item[%d] provider mismatch: REST=%s, MCP=%s", i, restItem.Provider, mcpItem.Provider)
		}
		if restItem.SkuID != mcpItem.SkuID {
			t.Errorf("item[%d] sku_id mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.SkuID, mcpItem.SkuID)
		}
		if restItem.MatchedSpec != mcpItem.MatchedSpec {
			t.Errorf("item[%d] matched_spec mismatch for %s: REST=%+v, MCP=%+v", i, restItem.Provider, restItem.MatchedSpec, mcpItem.MatchedSpec)
		}
		if restItem.MatchQuality != mcpItem.MatchQuality {
			t.Errorf("item[%d] match_quality mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.MatchQuality, mcpItem.MatchQuality)
		}
		if math.Abs(restItem.MatchDeltaPct-mcpItem.MatchDeltaPct) > 0.0001 {
			t.Errorf("item[%d] match_delta_pct mismatch for %s: REST=%f, MCP=%f", i, restItem.Provider, restItem.MatchDeltaPct, mcpItem.MatchDeltaPct)
		}
		if !reflect.DeepEqual(restItem.MissingAttributes, mcpItem.MissingAttributes) {
			t.Errorf("item[%d] missing_attributes mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.MissingAttributes, mcpItem.MissingAttributes)
		}
		if !restItem.Price.Amount.Equal(mcpItem.Price.Amount) {
			t.Errorf("item[%d] price.amount mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Amount, mcpItem.Price.Amount)
		}
		if restItem.Price.Unit != mcpItem.Price.Unit {
			t.Errorf("item[%d] price.unit mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Unit, mcpItem.Price.Unit)
		}
		if restItem.Price.Currency != mcpItem.Price.Currency {
			t.Errorf("item[%d] price.currency mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Currency, mcpItem.Price.Currency)
		}
		if !restItem.NormalizedHourlyUSD.Equal(mcpItem.NormalizedHourlyUSD) {
			t.Errorf("item[%d] normalized_hourly_usd mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.NormalizedHourlyUSD, mcpItem.NormalizedHourlyUSD)
		}
		if restItem.Stale != mcpItem.Stale {
			t.Errorf("item[%d] stale flag mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.Stale, mcpItem.Stale)
		}
		if !restItem.FetchedAt.Equal(mcpItem.FetchedAt) {
			t.Errorf("item[%d] fetched_at mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.FetchedAt, mcpItem.FetchedAt)
		}
	}

	assertWarningsParity(t, "kubernetes", restResp.Warnings, mcpResp.Warnings)
}

func assertServerlessParity(t *testing.T, restResp rest.ServerlessComparisonResponse, mcpResp mcp.ServerlessComparisonResponse) {
	t.Helper()

	if len(restResp.Results) != len(mcpResp.Results) {
		t.Fatalf("result count mismatch: REST=%d, MCP=%d", len(restResp.Results), len(mcpResp.Results))
	}

	for i := range restResp.Results {
		restItem := restResp.Results[i]
		mcpItem := mcpResp.Results[i]

		if restItem.Provider != mcpItem.Provider {
			t.Errorf("item[%d] provider mismatch: REST=%s, MCP=%s", i, restItem.Provider, mcpItem.Provider)
		}
		if restItem.SkuID != mcpItem.SkuID {
			t.Errorf("item[%d] sku_id mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.SkuID, mcpItem.SkuID)
		}
		if restItem.MatchedSpec != mcpItem.MatchedSpec {
			t.Errorf("item[%d] matched_spec mismatch for %s: REST=%+v, MCP=%+v", i, restItem.Provider, restItem.MatchedSpec, mcpItem.MatchedSpec)
		}
		if restItem.MatchQuality != mcpItem.MatchQuality {
			t.Errorf("item[%d] match_quality mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.MatchQuality, mcpItem.MatchQuality)
		}
		if math.Abs(restItem.MatchDeltaPct-mcpItem.MatchDeltaPct) > 0.0001 {
			t.Errorf("item[%d] match_delta_pct mismatch for %s: REST=%f, MCP=%f", i, restItem.Provider, restItem.MatchDeltaPct, mcpItem.MatchDeltaPct)
		}
		if !reflect.DeepEqual(restItem.MissingAttributes, mcpItem.MissingAttributes) {
			t.Errorf("item[%d] missing_attributes mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.MissingAttributes, mcpItem.MissingAttributes)
		}
		if !restItem.Price.Amount.Equal(mcpItem.Price.Amount) {
			t.Errorf("item[%d] price.amount mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Amount, mcpItem.Price.Amount)
		}
		if restItem.Price.Unit != mcpItem.Price.Unit {
			t.Errorf("item[%d] price.unit mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Unit, mcpItem.Price.Unit)
		}
		if restItem.Price.Currency != mcpItem.Price.Currency {
			t.Errorf("item[%d] price.currency mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.Price.Currency, mcpItem.Price.Currency)
		}
		if !restItem.NormalizedHourlyUSD.Equal(mcpItem.NormalizedHourlyUSD) {
			t.Errorf("item[%d] normalized_hourly_usd mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.NormalizedHourlyUSD, mcpItem.NormalizedHourlyUSD)
		}
		if !restItem.NormalizedMonthlyUSD.Equal(mcpItem.NormalizedMonthlyUSD) {
			t.Errorf("item[%d] normalized_monthly_usd mismatch for %s: REST=%s, MCP=%s", i, restItem.Provider, restItem.NormalizedMonthlyUSD, mcpItem.NormalizedMonthlyUSD)
		}
		if restItem.Stale != mcpItem.Stale {
			t.Errorf("item[%d] stale flag mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.Stale, mcpItem.Stale)
		}
		if !restItem.FetchedAt.Equal(mcpItem.FetchedAt) {
			t.Errorf("item[%d] fetched_at mismatch for %s: REST=%v, MCP=%v", i, restItem.Provider, restItem.FetchedAt, mcpItem.FetchedAt)
		}
	}

	assertWarningsParity(t, "serverless", restResp.Warnings, mcpResp.Warnings)
}

func assertCalculateParity(t *testing.T, restResp rest.CalculateResponse, mcpResp mcp.CalculateResponse) {
	t.Helper()

	if len(restResp.Results) != len(mcpResp.Results) {
		t.Fatalf("calculate results count mismatch: REST=%d, MCP=%d", len(restResp.Results), len(mcpResp.Results))
	}

	restMap := make(map[string]rest.CalculateProviderResult)
	for _, r := range restResp.Results {
		restMap[r.Provider] = r
	}

	for _, mcpItem := range mcpResp.Results {
		restItem, ok := restMap[mcpItem.Provider]
		if !ok {
			t.Fatalf("provider %s present in MCP results but missing in REST", mcpItem.Provider)
		}

		if restItem.Partial != mcpItem.Partial {
			t.Errorf("provider %s partial mismatch: REST=%v, MCP=%v", mcpItem.Provider, restItem.Partial, mcpItem.Partial)
		}

		// ADR 0022 Honesty Contract Verification
		if restItem.Partial {
			if restItem.TotalNormalizedHourlyUSD != nil || mcpItem.TotalNormalizedHourlyUSD != nil {
				t.Errorf("provider %s partial result must omit total_normalized_hourly_usd: REST=%v, MCP=%v", mcpItem.Provider, restItem.TotalNormalizedHourlyUSD, mcpItem.TotalNormalizedHourlyUSD)
			}
			if restItem.PartialTotalNormalizedHourlyUSD == nil || mcpItem.PartialTotalNormalizedHourlyUSD == nil {
				t.Errorf("provider %s partial result must populate partial_total_normalized_hourly_usd: REST=%v, MCP=%v", mcpItem.Provider, restItem.PartialTotalNormalizedHourlyUSD, mcpItem.PartialTotalNormalizedHourlyUSD)
			} else {
				if !restItem.PartialTotalNormalizedHourlyUSD.Equal(*mcpItem.PartialTotalNormalizedHourlyUSD) {
					t.Errorf("provider %s partial_total_normalized_hourly_usd mismatch: REST=%s, MCP=%s", mcpItem.Provider, restItem.PartialTotalNormalizedHourlyUSD, mcpItem.PartialTotalNormalizedHourlyUSD)
				}
				if restItem.PartialTotalNormalizedHourlyUSD.IsZero() {
					t.Errorf("provider %s partial_total_normalized_hourly_usd is zero (loud typo guard)", mcpItem.Provider)
				}
			}
		} else {
			if restItem.TotalNormalizedHourlyUSD == nil || mcpItem.TotalNormalizedHourlyUSD == nil {
				t.Errorf("provider %s complete result must populate total_normalized_hourly_usd: REST=%v, MCP=%v", mcpItem.Provider, restItem.TotalNormalizedHourlyUSD, mcpItem.TotalNormalizedHourlyUSD)
			} else {
				if !restItem.TotalNormalizedHourlyUSD.Equal(*mcpItem.TotalNormalizedHourlyUSD) {
					t.Errorf("provider %s total_normalized_hourly_usd mismatch: REST=%s, MCP=%s", mcpItem.Provider, restItem.TotalNormalizedHourlyUSD, mcpItem.TotalNormalizedHourlyUSD)
				}
				if restItem.TotalNormalizedHourlyUSD.IsZero() {
					t.Errorf("provider %s total_normalized_hourly_usd is zero (loud typo guard)", mcpItem.Provider)
				}
			}
			if restItem.PartialTotalNormalizedHourlyUSD != nil || mcpItem.PartialTotalNormalizedHourlyUSD != nil {
				t.Errorf("provider %s complete result must omit partial_total_normalized_hourly_usd: REST=%v, MCP=%v", mcpItem.Provider, restItem.PartialTotalNormalizedHourlyUSD, mcpItem.PartialTotalNormalizedHourlyUSD)
			}
		}

		if len(restItem.Categories) != len(mcpItem.Categories) {
			t.Fatalf("provider %s categories count mismatch: REST=%d, MCP=%d", mcpItem.Provider, len(restItem.Categories), len(mcpItem.Categories))
		}

		for catKey, mcpCategory := range mcpItem.Categories {
			restCategory, catExists := restItem.Categories[catKey]
			if !catExists {
				t.Fatalf("provider %s category %s present in MCP but missing in REST", mcpItem.Provider, catKey)
			}

			if restCategory.SkuID != mcpCategory.SkuID {
				t.Errorf("provider %s category %s sku_id mismatch: REST=%s, MCP=%s", mcpItem.Provider, catKey, restCategory.SkuID, mcpCategory.SkuID)
			}
			if restCategory.MatchQuality != mcpCategory.MatchQuality {
				t.Errorf("provider %s category %s match_quality mismatch: REST=%s, MCP=%s", mcpItem.Provider, catKey, restCategory.MatchQuality, mcpCategory.MatchQuality)
			}
			if math.Abs(restCategory.MatchDeltaPct-mcpCategory.MatchDeltaPct) > 0.0001 {
				t.Errorf("provider %s category %s match_delta_pct mismatch: REST=%f, MCP=%f", mcpItem.Provider, catKey, restCategory.MatchDeltaPct, mcpCategory.MatchDeltaPct)
			}
			if !reflect.DeepEqual(restCategory.MissingAttributes, mcpCategory.MissingAttributes) {
				t.Errorf("provider %s category %s missing_attributes mismatch: REST=%v, MCP=%v", mcpItem.Provider, catKey, restCategory.MissingAttributes, mcpCategory.MissingAttributes)
			}
			if restCategory.Stale != mcpCategory.Stale {
				t.Errorf("provider %s category %s stale mismatch: REST=%v, MCP=%v", mcpItem.Provider, catKey, restCategory.Stale, mcpCategory.Stale)
			}
			if !restCategory.NormalizedHourlyUSD.Equal(mcpCategory.NormalizedHourlyUSD) {
				t.Errorf("provider %s category %s normalized_hourly_usd mismatch: REST=%s, MCP=%s", mcpItem.Provider, catKey, restCategory.NormalizedHourlyUSD, mcpCategory.NormalizedHourlyUSD)
			}
			if restCategory.NormalizedHourlyUSD.IsZero() {
				t.Errorf("provider %s category %s normalized_hourly_usd is zero (loud typo guard)", mcpItem.Provider, catKey)
			}
		}
	}

	assertWarningsParity(t, "calculate", restResp.Warnings, mcpResp.Warnings)
}

func assertProviderStatusParity(t *testing.T, restResp rest.ProviderStatusResponse, mcpResp domain.ProviderStatus) {
	t.Helper()

	if restResp.Provider != mcpResp.Provider {
		t.Errorf("provider mismatch: REST=%s, MCP=%s", restResp.Provider, mcpResp.Provider)
	}
	if restResp.Status != mcpResp.Status {
		t.Errorf("status mismatch for %s: REST=%s, MCP=%s", restResp.Provider, restResp.Status, mcpResp.Status)
	}
	if restResp.Stale != mcpResp.Stale {
		t.Errorf("stale mismatch for %s: REST=%v, MCP=%v", restResp.Provider, restResp.Stale, mcpResp.Stale)
	}

	if (restResp.LastSuccessfulFetch == nil) != (mcpResp.LastSuccessfulFetch == nil) {
		t.Errorf("last_successful_fetch nilness mismatch for %s: REST=%v, MCP=%v", restResp.Provider, restResp.LastSuccessfulFetch, mcpResp.LastSuccessfulFetch)
	} else if restResp.LastSuccessfulFetch != nil && !restResp.LastSuccessfulFetch.Equal(*mcpResp.LastSuccessfulFetch) {
		t.Errorf("last_successful_fetch timestamp mismatch for %s: REST=%v, MCP=%v", restResp.Provider, restResp.LastSuccessfulFetch, mcpResp.LastSuccessfulFetch)
	}

	if len(restResp.Categories) != len(mcpResp.Categories) {
		t.Fatalf("categories count mismatch for %s: REST=%d, MCP=%d", restResp.Provider, len(restResp.Categories), len(mcpResp.Categories))
	}

	for catKey, mcpCat := range mcpResp.Categories {
		restCat, ok := restResp.Categories[catKey]
		if !ok {
			t.Fatalf("category %s missing in REST status response for %s", catKey, restResp.Provider)
		}

		if restCat.Category != mcpCat.Category {
			t.Errorf("category name mismatch for %s:%s: REST=%s, MCP=%s", restResp.Provider, catKey, restCat.Category, mcpCat.Category)
		}
		if restCat.Supported != mcpCat.Supported {
			t.Errorf("supported flag mismatch for %s:%s: REST=%v, MCP=%v", restResp.Provider, catKey, restCat.Supported, mcpCat.Supported)
		}
		if restCat.ObservationCount != mcpCat.ObservationCount {
			t.Errorf("observation_count mismatch for %s:%s: REST=%d, MCP=%d", restResp.Provider, catKey, restCat.ObservationCount, mcpCat.ObservationCount)
		}
		if restCat.Stale != mcpCat.Stale {
			t.Errorf("category stale flag mismatch for %s:%s: REST=%v, MCP=%v", restResp.Provider, catKey, restCat.Stale, mcpCat.Stale)
		}
		if math.Abs(restCat.StalenessThresholdHours-mcpCat.StalenessThresholdHours) > 0.0001 {
			t.Errorf("staleness_threshold_hours mismatch for %s:%s: REST=%f, MCP=%f", restResp.Provider, catKey, restCat.StalenessThresholdHours, mcpCat.StalenessThresholdHours)
		}

		if (restCat.DLQ == nil) != (mcpCat.DLQ == nil) {
			t.Errorf("DLQ nilness mismatch for %s:%s: REST=%v, MCP=%v", restResp.Provider, catKey, restCat.DLQ, mcpCat.DLQ)
		} else if restCat.DLQ != nil {
			if restCat.DLQ.Status != mcpCat.DLQ.Status || restCat.DLQ.LastError != mcpCat.DLQ.LastError || restCat.DLQ.ConsecutiveFailures != mcpCat.DLQ.ConsecutiveFailures {
				t.Errorf("DLQ content mismatch for %s:%s: REST=%+v, MCP=%+v", restResp.Provider, catKey, restCat.DLQ, mcpCat.DLQ)
			}
		}
	}

	if len(restResp.Warnings) != len(mcpResp.Warnings) {
		t.Fatalf("provider status warnings count mismatch for %s: REST=%d, MCP=%d", restResp.Provider, len(restResp.Warnings), len(mcpResp.Warnings))
	}
	for j := range restResp.Warnings {
		restWarn := restResp.Warnings[j]
		mcpWarn := mcpResp.Warnings[j]
		if restWarn.Provider != mcpWarn.Provider || restWarn.Code != mcpWarn.Code || restWarn.Message != mcpWarn.Message {
			t.Errorf("status warning[%d] mismatch for %s: REST=%+v, MCP=%+v", j, restResp.Provider, restWarn, mcpWarn)
		}
	}
}

// --- Test Suites ---

// 1. Dedicated Authentication Policy Divergence Test
func TestAuthPolicyDivergence_RESTAnonymousMCPRequired(t *testing.T) {
	harness := setupContractParityTest(t)
	defer harness.Close()

	ctx := context.Background()
	seedContractComputeObservations(ctx, t, harness.rdb, harness.fixedNow)

	// Wrap MCP handler into an HTTP test server guarded by authmw.RequireAuth
	mcpHandler := mcp.NewStreamableHandler(harness.mcpServer)
	limiter := ratelimit.NewRateLimiter(harness.rdb, ratelimit.Config{
		StandardTierRate: 120,
		FreeTierRate:     20,
		IPCeilingRate:    60,
		CookieSecret:     []byte("super-secret-cookie-signing-key-32b!"),
	})
	authMw := authmw.NewAuthMiddleware([]byte("super-secret-jwt-signing-key-32b-length!"), func() time.Time { return harness.fixedNow })

	mux := http.NewServeMux()
	mux.Handle("/mcp", authmw.RequireAuth(limiter.Handler(mcpHandler)))
	mcpHTTPServer := httptest.NewServer(authMw(mux))
	defer mcpHTTPServer.Close()

	// REST Test: Unauthenticated requests succeed under Anonymous Free Tier
	t.Run("REST_AnonymousFreeTier_Permitted", func(t *testing.T) {
		// 1. Compute
		_, statusCompute, bodyCompute := invokeREST[rest.ComputeComparisonResponse](t, harness.restRouter, http.MethodGet, "/api/v1/prices/compute?vcpu=2&ram_gb=4&region=us-east", nil, "")
		if statusCompute != http.StatusOK {
			t.Errorf("expected REST compute anonymous status 200, got %d: %s", statusCompute, bodyCompute)
		}

		// 2. Composite Calculate
		calcPayload := rest.CalculateRequestBody{
			Region: "us-east",
			Compute: &domain.ComputeAttributes{
				VCPU:   2,
				RAMGB:  4,
				Family: "general_purpose",
			},
		}
		_, statusCalc, bodyCalc := invokeREST[rest.CalculateResponse](t, harness.restRouter, http.MethodPost, "/api/v1/calculate", calcPayload, "")
		if statusCalc != http.StatusOK {
			t.Errorf("expected REST calculate anonymous status 200, got %d: %s", statusCalc, bodyCalc)
		}

		// 3. Provider Status
		_, statusProv, bodyProv := invokeREST[rest.ProviderStatusResponse](t, harness.restRouter, http.MethodGet, "/api/v1/providers/aws/status", nil, "")
		if statusProv != http.StatusOK {
			t.Errorf("expected REST provider status anonymous status 200, got %d: %s", statusProv, bodyProv)
		}
	})

	// MCP Test: Unauthenticated requests are rejected with 401 Unauthorized problem details
	t.Run("MCP_Unauthenticated_RejectedWith401ProblemDetails", func(t *testing.T) {
		reqBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
		req, err := http.NewRequest(http.MethodPost, mcpHTTPServer.URL+"/mcp", bytes.NewReader(reqBody))
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected MCP unauthenticated status 401, got %d", resp.StatusCode)
		}

		if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
			t.Errorf("expected Content-Type application/problem+json, got %s", ct)
		}

		var problem map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&problem); err != nil {
			t.Fatalf("failed to decode problem details JSON: %v", err)
		}

		if problem["title"] != "Unauthorized" {
			t.Errorf("expected problem title 'Unauthorized', got %v", problem["title"])
		}
		if problem["type"] != "https://cloudvitta.dev/errors/unauthorized" {
			t.Errorf("expected problem type 'https://cloudvitta.dev/errors/unauthorized', got %v", problem["type"])
		}
	})

	// MCP Test: Authenticated requests succeed with Standard Tier Bearer JWT
	t.Run("MCP_AuthenticatedBearerJWT_Permitted", func(t *testing.T) {
		clientTransport := &sdk.StreamableClientTransport{
			Endpoint: mcpHTTPServer.URL + "/mcp",
			HTTPClient: &http.Client{
				Transport: &bearerAuthTransport{token: harness.bearerToken},
			},
			DisableStandaloneSSE: true,
		}

		client := sdk.NewClient(&sdk.Implementation{Name: "parity-test-client", Version: "1.0.0"}, nil)
		session, err := client.Connect(ctx, clientTransport, nil)
		if err != nil {
			t.Fatalf("MCP authenticated client connect failed: %v", err)
		}
		defer func() { _ = session.Close() }()

		toolsList, err := session.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("ListTools failed: %v", err)
		}
		if len(toolsList.Tools) < 5 {
			t.Errorf("expected 5 tools, got %d", len(toolsList.Tools))
		}
	})
}

// 2. Compute Contract Parity Suite
func TestContractParity_Compute(t *testing.T) {
	harness := setupContractParityTest(t)
	defer harness.Close()

	ctx := context.Background()
	seedContractComputeObservations(ctx, t, harness.rdb, harness.fixedNow)

	t.Run("StandardSpecMatch", func(t *testing.T) {
		vcpu := 2.0
		ramGB := 4.0
		family := "general_purpose"
		region := "us-east"

		// 1. REST execution
		restResp, code, body := invokeREST[rest.ComputeComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/compute?vcpu=%.0f&ram_gb=%.0f&family=%s&region=%s", vcpu, ramGB, family, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		// 2. MCP execution
		mcpResp, err := invokeMCP[mcp.ComputeComparisonResponse](
			ctx, t, harness.mcpClient, "compare_compute",
			mcp.CompareComputeInput{
				VCPU:   &vcpu,
				RAMGB:  &ramGB,
				Family: family,
				Region: region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		// 3. Parity assertion
		assertComputeParity(t, restResp, mcpResp)
	})

	t.Run("FamilyFallbackRelaxedMatching", func(t *testing.T) {
		vcpu := 2.0
		ramGB := 4.0
		family := "compute_optimized"
		strictFamily := false
		region := "us-east"

		restResp, code, body := invokeREST[rest.ComputeComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/compute?vcpu=%.0f&ram_gb=%.0f&family=%s&strict_family=%v&region=%s", vcpu, ramGB, family, strictFamily, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.ComputeComparisonResponse](
			ctx, t, harness.mcpClient, "compare_compute",
			mcp.CompareComputeInput{
				VCPU:         &vcpu,
				RAMGB:        &ramGB,
				Family:       family,
				StrictFamily: &strictFamily,
				Region:       region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertComputeParity(t, restResp, mcpResp)
	})

	t.Run("NonUSDCurrencyWarning", func(t *testing.T) {
		vcpu := 2.0
		ramGB := 4.0
		currency := "EUR"
		region := "us-east"

		restResp, code, body := invokeREST[rest.ComputeComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/compute?vcpu=%.0f&ram_gb=%.0f&currency=%s&region=%s", vcpu, ramGB, currency, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.ComputeComparisonResponse](
			ctx, t, harness.mcpClient, "compare_compute",
			mcp.CompareComputeInput{
				VCPU:     &vcpu,
				RAMGB:    &ramGB,
				Currency: currency,
				Region:   region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertComputeParity(t, restResp, mcpResp)

		// Assert currency warning presence on both
		hasWarnREST := false
		for _, w := range restResp.Warnings {
			if w.Code == "non_usd_currency_unsupported" {
				hasWarnREST = true
				break
			}
		}
		if !hasWarnREST {
			t.Error("expected non_usd_currency_unsupported warning in REST response")
		}
	})

	t.Run("ValidationRejection", func(t *testing.T) {
		// Test negative vCPU
		_, codeREST, _ := invokeREST[rest.ComputeComparisonResponse](t, harness.restRouter, http.MethodGet, "/api/v1/prices/compute?vcpu=-1", nil, harness.bearerToken)
		if codeREST != http.StatusBadRequest {
			t.Errorf("expected REST 400 for negative vCPU, got %d", codeREST)
		}

		negVCPU := -1.0
		_, errMCP := invokeMCP[mcp.ComputeComparisonResponse](ctx, t, harness.mcpClient, "compare_compute", mcp.CompareComputeInput{VCPU: &negVCPU})
		if errMCP == nil {
			t.Error("expected MCP tool error for negative vCPU, got nil")
		}

		// Test negative RAM
		_, codeRAM, _ := invokeREST[rest.ComputeComparisonResponse](t, harness.restRouter, http.MethodGet, "/api/v1/prices/compute?ram_gb=0", nil, harness.bearerToken)
		if codeRAM != http.StatusBadRequest {
			t.Errorf("expected REST 400 for 0 RAM, got %d", codeRAM)
		}

		zeroRAM := 0.0
		_, errMCPRAM := invokeMCP[mcp.ComputeComparisonResponse](ctx, t, harness.mcpClient, "compare_compute", mcp.CompareComputeInput{RAMGB: &zeroRAM})
		if errMCPRAM == nil {
			t.Error("expected MCP tool error for 0 RAM, got nil")
		}
	})
}

// 3. Storage Contract Parity Suite
func TestContractParity_Storage(t *testing.T) {
	harness := setupContractParityTest(t)
	defer harness.Close()

	ctx := context.Background()
	seedContractStorageObservations(ctx, t, harness.rdb, harness.fixedNow)

	t.Run("StandardObjectStorage", func(t *testing.T) {
		sizeGB := 100.0
		storageClass := "standard"
		region := "us-east"

		restResp, code, body := invokeREST[rest.StorageComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/storage?size_gb=%.0f&storage_class=%s&region=%s", sizeGB, storageClass, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.StorageComparisonResponse](
			ctx, t, harness.mcpClient, "compare_storage",
			mcp.CompareStorageInput{
				SizeGB:       &sizeGB,
				StorageClass: storageClass,
				Region:       region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertStorageParity(t, restResp, mcpResp)
	})

	t.Run("DefaultCapacityBaseline", func(t *testing.T) {
		storageClass := "standard"
		region := "us-east"

		restResp, code, body := invokeREST[rest.StorageComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/storage?storage_class=%s&region=%s", storageClass, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.StorageComparisonResponse](
			ctx, t, harness.mcpClient, "compare_storage",
			mcp.CompareStorageInput{
				StorageClass: storageClass,
				Region:       region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertStorageParity(t, restResp, mcpResp)
	})

	t.Run("InfrequentAccessClass", func(t *testing.T) {
		sizeGB := 500.0
		storageClass := "infrequent_access"
		region := "us-east"

		restResp, code, body := invokeREST[rest.StorageComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/storage?size_gb=%.0f&storage_class=%s&region=%s", sizeGB, storageClass, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.StorageComparisonResponse](
			ctx, t, harness.mcpClient, "compare_storage",
			mcp.CompareStorageInput{
				SizeGB:       &sizeGB,
				StorageClass: storageClass,
				Region:       region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertStorageParity(t, restResp, mcpResp)
	})

	t.Run("BoundaryValidation", func(t *testing.T) {
		// Zero size
		_, codeZero, _ := invokeREST[rest.StorageComparisonResponse](t, harness.restRouter, http.MethodGet, "/api/v1/prices/storage?size_gb=0", nil, harness.bearerToken)
		if codeZero != http.StatusBadRequest {
			t.Errorf("expected REST 400 for 0 size_gb, got %d", codeZero)
		}

		zeroSize := 0.0
		_, errZeroMCP := invokeMCP[mcp.StorageComparisonResponse](ctx, t, harness.mcpClient, "compare_storage", mcp.CompareStorageInput{SizeGB: &zeroSize})
		if errZeroMCP == nil {
			t.Error("expected MCP tool error for 0 size_gb, got nil")
		}

		// Exceeds 1 PB (1,000,001 GB)
		_, codeOver, _ := invokeREST[rest.StorageComparisonResponse](t, harness.restRouter, http.MethodGet, "/api/v1/prices/storage?size_gb=1000001", nil, harness.bearerToken)
		if codeOver != http.StatusBadRequest {
			t.Errorf("expected REST 400 for size_gb > 1 PB, got %d", codeOver)
		}

		overSize := 1_000_001.0
		_, errOverMCP := invokeMCP[mcp.StorageComparisonResponse](ctx, t, harness.mcpClient, "compare_storage", mcp.CompareStorageInput{SizeGB: &overSize})
		if errOverMCP == nil {
			t.Error("expected MCP tool error for size_gb > 1 PB, got nil")
		}
	})
}

// 4. Network Contract Parity Suite
func TestContractParity_Network(t *testing.T) {
	harness := setupContractParityTest(t)
	defer harness.Close()

	ctx := context.Background()
	seedContractNetworkObservations(ctx, t, harness.rdb, harness.fixedNow)

	t.Run("InternetEgress", func(t *testing.T) {
		egressGB := 50.0
		transferType := "internet_egress"
		region := "us-east"

		restResp, code, body := invokeREST[rest.NetworkComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/network?egress_gb=%.0f&transfer_type=%s&region=%s", egressGB, transferType, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.NetworkComparisonResponse](
			ctx, t, harness.mcpClient, "compare_network",
			mcp.CompareNetworkInput{
				EgressGB:     &egressGB,
				TransferType: transferType,
				Region:       region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertNetworkParity(t, restResp, mcpResp)
	})

	t.Run("DefaultEgressBaseline", func(t *testing.T) {
		transferType := "internet_egress"
		region := "us-east"

		restResp, code, body := invokeREST[rest.NetworkComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/network?transfer_type=%s&region=%s", transferType, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.NetworkComparisonResponse](
			ctx, t, harness.mcpClient, "compare_network",
			mcp.CompareNetworkInput{
				TransferType: transferType,
				Region:       region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertNetworkParity(t, restResp, mcpResp)
	})

	t.Run("IntraRegionTransfer", func(t *testing.T) {
		egressGB := 100.0
		transferType := "intra_region"
		region := "us-east"

		restResp, code, body := invokeREST[rest.NetworkComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/network?egress_gb=%.0f&transfer_type=%s&region=%s", egressGB, transferType, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.NetworkComparisonResponse](
			ctx, t, harness.mcpClient, "compare_network",
			mcp.CompareNetworkInput{
				EgressGB:     &egressGB,
				TransferType: transferType,
				Region:       region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertNetworkParity(t, restResp, mcpResp)
	})

	t.Run("BoundaryValidation", func(t *testing.T) {
		// Negative egress
		_, codeNeg, _ := invokeREST[rest.NetworkComparisonResponse](t, harness.restRouter, http.MethodGet, "/api/v1/prices/network?egress_gb=-5", nil, harness.bearerToken)
		if codeNeg != http.StatusBadRequest {
			t.Errorf("expected REST 400 for negative egress_gb, got %d", codeNeg)
		}

		negEgress := -5.0
		_, errNegMCP := invokeMCP[mcp.NetworkComparisonResponse](ctx, t, harness.mcpClient, "compare_network", mcp.CompareNetworkInput{EgressGB: &negEgress})
		if errNegMCP == nil {
			t.Error("expected MCP tool error for negative egress_gb, got nil")
		}

		// Exceeds 10 PB (10,000,001 GB)
		_, codeOver, _ := invokeREST[rest.NetworkComparisonResponse](t, harness.restRouter, http.MethodGet, "/api/v1/prices/network?egress_gb=10000001", nil, harness.bearerToken)
		if codeOver != http.StatusBadRequest {
			t.Errorf("expected REST 400 for egress_gb > 10 PB, got %d", codeOver)
		}

		overEgress := 10_000_001.0
		_, errOverMCP := invokeMCP[mcp.NetworkComparisonResponse](ctx, t, harness.mcpClient, "compare_network", mcp.CompareNetworkInput{EgressGB: &overEgress})
		if errOverMCP == nil {
			t.Error("expected MCP tool error for egress_gb > 10 PB, got nil")
		}
	})
}

// Database (RDBMS) Contract Parity Suite
func TestContractParity_Database(t *testing.T) {
	harness := setupContractParityTest(t)
	defer harness.Close()

	ctx := context.Background()
	seedContractDatabaseObservations(ctx, t, harness.rdb, harness.fixedNow)

	t.Run("PostgreSQLMatch", func(t *testing.T) {
		engine := "postgresql"
		vcpu := 4.0
		ramGB := 16.0
		storageGB := 100.0
		region := "us-east"

		restResp, code, body := invokeREST[rest.DatabaseComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/database?engine=%s&vcpu=%.0f&ram_gb=%.0f&storage_gb=%.0f&region=%s", engine, vcpu, ramGB, storageGB, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.DatabaseComparisonResponse](
			ctx, t, harness.mcpClient, "compare_database",
			mcp.CompareDatabaseInput{
				Engine:    engine,
				VCPU:      &vcpu,
				RAMGB:     &ramGB,
				StorageGB: &storageGB,
				Region:    region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertDatabaseParity(t, restResp, mcpResp)
	})

	t.Run("MultiAZDeployment", func(t *testing.T) {
		engine := "postgresql"
		vcpu := 4.0
		ramGB := 16.0
		storageGB := 100.0
		multiAZ := true
		region := "us-east"

		restResp, code, body := invokeREST[rest.DatabaseComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/database?engine=%s&vcpu=%.0f&ram_gb=%.0f&storage_gb=%.0f&multi_az=true&region=%s", engine, vcpu, ramGB, storageGB, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.DatabaseComparisonResponse](
			ctx, t, harness.mcpClient, "compare_database",
			mcp.CompareDatabaseInput{
				Engine:    engine,
				VCPU:      &vcpu,
				RAMGB:     &ramGB,
				StorageGB: &storageGB,
				MultiAZ:   &multiAZ,
				Region:    region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertDatabaseParity(t, restResp, mcpResp)
	})

	t.Run("EngineMismatchExclusions", func(t *testing.T) {
		engine := "mysql"
		vcpu := 4.0
		ramGB := 16.0
		region := "us-east"

		restResp, code, body := invokeREST[rest.DatabaseComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/database?engine=%s&vcpu=%.0f&ram_gb=%.0f&region=%s", engine, vcpu, ramGB, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.DatabaseComparisonResponse](
			ctx, t, harness.mcpClient, "compare_database",
			mcp.CompareDatabaseInput{
				Engine: engine,
				VCPU:   &vcpu,
				RAMGB:  &ramGB,
				Region: region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertDatabaseParity(t, restResp, mcpResp)

		// Assert engine mismatch warning exists on both
		hasEngineMismatch := false
		for _, w := range restResp.Warnings {
			if w.Code == "engine_mismatch_excluded" {
				hasEngineMismatch = true
				break
			}
		}
		if !hasEngineMismatch {
			t.Error("expected engine_mismatch_excluded warning in response")
		}
	})

	t.Run("ValidationRejection", func(t *testing.T) {
		// Invalid engine
		_, codeEngine, _ := invokeREST[rest.DatabaseComparisonResponse](t, harness.restRouter, http.MethodGet, "/api/v1/prices/database?engine=unsupported_engine", nil, harness.bearerToken)
		if codeEngine != http.StatusBadRequest {
			t.Errorf("expected REST 400 for unsupported engine, got %d", codeEngine)
		}

		_, errMCPEngine := invokeMCP[mcp.DatabaseComparisonResponse](ctx, t, harness.mcpClient, "compare_database", mcp.CompareDatabaseInput{Engine: "unsupported_engine"})
		if errMCPEngine == nil {
			t.Error("expected MCP error for unsupported engine, got nil")
		}

		// Negative vCPU
		_, codeVCPU, _ := invokeREST[rest.DatabaseComparisonResponse](t, harness.restRouter, http.MethodGet, "/api/v1/prices/database?vcpu=-2", nil, harness.bearerToken)
		if codeVCPU != http.StatusBadRequest {
			t.Errorf("expected REST 400 for negative vCPU, got %d", codeVCPU)
		}

		negVCPU := -2.0
		_, errMCPVCPU := invokeMCP[mcp.DatabaseComparisonResponse](ctx, t, harness.mcpClient, "compare_database", mcp.CompareDatabaseInput{VCPU: &negVCPU})
		if errMCPVCPU == nil {
			t.Error("expected MCP error for negative vCPU, got nil")
		}
	})
}

// Database NoSQL Contract Parity Suite
func TestContractParity_DatabaseNoSQL(t *testing.T) {
	harness := setupContractParityTest(t)
	defer harness.Close()

	ctx := context.Background()
	seedContractDatabaseNoSQLObservations(ctx, t, harness.rdb, harness.fixedNow)

	t.Run("DocumentModelProvisioned", func(t *testing.T) {
		model := "document"
		mode := "provisioned"
		readUnits := 100.0
		writeUnits := 20.0
		storageGB := 50.0
		region := "us-east"

		restResp, code, body := invokeREST[rest.DatabaseNoSQLComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/database-nosql?data_model=%s&pricing_mode=%s&read_units=%.0f&write_units=%.0f&storage_gb=%.0f&region=%s", model, mode, readUnits, writeUnits, storageGB, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.DatabaseNoSQLComparisonResponse](
			ctx, t, harness.mcpClient, "compare_database_nosql",
			mcp.CompareDatabaseNoSQLInput{
				DataModel:   model,
				PricingMode: mode,
				ReadUnits:   &readUnits,
				WriteUnits:  &writeUnits,
				StorageGB:   &storageGB,
				Region:      region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertDatabaseNoSQLParity(t, restResp, mcpResp)
	})

	t.Run("FirestoreOnDemandThroughput", func(t *testing.T) {
		model := "document"
		mode := "on_demand"
		readUnits := 50000.0
		writeUnits := 10000.0
		storageGB := 20.0
		region := "us-east"

		restResp, code, body := invokeREST[rest.DatabaseNoSQLComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/database-nosql?data_model=%s&pricing_mode=%s&read_units=%.0f&write_units=%.0f&storage_gb=%.0f&region=%s", model, mode, readUnits, writeUnits, storageGB, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.DatabaseNoSQLComparisonResponse](
			ctx, t, harness.mcpClient, "compare_database_nosql",
			mcp.CompareDatabaseNoSQLInput{
				DataModel:   model,
				PricingMode: mode,
				ReadUnits:   &readUnits,
				WriteUnits:  &writeUnits,
				StorageGB:   &storageGB,
				Region:      region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertDatabaseNoSQLParity(t, restResp, mcpResp)
	})

	t.Run("ValidationRejection", func(t *testing.T) {
		// Invalid data model
		_, codeModel, _ := invokeREST[rest.DatabaseNoSQLComparisonResponse](t, harness.restRouter, http.MethodGet, "/api/v1/prices/database-nosql?data_model=unsupported_model", nil, harness.bearerToken)
		if codeModel != http.StatusBadRequest {
			t.Errorf("expected REST 400 for unsupported data model, got %d", codeModel)
		}

		_, errMCPModel := invokeMCP[mcp.DatabaseNoSQLComparisonResponse](ctx, t, harness.mcpClient, "compare_database_nosql", mcp.CompareDatabaseNoSQLInput{DataModel: "unsupported_model"})
		if errMCPModel == nil {
			t.Error("expected MCP error for unsupported data model, got nil")
		}

		// Negative read units
		_, codeReads, _ := invokeREST[rest.DatabaseNoSQLComparisonResponse](t, harness.restRouter, http.MethodGet, "/api/v1/prices/database-nosql?read_units=-5", nil, harness.bearerToken)
		if codeReads != http.StatusBadRequest {
			t.Errorf("expected REST 400 for negative read_units, got %d", codeReads)
		}

		negReads := -5.0
		_, errMCPReads := invokeMCP[mcp.DatabaseNoSQLComparisonResponse](ctx, t, harness.mcpClient, "compare_database_nosql", mcp.CompareDatabaseNoSQLInput{ReadUnits: &negReads})
		if errMCPReads == nil {
			t.Error("expected MCP error for negative read_units, got nil")
		}
	})
}

// 5. Kubernetes Contract Parity Suite
func TestContractParity_Kubernetes(t *testing.T) {
	harness := setupContractParityTest(t)
	defer harness.Close()

	ctx := context.Background()
	seedContractKubernetesObservations(ctx, t, harness.rdb, harness.fixedNow)

	t.Run("StandardTierMatch", func(t *testing.T) {
		tier := "standard"
		region := "us-east"

		restResp, code, body := invokeREST[rest.KubernetesComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/kubernetes?tier=%s&region=%s", tier, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.KubernetesComparisonResponse](
			ctx, t, harness.mcpClient, "compare_kubernetes",
			mcp.CompareKubernetesInput{
				Tier:   tier,
				Region: region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertKubernetesParity(t, restResp, mcpResp)
	})

	t.Run("FreeTierMatch", func(t *testing.T) {
		tier := "free"
		region := "us-east"

		restResp, code, body := invokeREST[rest.KubernetesComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/kubernetes?tier=%s&region=%s", tier, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.KubernetesComparisonResponse](
			ctx, t, harness.mcpClient, "compare_kubernetes",
			mcp.CompareKubernetesInput{
				Tier:   tier,
				Region: region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertKubernetesParity(t, restResp, mcpResp)
	})

	t.Run("ExtendedSupportTierMatch", func(t *testing.T) {
		tier := "extended_support"
		region := "us-east"

		restResp, code, body := invokeREST[rest.KubernetesComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/kubernetes?tier=%s&region=%s", tier, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.KubernetesComparisonResponse](
			ctx, t, harness.mcpClient, "compare_kubernetes",
			mcp.CompareKubernetesInput{
				Tier:   tier,
				Region: region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertKubernetesParity(t, restResp, mcpResp)
	})

	t.Run("GKECreditZonalTopology", func(t *testing.T) {
		tier := "standard"
		topology := "zonal"
		region := "us-east"

		restResp, code, body := invokeREST[rest.KubernetesComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/kubernetes?tier=%s&cluster_topology=%s&region=%s", tier, topology, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.KubernetesComparisonResponse](
			ctx, t, harness.mcpClient, "compare_kubernetes",
			mcp.CompareKubernetesInput{
				Tier:            tier,
				ClusterTopology: topology,
				Region:          region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertKubernetesParity(t, restResp, mcpResp)
	})

	t.Run("UnspecifiedClusterTopologyWarning", func(t *testing.T) {
		tier := "standard"
		region := "us-east"

		restResp, code, body := invokeREST[rest.KubernetesComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/kubernetes?tier=%s&region=%s", tier, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.KubernetesComparisonResponse](
			ctx, t, harness.mcpClient, "compare_kubernetes",
			mcp.CompareKubernetesInput{
				Tier:   tier,
				Region: region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertKubernetesParity(t, restResp, mcpResp)

		hasTopologyWarnREST := false
		for _, w := range restResp.Warnings {
			if w.Code == "cluster_topology_unspecified" && w.Provider == "gcp" {
				hasTopologyWarnREST = true
				break
			}
		}
		if !hasTopologyWarnREST {
			t.Error("expected cluster_topology_unspecified warning for GCP in REST response")
		}
	})

	t.Run("NonUSDCurrencyWarning", func(t *testing.T) {
		tier := "standard"
		currency := "EUR"
		region := "us-east"

		restResp, code, body := invokeREST[rest.KubernetesComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/kubernetes?tier=%s&currency=%s&region=%s", tier, currency, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.KubernetesComparisonResponse](
			ctx, t, harness.mcpClient, "compare_kubernetes",
			mcp.CompareKubernetesInput{
				Tier:     tier,
				Currency: currency,
				Region:   region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertKubernetesParity(t, restResp, mcpResp)

		hasWarnREST := false
		for _, w := range restResp.Warnings {
			if w.Code == "non_usd_currency_unsupported" {
				hasWarnREST = true
				break
			}
		}
		if !hasWarnREST {
			t.Error("expected non_usd_currency_unsupported warning in REST response")
		}
	})

	t.Run("ValidationRejection", func(t *testing.T) {
		// Invalid tier
		_, codeREST, _ := invokeREST[rest.KubernetesComparisonResponse](t, harness.restRouter, http.MethodGet, "/api/v1/prices/kubernetes?tier=quantum_tier", nil, harness.bearerToken)
		if codeREST != http.StatusBadRequest {
			t.Errorf("expected REST 400 for invalid tier, got %d", codeREST)
		}

		_, errMCP := invokeMCP[mcp.KubernetesComparisonResponse](ctx, t, harness.mcpClient, "compare_kubernetes", mcp.CompareKubernetesInput{Tier: "quantum_tier"})
		if errMCP == nil {
			t.Error("expected MCP tool error for invalid tier, got nil")
		}

		// Invalid cluster_topology
		_, codeTopo, _ := invokeREST[rest.KubernetesComparisonResponse](t, harness.restRouter, http.MethodGet, "/api/v1/prices/kubernetes?cluster_topology=invalid_topology", nil, harness.bearerToken)
		if codeTopo != http.StatusBadRequest {
			t.Errorf("expected REST 400 for invalid topology, got %d", codeTopo)
		}

		_, errMCPTopo := invokeMCP[mcp.KubernetesComparisonResponse](ctx, t, harness.mcpClient, "compare_kubernetes", mcp.CompareKubernetesInput{ClusterTopology: "invalid_topology"})
		if errMCPTopo == nil {
			t.Error("expected MCP tool error for invalid topology, got nil")
		}
	})
}

// 5.5 Serverless Compute Contract Parity Suite
func TestContractParity_Serverless(t *testing.T) {
	harness := setupContractParityTest(t)
	defer harness.Close()

	ctx := context.Background()
	seedContractServerlessObservations(ctx, t, harness.rdb, harness.fixedNow)

	t.Run("X86_64_AllProvidersMatch", func(t *testing.T) {
		arch := "x86_64"
		region := "us-east"
		reqPerMonth := 5000000.0
		memMB := 512.0
		durMS := 200.0

		restResp, code, body := invokeREST[rest.ServerlessComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/serverless?architecture=%s&requests_per_month=%.0f&memory_mb=%.0f&execution_duration_ms=%.0f&region=%s", arch, reqPerMonth, memMB, durMS, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.ServerlessComparisonResponse](
			ctx, t, harness.mcpClient, "compare_serverless",
			mcp.CompareServerlessInput{
				Architecture:        arch,
				RequestsPerMonth:    &reqPerMonth,
				MemoryMB:            &memMB,
				ExecutionDurationMS: &durMS,
				Region:              region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertServerlessParity(t, restResp, mcpResp)
		if len(mcpResp.Results) != 3 {
			t.Fatalf("expected 3 results, got %d", len(mcpResp.Results))
		}
	})

	t.Run("ARM64_AWSOnly_AzureAndGCPExcluded", func(t *testing.T) {
		arch := "arm64"
		region := "us-east"
		reqPerMonth := 5000000.0
		memMB := 512.0
		durMS := 200.0

		restResp, code, body := invokeREST[rest.ServerlessComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/serverless?architecture=%s&requests_per_month=%.0f&memory_mb=%.0f&execution_duration_ms=%.0f&region=%s", arch, reqPerMonth, memMB, durMS, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.ServerlessComparisonResponse](
			ctx, t, harness.mcpClient, "compare_serverless",
			mcp.CompareServerlessInput{
				Architecture:        arch,
				RequestsPerMonth:    &reqPerMonth,
				MemoryMB:            &memMB,
				ExecutionDurationMS: &durMS,
				Region:              region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertServerlessParity(t, restResp, mcpResp)
		if len(mcpResp.Results) != 1 {
			t.Fatalf("expected 1 result for arm64, got %d", len(mcpResp.Results))
		}
		if mcpResp.Results[0].Provider != "aws" {
			t.Errorf("expected AWS result, got %s", mcpResp.Results[0].Provider)
		}
	})

	t.Run("NonUSDCurrencyWarning", func(t *testing.T) {
		arch := "x86_64"
		currency := "EUR"
		region := "us-east"

		restResp, code, body := invokeREST[rest.ServerlessComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			fmt.Sprintf("/api/v1/prices/serverless?architecture=%s&currency=%s&region=%s", arch, currency, region),
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[mcp.ServerlessComparisonResponse](
			ctx, t, harness.mcpClient, "compare_serverless",
			mcp.CompareServerlessInput{
				Architecture: arch,
				Currency:     currency,
				Region:       region,
			},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertServerlessParity(t, restResp, mcpResp)

		hasWarnREST := false
		for _, w := range restResp.Warnings {
			if w.Code == "non_usd_currency_unsupported" {
				hasWarnREST = true
				break
			}
		}
		if !hasWarnREST {
			t.Error("expected non_usd_currency_unsupported warning in REST response")
		}
	})

	t.Run("ValidationRejection", func(t *testing.T) {
		// Invalid architecture
		_, codeREST, _ := invokeREST[rest.ServerlessComparisonResponse](t, harness.restRouter, http.MethodGet, "/api/v1/prices/serverless?architecture=invalid_arch", nil, harness.bearerToken)
		if codeREST != http.StatusBadRequest {
			t.Errorf("expected REST 400 for invalid architecture, got %d", codeREST)
		}

		_, errMCP := invokeMCP[mcp.ServerlessComparisonResponse](ctx, t, harness.mcpClient, "compare_serverless", mcp.CompareServerlessInput{Architecture: "invalid_arch"})
		if errMCP == nil {
			t.Error("expected MCP tool error for invalid architecture, got nil")
		}

		// Invalid memory_mb
		lowMem := 64.0
		_, codeMem, _ := invokeREST[rest.ServerlessComparisonResponse](t, harness.restRouter, http.MethodGet, "/api/v1/prices/serverless?memory_mb=64", nil, harness.bearerToken)
		if codeMem != http.StatusBadRequest {
			t.Errorf("expected REST 400 for invalid memory_mb, got %d", codeMem)
		}

		_, errMCPMem := invokeMCP[mcp.ServerlessComparisonResponse](ctx, t, harness.mcpClient, "compare_serverless", mcp.CompareServerlessInput{MemoryMB: &lowMem})
		if errMCPMem == nil {
			t.Error("expected MCP tool error for invalid memory_mb, got nil")
		}

		// Invalid execution_duration_ms
		zeroDur := 0.0
		_, codeDur, _ := invokeREST[rest.ServerlessComparisonResponse](t, harness.restRouter, http.MethodGet, "/api/v1/prices/serverless?execution_duration_ms=0", nil, harness.bearerToken)
		if codeDur != http.StatusBadRequest {
			t.Errorf("expected REST 400 for invalid execution_duration_ms, got %d", codeDur)
		}

		_, errMCPDur := invokeMCP[mcp.ServerlessComparisonResponse](ctx, t, harness.mcpClient, "compare_serverless", mcp.CompareServerlessInput{ExecutionDurationMS: &zeroDur})
		if errMCPDur == nil {
			t.Error("expected MCP tool error for invalid execution_duration_ms, got nil")
		}
	})
}

// 6. Composite Workload Contract Parity Suite
func TestContractParity_Calculate(t *testing.T) {
	harness := setupContractParityTest(t)
	defer harness.Close()

	ctx := context.Background()
	seedContractComputeObservations(ctx, t, harness.rdb, harness.fixedNow)
	seedContractStorageObservations(ctx, t, harness.rdb, harness.fixedNow)
	seedContractNetworkObservations(ctx, t, harness.rdb, harness.fixedNow)
	seedContractDatabaseObservations(ctx, t, harness.rdb, harness.fixedNow)
	seedContractDatabaseNoSQLObservations(ctx, t, harness.rdb, harness.fixedNow)
	seedContractKubernetesObservations(ctx, t, harness.rdb, harness.fixedNow)
	seedContractServerlessObservations(ctx, t, harness.rdb, harness.fixedNow)

	t.Run("CompleteWorkloadAllCategories", func(t *testing.T) {
		reqPerMonth := 5000000.0
		memMB := 512.0
		durMS := 200.0

		restReq := rest.CalculateRequestBody{
			Region: "us-east",
			Compute: &domain.ComputeAttributes{
				VCPU:   2,
				RAMGB:  4,
				Family: "general_purpose",
			},
			Storage: &domain.StorageAttributes{
				SizeGB:       100,
				StorageClass: "standard",
			},
			Network: &domain.NetworkAttributes{
				EgressGB:     50,
				TransferType: "internet_egress",
			},
			DatabaseRDBMS: &domain.DatabaseRDBMSAttributes{
				Engine:    "postgresql",
				VCPU:      4,
				RAMGB:     16,
				StorageGB: 100,
			},
			DatabaseNoSQL: &domain.DatabaseNoSQLAttributes{
				DataModel:   "document",
				PricingMode: "provisioned",
				ReadUnits:   100,
				WriteUnits:  20,
				StorageGB:   50,
			},
			Kubernetes: &domain.KubernetesAttributes{
				Tier: domain.KubernetesTierStandard,
			},
			Serverless: &rest.ServerlessWorkloadPayload{
				Architecture:        "x86_64",
				Tier:                "consumption",
				RequestsPerMonth:    ptrDecimal(decimal.NewFromFloat(reqPerMonth)),
				MemoryMB:            ptrDecimal(decimal.NewFromFloat(memMB)),
				ExecutionDurationMS: ptrDecimal(decimal.NewFromFloat(durMS)),
			},
		}

		restResp, code, body := invokeREST[rest.CalculateResponse](
			t, harness.restRouter, http.MethodPost, "/api/v1/calculate", restReq, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpReq := mcp.CalculateWorkloadInput{
			Region: "us-east",
			Compute: &mcp.ComputeRequirements{
				VCPU:   2,
				RAMGB:  4,
				Family: "general_purpose",
			},
			Storage: &mcp.StorageRequirements{
				SizeGB:       100,
				StorageClass: "standard",
			},
			Network: &mcp.NetworkRequirements{
				EgressGB:     50,
				TransferType: "internet_egress",
			},
			DatabaseRDBMS: &mcp.DatabaseRequirements{
				Engine:    "postgresql",
				VCPU:      4,
				RAMGB:     16,
				StorageGB: 100,
			},
			DatabaseNoSQL: &mcp.DatabaseNoSQLRequirements{
				DataModel:   "document",
				PricingMode: "provisioned",
				ReadUnits:   100,
				WriteUnits:  20,
				StorageGB:   50,
			},
			Kubernetes: &mcp.KubernetesRequirements{
				Tier: "standard",
			},
			Serverless: &mcp.ServerlessRequirements{
				Architecture:        "x86_64",
				Tier:                "consumption",
				RequestsPerMonth:    &reqPerMonth,
				MemoryMB:            &memMB,
				ExecutionDurationMS: &durMS,
			},
		}

		mcpResp, err := invokeMCP[mcp.CalculateResponse](
			ctx, t, harness.mcpClient, "calculate_workload", mcpReq,
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertCalculateParity(t, restResp, mcpResp)

		// Assert full categories for providers that support and seed all 7 categories (aws, azure, gcp),
		// and partial category results for providers with focused/partial seed catalogs (oracle, ibm, alibaba, digitalocean).
		for _, pr := range restResp.Results {
			switch pr.Provider {
			case "aws", "azure", "gcp":
				if len(pr.Categories) != 7 {
					t.Errorf("provider %s: expected 7 categories in complete workload, got %d", pr.Provider, len(pr.Categories))
				}
				if pr.Partial {
					t.Errorf("provider %s: expected partial=false, got partial=true", pr.Provider)
				}
			default:
				if len(pr.Categories) != 3 {
					t.Errorf("provider %s: expected 3 categories in complete workload, got %d", pr.Provider, len(pr.Categories))
				}
				if !pr.Partial {
					t.Errorf("provider %s: expected partial=true, got partial=false", pr.Provider)
				}
			}
		}
	})

	t.Run("ADR0022PartialWorkloadHonesty", func(t *testing.T) {
		// Request archive storage class which is not seeded in test fixtures -> partial: true
		restReq := rest.CalculateRequestBody{
			Region: "us-east",
			Compute: &domain.ComputeAttributes{
				VCPU:   2,
				RAMGB:  4,
				Family: "general_purpose",
			},
			Storage: &domain.StorageAttributes{
				SizeGB:       100,
				StorageClass: "archive",
			},
		}

		restResp, code, body := invokeREST[rest.CalculateResponse](
			t, harness.restRouter, http.MethodPost, "/api/v1/calculate", restReq, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpReq := mcp.CalculateWorkloadInput{
			Region: "us-east",
			Compute: &mcp.ComputeRequirements{
				VCPU:   2,
				RAMGB:  4,
				Family: "general_purpose",
			},
			Storage: &mcp.StorageRequirements{
				SizeGB:       100,
				StorageClass: "archive",
			},
		}

		mcpResp, err := invokeMCP[mcp.CalculateResponse](
			ctx, t, harness.mcpClient, "calculate_workload", mcpReq,
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertCalculateParity(t, restResp, mcpResp)

		// Explicit verification of ADR 0022 contract
		for _, r := range restResp.Results {
			if !r.Partial {
				t.Errorf("expected partial=true for %s in REST", r.Provider)
			}
			if r.TotalNormalizedHourlyUSD != nil {
				t.Errorf("expected total_normalized_hourly_usd=nil for %s in REST", r.Provider)
			}
			if r.PartialTotalNormalizedHourlyUSD == nil {
				t.Errorf("expected partial_total_normalized_hourly_usd populated for %s in REST", r.Provider)
			}
		}
	})

	t.Run("SingleCategoryWorkload", func(t *testing.T) {
		restReq := rest.CalculateRequestBody{
			Region: "us-east",
			Compute: &domain.ComputeAttributes{
				VCPU:   2,
				RAMGB:  4,
				Family: "general_purpose",
			},
		}

		restResp, code, body := invokeREST[rest.CalculateResponse](
			t, harness.restRouter, http.MethodPost, "/api/v1/calculate", restReq, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpReq := mcp.CalculateWorkloadInput{
			Region: "us-east",
			Compute: &mcp.ComputeRequirements{
				VCPU:   2,
				RAMGB:  4,
				Family: "general_purpose",
			},
		}

		mcpResp, err := invokeMCP[mcp.CalculateResponse](
			ctx, t, harness.mcpClient, "calculate_workload", mcpReq,
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertCalculateParity(t, restResp, mcpResp)
	})

	t.Run("AliasConflictValidation", func(t *testing.T) {
		conflictREST := rest.CalculateRequestBody{
			Region: "us-east",
			Database: &domain.DatabaseRDBMSAttributes{
				Engine: "postgresql", VCPU: 2, RAMGB: 4, StorageGB: 100,
			},
			DatabaseRDBMS: &domain.DatabaseRDBMSAttributes{
				Engine: "mysql", VCPU: 4, RAMGB: 16, StorageGB: 200,
			},
		}
		_, codeConflictREST, bodyConflictREST := invokeREST[rest.CalculateResponse](t, harness.restRouter, http.MethodPost, "/api/v1/calculate", conflictREST, harness.bearerToken)
		if codeConflictREST != http.StatusBadRequest {
			t.Errorf("expected REST 400 for conflicting database aliases, got %d: %s", codeConflictREST, bodyConflictREST)
		}

		conflictMCP := mcp.CalculateWorkloadInput{
			Region: "us-east",
			Database: &mcp.DatabaseRequirements{
				Engine: "postgresql", VCPU: 2, RAMGB: 4, StorageGB: 100,
			},
			DatabaseRDBMS: &mcp.DatabaseRequirements{
				Engine: "mysql", VCPU: 4, RAMGB: 16, StorageGB: 200,
			},
		}
		_, errConflictMCP := invokeMCP[mcp.CalculateResponse](ctx, t, harness.mcpClient, "calculate_workload", conflictMCP)
		if errConflictMCP == nil {
			t.Error("expected MCP error for conflicting database aliases, got nil")
		}
	})

	t.Run("EmptyRequestRejection", func(t *testing.T) {
		emptyRest := rest.CalculateRequestBody{Region: "us-east"}
		_, codeREST, _ := invokeREST[rest.CalculateResponse](t, harness.restRouter, http.MethodPost, "/api/v1/calculate", emptyRest, harness.bearerToken)
		if codeREST != http.StatusBadRequest {
			t.Errorf("expected REST 400 for empty calculate request, got %d", codeREST)
		}

		emptyMCP := mcp.CalculateWorkloadInput{Region: "us-east"}
		_, errMCP := invokeMCP[mcp.CalculateResponse](ctx, t, harness.mcpClient, "calculate_workload", emptyMCP)
		if errMCP == nil {
			t.Error("expected MCP tool error for empty calculate request, got nil")
		}
	})
}

// 7. Standalone vs Composite Parity Tests
func TestContractParity_StandaloneVsComposite_RDBMSMismatchWarning(t *testing.T) {
	harness := setupContractParityTest(t)
	defer harness.Close()

	ctx := context.Background()
	seedContractDatabaseObservations(ctx, t, harness.rdb, harness.fixedNow)

	// Standalone compare query for mysql (only postgresql is seeded)
	standaloneResp, code, body := invokeREST[rest.DatabaseComparisonResponse](
		t, harness.restRouter, http.MethodGet,
		"/api/v1/prices/database?engine=mysql&vcpu=4&ram_gb=16&storage_gb=100&region=us-east",
		nil, harness.bearerToken,
	)
	if code != http.StatusOK {
		t.Fatalf("standalone database compare failed: %d: %s", code, body)
	}

	// Composite calculate query with mysql
	calcReq := rest.CalculateRequestBody{
		Region: "us-east",
		DatabaseRDBMS: &domain.DatabaseRDBMSAttributes{
			Engine:    "mysql",
			VCPU:      4,
			RAMGB:     16,
			StorageGB: 100,
		},
	}
	calcResp, codeCalc, bodyCalc := invokeREST[rest.CalculateResponse](
		t, harness.restRouter, http.MethodPost, "/api/v1/calculate", calcReq, harness.bearerToken,
	)
	if codeCalc != http.StatusOK {
		t.Fatalf("composite calculate failed: %d: %s", codeCalc, bodyCalc)
	}

	// Assert standalone emitted engine_mismatch_excluded
	var standaloneWarningCodes []string
	for _, w := range standaloneResp.Warnings {
		standaloneWarningCodes = append(standaloneWarningCodes, w.Code)
	}

	var compositeWarningCodes []string
	for _, w := range calcResp.Warnings {
		compositeWarningCodes = append(compositeWarningCodes, w.Code)
	}

	hasMismatchStandalone := false
	for _, c := range standaloneWarningCodes {
		if c == "engine_mismatch_excluded" {
			hasMismatchStandalone = true
			break
		}
	}
	if !hasMismatchStandalone {
		t.Errorf("standalone database expected engine_mismatch_excluded warning, got: %v", standaloneWarningCodes)
	}

	hasMismatchComposite := false
	for _, c := range compositeWarningCodes {
		if c == "engine_mismatch_excluded" {
			hasMismatchComposite = true
			break
		}
	}
	if !hasMismatchComposite {
		t.Errorf("composite calculate expected engine_mismatch_excluded warning, got: %v", compositeWarningCodes)
	}
}

func TestContractParity_StandaloneVsComposite_NoSQLThroughputAndStorage(t *testing.T) {
	harness := setupContractParityTest(t)
	defer harness.Close()

	ctx := context.Background()
	seedContractDatabaseNoSQLObservations(ctx, t, harness.rdb, harness.fixedNow)

	// Standalone compare NoSQL query
	standaloneResp, code, body := invokeREST[rest.DatabaseNoSQLComparisonResponse](
		t, harness.restRouter, http.MethodGet,
		"/api/v1/prices/database-nosql?data_model=document&pricing_mode=provisioned&read_units=100&write_units=20&storage_gb=50&region=us-east",
		nil, harness.bearerToken,
	)
	if code != http.StatusOK {
		t.Fatalf("standalone database-nosql compare failed: %d: %s", code, body)
	}

	// Composite calculate query
	calcReq := rest.CalculateRequestBody{
		Region: "us-east",
		DatabaseNoSQL: &domain.DatabaseNoSQLAttributes{
			DataModel:   "document",
			PricingMode: "provisioned",
			ReadUnits:   100,
			WriteUnits:  20,
			StorageGB:   50,
		},
	}
	calcResp, codeCalc, bodyCalc := invokeREST[rest.CalculateResponse](
		t, harness.restRouter, http.MethodPost, "/api/v1/calculate", calcReq, harness.bearerToken,
	)
	if codeCalc != http.StatusOK {
		t.Fatalf("composite calculate failed: %d: %s", codeCalc, bodyCalc)
	}

	standaloneMap := make(map[string]rest.DatabaseNoSQLResultEntry)
	for _, r := range standaloneResp.Results {
		standaloneMap[r.Provider] = r
	}

	for _, cr := range calcResp.Results {
		stResult, ok := standaloneMap[cr.Provider]
		if !ok {
			t.Fatalf("provider %s in composite calculate missing from standalone", cr.Provider)
		}
		catRes, hasCat := cr.Categories["database_nosql"]
		if !hasCat {
			t.Fatalf("provider %s missing database_nosql in composite calculate", cr.Provider)
		}

		if catRes.SkuID != stResult.SkuID {
			t.Errorf("provider %s sku_id mismatch: composite=%s, standalone=%s", cr.Provider, catRes.SkuID, stResult.SkuID)
		}
		if !catRes.NormalizedHourlyUSD.Equal(stResult.NormalizedHourlyUSD) {
			t.Errorf("provider %s cost mismatch: composite=%s, standalone=%s", cr.Provider, catRes.NormalizedHourlyUSD, stResult.NormalizedHourlyUSD)
		}
	}
}

func TestContractParity_StandaloneVsComposite_KubernetesUnspecifiedTopologyWarning(t *testing.T) {
	harness := setupContractParityTest(t)
	defer harness.Close()

	ctx := context.Background()
	seedContractKubernetesObservations(ctx, t, harness.rdb, harness.fixedNow)

	// Standalone compare with omitted cluster_topology
	standaloneResp, code, body := invokeREST[rest.KubernetesComparisonResponse](
		t, harness.restRouter, http.MethodGet,
		"/api/v1/prices/kubernetes?tier=standard&region=us-east",
		nil, harness.bearerToken,
	)
	if code != http.StatusOK {
		t.Fatalf("standalone kubernetes compare failed: %d: %s", code, body)
	}

	// Composite calculate with omitted cluster_topology
	calcReq := rest.CalculateRequestBody{
		Region: "us-east",
		Kubernetes: &domain.KubernetesAttributes{
			Tier: domain.KubernetesTierStandard,
		},
	}
	calcResp, codeCalc, bodyCalc := invokeREST[rest.CalculateResponse](
		t, harness.restRouter, http.MethodPost, "/api/v1/calculate", calcReq, harness.bearerToken,
	)
	if codeCalc != http.StatusOK {
		t.Fatalf("composite calculate failed: %d: %s", codeCalc, bodyCalc)
	}

	hasWarnStandalone := false
	for _, w := range standaloneResp.Warnings {
		if w.Code == "cluster_topology_unspecified" {
			hasWarnStandalone = true
			break
		}
	}
	if !hasWarnStandalone {
		t.Errorf("expected cluster_topology_unspecified in standalone kubernetes warnings")
	}

	hasWarnComposite := false
	for _, w := range calcResp.Warnings {
		if w.Code == "cluster_topology_unspecified" {
			hasWarnComposite = true
			break
		}
	}
	if !hasWarnComposite {
		t.Errorf("expected cluster_topology_unspecified in composite calculate warnings")
	}
}

func TestContractParity_StandaloneVsComposite_ServerlessDualDurationPrecision(t *testing.T) {
	harness := setupContractParityTest(t)
	defer harness.Close()

	ctx := context.Background()
	seedContractServerlessObservations(ctx, t, harness.rdb, harness.fixedNow)

	reqPerMonth := 10000000.0
	memMB := 1024.0
	durMS := 300.0

	// Standalone compare query
	standaloneResp, code, body := invokeREST[rest.ServerlessComparisonResponse](
		t, harness.restRouter, http.MethodGet,
		fmt.Sprintf("/api/v1/prices/serverless?architecture=x86_64&requests_per_month=%.0f&memory_mb=%.0f&execution_duration_ms=%.0f&region=us-east", reqPerMonth, memMB, durMS),
		nil, harness.bearerToken,
	)
	if code != http.StatusOK {
		t.Fatalf("standalone serverless compare failed: %d: %s", code, body)
	}

	// Composite calculate query
	calcReq := rest.CalculateRequestBody{
		Region: "us-east",
		Serverless: &rest.ServerlessWorkloadPayload{
			Architecture:        "x86_64",
			Tier:                "consumption",
			RequestsPerMonth:    ptrDecimal(decimal.NewFromFloat(reqPerMonth)),
			MemoryMB:            ptrDecimal(decimal.NewFromFloat(memMB)),
			ExecutionDurationMS: ptrDecimal(decimal.NewFromFloat(durMS)),
		},
	}
	calcResp, codeCalc, bodyCalc := invokeREST[rest.CalculateResponse](
		t, harness.restRouter, http.MethodPost, "/api/v1/calculate", calcReq, harness.bearerToken,
	)
	if codeCalc != http.StatusOK {
		t.Fatalf("composite calculate failed: %d: %s", codeCalc, bodyCalc)
	}

	standaloneMap := make(map[string]rest.ServerlessResultEntry)
	for _, r := range standaloneResp.Results {
		standaloneMap[r.Provider] = r
	}

	for _, cr := range calcResp.Results {
		stResult, ok := standaloneMap[cr.Provider]
		if !ok {
			t.Fatalf("provider %s missing in standalone response", cr.Provider)
		}
		catRes, hasCat := cr.Categories["serverless"]
		if !hasCat {
			t.Fatalf("provider %s missing serverless in composite calculate", cr.Provider)
		}

		if !catRes.NormalizedHourlyUSD.Equal(stResult.NormalizedHourlyUSD) {
			t.Errorf("provider %s normalized_hourly_usd precision mismatch: composite=%s, standalone=%s", cr.Provider, catRes.NormalizedHourlyUSD, stResult.NormalizedHourlyUSD)
		}
	}
}

func TestContractParity_AllProvidersUnavailable(t *testing.T) {
	harness := setupContractParityTest(t)
	defer harness.Close()

	ctx := context.Background()
	// Deliberately DO NOT seed any observations

	// 1. REST calculate with no cached data and no DB -> 502 Bad Gateway
	calcReq := rest.CalculateRequestBody{
		Region: "us-east",
		Compute: &domain.ComputeAttributes{
			VCPU:   2,
			RAMGB:  4,
			Family: "general_purpose",
		},
	}
	_, codeREST, bodyREST := invokeREST[rest.CalculateResponse](
		t, harness.restRouter, http.MethodPost, "/api/v1/calculate", calcReq, harness.bearerToken,
	)
	if codeREST != http.StatusBadGateway {
		t.Errorf("expected REST 502 Bad Gateway when all providers unavailable, got %d: %s", codeREST, bodyREST)
	}

	// 2. MCP calculate_workload with no cached data and no DB -> structured error
	mcpReq := mcp.CalculateWorkloadInput{
		Region: "us-east",
		Compute: &mcp.ComputeRequirements{
			VCPU:   2,
			RAMGB:  4,
			Family: "general_purpose",
		},
	}
	_, errMCP := invokeMCP[mcp.CalculateResponse](ctx, t, harness.mcpClient, "calculate_workload", mcpReq)
	if errMCP == nil {
		t.Error("expected MCP tool error when all providers unavailable, got nil")
	} else if !strings.Contains(errMCP.Error(), "all cloud providers failed to retrieve pricing data") {
		t.Errorf("expected MCP error 'all cloud providers failed to retrieve pricing data', got: %v", errMCP)
	}
}

// 6. Provider Status & Freshness Contract Parity Suite
func TestContractParity_ProviderStatus(t *testing.T) {
	harness := setupContractParityTest(t)
	defer harness.Close()

	ctx := context.Background()

	t.Run("HealthyIngestedProvider_AWS", func(t *testing.T) {
		restResp, code, body := invokeREST[rest.ProviderStatusResponse](
			t, harness.restRouter, http.MethodGet, "/api/v1/providers/aws/status", nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[domain.ProviderStatus](
			ctx, t, harness.mcpClient, "get_provider_status",
			mcp.GetProviderStatusInput{Provider: "aws"},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertProviderStatusParity(t, restResp, mcpResp)
		if mcpResp.Status != "healthy" {
			t.Errorf("expected status 'healthy', got %s", mcpResp.Status)
		}
		if mcpResp.Stale {
			t.Error("expected Stale=false for healthy provider")
		}
	})

	t.Run("StaleProvider_Azure", func(t *testing.T) {
		restResp, code, body := invokeREST[rest.ProviderStatusResponse](
			t, harness.restRouter, http.MethodGet, "/api/v1/providers/azure/status", nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[domain.ProviderStatus](
			ctx, t, harness.mcpClient, "get_provider_status",
			mcp.GetProviderStatusInput{Provider: "azure"},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertProviderStatusParity(t, restResp, mcpResp)
		if mcpResp.Status != "stale" {
			t.Errorf("expected status 'stale', got %s", mcpResp.Status)
		}
		if !mcpResp.Stale {
			t.Error("expected Stale=true for stale provider")
		}
	})

	t.Run("DegradedProviderWithDLQ_GCP", func(t *testing.T) {
		restResp, code, body := invokeREST[rest.ProviderStatusResponse](
			t, harness.restRouter, http.MethodGet, "/api/v1/providers/gcp/status", nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST returned status %d: %s", code, body)
		}

		mcpResp, err := invokeMCP[domain.ProviderStatus](
			ctx, t, harness.mcpClient, "get_provider_status",
			mcp.GetProviderStatusInput{Provider: "gcp"},
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertProviderStatusParity(t, restResp, mcpResp)
		if mcpResp.Status != "degraded" {
			t.Errorf("expected status 'degraded', got %s", mcpResp.Status)
		}
	})

	t.Run("Stage3Providers_AllCatalogItems", func(t *testing.T) {
		stage3Providers := []string{"oracle", "ibm", "alibaba", "digitalocean"}

		for _, p := range stage3Providers {
			t.Run(p, func(t *testing.T) {
				restResp, code, body := invokeREST[rest.ProviderStatusResponse](
					t, harness.restRouter, http.MethodGet, fmt.Sprintf("/api/v1/providers/%s/status", p), nil, harness.bearerToken,
				)
				if code != http.StatusOK {
					t.Fatalf("REST returned status %d for %s: %s", code, p, body)
				}

				mcpResp, err := invokeMCP[domain.ProviderStatus](
					ctx, t, harness.mcpClient, "get_provider_status",
					mcp.GetProviderStatusInput{Provider: p},
				)
				if err != nil {
					t.Fatalf("MCP invoke failed for %s: %v", p, err)
				}

				assertProviderStatusParity(t, restResp, mcpResp)
				if mcpResp.Status != "not_yet_ingested" {
					t.Errorf("expected status 'not_yet_ingested' for %s, got %s", p, mcpResp.Status)
				}
				if !mcpResp.Stale {
					t.Errorf("expected Stale=true for %s", p)
				}
			})
		}
	})

	t.Run("UnknownProviderNotFound", func(t *testing.T) {
		_, codeREST, bodyREST := invokeREST[rest.ProviderStatusResponse](
			t, harness.restRouter, http.MethodGet, "/api/v1/providers/unknown_cloud/status", nil, harness.bearerToken,
		)
		if codeREST != http.StatusNotFound {
			t.Errorf("expected REST 404 for unknown provider, got %d: %s", codeREST, bodyREST)
		}

		_, errMCP := invokeMCP[domain.ProviderStatus](
			ctx, t, harness.mcpClient, "get_provider_status",
			mcp.GetProviderStatusInput{Provider: "unknown_cloud"},
		)
		if errMCP == nil {
			t.Error("expected MCP tool error for unknown provider, got nil")
		}
	})
}

// 8. 7-Provider Complete Fanout & Parity Verification
func TestContractParity_SevenProviders_FullFanout(t *testing.T) {
	harness := setupContractParityTest(t)
	defer harness.Close()

	ctx := context.Background()
	seedContractComputeObservations(ctx, t, harness.rdb, harness.fixedNow)
	seedContractStorageObservations(ctx, t, harness.rdb, harness.fixedNow)
	seedContractNetworkObservations(ctx, t, harness.rdb, harness.fixedNow)

	expectedProviders := []string{"aws", "azure", "gcp", "oracle", "ibm", "alibaba", "digitalocean"}

	t.Run("Compute_SevenProviders", func(t *testing.T) {
		restResp, code, body := invokeREST[rest.ComputeComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			"/api/v1/prices/compute?vcpu=2&ram_gb=4&family=general_purpose&region=us-east",
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST compute comparison failed: %d: %s", code, body)
		}

		vcpu := 2.0
		ramGB := 4.0
		mcpResp, err := invokeMCP[mcp.ComputeComparisonResponse](
			ctx, t, harness.mcpClient, "compare_compute",
			mcp.CompareComputeInput{
				VCPU:   &vcpu,
				RAMGB:  &ramGB,
				Family: "general_purpose",
				Region: "us-east",
			},
		)
		if err != nil {
			t.Fatalf("MCP compare_compute failed: %v", err)
		}

		if len(restResp.Results) != len(expectedProviders) {
			t.Errorf("expected %d providers in REST compute comparison, got %d", len(expectedProviders), len(restResp.Results))
		}
		if len(mcpResp.Results) != len(expectedProviders) {
			t.Errorf("expected %d providers in MCP compute comparison, got %d", len(expectedProviders), len(mcpResp.Results))
		}

		assertComputeParity(t, restResp, mcpResp)
	})

	t.Run("Storage_SevenProviders", func(t *testing.T) {
		restResp, code, body := invokeREST[rest.StorageComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			"/api/v1/prices/storage?size_gb=100&storage_class=standard&region=us-east",
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST storage comparison failed: %d: %s", code, body)
		}

		sizeGB := 100.0
		mcpResp, err := invokeMCP[mcp.StorageComparisonResponse](
			ctx, t, harness.mcpClient, "compare_storage",
			mcp.CompareStorageInput{
				SizeGB:       &sizeGB,
				StorageClass: "standard",
				Region:       "us-east",
			},
		)
		if err != nil {
			t.Fatalf("MCP compare_storage failed: %v", err)
		}

		if len(restResp.Results) != len(expectedProviders) {
			t.Errorf("expected %d providers in REST storage comparison, got %d", len(expectedProviders), len(restResp.Results))
		}
		if len(mcpResp.Results) != len(expectedProviders) {
			t.Errorf("expected %d providers in MCP storage comparison, got %d", len(expectedProviders), len(mcpResp.Results))
		}

		assertStorageParity(t, restResp, mcpResp)
	})

	t.Run("Network_SevenProviders", func(t *testing.T) {
		restResp, code, body := invokeREST[rest.NetworkComparisonResponse](
			t, harness.restRouter, http.MethodGet,
			"/api/v1/prices/network?egress_gb=50&transfer_type=internet_egress&region=us-east",
			nil, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST network comparison failed: %d: %s", code, body)
		}

		egressGB := 50.0
		mcpResp, err := invokeMCP[mcp.NetworkComparisonResponse](
			ctx, t, harness.mcpClient, "compare_network",
			mcp.CompareNetworkInput{
				EgressGB:     &egressGB,
				TransferType: "internet_egress",
				Region:       "us-east",
			},
		)
		if err != nil {
			t.Fatalf("MCP compare_network failed: %v", err)
		}

		if len(restResp.Results) != len(expectedProviders) {
			t.Errorf("expected %d providers in REST network comparison, got %d", len(expectedProviders), len(restResp.Results))
		}
		if len(mcpResp.Results) != len(expectedProviders) {
			t.Errorf("expected %d providers in MCP network comparison, got %d", len(expectedProviders), len(mcpResp.Results))
		}

		assertNetworkParity(t, restResp, mcpResp)
	})

	t.Run("CalculateWorkload_SevenProviders_CompositeParity", func(t *testing.T) {
		calcReqREST := rest.CalculateRequestBody{
			Region: "us-east",
			Compute: &domain.ComputeAttributes{
				VCPU:   2,
				RAMGB:  4,
				Family: "general_purpose",
			},
			Storage: &domain.StorageAttributes{
				SizeGB:       100,
				StorageClass: "standard",
			},
			Network: &domain.NetworkAttributes{
				EgressGB:     50,
				TransferType: "internet_egress",
			},
		}

		restResp, code, body := invokeREST[rest.CalculateResponse](
			t, harness.restRouter, http.MethodPost,
			"/api/v1/calculate", calcReqREST, harness.bearerToken,
		)
		if code != http.StatusOK {
			t.Fatalf("REST calculate failed: %d: %s", code, body)
		}

		calcReqMCP := mcp.CalculateWorkloadInput{
			Region: "us-east",
			Compute: &mcp.ComputeRequirements{
				VCPU:   2,
				RAMGB:  4,
				Family: "general_purpose",
			},
			Storage: &mcp.StorageRequirements{
				SizeGB:       100,
				StorageClass: "standard",
			},
			Network: &mcp.NetworkRequirements{
				EgressGB:     50,
				TransferType: "internet_egress",
			},
		}

		mcpResp, err := invokeMCP[mcp.CalculateResponse](
			ctx, t, harness.mcpClient, "calculate_workload", calcReqMCP,
		)
		if err != nil {
			t.Fatalf("MCP calculate_workload failed: %v", err)
		}

		if len(restResp.Results) != len(expectedProviders) {
			t.Errorf("expected %d providers in REST composite calculate, got %d", len(expectedProviders), len(restResp.Results))
		}
		if len(mcpResp.Results) != len(expectedProviders) {
			t.Errorf("expected %d providers in MCP composite calculate, got %d", len(expectedProviders), len(mcpResp.Results))
		}

		assertCalculateParity(t, restResp, mcpResp)
	})
}
