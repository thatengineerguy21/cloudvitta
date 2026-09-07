package gcp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/gcp"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
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
		receivedKey = r.Header.Get("X-Goog-Api-Key")
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

func TestAdapter_Fetch_UnmappedProduct_Ignored(t *testing.T) {
	jsonBody := `{
		"skus": [
			{
				"name": "services/6F81-5844-456A/skus/SKU-UNMAPPED-PRODUCT",
				"skuId": "SKU-UNMAPPED-PRODUCT",
				"description": "Unmapped GCP Service SKU",
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

	result, err := adapter.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}
	if result.UnmappedCount != 0 {
		t.Errorf("Fetch() unmapped count = %d, want 0", result.UnmappedCount)
	}
	if result.IgnoredCount != 1 {
		t.Errorf("Fetch() ignored count = %d, want 1", result.IgnoredCount)
	}
	if len(result.Observations) != 0 {
		t.Errorf("Fetch() observations = %d, want 0", len(result.Observations))
	}
}

func TestAdapter_Fetch_UnmappedRegion_FailsLoudly(t *testing.T) {
	jsonBody := `{
		"skus": [
			{
				"skuId": "SKU-UNMAPPED-REGION",
				"description": "GCP VM in unmapped region (n1-standard-1)",
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

	result, err := adapter.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}
	if result.UnmappedCount != 1 {
		t.Errorf("Fetch() unmapped count = %d, want 1", result.UnmappedCount)
	}
	if len(result.Observations) != 0 {
		t.Errorf("Fetch() observations = %d, want 0", len(result.Observations))
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
	if !gcp.IsCategorySupported("storage") {
		t.Errorf("expected storage to be supported")
	}
	if gcp.IsCategorySupported("unknown-category") {
		t.Errorf("expected unknown-category to not be supported")
	}
}

func TestAdapter_Fetch_Storage_HappyPath(t *testing.T) {
	fixtureBytes, err := os.ReadFile(filepath.Join("testdata", "gcp-storage-us-east-sample.json"))
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

	// In the storage sample fixture:
	// - 1 Standard Storage -> included
	// - 1 Nearline Storage -> included
	// - 1 Archive Storage -> included
	// - 1 Operations -> excluded
	if len(result.Observations) != 3 {
		t.Fatalf("Fetch() returned %d observations, want 3", len(result.Observations))
	}

	var stdObs *domain.PriceObservation
	for i := range result.Observations {
		if result.Observations[i].SkuID == "SKU-GCP-GCS-STD-001" {
			stdObs = &result.Observations[i]
			break
		}
	}
	if stdObs == nil {
		t.Fatalf("missing SKU-GCP-GCS-STD-001 observation")
	}

	if stdObs.Provider != "gcp" {
		t.Errorf("Provider = %q, want gcp", stdObs.Provider)
	}
	if stdObs.ServiceCategory != "storage" {
		t.Errorf("ServiceCategory = %q, want storage", stdObs.ServiceCategory)
	}
	if stdObs.RegionGroup != "us-east" {
		t.Errorf("RegionGroup = %q, want us-east", stdObs.RegionGroup)
	}
	if stdObs.PriceAmount.String() != "0.02" {
		t.Errorf("PriceAmount = %s, want 0.02", stdObs.PriceAmount.String())
	}
	if stdObs.StorageAttributes.StorageClass != "standard" {
		t.Errorf("StorageAttributes.StorageClass = %q, want standard", stdObs.StorageAttributes.StorageClass)
	}
	if stdObs.StorageAttributes.SizeGB != 1 {
		t.Errorf("StorageAttributes.SizeGB = %v, want 1", stdObs.StorageAttributes.SizeGB)
	}
}

func TestAdapter_Fetch_Network_HappyPath(t *testing.T) {
	fixtureBytes, err := os.ReadFile(filepath.Join("testdata", "gcp-network-sample.json"))
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

	// In the network sample fixture:
	// - SKU-GCP-NET-FLAT -> included (1 flat rate)
	// - SKU-GCP-NET-TIERED -> excluded (tiered pricing)
	// - SKU-GCP-NONMATCH -> excluded (Preemptible)
	if len(result.Observations) != 1 {
		t.Fatalf("Fetch() returned %d observations, want 1 (flat rate only)", len(result.Observations))
	}

	obs := result.Observations[0]
	if obs.SkuID != "SKU-GCP-NET-FLAT" {
		t.Errorf("SkuID = %q, want SKU-GCP-NET-FLAT", obs.SkuID)
	}
	if obs.Provider != "gcp" {
		t.Errorf("Provider = %q, want gcp", obs.Provider)
	}
	if obs.ServiceCategory != "network" {
		t.Errorf("ServiceCategory = %q, want network", obs.ServiceCategory)
	}
	if obs.RegionGroup != "us-east" {
		t.Errorf("RegionGroup = %q, want us-east", obs.RegionGroup)
	}
	if obs.PriceAmount.String() != "0.085" {
		t.Errorf("PriceAmount = %s, want 0.085", obs.PriceAmount.String())
	}
	if obs.NetworkAttributes.EgressGB != 1 {
		t.Errorf("NetworkAttributes.EgressGB = %v, want 1", obs.NetworkAttributes.EgressGB)
	}
}

func TestAdapter_Fetch_CategoryFiltering_ExcludesOtherCategories(t *testing.T) {
	fixtureBytes, err := os.ReadFile(filepath.Join("testdata", "gcp-network-sample.json"))
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
	adapter := gcp.NewAdapter(client, memStorage, gcp.WithCategory("compute"))

	result, err := adapter.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}

	// Should filter out all network observations because adapter is configured for "compute"
	if len(result.Observations) != 0 {
		t.Errorf("Fetch() returned %d observations, want 0 (network observations filtered from compute adapter)", len(result.Observations))
	}
}
