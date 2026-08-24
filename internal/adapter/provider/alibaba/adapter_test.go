package alibaba_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/alibaba"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
)

func TestAdapter_Fetch_HappyPath(t *testing.T) {
	fixtureBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "testdata", "golden", "alibaba", "compute.json"))
	if err != nil {
		t.Fatalf("failed to read test fixture: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixtureBytes)
	}))
	defer ts.Close()

	client := alibaba.NewClient(alibaba.WithURL(ts.URL), alibaba.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := alibaba.NewAdapter(client, memStorage)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := adapter.Fetch(ctx, nil)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}
	if result.RawGCSPath == "" {
		t.Fatalf("Fetch() returned empty gcsPath")
	}

	if len(result.Observations) != 55 {
		t.Fatalf("Fetch() returned %d observations, want 55", len(result.Observations))
	}

	var foundG7_2vCPU *domain.PriceObservation
	for i := range result.Observations {
		if result.Observations[i].SkuID == "SKU-ALI-ECS-G7-LARGE" && result.Observations[i].Region == "us-east-1" {
			foundG7_2vCPU = &result.Observations[i]
			break
		}
	}

	if foundG7_2vCPU == nil {
		t.Fatalf("Fetch() missing observation for SKU-ALI-ECS-G7-LARGE in us-east-1")
	}

	if foundG7_2vCPU.Provider != "alibaba" {
		t.Errorf("Provider = %q, want alibaba", foundG7_2vCPU.Provider)
	}
	if foundG7_2vCPU.ServiceCategory != "compute" {
		t.Errorf("ServiceCategory = %q, want compute", foundG7_2vCPU.ServiceCategory)
	}
	if foundG7_2vCPU.RegionGroup != "us-east" {
		t.Errorf("RegionGroup = %q, want us-east", foundG7_2vCPU.RegionGroup)
	}
	if foundG7_2vCPU.PriceAmount.String() != "0.096" {
		t.Errorf("PriceAmount = %s, want 0.096", foundG7_2vCPU.PriceAmount.String())
	}
	if foundG7_2vCPU.PriceCurrency != "USD" {
		t.Errorf("PriceCurrency = %q, want USD", foundG7_2vCPU.PriceCurrency)
	}
	if foundG7_2vCPU.Attributes.VCPU != 2 {
		t.Errorf("Attributes.VCPU = %v, want 2", foundG7_2vCPU.Attributes.VCPU)
	}
	if foundG7_2vCPU.Attributes.RAMGB != 8 {
		t.Errorf("Attributes.RAMGB = %v, want 8", foundG7_2vCPU.Attributes.RAMGB)
	}
	if foundG7_2vCPU.Attributes.Family != "general_purpose" {
		t.Errorf("Attributes.Family = %q, want general_purpose", foundG7_2vCPU.Attributes.Family)
	}

	// Verify raw storage payload write
	todayStr := time.Now().UTC().Format("2006-01-02")
	prefix := "raw/alibaba/compute/" + todayStr + "/"

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

func TestAdapter_Fetch_WithHMACAuth(t *testing.T) {
	var queryReceived string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryReceived = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"InstanceTypes": [
				{
					"InstanceTypeId": "ecs.g7.large",
					"CpuCoreCount": 2,
					"MemorySize": 8.0,
					"InstanceTypeFamily": "ecs.g7",
					"TradePrice": 0.096,
					"Regions": ["us-east-1"]
				}
			]
		}`))
	}))
	defer server.Close()

	client := alibaba.NewClient(
		alibaba.WithURL(server.URL),
		alibaba.WithCredentials("test-access-key-id", "test-access-key-secret"),
		alibaba.WithHTTPClient(server.Client()),
	)
	memStorage := storage.NewMemoryRawStorage()
	adapter := alibaba.NewAdapter(client, memStorage)

	result, err := adapter.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}

	if !strings.Contains(queryReceived, "Signature=") {
		t.Errorf("expected Signature param in query, got %q", queryReceived)
	}
	if !strings.Contains(queryReceived, "AccessKeyId=test-access-key-id") {
		t.Errorf("expected AccessKeyId in query, got %q", queryReceived)
	}
	if len(result.Observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(result.Observations))
	}
}

func TestAdapter_Fetch_UnmappedProduct_Quarantined(t *testing.T) {
	jsonBody := `{
		"InstanceTypes": [
			{
				"ProductCode": "unmapped.alibaba.service",
				"InstanceTypeId": "ecs.unknown.large",
				"CpuCoreCount": 2,
				"MemorySize": 8.0,
				"TradePrice": 0.50,
				"Regions": ["us-east-1"]
			}
		]
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(jsonBody))
	}))
	defer ts.Close()

	client := alibaba.NewClient(alibaba.WithURL(ts.URL), alibaba.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := alibaba.NewAdapter(client, memStorage)

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

	client := alibaba.NewClient(alibaba.WithURL(ts.URL), alibaba.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := alibaba.NewAdapter(client, memStorage)

	_, err := adapter.Fetch(context.Background(), nil)
	if err == nil {
		t.Fatalf("Fetch() expected error for HTTP 500, got nil")
	}
}

func TestSupportedCategories(t *testing.T) {
	if !alibaba.IsCategorySupported("compute") {
		t.Errorf("expected compute to be supported")
	}
	if alibaba.IsCategorySupported("unknown-category") {
		t.Errorf("expected unknown-category to not be supported")
	}
	cats := alibaba.SupportedCategories()
	if len(cats) != 1 || cats[0] != "compute" {
		t.Errorf("SupportedCategories() = %v, want [compute]", cats)
	}
}

func TestAdapter_Fetch_GCSCompletesOnNormalizeFailure(t *testing.T) {
	invalidJSON := `{"InstanceTypes": [ { "broken_json`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(invalidJSON))
	}))
	defer ts.Close()

	client := alibaba.NewClient(alibaba.WithURL(ts.URL), alibaba.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := alibaba.NewAdapter(client, memStorage)

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
