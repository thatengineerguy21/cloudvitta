package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

type failingDBTX struct {
	store.DBTX
}

func (f *failingDBTX) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("db connection down")
}

func TestStorageHandler_InvalidSizeGB_ReturnsRFC7807(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewStorageHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/storage?size_gb=-10", nil)
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
	if !strings.Contains(rfcErr.Detail, "size_gb must be a positive number") {
		t.Errorf("Detail = %q, want size_gb error detail", rfcErr.Detail)
	}
}

func TestStorageHandler_ExceedsMaxSizeGB_ReturnsRFC7807(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewStorageHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/storage?size_gb=1000001", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request", rec.Code)
	}

	var rfcErr middleware.RFC7807Error
	if err := json.NewDecoder(rec.Body).Decode(&rfcErr); err != nil {
		t.Fatalf("failed to decode RFC7807 JSON: %v", err)
	}

	if !strings.Contains(rfcErr.Detail, "no greater than 1000000") {
		t.Errorf("Detail = %q, want upper bound error message", rfcErr.Detail)
	}
}

func TestStorageHandler_MethodNotAllowed(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewStorageHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/prices/storage", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 Method Not Allowed", rec.Code)
	}
}

func TestStorageHandler_HappyPath_CalculatesMonthlyCosts(t *testing.T) {
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
			StorageAttributes: domain.StorageAttributes{SizeGB: 500, StorageClass: "standard"},
			FetchedAt:         time.Now().UTC(),
		},
	}
	azureObs := []domain.PriceObservation{
		{
			Provider:          "azure",
			ServiceCategory:   "storage",
			SkuID:             "SKU-AZURE-BLOB-HOT",
			DisplayName:       "Blob Storage Hot",
			Region:            "eastus",
			RegionGroup:       "us-east",
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.0184"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 500, StorageClass: "standard"},
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
			StorageAttributes: domain.StorageAttributes{SizeGB: 500, StorageClass: "standard"},
			FetchedAt:         time.Now().UTC(),
		},
	}

	oracleObs := []domain.PriceObservation{
		{
			Provider:          "oracle",
			ServiceCategory:   "storage",
			SkuID:             "SKU-OCI-OBJ-STD",
			DisplayName:       "Object Storage Standard",
			Region:            "us-ashburn-1",
			RegionGroup:       "us-east",
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.0255"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 500, StorageClass: "standard"},
			FetchedAt:         time.Now().UTC(),
		},
	}
	ibmObs := []domain.PriceObservation{
		{
			Provider:          "ibm",
			ServiceCategory:   "storage",
			SkuID:             "SKU-IBM-COS-STD",
			DisplayName:       "Cloud Object Storage Standard",
			Region:            "us-east",
			RegionGroup:       "us-east",
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.022"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 500, StorageClass: "standard"},
			FetchedAt:         time.Now().UTC(),
		},
	}
	aliObs := []domain.PriceObservation{
		{
			Provider:          "alibaba",
			ServiceCategory:   "storage",
			SkuID:             "SKU-ALI-OSS-STD",
			DisplayName:       "Object Storage Standard",
			Region:            "us-east-1",
			RegionGroup:       "us-east",
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.019"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 500, StorageClass: "standard"},
			FetchedAt:         time.Now().UTC(),
		},
	}
	doObs := []domain.PriceObservation{
		{
			Provider:          "digitalocean",
			ServiceCategory:   "storage",
			SkuID:             "SKU-DO-SPACES-STD",
			DisplayName:       "Spaces Standard Storage",
			Region:            "nyc3",
			RegionGroup:       "us-east",
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.020"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 500, StorageClass: "standard"},
			FetchedAt:         time.Now().UTC(),
		},
	}

	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "storage", "us-east-1"), awsObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "storage", "eastus"), azureObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "storage", "us-east4"), gcpObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "oracle", "storage", "us-ashburn-1"), oracleObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "ibm", "storage", "us-east"), ibmObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "alibaba", "storage", "us-east-1"), aliObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "digitalocean", "storage", "nyc3"), doObs, cache.DefaultTTL)

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewStorageHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/storage?size_gb=500&region=us-east", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK. Body: %s", rec.Code, rec.Body.String())
	}

	var resp rest.StorageComparisonResponse
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

	resultsByProvider := make(map[string]rest.StorageResultEntry)
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
		if res.MonthlyCostUSD.LessThanOrEqual(decimal.Zero) {
			t.Errorf("provider %q MonthlyCostUSD = %s, want > 0", p, res.MonthlyCostUSD)
		}
		if res.NormalizedHourlyUSD.LessThanOrEqual(decimal.Zero) {
			t.Errorf("provider %q NormalizedHourlyUSD = %s, want > 0", p, res.NormalizedHourlyUSD)
		}
	}

	// Verify specific calculated monthly costs:
	// AWS: 500 * 0.023 = 11.5
	if awsRes := resultsByProvider["aws"]; awsRes.MonthlyCostUSD.String() != "11.5" {
		t.Errorf("AWS MonthlyCostUSD = %s, want 11.5", awsRes.MonthlyCostUSD.String())
	}
	// Azure: 500 * 0.0184 = 9.2
	if azRes := resultsByProvider["azure"]; azRes.MonthlyCostUSD.String() != "9.2" {
		t.Errorf("Azure MonthlyCostUSD = %s, want 9.2", azRes.MonthlyCostUSD.String())
	}
	// GCP: 500 * 0.020 = 10
	if gcpRes := resultsByProvider["gcp"]; gcpRes.MonthlyCostUSD.String() != "10" {
		t.Errorf("GCP MonthlyCostUSD = %s, want 10", gcpRes.MonthlyCostUSD.String())
	}
}

func TestStorageHandler_PartialProviderFailure_Returns200WithWarning(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	// Seed only AWS in cache; DB fails when trying to query Azure/GCP
	ctx := context.Background()
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
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "storage", "us-east-1"), awsObs, cache.DefaultTTL)

	failingQueries := store.New(&failingDBTX{})
	pricingSvc := service.NewPricingService(failingQueries, rdb)
	handler := rest.NewStorageHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/storage?size_gb=100&region=us-east", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should still return 200 OK because AWS succeeded
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK. Body: %s", rec.Code, rec.Body.String())
	}

	var resp rest.StorageComparisonResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result (AWS), got %d", len(resp.Results))
	}

	// Verify warnings contain fetch_failed entries for azure and gcp
	var hasAzureFail, hasGCPFail bool
	for _, w := range resp.Warnings {
		if w.Provider == "azure" && w.Code == "fetch_failed" {
			hasAzureFail = true
		}
		if w.Provider == "gcp" && w.Code == "fetch_failed" {
			hasGCPFail = true
		}
	}
	if !hasAzureFail || !hasGCPFail {
		t.Errorf("expected fetch_failed warnings for azure and gcp, got warnings: %+v", resp.Warnings)
	}
}

func TestStorageHandler_EmptyProviderResults_AddsWarning(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	// Seed empty array for Azure in cache
	ctx := context.Background()
	awsObs := []domain.PriceObservation{
		{
			Provider:          "aws",
			ServiceCategory:   "storage",
			SkuID:             "SKU-AWS-S3-STD",
			PriceAmount:       decimal.RequireFromString("0.023"),
			StorageAttributes: domain.StorageAttributes{SizeGB: 100, StorageClass: "standard"},
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "storage", "us-east-1"), awsObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "storage", "eastus"), []domain.PriceObservation{}, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "storage", "us-east4"), []domain.PriceObservation{}, cache.DefaultTTL)

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewStorageHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/storage?size_gb=100&region=us-east", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK", rec.Code)
	}

	var resp rest.StorageComparisonResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	var hasAzureNoData, hasGCPNoData bool
	for _, w := range resp.Warnings {
		if w.Provider == "azure" && w.Code == "no_data_available" {
			hasAzureNoData = true
		}
		if w.Provider == "gcp" && w.Code == "no_data_available" {
			hasGCPNoData = true
		}
	}
	if !hasAzureNoData || !hasGCPNoData {
		t.Errorf("expected no_data_available warnings for azure and gcp, got warnings: %+v", resp.Warnings)
	}
}

func TestStorageHandler_AllProvidersFail_Returns502(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	failingQueries := store.New(&failingDBTX{})
	pricingSvc := service.NewPricingService(failingQueries, rdb)
	handler := rest.NewStorageHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/storage?region=us-east", nil)
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
