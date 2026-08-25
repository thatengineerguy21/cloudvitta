package oracle_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/oracle"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
)

func TestAdapter_Fetch_HappyPath(t *testing.T) {
	fixtureBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "testdata", "golden", "oracle", "compute.json"))
	if err != nil {
		t.Fatalf("failed to read test fixture: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixtureBytes)
	}))
	defer ts.Close()

	client := oracle.NewClient(oracle.WithURL(ts.URL), oracle.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := oracle.NewAdapter(client, memStorage)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := adapter.Fetch(ctx, nil)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}
	if result.RawGCSPath == "" {
		t.Fatalf("Fetch() returned empty gcsPath")
	}

	if len(result.Observations) != 12 {
		t.Fatalf("Fetch() returned %d observations, want 12", len(result.Observations))
	}

	var foundE4_2vCPU *domain.PriceObservation
	for i := range result.Observations {
		if result.Observations[i].SkuID == "SKU-OCI-VM-STANDARD-E4-FLEX-2VCPU-8GB" {
			foundE4_2vCPU = &result.Observations[i]
			break
		}
	}

	if foundE4_2vCPU == nil {
		t.Fatalf("Fetch() missing observation for SKU-OCI-VM-STANDARD-E4-FLEX-2VCPU-8GB")
	}

	if foundE4_2vCPU.Provider != "oracle" {
		t.Errorf("Provider = %q, want oracle", foundE4_2vCPU.Provider)
	}
	if foundE4_2vCPU.ServiceCategory != "compute" {
		t.Errorf("ServiceCategory = %q, want compute", foundE4_2vCPU.ServiceCategory)
	}
	if foundE4_2vCPU.RegionGroup != "us-east" {
		t.Errorf("RegionGroup = %q, want us-east", foundE4_2vCPU.RegionGroup)
	}
	if foundE4_2vCPU.PriceAmount.String() != "0.037" {
		t.Errorf("PriceAmount = %s, want 0.037", foundE4_2vCPU.PriceAmount.String())
	}
	if foundE4_2vCPU.Attributes.VCPU != 2 {
		t.Errorf("Attributes.VCPU = %v, want 2", foundE4_2vCPU.Attributes.VCPU)
	}
	if foundE4_2vCPU.Attributes.RAMGB != 8 {
		t.Errorf("Attributes.RAMGB = %v, want 8", foundE4_2vCPU.Attributes.RAMGB)
	}

	// Verify raw storage payload write
	todayStr := time.Now().UTC().Format("2006-01-02")
	prefix := "raw/oracle/compute/" + todayStr + "/"

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

func TestAdapter_Fetch_UnmappedProduct_Ignored(t *testing.T) {
	jsonBody := `{
		"items": [
			{
				"partNumber": "B99999",
				"displayName": "Unknown Oracle Cloud Product",
				"serviceCategory": "UnmappedOracleProductCategory",
				"prices": [
					{
						"currencyCode": "USD",
						"prices": [{"model": "PAY_AS_YOU_GO", "value": 0.50}]
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

	client := oracle.NewClient(oracle.WithURL(ts.URL), oracle.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := oracle.NewAdapter(client, memStorage)

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

func TestAdapter_Fetch_HTTPError_FailsCleanly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := oracle.NewClient(oracle.WithURL(ts.URL), oracle.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := oracle.NewAdapter(client, memStorage)

	_, err := adapter.Fetch(context.Background(), nil)
	if err == nil {
		t.Fatalf("Fetch() expected error for HTTP 500, got nil")
	}
}

func TestSupportedCategories(t *testing.T) {
	if !oracle.IsCategorySupported("compute") {
		t.Errorf("expected compute to be supported")
	}
	if !oracle.IsCategorySupported("storage") {
		t.Errorf("expected storage to be supported")
	}
	if !oracle.IsCategorySupported("network") {
		t.Errorf("expected network to be supported")
	}
	if oracle.IsCategorySupported("unknown-category") {
		t.Errorf("expected unknown-category to not be supported")
	}
	cats := oracle.SupportedCategories()
	if len(cats) != 3 {
		t.Errorf("SupportedCategories() = %v, want 3 categories", cats)
	}
}

func TestAdapter_Fetch_GCSCompletesOnNormalizeFailure(t *testing.T) {
	invalidJSON := `{"items": [ { "broken_json_here`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(invalidJSON))
	}))
	defer ts.Close()

	client := oracle.NewClient(oracle.WithURL(ts.URL), oracle.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := oracle.NewAdapter(client, memStorage)

	_, err := adapter.Fetch(context.Background(), nil)
	if err == nil {
		t.Fatalf("Fetch() expected error due to invalid JSON, got nil")
	}

	files := memStorage.GetFiles()
	if len(files) == 0 {
		t.Fatalf("Fetch() did not write to GCS on normalize failure")
	}

	var storedBytes []byte
	for _, v := range files {
		storedBytes = v
		break
	}

	if string(storedBytes) != invalidJSON {
		t.Fatalf("Fetch() GCS payload mismatch.\nGot: %s\nWant: %s", string(storedBytes), invalidJSON)
	}
}
