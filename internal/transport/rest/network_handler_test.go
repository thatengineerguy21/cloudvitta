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

func TestNetworkHandler_InvalidEgressGB_ReturnsRFC7807(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewNetworkHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/network?egress_gb=-10", nil)
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
	if !strings.Contains(rfcErr.Detail, "egress_gb must be a positive number") {
		t.Errorf("Detail = %q, want egress_gb error detail", rfcErr.Detail)
	}
}

func TestNetworkHandler_ExceedsMaxEgressGB_ReturnsRFC7807(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewNetworkHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/network?egress_gb=10000001", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request", rec.Code)
	}

	var rfcErr middleware.RFC7807Error
	if err := json.NewDecoder(rec.Body).Decode(&rfcErr); err != nil {
		t.Fatalf("failed to decode RFC7807 JSON: %v", err)
	}

	if !strings.Contains(rfcErr.Detail, "no greater than 10000000") {
		t.Errorf("Detail = %q, want upper bound error message", rfcErr.Detail)
	}
}

func TestNetworkHandler_MethodNotAllowed(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewNetworkHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/prices/network", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 Method Not Allowed", rec.Code)
	}
}

func TestNetworkHandler_HappyPath_CalculatesMonthlyCosts(t *testing.T) {
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
			ServiceCategory:   "network",
			SkuID:             "SKU-AWS-DT-FLAT",
			DisplayName:       "AWS Data Transfer Out",
			Region:            "us-east-1",
			RegionGroup:       "us-east",
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.090"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 1000},
			FetchedAt:         time.Now().UTC(),
		},
	}
	azureObs := []domain.PriceObservation{
		{
			Provider:          "azure",
			ServiceCategory:   "network",
			SkuID:             "SKU-AZ-BW-FLAT",
			DisplayName:       "Bandwidth Data Transfer Out",
			Region:            "eastus",
			RegionGroup:       "us-east",
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.087"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 1000},
			FetchedAt:         time.Now().UTC(),
		},
	}
	gcpObs := []domain.PriceObservation{
		{
			Provider:          "gcp",
			ServiceCategory:   "network",
			SkuID:             "SKU-GCP-NET-FLAT",
			DisplayName:       "Network Internet Egress",
			Region:            "us-east4",
			RegionGroup:       "us-east",
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.085"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 1000},
			FetchedAt:         time.Now().UTC(),
		},
	}

	oracleObs := []domain.PriceObservation{
		{
			Provider:          "oracle",
			ServiceCategory:   "network",
			SkuID:             "SKU-OCI-NET-EGRESS",
			DisplayName:       "Outbound Data Transfer",
			Region:            "us-ashburn-1",
			RegionGroup:       "us-east",
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.0085"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 1000},
			FetchedAt:         time.Now().UTC(),
		},
	}
	ibmObs := []domain.PriceObservation{
		{
			Provider:          "ibm",
			ServiceCategory:   "network",
			SkuID:             "SKU-IBM-NET-EGRESS",
			DisplayName:       "Public Egress",
			Region:            "us-east",
			RegionGroup:       "us-east",
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.090"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 1000},
			FetchedAt:         time.Now().UTC(),
		},
	}
	aliObs := []domain.PriceObservation{
		{
			Provider:          "alibaba",
			ServiceCategory:   "network",
			SkuID:             "SKU-ALI-NET-EGRESS",
			DisplayName:       "Data Transfer Out",
			Region:            "us-east-1",
			RegionGroup:       "us-east",
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.080"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 1000},
			FetchedAt:         time.Now().UTC(),
		},
	}
	doObs := []domain.PriceObservation{
		{
			Provider:          "digitalocean",
			ServiceCategory:   "network",
			SkuID:             "SKU-DO-NET-EGRESS",
			DisplayName:       "Outbound Bandwidth",
			Region:            "nyc3",
			RegionGroup:       "us-east",
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.010"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 1000},
			FetchedAt:         time.Now().UTC(),
		},
	}

	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "network", "us-east-1"), awsObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "network", "eastus"), azureObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "network", "us-east4"), gcpObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "oracle", "network", "us-ashburn-1"), oracleObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "ibm", "network", "us-east"), ibmObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "alibaba", "network", "us-east-1"), aliObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "digitalocean", "network", "nyc3"), doObs, cache.DefaultTTL)

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewNetworkHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/network?egress_gb=1000&region=us-east", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK. Body: %s", rec.Code, rec.Body.String())
	}

	var resp rest.NetworkComparisonResponse
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

	resultsByProvider := make(map[string]rest.NetworkResultEntry)
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
	// AWS: 1000 * 0.090 = 90
	if awsRes := resultsByProvider["aws"]; awsRes.MonthlyCostUSD.String() != "90" {
		t.Errorf("AWS MonthlyCostUSD = %s, want 90", awsRes.MonthlyCostUSD.String())
	}
	// Azure: 1000 * 0.087 = 87
	if azRes := resultsByProvider["azure"]; azRes.MonthlyCostUSD.String() != "87" {
		t.Errorf("Azure MonthlyCostUSD = %s, want 87", azRes.MonthlyCostUSD.String())
	}
	// GCP: 1000 * 0.085 = 85
	if gcpRes := resultsByProvider["gcp"]; gcpRes.MonthlyCostUSD.String() != "85" {
		t.Errorf("GCP MonthlyCostUSD = %s, want 85", gcpRes.MonthlyCostUSD.String())
	}
}

func TestNetworkHandler_PartialProviderFailure_Returns200WithWarning(t *testing.T) {
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
			ServiceCategory:   "network",
			SkuID:             "SKU-AWS-DT-FLAT",
			DisplayName:       "AWS Data Transfer Out",
			Region:            "us-east-1",
			RegionGroup:       "us-east",
			Unit:              "GB",
			PriceAmount:       decimal.RequireFromString("0.090"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 100},
			FetchedAt:         time.Now().UTC(),
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "network", "us-east-1"), awsObs, cache.DefaultTTL)

	failingQueries := store.New(&failingDBTX{})
	pricingSvc := service.NewPricingService(failingQueries, rdb)
	handler := rest.NewNetworkHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/network?egress_gb=100&region=us-east", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK. Body: %s", rec.Code, rec.Body.String())
	}

	var resp rest.NetworkComparisonResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result (AWS), got %d", len(resp.Results))
	}

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

func TestNetworkHandler_EmptyProviderResults_AddsWarning(t *testing.T) {
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
			ServiceCategory:   "network",
			SkuID:             "SKU-AWS-DT-FLAT",
			PriceAmount:       decimal.RequireFromString("0.090"),
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 100},
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "network", "us-east-1"), awsObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "network", "eastus"), []domain.PriceObservation{}, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "network", "us-east4"), []domain.PriceObservation{}, cache.DefaultTTL)

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewNetworkHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/network?egress_gb=100&region=us-east", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK", rec.Code)
	}

	var resp rest.NetworkComparisonResponse
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

func TestNetworkHandler_AllProvidersFail_Returns502(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	failingQueries := store.New(&failingDBTX{})
	pricingSvc := service.NewPricingService(failingQueries, rdb)
	handler := rest.NewNetworkHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/network?region=us-east", nil)
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
