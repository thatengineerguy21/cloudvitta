package gcp_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/gcp"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/catalogmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
)

func TestAdapter_Fetch_HappyPath(t *testing.T) {
	fixtureBytes, err := os.ReadFile(filepath.Join("testdata", "gcp-compute-us-east-sample.json"))
	if err != nil {
		t.Fatalf("failed to read test fixture: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixtureBytes)
	}))
	defer ts.Close()

	client := gcp.NewClient(gcp.WithURL(ts.URL), gcp.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := gcp.NewAdapter(client, memStorage)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := adapter.Fetch(ctx, nil)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}
	if result.RawGCSPath == "" {
		t.Fatalf("Fetch() returned empty gcsPath")
	}

	// In the sample fixture:
	// - 1 N2 Standard 4 (OnDemand, Linux) -> included
	// - 1 E2 Standard 2 (OnDemand, Linux) -> included
	// - 1 Windows -> excluded
	// - 1 Spot -> excluded
	// - 1 Commit1Yr -> excluded
	// Total expected: 2 observations
	if len(result.Observations) != 2 {
		t.Fatalf("Fetch() returned %d observations, want 2", len(result.Observations))
	}

	// Verify N2 Standard 4 instance
	var n2Obs *domain.PriceObservation
	for i := range result.Observations {
		if result.Observations[i].SkuID == "SKU-GCP-N2-STD-4" {
			n2Obs = &result.Observations[i]
			break
		}
	}

	if n2Obs == nil {
		t.Fatalf("Fetch() missing observation for SKU-GCP-N2-STD-4")
	}

	if n2Obs.Provider != "gcp" {
		t.Errorf("Provider = %q, want %q", n2Obs.Provider, "gcp")
	}
	if n2Obs.ServiceCategory != "compute" {
		t.Errorf("ServiceCategory = %q, want %q", n2Obs.ServiceCategory, "compute")
	}
	if n2Obs.RegionGroup != "us-east" {
		t.Errorf("RegionGroup = %q, want %q", n2Obs.RegionGroup, "us-east")
	}
	if n2Obs.PriceAmount.String() != "0.1944" {
		t.Errorf("PriceAmount = %s, want 0.1944", n2Obs.PriceAmount.String())
	}
	if n2Obs.Attributes.VCPU != 4 {
		t.Errorf("Attributes.VCPU = %v, want 4", n2Obs.Attributes.VCPU)
	}
	if n2Obs.Attributes.RAMGB != 16 {
		t.Errorf("Attributes.RAMGB = %v, want 16", n2Obs.Attributes.RAMGB)
	}
	if n2Obs.Attributes.Family != "n2" {
		t.Errorf("Attributes.Family = %q, want %q", n2Obs.Attributes.Family, "n2")
	}

	// Verify raw storage payload write
	todayStr := time.Now().UTC().Format("2006-01-02")
	prefix := "raw/gcp/compute/" + todayStr + "/"

	var storedPath string
	for path := range memStorage.GetFiles() {
		if strings.HasPrefix(path, prefix) {
			storedPath = path
			break
		}
	}

	if storedPath == "" {
		t.Errorf("expected file in raw storage with prefix %q", prefix)
	}
}

func TestAdapter_Fetch_WithAPIKey(t *testing.T) {
	var receivedKey string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedKey = r.URL.Query().Get("key")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"skus": []}`))
	}))
	defer ts.Close()

	client := gcp.NewClient(gcp.WithURL(ts.URL), gcp.WithHTTPClient(ts.Client()), gcp.WithAPIKey("test-gcp-api-key-123"))
	memStorage := storage.NewMemoryRawStorage()
	adapter := gcp.NewAdapter(client, memStorage)

	_, err := adapter.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}

	if receivedKey != "test-gcp-api-key-123" {
		t.Errorf("received key = %q, want %q", receivedKey, "test-gcp-api-key-123")
	}
}

func TestAdapter_Fetch_UnmappedProduct_FailsLoudly(t *testing.T) {
	jsonBody := `{
		"skus": [
			{
				"skuId": "SKU-UNMAPPED",
				"description": "Unmapped GCP Service",
				"category": {
					"serviceDisplayName": "UnmappedGCPService",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["us-east1"],
				"pricingInfo": [
					{
						"pricingExpression": {
							"usageUnit": "h",
							"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "1", "nanos": 0}}]
						}
					}
				]
			}
		]
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(jsonBody))
	}))
	defer ts.Close()

	client := gcp.NewClient(gcp.WithURL(ts.URL), gcp.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := gcp.NewAdapter(client, memStorage)

	_, err := adapter.Fetch(context.Background(), nil)
	if err == nil {
		t.Fatalf("Fetch() expected error for unmapped product, got nil")
	}
	if !errors.Is(err, catalogmap.ErrUnmappedProduct) {
		t.Fatalf("Fetch() error = %v, want errors.Is ErrUnmappedProduct", err)
	}
}

func TestAdapter_Fetch_UnmappedRegion_FailsLoudly(t *testing.T) {
	jsonBody := `{
		"skus": [
			{
				"skuId": "SKU-UNMAPPED-REGION",
				"description": "GCP VM in unmapped region",
				"category": {
					"serviceDisplayName": "Compute Engine",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["unmapped-gcp-region-99"],
				"pricingInfo": [
					{
						"pricingExpression": {
							"usageUnit": "h",
							"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "1", "nanos": 0}}]
						}
					}
				]
			}
		]
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(jsonBody))
	}))
	defer ts.Close()

	client := gcp.NewClient(gcp.WithURL(ts.URL), gcp.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := gcp.NewAdapter(client, memStorage)

	_, err := adapter.Fetch(context.Background(), nil)
	if err == nil {
		t.Fatalf("Fetch() expected error for unmapped region, got nil")
	}
	if !errors.Is(err, regionmap.ErrUnmappedRegion) {
		t.Fatalf("Fetch() error = %v, want errors.Is ErrUnmappedRegion", err)
	}
}

func TestAdapter_Fetch_HTTPError_FailsCleanly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := gcp.NewClient(gcp.WithURL(ts.URL), gcp.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := gcp.NewAdapter(client, memStorage)

	_, err := adapter.Fetch(context.Background(), nil)
	if err == nil {
		t.Fatalf("Fetch() expected error for HTTP 500, got nil")
	}
}

func TestSupportedCategories(t *testing.T) {
	if !gcp.IsCategorySupported("compute") {
		t.Errorf("expected compute to be supported")
	}
	if gcp.IsCategorySupported("unknown-category") {
		t.Errorf("expected unknown-category to not be supported")
	}
}
