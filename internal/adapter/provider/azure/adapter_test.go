package azure_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/azure"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
)

func TestAdapter_Fetch_HappyPath(t *testing.T) {
	fixtureBytes, err := os.ReadFile(filepath.Join("testdata", "azure-compute-eastus-20260815-sample.json"))
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

	result, err := adapter.Fetch(ctx, nil)
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

func TestAdapter_Fetch_Pagination(t *testing.T) {
	page1Bytes, err := os.ReadFile(filepath.Join("testdata", "azure-compute-eastus-20260815-page1.json"))
	if err != nil {
		t.Fatalf("failed to read page 1 fixture: %v", err)
	}
	page2Bytes, err := os.ReadFile(filepath.Join("testdata", "azure-compute-eastus-20260815-page2.json"))
	if err != nil {
		t.Fatalf("failed to read page 2 fixture: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if strings.Contains(r.URL.Path, "page2") {
			_, _ = w.Write(page2Bytes)
			return
		}

		// Replace placeholder with the actual page2 URL of the test server
		page2URL := "http://" + r.Host + "/page2"
		page1Str := strings.Replace(string(page1Bytes), "DYNAMIC_PAGE2_URL", page2URL, 1)
		_, _ = w.Write([]byte(page1Str))
	}))
	defer ts.Close()

	client := azure.NewClient(azure.WithURL(ts.URL), azure.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := azure.NewAdapter(client, memStorage)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := adapter.Fetch(ctx, nil)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}

	// We expect 3 total observations: 2 from page 1, 1 from page 2
	if len(result.Observations) != 3 {
		t.Fatalf("Fetch() returned %d observations, want 3", len(result.Observations))
	}

	// Verify that we wrote two raw GCS files
	files := memStorage.GetFiles()
	fileCount := 0
	for path := range files {
		if strings.Contains(path, "raw/azure/compute") {
			fileCount++
		}
	}
	if fileCount != 2 {
		t.Errorf("Fetch() wrote %d raw files, want 2", fileCount)
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

	client := azure.NewClient(azure.WithURL(ts.URL), azure.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := azure.NewAdapter(client, memStorage)

	_, err := adapter.Fetch(context.Background(), nil)
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

func TestAdapter_Fetch_GCSCompletesOnNormalizeFailure(t *testing.T) {
	invalidJSON := `{"Items": [ { "broken_json_here`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(invalidJSON))
	}))
	defer ts.Close()

	client := azure.NewClient(azure.WithURL(ts.URL), azure.WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := azure.NewAdapter(client, memStorage)

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

func TestAdapter_Fetch_Storage_HappyPath(t *testing.T) {
	fixtureBytes, err := os.ReadFile(filepath.Join("testdata", "azure-storage-eastus-sample.json"))
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

	result, err := adapter.Fetch(ctx, nil)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}
	if result.RawGCSPath == "" {
		t.Fatalf("Fetch() returned empty gcsPath")
	}

	// In the storage sample fixture:
	// - 1 Hot LRS -> included
	// - 1 Cool LRS -> included
	// - 1 Archive LRS -> included
	// - 1 Reservation -> excluded
	if len(result.Observations) != 3 {
		t.Fatalf("Fetch() returned %d observations, want 3", len(result.Observations))
	}

	var hotObs *domain.PriceObservation
	for i := range result.Observations {
		if strings.HasPrefix(result.Observations[i].SkuID, "SKU-AZ-BLOB-HOT-001") {
			hotObs = &result.Observations[i]
			break
		}
	}
	if hotObs == nil {
		t.Fatalf("missing SKU-AZ-BLOB-HOT-001 observation")
	}

	if hotObs.Provider != "azure" {
		t.Errorf("Provider = %q, want azure", hotObs.Provider)
	}
	if hotObs.ServiceCategory != "storage" {
		t.Errorf("ServiceCategory = %q, want storage", hotObs.ServiceCategory)
	}
	if hotObs.RegionGroup != "us-east" {
		t.Errorf("RegionGroup = %q, want us-east", hotObs.RegionGroup)
	}
	if hotObs.PriceAmount.String() != "0.0184" {
		t.Errorf("PriceAmount = %s, want 0.0184", hotObs.PriceAmount.String())
	}
	if hotObs.StorageAttributes.StorageClass != "standard" {
		t.Errorf("StorageAttributes.StorageClass = %q, want standard", hotObs.StorageAttributes.StorageClass)
	}
	if hotObs.StorageAttributes.SizeGB != 1 {
		t.Errorf("StorageAttributes.SizeGB = %v, want 1", hotObs.StorageAttributes.SizeGB)
	}
}

func TestAdapter_Fetch_Network_HappyPath(t *testing.T) {
	fixtureBytes, err := os.ReadFile(filepath.Join("testdata", "azure-bandwidth-sample.json"))
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

	result, err := adapter.Fetch(ctx, nil)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}
	if result.RawGCSPath == "" {
		t.Fatalf("Fetch() returned empty gcsPath")
	}

	// In the bandwidth sample fixture:
	// - SKU-AZ-BW-FLAT -> included (tierMinimumUnits: 0.0)
	// - SKU-AZ-BW-TIERED -> excluded (tierMinimumUnits: 10000.0)
	// - SKU-AZ-BW-RES -> excluded (type: Reservation)
	if len(result.Observations) != 1 {
		t.Fatalf("Fetch() returned %d observations, want 1 (flat rate only)", len(result.Observations))
	}

	obs := result.Observations[0]
	if !strings.HasPrefix(obs.SkuID, "SKU-AZ-BW-FLAT") {
		t.Errorf("SkuID = %q, want prefix SKU-AZ-BW-FLAT", obs.SkuID)
	}
	if obs.Provider != "azure" {
		t.Errorf("Provider = %q, want azure", obs.Provider)
	}
	if obs.ServiceCategory != "network" {
		t.Errorf("ServiceCategory = %q, want network", obs.ServiceCategory)
	}
	if obs.RegionGroup != "us-east" {
		t.Errorf("RegionGroup = %q, want us-east", obs.RegionGroup)
	}
	if obs.PriceAmount.String() != "0.087" {
		t.Errorf("PriceAmount = %s, want 0.087", obs.PriceAmount.String())
	}
	if obs.NetworkAttributes.EgressGB != 1 {
		t.Errorf("NetworkAttributes.EgressGB = %v, want 1", obs.NetworkAttributes.EgressGB)
	}
}

func TestBuildRegionalRetailPricesURL(t *testing.T) {
	u := azure.BuildRegionalRetailPricesURL("compute", "germanywestcentral")
	expected := "https://prices.azure.com/api/retail/prices?$filter=serviceName%20eq%20'Virtual%20Machines'%20and%20armRegionName%20eq%20'germanywestcentral'%20and%20priceType%20eq%20'Consumption'"
	if u != expected {
		t.Errorf("BuildRegionalRetailPricesURL() = %q, want %q", u, expected)
	}

	uNet := azure.BuildRegionalRetailPricesURL("network", "germanywestcentral")
	if uNet != azure.DefaultNetworkRetailPricesURL {
		t.Errorf("BuildRegionalRetailPricesURL(network) = %q, want %q", uNet, azure.DefaultNetworkRetailPricesURL)
	}
}
