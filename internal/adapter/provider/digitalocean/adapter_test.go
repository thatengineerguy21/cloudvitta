package digitalocean_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/digitalocean"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
)

func TestAdapter_Fetch_HappyPath(t *testing.T) {
	fixtureBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "testdata", "golden", "digitalocean", "compute.json"))
	if err != nil {
		t.Fatalf("failed to read test fixture: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixtureBytes)
	}))
	defer ts.Close()

	client := digitalocean.NewClient(digitalocean.WithURL(ts.URL), digitalocean.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := digitalocean.NewAdapter(client, memStorage)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := adapter.Fetch(ctx, nil)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}
	if result.RawGCSPath == "" {
		t.Fatalf("Fetch() returned empty gcsPath")
	}

	if len(result.Observations) != 58 {
		t.Fatalf("Fetch() returned %d observations, want 58", len(result.Observations))
	}

	var foundBasic *domain.PriceObservation
	for i := range result.Observations {
		if result.Observations[i].SkuID == "SKU-DO-DROPLET-S-1VCPU-1GB" && result.Observations[i].Region == "nyc1" {
			foundBasic = &result.Observations[i]
			break
		}
	}

	if foundBasic == nil {
		t.Fatalf("Fetch() missing observation for SKU-DO-DROPLET-S-1VCPU-1GB in nyc1")
	}

	if foundBasic.Provider != "digitalocean" {
		t.Errorf("Provider = %q, want digitalocean", foundBasic.Provider)
	}
	if foundBasic.ServiceCategory != "compute" {
		t.Errorf("ServiceCategory = %q, want compute", foundBasic.ServiceCategory)
	}
	if foundBasic.RegionGroup != "us-east" {
		t.Errorf("RegionGroup = %q, want us-east", foundBasic.RegionGroup)
	}
	if foundBasic.PriceAmount.String() != "0.00893" {
		t.Errorf("PriceAmount = %s, want 0.00893", foundBasic.PriceAmount.String())
	}
	if foundBasic.Attributes.VCPU != 1 {
		t.Errorf("Attributes.VCPU = %v, want 1", foundBasic.Attributes.VCPU)
	}
	if foundBasic.Attributes.RAMGB != 1 {
		t.Errorf("Attributes.RAMGB = %v, want 1", foundBasic.Attributes.RAMGB)
	}
	if foundBasic.Attributes.Family != "general_purpose" {
		t.Errorf("Attributes.Family = %q, want general_purpose", foundBasic.Attributes.Family)
	}

	// Verify raw storage payload write
	todayStr := time.Now().UTC().Format("2006-01-02")
	prefix := "raw/digitalocean/compute/" + todayStr + "/"

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

func TestAdapter_Fetch_UnmappedRegion_Quarantined(t *testing.T) {
	jsonBody := `{
		"sizes": [
			{
				"slug": "s-unmapped-droplet",
				"memory": 1024,
				"vcpus": 1,
				"price_hourly": 0.01,
				"regions": ["unmapped-mars-region"]
			}
		]
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(jsonBody))
	}))
	defer ts.Close()

	client := digitalocean.NewClient(digitalocean.WithURL(ts.URL), digitalocean.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := digitalocean.NewAdapter(client, memStorage)

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

	client := digitalocean.NewClient(digitalocean.WithURL(ts.URL), digitalocean.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := digitalocean.NewAdapter(client, memStorage)

	_, err := adapter.Fetch(context.Background(), nil)
	if err == nil {
		t.Fatalf("Fetch() expected error for HTTP 500, got nil")
	}
}

func TestSupportedCategories(t *testing.T) {
	if !digitalocean.IsCategorySupported("compute") {
		t.Errorf("expected compute to be supported")
	}
	if !digitalocean.IsCategorySupported("storage") {
		t.Errorf("expected storage to be supported")
	}
	if !digitalocean.IsCategorySupported("network") {
		t.Errorf("expected network to be supported")
	}
	if digitalocean.IsCategorySupported("database_rdbms") {
		t.Errorf("expected database_rdbms to NOT be supported by DigitalOcean")
	}
	if digitalocean.IsCategorySupported("kubernetes") {
		t.Errorf("expected kubernetes to NOT be supported by DigitalOcean")
	}
	if digitalocean.IsCategorySupported("serverless") {
		t.Errorf("expected serverless to NOT be supported by DigitalOcean")
	}
	if digitalocean.IsCategorySupported("database_nosql") {
		t.Errorf("expected database_nosql to NOT be supported by DigitalOcean (Open Question #7)")
	}
	if digitalocean.IsCategorySupported("unknown-category") {
		t.Errorf("expected unknown-category to NOT be supported")
	}

	cats := digitalocean.SupportedCategories()
	if len(cats) != 3 {
		t.Errorf("SupportedCategories() length = %d, want 3", len(cats))
	}
}

func TestAdapter_Fetch_GCSCompletesOnNormalizeFailure(t *testing.T) {
	invalidJSON := `{"sizes": [ { "broken_json_here`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(invalidJSON))
	}))
	defer ts.Close()

	client := digitalocean.NewClient(digitalocean.WithURL(ts.URL), digitalocean.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := digitalocean.NewAdapter(client, memStorage)

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

func TestClient_FetchRegions_Success(t *testing.T) {
	regionsJSON := `{"regions": [{"slug": "nyc1", "name": "New York 1", "available": true}]}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-do-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(regionsJSON))
	}))
	defer ts.Close()

	client := digitalocean.NewClient(
		digitalocean.WithRegionsURL(ts.URL),
		digitalocean.WithToken("test-do-token"),
		digitalocean.WithHTTPClient(ts.Client()),
	)

	body, err := client.FetchRegions(context.Background())
	if err != nil {
		t.Fatalf("FetchRegions() unexpected error: %v", err)
	}
	defer func() { _ = body.Close() }()
}
