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
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

// --- Mock Querier & DLQ Harness ---

type contractMockQuerier struct {
	getProviderCategoryStatusFunc func(ctx context.Context, provider string) ([]store.GetProviderCategoryStatusRow, error)
}

func (m *contractMockQuerier) CreateUser(ctx context.Context, arg store.CreateUserParams) (store.User, error) {
	return store.User{}, errors.New("CreateUser not implemented")
}

func (m *contractMockQuerier) GetUserByEmail(ctx context.Context, email string) (store.User, error) {
	return store.User{}, pgx.ErrNoRows
}

func (m *contractMockQuerier) GetUserByID(ctx context.Context, id pgtype.UUID) (store.User, error) {
	return store.User{}, pgx.ErrNoRows
}

func (m *contractMockQuerier) InsertRefreshToken(ctx context.Context, arg store.InsertRefreshTokenParams) (store.RefreshToken, error) {
	return store.RefreshToken{}, errors.New("InsertRefreshToken not implemented")
}

func (m *contractMockQuerier) GetRefreshTokenByHashForUpdate(ctx context.Context, tokenHash string) (store.RefreshToken, error) {
	return store.RefreshToken{}, pgx.ErrNoRows
}

func (m *contractMockQuerier) GetRefreshTokenByID(ctx context.Context, id pgtype.UUID) (store.RefreshToken, error) {
	return store.RefreshToken{}, pgx.ErrNoRows
}

func (m *contractMockQuerier) RevokeRefreshTokenWithReplacement(ctx context.Context, arg store.RevokeRefreshTokenWithReplacementParams) error {
	return nil
}

func (m *contractMockQuerier) RevokeRefreshTokenByHash(ctx context.Context, arg store.RevokeRefreshTokenByHashParams) error {
	return nil
}

func (m *contractMockQuerier) RevokeRefreshTokenFamily(ctx context.Context, arg store.RevokeRefreshTokenFamilyParams) error {
	return nil
}

func (m *contractMockQuerier) ListRefreshTokensByFamilyID(ctx context.Context, familyID pgtype.UUID) ([]store.RefreshToken, error) {
	return nil, nil
}

func (m *contractMockQuerier) GetPriceObservations(ctx context.Context, arg store.GetPriceObservationsParams) ([]store.PriceObservation, error) {
	return nil, errors.New("GetPriceObservations not implemented")
}

func (m *contractMockQuerier) GetLatestPriceForSKU(ctx context.Context, arg store.GetLatestPriceForSKUParams) (store.PriceObservation, error) {
	return store.PriceObservation{}, errors.New("GetLatestPriceForSKU not implemented")
}

func (m *contractMockQuerier) GetLatestPriceForSKUAndCategory(ctx context.Context, arg store.GetLatestPriceForSKUAndCategoryParams) (store.PriceObservation, error) {
	return store.PriceObservation{}, errors.New("GetLatestPriceForSKUAndCategory not implemented")
}

func (m *contractMockQuerier) UpdatePriceObservationLastSeenAt(ctx context.Context, arg store.UpdatePriceObservationLastSeenAtParams) error {
	return nil
}

func (m *contractMockQuerier) InsertPriceObservation(ctx context.Context, arg store.InsertPriceObservationParams) (int64, error) {
	return 0, errors.New("InsertPriceObservation not implemented")
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

	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1"), awsObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "compute", "eastus"), azureObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "compute", "us-east4"), gcpObs, cache.DefaultTTL)
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

	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "storage", "us-east-1"), awsObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "storage", "eastus"), azureObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "storage", "us-east4"), gcpObs, cache.DefaultTTL)
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

	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "network", "us-east-1"), awsObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "network", "eastus"), azureObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "network", "us-east4"), gcpObs, cache.DefaultTTL)
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
			if w.Code == "currency_conversion_not_yet_supported" {
				hasWarnREST = true
				break
			}
		}
		if !hasWarnREST {
			t.Error("expected currency_conversion_not_yet_supported warning in REST response")
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

// 5. Composite Workload Contract Parity Suite
func TestContractParity_Calculate(t *testing.T) {
	harness := setupContractParityTest(t)
	defer harness.Close()

	ctx := context.Background()
	seedContractComputeObservations(ctx, t, harness.rdb, harness.fixedNow)
	seedContractStorageObservations(ctx, t, harness.rdb, harness.fixedNow)
	seedContractNetworkObservations(ctx, t, harness.rdb, harness.fixedNow)

	t.Run("CompleteWorkloadAllCategories", func(t *testing.T) {
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
		}

		mcpResp, err := invokeMCP[mcp.CalculateResponse](
			ctx, t, harness.mcpClient, "calculate_workload", mcpReq,
		)
		if err != nil {
			t.Fatalf("MCP invoke failed: %v", err)
		}

		assertCalculateParity(t, restResp, mcpResp)
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
