package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

func TestCalculateHandler_CompleteRequest_200OK(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	regionGroup := "us-east"

	// Warm Redis for AWS
	awsCompute := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "compute",
			SkuID:           "m5.xlarge",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.192"),
			PriceCurrency:   "USD",
			Attributes: domain.ComputeAttributes{
				VCPU:   4,
				RAMGB:  16,
				Family: "general_purpose",
			},
			FetchedAt: time.Now().UTC(),
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "compute", regionGroup), awsCompute, cache.DefaultTTL)

	awsStorage := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "storage",
			SkuID:           "s3-standard",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.023"),
			PriceCurrency:   "USD",
			StorageAttributes: domain.StorageAttributes{
				SizeGB:       500,
				StorageClass: "standard",
			},
			FetchedAt: time.Now().UTC(),
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "storage", regionGroup), awsStorage, cache.DefaultTTL)

	awsNetwork := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "network",
			SkuID:           "data-transfer-out",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.090"),
			PriceCurrency:   "USD",
			NetworkAttributes: domain.NetworkAttributes{
				EgressGB:     100,
				TransferType: "internet_egress",
			},
			FetchedAt: time.Now().UTC(),
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "network", regionGroup), awsNetwork, cache.DefaultTTL)

	pricingSvc := service.NewPricingService(nil, rdb)
	router := rest.NewRouter(pricingSvc, nil, rdb)

	reqBody := map[string]interface{}{
		"region":   regionGroup,
		"currency": "USD",
		"compute": map[string]interface{}{
			"vcpu":          4,
			"ram_gb":        16,
			"family":        "general_purpose",
			"strict_family": true,
		},
		"storage": map[string]interface{}{
			"size_gb":       500,
			"storage_class": "standard",
		},
		"network": map[string]interface{}{
			"egress_gb":     100,
			"transfer_type": "internet_egress",
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d. Body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var rawResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &rawResp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}

	meta, ok := rawResp["meta"].(map[string]interface{})
	if !ok || meta["api_version"] != "v1" {
		t.Errorf("meta.api_version = %v, want 'v1'", meta["api_version"])
	}

	results, ok := rawResp["results"].([]interface{})
	if !ok || len(results) == 0 {
		t.Fatalf("results is empty or invalid type: %v", rawResp["results"])
	}

	awsResult := results[0].(map[string]interface{})
	if awsResult["provider"] != "aws" {
		t.Errorf("provider = %v, want 'aws'", awsResult["provider"])
	}
	if awsResult["partial"] != false {
		t.Errorf("partial = %v, want false", awsResult["partial"])
	}

	// Verify total_normalized_hourly_usd is present in complete result
	if _, hasTotal := awsResult["total_normalized_hourly_usd"]; !hasTotal {
		t.Errorf("total_normalized_hourly_usd key missing from complete result")
	}

	// Verify partial_total_normalized_hourly_usd is NOT present in complete result
	if _, hasPartialTotal := awsResult["partial_total_normalized_hourly_usd"]; hasPartialTotal {
		t.Errorf("partial_total_normalized_hourly_usd should NOT exist in complete result, got: %v", awsResult["partial_total_normalized_hourly_usd"])
	}

	categories, ok := awsResult["categories"].(map[string]interface{})
	if !ok || len(categories) != 3 {
		t.Fatalf("got %d categories, want 3", len(categories))
	}
}

func TestCalculateHandler_PartialProvider_ADR0022_KeyAbsence(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	regionGroup := "us-east"

	// AWS has Compute and Storage, but NO Network data
	awsCompute := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "compute",
			SkuID:           "m5.xlarge",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.192"),
			PriceCurrency:   "USD",
			Attributes: domain.ComputeAttributes{
				VCPU:  4,
				RAMGB: 16,
			},
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "compute", regionGroup), awsCompute, cache.DefaultTTL)

	awsStorage := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "storage",
			SkuID:           "s3-standard",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.023"),
			PriceCurrency:   "USD",
			StorageAttributes: domain.StorageAttributes{
				SizeGB:       500,
				StorageClass: "standard",
			},
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "storage", regionGroup), awsStorage, cache.DefaultTTL)

	pricingSvc := service.NewPricingService(nil, rdb)
	router := rest.NewRouter(pricingSvc, nil, rdb)

	reqBody := map[string]interface{}{
		"region": regionGroup,
		"compute": map[string]interface{}{
			"vcpu":   4,
			"ram_gb": 16,
		},
		"storage": map[string]interface{}{
			"size_gb": 500,
		},
		"network": map[string]interface{}{
			"egress_gb": 100,
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d. Body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var rawResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &rawResp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}

	results, ok := rawResp["results"].([]interface{})
	if !ok || len(results) == 0 {
		t.Fatalf("results is empty: %v", rawResp["results"])
	}

	awsResult := results[0].(map[string]interface{})
	if awsResult["provider"] != "aws" {
		t.Errorf("provider = %v, want 'aws'", awsResult["provider"])
	}

	// Mandatory ADR 0022 Honesty Contract assertions:
	// 1. partial MUST be true
	if awsResult["partial"] != true {
		t.Errorf("partial = %v, want true for partial provider", awsResult["partial"])
	}

	// 2. total_normalized_hourly_usd MUST NOT exist in JSON (key absence check)
	if val, hasTotal := awsResult["total_normalized_hourly_usd"]; hasTotal {
		t.Fatalf("VIOLATION OF ADR 0022: total_normalized_hourly_usd key MUST NOT exist for partial provider! Got: %v", val)
	}

	// 3. partial_total_normalized_hourly_usd MUST exist
	if _, hasPartialTotal := awsResult["partial_total_normalized_hourly_usd"]; !hasPartialTotal {
		t.Fatalf("partial_total_normalized_hourly_usd key missing for partial provider")
	}

	// 4. Missing category 'network' MUST NOT be in categories map
	categories := awsResult["categories"].(map[string]interface{})
	if _, hasNetwork := categories["network"]; hasNetwork {
		t.Errorf("missing category 'network' must not appear in categories map")
	}
	if _, hasCompute := categories["compute"]; !hasCompute {
		t.Errorf("category 'compute' should appear in categories map")
	}
	if _, hasStorage := categories["storage"]; !hasStorage {
		t.Errorf("category 'storage' should appear in categories map")
	}
}

func TestCalculateHandler_EmptyCategoryRequest_Returns400RFC7807(t *testing.T) {
	pricingSvc := service.NewPricingService(nil, nil)
	router := rest.NewRouter(pricingSvc, nil, nil)

	reqBody := map[string]interface{}{
		"region":   "us-east",
		"currency": "USD",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var rfcErr middleware.RFC7807Error
	if err := json.Unmarshal(w.Body.Bytes(), &rfcErr); err != nil {
		t.Fatalf("failed to decode RFC 7807 error: %v", err)
	}
	if rfcErr.Status != http.StatusBadRequest {
		t.Errorf("RFC7807 status = %d, want 400", rfcErr.Status)
	}
}

func TestCalculateHandler_InvalidParameters_Returns400RFC7807(t *testing.T) {
	pricingSvc := service.NewPricingService(nil, nil)
	router := rest.NewRouter(pricingSvc, nil, nil)

	testCases := []struct {
		name    string
		reqBody map[string]interface{}
	}{
		{
			name: "negative vcpu",
			reqBody: map[string]interface{}{
				"compute": map[string]interface{}{"vcpu": -4},
			},
		},
		{
			name: "negative ram_gb",
			reqBody: map[string]interface{}{
				"compute": map[string]interface{}{"ram_gb": -16},
			},
		},
		{
			name: "negative size_gb",
			reqBody: map[string]interface{}{
				"storage": map[string]interface{}{"size_gb": -500},
			},
		},
		{
			name: "size_gb exceeding 1,000,000",
			reqBody: map[string]interface{}{
				"storage": map[string]interface{}{"size_gb": 2_000_000},
			},
		},
		{
			name: "negative egress_gb",
			reqBody: map[string]interface{}{
				"network": map[string]interface{}{"egress_gb": -100},
			},
		},
		{
			name: "egress_gb exceeding 10,000,000",
			reqBody: map[string]interface{}{
				"network": map[string]interface{}{"egress_gb": 20_000_000},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tc.reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("[%s] status code = %d, want %d", tc.name, w.Code, http.StatusBadRequest)
			}

			var rfcErr middleware.RFC7807Error
			if err := json.Unmarshal(w.Body.Bytes(), &rfcErr); err != nil {
				t.Fatalf("[%s] failed to decode RFC7807 JSON: %v", tc.name, err)
			}
			if rfcErr.Status != http.StatusBadRequest {
				t.Errorf("[%s] RFC7807 status = %d, want 400", tc.name, rfcErr.Status)
			}
		})
	}
}

func TestCalculateHandler_MethodNotAllowed_Returns405(t *testing.T) {
	pricingSvc := service.NewPricingService(nil, nil)
	router := rest.NewRouter(pricingSvc, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/calculate", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status code = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestCalculateHandler_NonUSDCurrency_Warning(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	regionGroup := "us-east"

	awsCompute := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "compute",
			SkuID:           "m5.xlarge",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.192"),
			Attributes: domain.ComputeAttributes{
				VCPU:  4,
				RAMGB: 16,
			},
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "compute", regionGroup), awsCompute, cache.DefaultTTL)

	pricingSvc := service.NewPricingService(nil, rdb)
	router := rest.NewRouter(pricingSvc, nil, rdb)

	reqBody := map[string]interface{}{
		"region":   regionGroup,
		"currency": "EUR",
		"compute": map[string]interface{}{
			"vcpu":   4,
			"ram_gb": 16,
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	var rawResp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &rawResp)
	warnings := rawResp["warnings"].([]interface{})

	var currencyWarnFound bool
	for _, wItem := range warnings {
		wm := wItem.(map[string]interface{})
		if wm["provider"] == "system" && wm["code"] == "currency_conversion_not_yet_supported" {
			currencyWarnFound = true
			break
		}
	}
	if !currencyWarnFound {
		t.Errorf("expected currency warning in response, got: %+v", warnings)
	}
}
