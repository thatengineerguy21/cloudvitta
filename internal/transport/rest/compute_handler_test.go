package rest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

func TestComputeHandler_InvalidVCPU_ReturnsRFC7807(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewComputeHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute?vcpu=-1", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/problem+json") {
		t.Errorf("Content-Type = %q, want application/problem+json", contentType)
	}

	var rfcErr middleware.RFC7807Error
	if err := json.NewDecoder(rec.Body).Decode(&rfcErr); err != nil {
		t.Fatalf("failed to decode RFC7807 JSON: %v", err)
	}

	if rfcErr.Status != 400 {
		t.Errorf("Status = %d, want 400", rfcErr.Status)
	}
	if rfcErr.Title != "Invalid query parameter" {
		t.Errorf("Title = %q, want Invalid query parameter", rfcErr.Title)
	}
	if !strings.Contains(rfcErr.Detail, "vcpu must be a positive number") {
		t.Errorf("Detail = %q, want vcpu error detail", rfcErr.Detail)
	}
	if rfcErr.Instance != "/api/v1/prices/compute?vcpu=-1" {
		t.Errorf("Instance = %q, want /api/v1/prices/compute?vcpu=-1", rfcErr.Instance)
	}
}

func TestComputeHandler_InvalidRAM_ReturnsRFC7807(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewComputeHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute?ram_gb=invalid", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request", rec.Code)
	}

	var rfcErr middleware.RFC7807Error
	if err := json.NewDecoder(rec.Body).Decode(&rfcErr); err != nil {
		t.Fatalf("failed to decode RFC7807 JSON: %v", err)
	}

	if rfcErr.Status != 400 {
		t.Errorf("Status = %d, want 400", rfcErr.Status)
	}
}

func TestComputeHandler_HappyPath_MultiProvider(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
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

	oracleObs := []domain.PriceObservation{
		{
			Provider:        "oracle",
			ServiceCategory: "compute",
			SkuID:           "SKU-OCI-E4-FLEX-2-4",
			DisplayName:     "VM.Standard.E4.Flex",
			Region:          "us-ashburn-1",
			RegionGroup:     "us-east",
			Unit:            "hour",
			PriceAmount:     decimal.RequireFromString("0.025"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "general_purpose"},
			FetchedAt:       time.Now().UTC(),
		},
	}
	ibmObs := []domain.PriceObservation{
		{
			Provider:        "ibm",
			ServiceCategory: "compute",
			SkuID:           "SKU-IBM-BX2-2X4",
			DisplayName:     "bx2-2x4",
			Region:          "us-east",
			RegionGroup:     "us-east",
			Unit:            "hour",
			PriceAmount:     decimal.RequireFromString("0.048"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "general_purpose"},
			FetchedAt:       time.Now().UTC(),
		},
	}
	aliObs := []domain.PriceObservation{
		{
			Provider:        "alibaba",
			ServiceCategory: "compute",
			SkuID:           "SKU-ALI-ECS-G7-2X4",
			DisplayName:     "ecs.g7.large",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			Unit:            "hour",
			PriceAmount:     decimal.RequireFromString("0.045"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "general_purpose"},
			FetchedAt:       time.Now().UTC(),
		},
	}
	doObs := []domain.PriceObservation{
		{
			Provider:        "digitalocean",
			ServiceCategory: "compute",
			SkuID:           "SKU-DO-S-2VCPU-4GB",
			DisplayName:     "s-2vcpu-4gb",
			Region:          "nyc3",
			RegionGroup:     "us-east",
			Unit:            "hour",
			PriceAmount:     decimal.RequireFromString("0.0357"),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "general_purpose"},
			FetchedAt:       time.Now().UTC(),
		},
	}

	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1"), awsObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "compute", "eastus"), azureObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "compute", "us-east4"), gcpObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "oracle", "compute", "us-ashburn-1"), oracleObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "ibm", "compute", "us-east"), ibmObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "alibaba", "compute", "us-east-1"), aliObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "digitalocean", "compute", "nyc3"), doObs, cache.DefaultTTL)

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewComputeHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute?vcpu=2&ram_gb=4&region=us-east", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK. Body: %s", rec.Code, rec.Body.String())
	}

	var resp rest.ComputeComparisonResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Meta.APIVersion != "v1" {
		t.Errorf("APIVersion = %q, want v1", resp.Meta.APIVersion)
	}
	expectedProviders := []string{"aws", "azure", "gcp", "oracle", "ibm", "alibaba", "digitalocean"}
	if len(resp.Results) != len(expectedProviders) {
		t.Fatalf("got %d results, want %d (%v)", len(resp.Results), len(expectedProviders), expectedProviders)
	}

	resultsByProvider := make(map[string]rest.ComputeResultEntry)
	for _, res := range resp.Results {
		resultsByProvider[res.Provider] = res
	}

	for _, p := range expectedProviders {
		res, ok := resultsByProvider[p]
		if !ok {
			t.Errorf("missing result for provider %q", p)
			continue
		}
		if res.MatchQuality != "exact" {
			t.Errorf("provider %q MatchQuality = %q, want exact", p, res.MatchQuality)
		}
		if res.Price.Currency != "USD" {
			t.Errorf("provider %q currency = %q, want USD", p, res.Price.Currency)
		}
		if res.NormalizedHourlyUSD.LessThanOrEqual(decimal.Zero) {
			t.Errorf("provider %q NormalizedHourlyUSD = %s, want > 0", p, res.NormalizedHourlyUSD)
		}
	}
}

func TestComputeHandler_CurrencyWarning(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewComputeHandler(pricingSvc)

	// Pre-warm the cache so it doesn't hit the nil DB
	cacheKeyAWS := cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1")
	cacheKeyAzure := cache.BuildKey(cache.SchemaVersion, "azure", "compute", "eastus")
	cacheKeyGCP := cache.BuildKey(cache.SchemaVersion, "gcp", "compute", "us-east4")
	_ = cache.Warm(context.Background(), rdb, cacheKeyAWS, []domain.PriceObservation{}, cache.DefaultTTL)
	_ = cache.Warm(context.Background(), rdb, cacheKeyAzure, []domain.PriceObservation{}, cache.DefaultTTL)
	_ = cache.Warm(context.Background(), rdb, cacheKeyGCP, []domain.PriceObservation{}, cache.DefaultTTL)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute?currency=EUR", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Still returns 200, but has warnings
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK", rec.Code)
	}

	var resp rest.ComputeComparisonResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	foundWarning := false
	for _, w := range resp.Warnings {
		if w.Code == "non_usd_currency_unsupported" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected currency conversion warning, got none")
	}

	if resp.Meta.Query["currency"] != "EUR" {
		t.Errorf("expected Meta query currency to reflect requested EUR, got %v", resp.Meta.Query["currency"])
	}
}

func TestComputeHandler_StrictFamily(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewComputeHandler(pricingSvc)

	cacheKeyAWS := cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1")
	cacheKeyAzure := cache.BuildKey(cache.SchemaVersion, "azure", "compute", "eastus")
	cacheKeyGCP := cache.BuildKey(cache.SchemaVersion, "gcp", "compute", "us-east4")
	_ = cache.Warm(context.Background(), rdb, cacheKeyAWS, []domain.PriceObservation{}, cache.DefaultTTL)
	_ = cache.Warm(context.Background(), rdb, cacheKeyAzure, []domain.PriceObservation{}, cache.DefaultTTL)
	_ = cache.Warm(context.Background(), rdb, cacheKeyGCP, []domain.PriceObservation{}, cache.DefaultTTL)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute?strict_family=true&family=general_purpose", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK", rec.Code)
	}

	var resp rest.ComputeComparisonResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Meta.Query["family"] != "general_purpose" {
		t.Errorf("expected family query meta = general_purpose, got %v", resp.Meta.Query["family"])
	}
}

func TestComputeHandler_PartialProviderFailure_Returns200WithWarning(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	awsObs := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "compute",
			SkuID:           "SKU-AWS-T3-MED",
			PriceAmount:     decimal.RequireFromString("0.0416"),
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4},
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1"), awsObs, cache.DefaultTTL)

	failingQueries := store.New(&failingDBTX{})
	pricingSvc := service.NewPricingService(failingQueries, rdb)
	handler := rest.NewComputeHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute?vcpu=2&ram_gb=4&region=us-east", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK. Body: %s", rec.Code, rec.Body.String())
	}

	var resp rest.ComputeComparisonResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result (AWS), got %d", len(resp.Results))
	}
}

func TestComputeHandler_AllProvidersFail_Returns502(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	failingQueries := store.New(&failingDBTX{})
	pricingSvc := service.NewPricingService(failingQueries, rdb)
	handler := rest.NewComputeHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute?region=us-east", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 Bad Gateway", rec.Code)
	}

	var rfcErr middleware.RFC7807Error
	if err := json.NewDecoder(rec.Body).Decode(&rfcErr); err != nil {
		t.Fatalf("failed to decode RFC7807 JSON: %v", err)
	}
	if rfcErr.Status != 502 {
		t.Errorf("Status = %d, want 502", rfcErr.Status)
	}
}
