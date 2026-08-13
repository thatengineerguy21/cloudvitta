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

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/catalogmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
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

	obs, gcsPath, err := adapter.Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch() unexpected error: %v", err)
	}
	if gcsPath == "" {
		t.Fatalf("Fetch() returned empty gcsPath")
	}

	if len(obs) != 2 {
		t.Fatalf("Fetch() returned %d observations, want 2", len(obs))
	}

	// Verify c5.xlarge instance
	var c5Obs *domain.PriceObservation
	for i := range obs {
		if obs[i].SkuID == "SKU-C5-XLARGE" {
			c5Obs = &obs[i]
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

	_, _, err := adapter.Fetch(context.Background())
	if err == nil {
		t.Fatalf("Fetch() expected error for unmapped product, got nil")
	}
	if !errors.Is(err, catalogmap.ErrUnmappedProduct) {
		t.Fatalf("Fetch() error = %v, want errors.Is ErrUnmappedProduct", err)
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

	_, _, err := adapter.Fetch(context.Background())
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

	client := NewClient(WithURL(ts.URL), WithHTTPClient(ts.Client()))
	memStorage := storage.NewMemoryRawStorage()
	adapter := NewAdapter(client, memStorage)

	_, _, err := adapter.Fetch(context.Background())
	if err == nil {
		t.Fatalf("Fetch() expected error for HTTP 500, got nil")
	}
}
