package aws

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

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
)

func TestAdapter_Fetch_HappyPath(t *testing.T) {
	fixtureBytes, err := os.ReadFile(filepath.Join("testdata", "ec2-us-east-1-sample.json"))
	if err != nil {
		t.Fatalf("failed to read test fixture: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixtureBytes)
	}))
	defer ts.Close()

	client := NewClient(WithURL(ts.URL), WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := NewAdapter(client, memStorage)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := adapter.Fetch(ctx, nil)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}
	if result.RawGCSPath == "" {
		t.Fatalf("Fetch() returned empty gcsPath")
	}

	if len(result.Observations) != 2 {
		t.Fatalf("Fetch() returned %d observations, want 2", len(result.Observations))
	}

	// Verify c5.xlarge instance
	var c5Obs *domain.PriceObservation
	for i := range result.Observations {
		if result.Observations[i].SkuID == "SKU-C5-XLARGE" {
			c5Obs = &result.Observations[i]
			break
		}
	}

	if c5Obs == nil {
		t.Fatalf("Fetch() missing observation for SKU-C5-XLARGE")
	}

	if c5Obs.Provider != "aws" {
		t.Errorf("Provider = %q, want %q", c5Obs.Provider, "aws")
	}
	if c5Obs.ServiceCategory != "compute" {
		t.Errorf("ServiceCategory = %q, want %q", c5Obs.ServiceCategory, "compute")
	}
	if c5Obs.RegionGroup != "us-east" {
		t.Errorf("RegionGroup = %q, want %q", c5Obs.RegionGroup, "us-east")
	}
	if c5Obs.PriceAmount.String() != "0.17" {
		t.Errorf("PriceAmount = %s, want 0.17", c5Obs.PriceAmount.String())
	}
	if c5Obs.Attributes.VCPU != 4 {
		t.Errorf("Attributes.VCPU = %v, want 4", c5Obs.Attributes.VCPU)
	}
	if c5Obs.Attributes.RAMGB != 8 {
		t.Errorf("Attributes.RAMGB = %v, want 8", c5Obs.Attributes.RAMGB)
	}
	if c5Obs.Attributes.Family != "c5" {
		t.Errorf("Attributes.Family = %q, want %q", c5Obs.Attributes.Family, "c5")
	}

	// Verify raw storage payload write
	todayStr := time.Now().UTC().Format("2006-01-02")
	prefix := "raw/aws/compute/" + todayStr + "/"

	// Find the stored path in memory storage
	var storedPath string
	for path := range memStorage.GetFiles() {
		if strings.HasPrefix(path, prefix) {
			storedPath = path
			break
		}
	}

	if storedPath == "" {
		t.Fatalf("Fetch() did not write raw payload to GCS path under prefix %s", prefix)
	}

	storedBytes, ok := memStorage.Get(storedPath)
	if !ok || len(storedBytes) == 0 {
		t.Fatalf("Fetch() stored payload at %s is empty", storedPath)
	}
}

func TestAdapter_Fetch_UnmappedProduct_FailsLoudly(t *testing.T) {
	jsonBody := `{
		"offerCode": "UnknownProductCode",
		"products": {
			"SKU-1": {
				"sku": "SKU-1",
				"attributes": {
					"servicecode": "UnknownProductCode",
					"location": "US East (N. Virginia)",
					"instanceType": "c5.xlarge",
					"operatingSystem": "Linux",
					"tenancy": "Shared"
				}
			}
		},
		"terms": {
			"OnDemand": {
				"SKU-1": {
					"TERM-1": {
						"priceDimensions": {
							"DIM-1": {
								"unit": "Hrs",
								"pricePerUnit": {"USD": "0.10"}
							}
						}
					}
				}
			}
		}
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(jsonBody))
	}))
	defer ts.Close()

	client := NewClient(WithURL(ts.URL), WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := NewAdapter(client, memStorage)

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
		"offerCode": "AmazonEC2",
		"products": {
			"SKU-1": {
				"sku": "SKU-1",
				"attributes": {
					"servicecode": "AmazonEC2",
					"location": "us-unknown-region-99",
					"instanceType": "c5.xlarge",
					"operatingSystem": "Linux",
					"tenancy": "Shared"
				}
			}
		},
		"terms": {
			"OnDemand": {
				"SKU-1": {
					"TERM-1": {
						"priceDimensions": {
							"DIM-1": {
								"unit": "Hrs",
								"pricePerUnit": {"USD": "0.10"}
							}
						}
					}
				}
			}
		}
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(jsonBody))
	}))
	defer ts.Close()

	client := NewClient(WithURL(ts.URL), WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := NewAdapter(client, memStorage)

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

	client := NewClient(WithURL(ts.URL), WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := NewAdapter(client, memStorage)

	_, err := adapter.Fetch(context.Background(), nil)
	if err == nil {
		t.Fatalf("Fetch() expected error for HTTP 500, got nil")
	}
}

func TestNormalize_TermsBeforeProducts_ReturnsPermanentFailure(t *testing.T) {
	// Payload where terms appears before products
	jsonBody := `{
		"offerCode": "AmazonEC2",
		"terms": {
			"OnDemand": {
				"SKU-1": {
					"TERM-1": {
						"priceDimensions": {
							"DIM-1": {
								"unit": "Hrs",
								"pricePerUnit": {"USD": "0.10"}
							}
						}
					}
				}
			}
		},
		"products": {
			"SKU-1": {
				"sku": "SKU-1",
				"attributes": {
					"servicecode": "AmazonEC2",
					"location": "US East (N. Virginia)",
					"instanceType": "c5.xlarge",
					"operatingSystem": "Linux",
					"tenancy": "Shared"
				}
			}
		}
	}`

	_, err := Normalize(strings.NewReader(jsonBody), time.Now().UTC())
	if err == nil {
		t.Fatalf("Normalize() expected error when terms precedes products, got nil")
	}
	if !errors.Is(err, provider.ErrPermanentFailure) {
		t.Errorf("Normalize() error = %v, want errors.Is ErrPermanentFailure", err)
	}
}

func TestNormalize_SkipsMassiveReservedBlocksWithoutSpike(t *testing.T) {
	// Build a JSON payload with a massive Reserved block and many skipped keys
	var b strings.Builder
	b.WriteString(`{
		"offerCode": "AmazonEC2",
		"products": {
			"SKU-C5": {
				"sku": "SKU-C5",
				"attributes": {
					"servicecode": "AmazonEC2",
					"location": "US East (N. Virginia)",
					"instanceType": "c5.xlarge",
					"operatingSystem": "Linux",
					"tenancy": "Shared",
					"vcpu": "4",
					"memory": "8 GiB"
				}
			}
		},
		"terms": {
			"Reserved": {`)

	// Generate 1000 dummy reserved term entries to simulate large skipped blocks
	for i := 0; i < 1000; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`"SKU-RES-`)
		b.WriteString(string(rune('A' + (i % 26))))
		b.WriteString(`": {"term": {"dimensions": {"dim": {"price": "100.00"}}}}`)
	}

	b.WriteString(`},
			"OnDemand": {
				"SKU-C5": {
					"TERM-1": {
						"priceDimensions": {
							"DIM-1": {
								"unit": "Hrs",
								"pricePerUnit": {"USD": "0.17"}
							}
						}
					}
				}
			}
		},
		"hugeExtraMetadata": {
			"nested": [1, 2, 3, {"deep": "value"}]
		}
	}`)

	payloadStr := b.String()

	res, err := Normalize(strings.NewReader(payloadStr), time.Now().UTC())
	if err != nil {
		t.Fatalf("Normalize() failed: %v", err)
	}
	obs := res.Observations
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(obs))
	}
	if obs[0].PriceAmount.String() != "0.17" {
		t.Errorf("PriceAmount = %s, want 0.17", obs[0].PriceAmount.String())
	}
}

func TestAdapter_Fetch_GCSCompletesOnNormalizeFailure(t *testing.T) {
	invalidJSON := `{"offerCode": "AmazonEC2", "products": { "broken`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(invalidJSON))
	}))
	defer ts.Close()

	client := NewClient(WithURL(ts.URL), WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := NewAdapter(client, memStorage)

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
	fixtureBytes, err := os.ReadFile(filepath.Join("testdata", "aws-storage-us-east-1-sample.json"))
	if err != nil {
		t.Fatalf("failed to read test fixture: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixtureBytes)
	}))
	defer ts.Close()

	client := NewClient(WithURL(ts.URL), WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := NewAdapter(client, memStorage)

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
	// - 1 S3 Standard -> included
	// - 1 S3 Standard-IA -> included
	// - 1 API Requests -> excluded
	if len(result.Observations) != 2 {
		t.Fatalf("Fetch() returned %d observations, want 2", len(result.Observations))
	}

	var stdObs *domain.PriceObservation
	for i := range result.Observations {
		if result.Observations[i].SkuID == "SKU-S3-STD-001" {
			stdObs = &result.Observations[i]
			break
		}
	}
	if stdObs == nil {
		t.Fatalf("missing SKU-S3-STD-001 observation")
	}

	if stdObs.Provider != "aws" {
		t.Errorf("Provider = %q, want aws", stdObs.Provider)
	}
	if stdObs.ServiceCategory != "storage" {
		t.Errorf("ServiceCategory = %q, want storage", stdObs.ServiceCategory)
	}
	if stdObs.RegionGroup != "us-east" {
		t.Errorf("RegionGroup = %q, want us-east", stdObs.RegionGroup)
	}
	if stdObs.PriceAmount.String() != "0.023" {
		t.Errorf("PriceAmount = %s, want 0.023", stdObs.PriceAmount.String())
	}
	if stdObs.StorageAttributes.StorageClass != "standard" {
		t.Errorf("StorageAttributes.StorageClass = %q, want standard", stdObs.StorageAttributes.StorageClass)
	}
	if stdObs.StorageAttributes.SizeGB != 1 {
		t.Errorf("StorageAttributes.SizeGB = %v, want 1", stdObs.StorageAttributes.SizeGB)
	}
}

func TestAdapter_Fetch_Network_HappyPath(t *testing.T) {
	fixtureBytes, err := os.ReadFile(filepath.Join("testdata", "datatransfer_us_east_1_sample.json"))
	if err != nil {
		t.Fatalf("failed to read test fixture: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixtureBytes)
	}))
	defer ts.Close()

	client := NewClient(WithURL(ts.URL), WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := NewAdapter(client, memStorage)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := adapter.Fetch(ctx, nil)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}
	if result.RawGCSPath == "" {
		t.Fatalf("Fetch() returned empty gcsPath")
	}

	// In the data transfer sample fixture:
	// - SKU-AWS-DT-FLAT -> included (flat rate, 1 dimension)
	// - SKU-AWS-DT-TIERED -> excluded (tiered pricing, 2 dimensions)
	// - SKU-AWS-NONMATCH -> excluded
	if len(result.Observations) != 1 {
		t.Fatalf("Fetch() returned %d observations, want 1 (flat rate only)", len(result.Observations))
	}

	obs := result.Observations[0]
	if obs.SkuID != "SKU-AWS-DT-FLAT" {
		t.Errorf("SkuID = %q, want SKU-AWS-DT-FLAT", obs.SkuID)
	}
	if obs.Provider != "aws" {
		t.Errorf("Provider = %q, want aws", obs.Provider)
	}
	if obs.ServiceCategory != "network" {
		t.Errorf("ServiceCategory = %q, want network", obs.ServiceCategory)
	}
	if obs.RegionGroup != "us-east" {
		t.Errorf("RegionGroup = %q, want us-east", obs.RegionGroup)
	}
	if obs.PriceAmount.String() != "0.09" {
		t.Errorf("PriceAmount = %s, want 0.09", obs.PriceAmount.String())
	}
	if obs.NetworkAttributes.EgressGB != 1 {
		t.Errorf("NetworkAttributes.EgressGB = %v, want 1", obs.NetworkAttributes.EgressGB)
	}
}

func TestAdapter_Fetch_CategoryFiltering_ExcludesOtherCategories(t *testing.T) {
	fixtureBytes, err := os.ReadFile(filepath.Join("testdata", "datatransfer_us_east_1_sample.json"))
	if err != nil {
		t.Fatalf("failed to read test fixture: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixtureBytes)
	}))
	defer ts.Close()

	client := NewClient(WithURL(ts.URL), WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := NewAdapter(client, memStorage, WithCategory("compute"))

	result, err := adapter.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}

	// Should filter out all network observations because adapter is configured for "compute"
	if len(result.Observations) != 0 {
		t.Errorf("Fetch() returned %d observations, want 0 (network observations filtered from compute adapter)", len(result.Observations))
	}
}

func TestBuildRegionalURL(t *testing.T) {
	tests := []struct {
		offerCode string
		region    string
		expected  string
	}{
		{
			offerCode: "AmazonEC2",
			region:    "us-west-2",
			expected:  "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/current/us-west-2/index.json",
		},
		{
			offerCode: "AmazonS3",
			region:    "eu-central-1",
			expected:  "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonS3/current/eu-central-1/index.json",
		},
		{
			offerCode: "AmazonRDS",
			region:    "ap-south-1",
			expected:  "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonRDS/current/ap-south-1/index.json",
		},
		{
			offerCode: "AmazonDynamoDB",
			region:    "ap-northeast-1",
			expected:  "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonDynamoDB/current/ap-northeast-1/index.json",
		},
		{
			offerCode: "AmazonEKS",
			region:    "eu-west-2",
			expected:  "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEKS/current/eu-west-2/index.json",
		},
		{
			offerCode: "AWSLambda",
			region:    "ap-southeast-2",
			expected:  "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AWSLambda/current/ap-southeast-2/index.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.offerCode+"_"+tt.region, func(t *testing.T) {
			url := BuildRegionalURL(tt.offerCode, tt.region)
			if url != tt.expected {
				t.Errorf("BuildRegionalURL(%q, %q) = %q, want %q", tt.offerCode, tt.region, url, tt.expected)
			}
		})
	}
}

func TestOfferCodeForCategory(t *testing.T) {
	tests := []struct {
		category string
		expected string
	}{
		{"compute", "AmazonEC2"},
		{"storage", "AmazonS3"},
		{"network", "AWSDataTransfer"},
		{"database_rdbms", "AmazonRDS"},
		{"database_nosql", "AmazonDynamoDB"},
		{"kubernetes", "AmazonEKS"},
		{"serverless", "AWSLambda"},
		{"unknown", "AmazonEC2"},
	}

	for _, tt := range tests {
		got := OfferCodeForCategory(tt.category)
		if got != tt.expected {
			t.Errorf("OfferCodeForCategory(%q) = %q, want %q", tt.category, got, tt.expected)
		}
	}
}
