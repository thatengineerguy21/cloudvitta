package azure_test

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

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/azure"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/catalogmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
)

func TestAdapter_Fetch_HappyPath(t *testing.T) {
	fixtureBytes, err := os.ReadFile(filepath.Join("testdata", "azure-compute-eastus-sample.json"))
	if err != nil {
		t.Fatalf("failed to read test fixture: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixtureBytes)
	}))
	defer ts.Close()

	client := azure.NewClient(azure.WithURL(ts.URL), azure.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := azure.NewAdapter(client, memStorage)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := adapter.Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}
	if result.RawGCSPath == "" {
		t.Fatalf("Fetch() returned empty gcsPath")
	}

	// In the sample fixture:
	// - 1 Standard_D4s_v3 (Consumption, Linux) -> included
	// - 1 Standard_D2s_v3 (Consumption, Linux) -> included
	// - 1 Standard_B2s (Consumption, Linux) -> included
	// - 1 Windows -> excluded
	// - 1 Spot -> excluded
	// - 1 Reservation -> excluded
	// Total expected: 3 observations
	if len(result.Observations) != 3 {
		t.Fatalf("Fetch() returned %d observations, want 3", len(result.Observations))
	}

	// Verify Standard_D4s_v3 instance
	var d4sObs *domain.PriceObservation
	for i := range result.Observations {
		if result.Observations[i].DisplayName == "Standard_D4s_v3" {
			d4sObs = &result.Observations[i]
			break
		}
	}

	if d4sObs == nil {
		t.Fatalf("Fetch() missing observation for Standard_D4s_v3")
	}

	if d4sObs.Provider != "azure" {
		t.Errorf("Provider = %q, want %q", d4sObs.Provider, "azure")
	}
	if d4sObs.ServiceCategory != "compute" {
		t.Errorf("ServiceCategory = %q, want %q", d4sObs.ServiceCategory, "compute")
	}
	if d4sObs.RegionGroup != "us-east" {
		t.Errorf("RegionGroup = %q, want %q", d4sObs.RegionGroup, "us-east")
	}
	if d4sObs.PriceAmount.String() != "0.192" {
		t.Errorf("PriceAmount = %s, want 0.192", d4sObs.PriceAmount.String())
	}
	if d4sObs.Attributes.VCPU != 4 {
		t.Errorf("Attributes.VCPU = %v, want 4", d4sObs.Attributes.VCPU)
	}
	if d4sObs.Attributes.RAMGB != 16 {
		t.Errorf("Attributes.RAMGB = %v, want 16", d4sObs.Attributes.RAMGB)
	}
	if d4sObs.Attributes.Family != "d" {
		t.Errorf("Attributes.Family = %q, want %q", d4sObs.Attributes.Family, "d")
	}

	// Verify raw storage payload write
	todayStr := time.Now().UTC().Format("2006-01-02")
	prefix := "raw/azure/compute/" + todayStr + "/"

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

func TestAdapter_Fetch_UnmappedProduct_FailsLoudly(t *testing.T) {
	jsonBody := `{
		"Items": [
			{
				"serviceName": "UnmappedAzureService",
				"armRegionName": "eastus",
				"type": "Consumption",
				"armSkuName": "Standard_D4s_v3",
				"unitPrice": 0.10,
				"currencyCode": "USD"
			}
		]
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(jsonBody))
	}))
	defer ts.Close()

	client := azure.NewClient(azure.WithURL(ts.URL), azure.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := azure.NewAdapter(client, memStorage)

	_, err := adapter.Fetch(context.Background())
	if err == nil {
		t.Fatalf("Fetch() expected error for unmapped product, got nil")
	}
	if !errors.Is(err, catalogmap.ErrUnmappedProduct) {
		t.Fatalf("Fetch() error = %v, want errors.Is ErrUnmappedProduct", err)
	}
}

func TestAdapter_Fetch_UnmappedRegion_FailsLoudly(t *testing.T) {
	jsonBody := `{
		"Items": [
			{
				"serviceName": "Virtual Machines",
				"armRegionName": "unmapped-azure-region-99",
				"type": "Consumption",
				"armSkuName": "Standard_D4s_v3",
				"unitPrice": 0.10,
				"currencyCode": "USD"
			}
		]
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(jsonBody))
	}))
	defer ts.Close()

	client := azure.NewClient(azure.WithURL(ts.URL), azure.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := azure.NewAdapter(client, memStorage)

	_, err := adapter.Fetch(context.Background())
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

	client := azure.NewClient(azure.WithURL(ts.URL), azure.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := azure.NewAdapter(client, memStorage)

	_, err := adapter.Fetch(context.Background())
	if err == nil {
		t.Fatalf("Fetch() expected error for HTTP 500, got nil")
	}
}

func TestSupportedCategories(t *testing.T) {
	if !azure.IsCategorySupported("compute") {
		t.Errorf("expected compute to be supported")
	}
	if azure.IsCategorySupported("unknown-category") {
		t.Errorf("expected unknown-category to not be supported")
	}
}
