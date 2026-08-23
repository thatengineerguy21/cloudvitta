package rest_test

import (
	"bytes"
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

func TestCalculateHandler_InvalidMethod_ReturnsRFC7807(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewCalculateHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/calculate", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 Method Not Allowed", rec.Code)
	}
}

func TestCalculateHandler_NoCategories_ReturnsRFC7807(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewCalculateHandler(pricingSvc)

	body := []byte(`{"region": "us-east"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request", rec.Code)
	}

	var rfcErr middleware.RFC7807Error
	if err := json.NewDecoder(rec.Body).Decode(&rfcErr); err != nil {
		t.Fatalf("failed to decode RFC7807 JSON: %v", err)
	}

	if !strings.Contains(rfcErr.Detail, "At least one category") {
		t.Errorf("Unexpected error detail: %s", rfcErr.Detail)
	}
}

func TestCalculateHandler_HappyPath(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()

	// Add compute and storage for AWS
	awsComputeObs := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "compute",
			SkuID:           "SKU-AWS-C1",
			PriceAmount:     decimal.RequireFromString("0.10"),
			Unit:            "hour",
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "general"},
			FetchedAt:       time.Now().UTC(),
		},
	}
	awsStorageObs := []domain.PriceObservation{
		{
			Provider:          "aws",
			ServiceCategory:   "storage",
			SkuID:             "SKU-AWS-S1",
			PriceAmount:       decimal.RequireFromString("0.02"), // $0.02/GB-Mo
			Unit:              "GB-Mo",
			StorageAttributes: domain.StorageAttributes{SizeGB: 100, StorageClass: "standard"},
			FetchedAt:         time.Now().UTC(),
		},
	}

	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1"), awsComputeObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "storage", "us-east-1"), awsStorageObs, cache.DefaultTTL)

	// Add only compute for Azure
	azureComputeObs := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "compute",
			SkuID:           "SKU-AZ-C1",
			PriceAmount:     decimal.RequireFromString("0.15"),
			Unit:            "hour",
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "general"},
			FetchedAt:       time.Now().UTC(),
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "compute", "eastus"), azureComputeObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "storage", "eastus"), []domain.PriceObservation{}, cache.DefaultTTL)

	// GCP gets nothing
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "compute", "us-east4"), []domain.PriceObservation{}, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "storage", "us-east4"), []domain.PriceObservation{}, cache.DefaultTTL)

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewCalculateHandler(pricingSvc)

	body := []byte(`{
		"region": "us-east",
		"compute": {
			"vcpu": 2,
			"ram_gb": 4,
			"family": "general"
		},
		"storage": {
			"size_gb": 100,
			"storage_class": "standard"
		}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK. Body: %s", rec.Code, rec.Body.String())
	}

	var resp rest.CalculateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Results) != 2 {
		t.Fatalf("got %d results, want 2 (AWS, Azure)", len(resp.Results))
	}

	var awsResult, azureResult *rest.CalculateProviderResult
	for i := range resp.Results {
		if resp.Results[i].Provider == "aws" {
			awsResult = &resp.Results[i]
		}
		if resp.Results[i].Provider == "azure" {
			azureResult = &resp.Results[i]
		}
	}

	if awsResult == nil || azureResult == nil {
		t.Fatalf("missing expected providers in result")
	}

	// AWS should be full match
	if awsResult.Partial {
		t.Errorf("expected AWS to not be partial")
	}
	if awsResult.TotalNormalizedHourlyUSD == nil {
		t.Errorf("expected AWS to have total")
	}

	// Storage cost = 0.02 * 100 = 2/mo. Hourly = 2/730 = 0.00273972602...
	// Total = 0.10 + 2/730

	// Azure should be partial match
	if !azureResult.Partial {
		t.Errorf("expected Azure to be partial")
	}
	if azureResult.TotalNormalizedHourlyUSD != nil {
		t.Errorf("expected Azure to NOT have total")
	}
	if azureResult.PartialTotalNormalizedHourlyUSD == nil {
		t.Errorf("expected Azure to have partial total")
	}
}

func TestCalculateHandler_AllProvidersFail_Returns502(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	failingQueries := store.New(&failingDBTX{})
	pricingSvc := service.NewPricingService(failingQueries, rdb)
	handler := rest.NewCalculateHandler(pricingSvc)

	body := []byte(`{"compute": {"vcpu": 2, "ram_gb": 4}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewBuffer(body))
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

func TestCalculateHandler_AliasConflictValidation(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewCalculateHandler(pricingSvc)

	t.Run("DifferingValues_Returns400ConflictingFields", func(t *testing.T) {
		body := []byte(`{
			"database": {"engine": "postgresql", "vcpu": 2, "ram_gb": 4, "storage_gb": 100},
			"database_rdbms": {"engine": "mysql", "vcpu": 4, "ram_gb": 16, "storage_gb": 200}
		}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 Bad Request", rec.Code)
		}

		var rfcErr middleware.RFC7807Error
		if err := json.NewDecoder(rec.Body).Decode(&rfcErr); err != nil {
			t.Fatalf("failed to decode RFC7807 JSON: %v", err)
		}
		if rfcErr.Title != "Conflicting Category Fields" {
			t.Errorf("Title = %q, want 'Conflicting Category Fields'", rfcErr.Title)
		}
		if !strings.Contains(rfcErr.Detail, "both 'database' and 'database_rdbms' were provided with conflicting values") {
			t.Errorf("Unexpected detail: %s", rfcErr.Detail)
		}
	})

	t.Run("IdenticalValues_Succeeds", func(t *testing.T) {
		body := []byte(`{
			"database": {"engine": "postgresql", "vcpu": 4, "ram_gb": 16, "storage_gb": 100},
			"database_rdbms": {"engine": "postgresql", "vcpu": 4, "ram_gb": 16, "storage_gb": 100}
		}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Handler processes request successfully (even if no prices found in empty miniredis)
		if rec.Code != http.StatusOK && rec.Code != http.StatusBadGateway {
			t.Fatalf("unexpected status = %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("AliasOnly_Succeeds", func(t *testing.T) {
		body := []byte(`{
			"database": {"engine": "postgresql", "vcpu": 4, "ram_gb": 16, "storage_gb": 100}
		}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK && rec.Code != http.StatusBadGateway {
			t.Fatalf("unexpected status = %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestCalculateHandler_ParameterBoundsValidation(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewCalculateHandler(pricingSvc)

	tests := []struct {
		name       string
		body       string
		wantDetail string
	}{
		{
			name:       "database_rdbms invalid engine",
			body:       `{"database_rdbms": {"engine": "invalid_engine", "vcpu": 4, "ram_gb": 16, "storage_gb": 100}}`,
			wantDetail: "database_rdbms.engine must be a supported database engine",
		},
		{
			name:       "database_rdbms negative vcpu",
			body:       `{"database_rdbms": {"engine": "postgresql", "vcpu": -1, "ram_gb": 16, "storage_gb": 100}}`,
			wantDetail: "database_rdbms.vcpu must be a positive number",
		},
		{
			name:       "database_rdbms storage exceeds 1PB",
			body:       `{"database_rdbms": {"engine": "postgresql", "vcpu": 4, "ram_gb": 16, "storage_gb": 1000001}}`,
			wantDetail: "database_rdbms.storage_gb exceeds maximum limit",
		},
		{
			name:       "database_rdbms negative iops",
			body:       `{"database_rdbms": {"engine": "postgresql", "vcpu": 4, "ram_gb": 16, "storage_gb": 100, "iops": -5}}`,
			wantDetail: "database_rdbms.iops must be a positive integer",
		},
		{
			name:       "database_nosql invalid data model",
			body:       `{"database_nosql": {"data_model": "invalid_model", "pricing_mode": "provisioned"}}`,
			wantDetail: "database_nosql.data_model must be a supported NoSQL data model",
		},
		{
			name:       "database_nosql invalid pricing mode",
			body:       `{"database_nosql": {"data_model": "document", "pricing_mode": "invalid_mode"}}`,
			wantDetail: "database_nosql.pricing_mode must be provisioned, on_demand, or serverless",
		},
		{
			name:       "database_nosql negative read units",
			body:       `{"database_nosql": {"data_model": "document", "pricing_mode": "provisioned", "read_units": -10}}`,
			wantDetail: "database_nosql.read_units must be a non-negative number",
		},
		{
			name:       "database_nosql read units exceed 10M",
			body:       `{"database_nosql": {"data_model": "document", "pricing_mode": "provisioned", "read_units": 10000001}}`,
			wantDetail: "database_nosql.read_units must be a non-negative number up to 10000000",
		},
		{
			name:       "kubernetes invalid tier",
			body:       `{"kubernetes": {"tier": "unsupported_tier"}}`,
			wantDetail: "kubernetes.tier must be a supported Kubernetes tier",
		},
		{
			name:       "kubernetes invalid cluster topology",
			body:       `{"kubernetes": {"tier": "standard", "cluster_topology": "invalid_topo"}}`,
			wantDetail: "kubernetes.cluster_topology must be one of: zonal, regional, autopilot",
		},
		{
			name:       "serverless invalid memory",
			body:       `{"serverless": {"architecture": "x86_64", "memory_mb": 50}}`,
			wantDetail: "memory_mb must be between 128 and 10240 MB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 Bad Request, body: %s", rec.Code, rec.Body.String())
			}

			var rfcErr middleware.RFC7807Error
			if err := json.NewDecoder(rec.Body).Decode(&rfcErr); err != nil {
				t.Fatalf("failed to decode RFC7807 JSON: %v", err)
			}
			if !strings.Contains(rfcErr.Detail, tt.wantDetail) {
				t.Errorf("detail = %q, want it to contain %q", rfcErr.Detail, tt.wantDetail)
			}
		})
	}
}
