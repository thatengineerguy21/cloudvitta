package ibm_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/ibm"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
)

func TestAdapter_Fetch_HappyPath(t *testing.T) {
	fixtureBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "testdata", "golden", "ibm", "compute.json"))
	if err != nil {
		t.Fatalf("failed to read test fixture: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixtureBytes)
	}))
	defer ts.Close()

	client := ibm.NewClient(ibm.WithCatalogURL(ts.URL), ibm.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := ibm.NewAdapter(client, memStorage)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := adapter.Fetch(ctx, nil)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}
	if result.RawGCSPath == "" {
		t.Fatalf("Fetch() returned empty gcsPath")
	}

	if len(result.Observations) != 60 {
		t.Fatalf("Fetch() returned %d observations, want 60", len(result.Observations))
	}

	var foundBX2_2vCPU *domain.PriceObservation
	for i := range result.Observations {
		if result.Observations[i].SkuID == "SKU-IBM-VPC-BX2-2X8" && result.Observations[i].Region == "us-east" {
			foundBX2_2vCPU = &result.Observations[i]
			break
		}
	}

	if foundBX2_2vCPU == nil {
		t.Fatalf("Fetch() missing observation for SKU-IBM-VPC-BX2-2X8 in us-east")
	}

	if foundBX2_2vCPU.Provider != "ibm" {
		t.Errorf("Provider = %q, want ibm", foundBX2_2vCPU.Provider)
	}
	if foundBX2_2vCPU.ServiceCategory != "compute" {
		t.Errorf("ServiceCategory = %q, want compute", foundBX2_2vCPU.ServiceCategory)
	}
	if foundBX2_2vCPU.RegionGroup != "us-east" {
		t.Errorf("RegionGroup = %q, want us-east", foundBX2_2vCPU.RegionGroup)
	}
	if foundBX2_2vCPU.PriceAmount.String() != "0.096" {
		t.Errorf("PriceAmount = %s, want 0.096", foundBX2_2vCPU.PriceAmount.String())
	}
	if foundBX2_2vCPU.Attributes.VCPU != 2 {
		t.Errorf("Attributes.VCPU = %v, want 2", foundBX2_2vCPU.Attributes.VCPU)
	}
	if foundBX2_2vCPU.Attributes.RAMGB != 8 {
		t.Errorf("Attributes.RAMGB = %v, want 8", foundBX2_2vCPU.Attributes.RAMGB)
	}
	if foundBX2_2vCPU.Attributes.Family != "general_purpose" {
		t.Errorf("Attributes.Family = %q, want general_purpose", foundBX2_2vCPU.Attributes.Family)
	}

	// Verify raw storage payload write
	todayStr := time.Now().UTC().Format("2006-01-02")
	prefix := "raw/ibm/compute/" + todayStr + "/"

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

func TestAdapter_Fetch_WithIAMAuth(t *testing.T) {
	var tokenReceived string

	iamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token": "valid-iam-token-12345", "token_type": "Bearer", "expires_in": 3600}`))
	}))
	defer iamServer.Close()

	catalogServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenReceived = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"resources": [
				{
					"id": "is.instance",
					"name": "is.instance",
					"geo_tags": ["us-east"],
					"pricing": {
						"metrics": [
							{
								"metric_id": "bx2-2x8",
								"amounts": [{"currency": "USD", "prices": [{"price": 0.096}]}]
							}
						]
					}
				}
			]
		}`))
	}))
	defer catalogServer.Close()

	client := ibm.NewClient(
		ibm.WithIAMURL(iamServer.URL),
		ibm.WithCatalogURL(catalogServer.URL),
		ibm.WithAPIKey("secret-api-key"),
		ibm.WithHTTPClient(catalogServer.Client()),
	)
	memStorage := storage.NewMemoryRawStorage()
	adapter := ibm.NewAdapter(client, memStorage)

	result, err := adapter.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}

	if tokenReceived != "Bearer valid-iam-token-12345" {
		t.Errorf("expected Bearer valid-iam-token-12345, got %q", tokenReceived)
	}
	if len(result.Observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(result.Observations))
	}
}

func TestAdapter_Fetch_UnmappedProduct_Quarantined(t *testing.T) {
	jsonBody := `{
		"resources": [
			{
				"id": "unmapped.ibm.service",
				"name": "unmapped.ibm.service",
				"pricing": {
					"metrics": [
						{
							"metric_id": "custom-metric-1",
							"amounts": [{"currency": "USD", "prices": [{"price": 0.50}]}]
						}
					]
				}
			}
		]
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(jsonBody))
	}))
	defer ts.Close()

	client := ibm.NewClient(ibm.WithCatalogURL(ts.URL), ibm.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := ibm.NewAdapter(client, memStorage)

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

	client := ibm.NewClient(ibm.WithCatalogURL(ts.URL), ibm.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := ibm.NewAdapter(client, memStorage)

	_, err := adapter.Fetch(context.Background(), nil)
	if err == nil {
		t.Fatalf("Fetch() expected error for HTTP 500, got nil")
	}
}

func TestSupportedCategories(t *testing.T) {
	if !ibm.IsCategorySupported("compute") {
		t.Errorf("expected compute to be supported")
	}
	if ibm.IsCategorySupported("unknown-category") {
		t.Errorf("expected unknown-category to not be supported")
	}
	cats := ibm.SupportedCategories()
	if len(cats) != 1 || cats[0] != "compute" {
		t.Errorf("SupportedCategories() = %v, want [compute]", cats)
	}
}

func TestAdapter_Fetch_GCSCompletesOnNormalizeFailure(t *testing.T) {
	invalidJSON := `{"resources": [ { "broken_json`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(invalidJSON))
	}))
	defer ts.Close()

	client := ibm.NewClient(ibm.WithCatalogURL(ts.URL), ibm.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := ibm.NewAdapter(client, memStorage)

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
