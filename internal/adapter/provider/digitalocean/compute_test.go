package digitalocean

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

type memoryQuarantineSink struct {
	items []quarantine.UnmappedItem
}

func (s *memoryQuarantineSink) Record(ctx context.Context, item quarantine.UnmappedItem) error {
	s.items = append(s.items, item)
	return nil
}

func (s *memoryQuarantineSink) Count() int {
	return len(s.items)
}

func (s *memoryQuarantineSink) Flush(ctx context.Context) error {
	return nil
}

func TestNormalize_DropletProfiles(t *testing.T) {
	jsonPayload := `{
		"sizes": [
			{
				"slug": "s-1vcpu-1gb",
				"memory": 1024,
				"vcpus": 1,
				"disk": 25,
				"transfer": 1.0,
				"price_monthly": 6.0,
				"price_hourly": 0.00893,
				"regions": ["nyc1", "sfo3"],
				"available": true,
				"description": "Basic"
			},
			{
				"slug": "c-4",
				"memory": 8192,
				"vcpus": 4,
				"disk": 50,
				"transfer": 5.0,
				"price_monthly": 84.0,
				"price_hourly": 0.125,
				"regions": ["fra1"],
				"available": true,
				"description": "CPU-Optimized"
			},
			{
				"slug": "m-4vcpu-32gb",
				"memory": 32768,
				"vcpus": 4,
				"disk": 100,
				"transfer": 5.0,
				"price_monthly": 126.0,
				"price_hourly": 0.1875,
				"regions": ["lon1"],
				"available": true,
				"description": "Memory-Optimized"
			},
			{
				"slug": "so-2vcpu-16gb",
				"memory": 16384,
				"vcpus": 2,
				"disk": 300,
				"transfer": 4.0,
				"price_monthly": 131.0,
				"price_hourly": 0.19494,
				"regions": ["sgp1"],
				"available": true,
				"description": "Storage-Optimized"
			},
			{
				"slug": "g-2vcpu-8gb",
				"memory": 8192,
				"vcpus": 2,
				"disk": 25,
				"transfer": 4.0,
				"price_monthly": 63.0,
				"price_hourly": 0.09375,
				"regions": ["nyc3"],
				"available": true,
				"description": "General Purpose Dedicated"
			}
		]
	}`

	now := time.Now().UTC()
	res, err := Normalize(strings.NewReader(jsonPayload), now)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	obs := res.Observations

	// 2 (s-1vcpu-1gb) + 1 (c-4) + 1 (m-4vcpu-32gb) + 1 (so-2vcpu-16gb) + 1 (g-2vcpu-8gb) = 6 observations
	if len(obs) != 6 {
		t.Fatalf("expected 6 observations, got %d", len(obs))
	}

	var foundBasicNYC1, foundBasicSFO3, foundC4, foundM4, foundSO2, foundG2 bool
	for _, o := range obs {
		if o.Provider != "digitalocean" {
			t.Errorf("expected Provider digitalocean, got %q", o.Provider)
		}
		if o.ServiceCategory != "compute" {
			t.Errorf("expected ServiceCategory compute, got %q", o.ServiceCategory)
		}
		if o.Unit != "Hrs" {
			t.Errorf("expected Unit Hrs, got %q", o.Unit)
		}
		if o.PriceCurrency != "USD" {
			t.Errorf("expected PriceCurrency USD, got %q", o.PriceCurrency)
		}
		if o.PricingModel != "OnDemand" {
			t.Errorf("expected PricingModel OnDemand, got %q", o.PricingModel)
		}

		switch o.SkuID {
		case "SKU-DO-DROPLET-S-1VCPU-1GB":
			switch o.Region {
			case "nyc1":
				foundBasicNYC1 = true
				if o.RegionGroup != "us-east" {
					t.Errorf("expected RegionGroup us-east, got %s", o.RegionGroup)
				}
				if o.Attributes.VCPU != 1 || o.Attributes.RAMGB != 1 || o.Attributes.Family != "general_purpose" {
					t.Errorf("Basic attrs mismatch: %+v", o.Attributes)
				}
				expectedPrice := decimal.RequireFromString("0.00893")
				if !o.PriceAmount.Equal(expectedPrice) {
					t.Errorf("Basic price = %s, want %s", o.PriceAmount, expectedPrice)
				}
			case "sfo3":
				foundBasicSFO3 = true
				if o.RegionGroup != "us-west" {
					t.Errorf("expected RegionGroup us-west, got %s", o.RegionGroup)
				}
			}
		case "SKU-DO-DROPLET-C-4":
			foundC4 = true
			if o.Region != "fra1" || o.RegionGroup != "eu-central" {
				t.Errorf("C4 region mismatch: %s / %s", o.Region, o.RegionGroup)
			}
			if o.Attributes.VCPU != 4 || o.Attributes.RAMGB != 8 || o.Attributes.Family != "compute_optimized" {
				t.Errorf("C4 attrs mismatch: %+v", o.Attributes)
			}
			expectedPrice := decimal.RequireFromString("0.125")
			if !o.PriceAmount.Equal(expectedPrice) {
				t.Errorf("C4 price = %s, want %s", o.PriceAmount, expectedPrice)
			}
		case "SKU-DO-DROPLET-M-4VCPU-32GB":
			foundM4 = true
			if o.Region != "lon1" || o.RegionGroup != "uk-south" {
				t.Errorf("M4 region mismatch: %s / %s", o.Region, o.RegionGroup)
			}
			if o.Attributes.VCPU != 4 || o.Attributes.RAMGB != 32 || o.Attributes.Family != "memory_optimized" {
				t.Errorf("M4 attrs mismatch: %+v", o.Attributes)
			}
			expectedPrice := decimal.RequireFromString("0.1875")
			if !o.PriceAmount.Equal(expectedPrice) {
				t.Errorf("M4 price = %s, want %s", o.PriceAmount, expectedPrice)
			}
		case "SKU-DO-DROPLET-SO-2VCPU-16GB":
			foundSO2 = true
			if o.Region != "sgp1" || o.RegionGroup != "ap-southeast" {
				t.Errorf("SO2 region mismatch: %s / %s", o.Region, o.RegionGroup)
			}
			if o.Attributes.VCPU != 2 || o.Attributes.RAMGB != 16 || o.Attributes.Family != "storage_optimized" {
				t.Errorf("SO2 attrs mismatch: %+v", o.Attributes)
			}
			expectedPrice := decimal.RequireFromString("0.19494")
			if !o.PriceAmount.Equal(expectedPrice) {
				t.Errorf("SO2 price = %s, want %s", o.PriceAmount, expectedPrice)
			}
		case "SKU-DO-DROPLET-G-2VCPU-8GB":
			foundG2 = true
			if o.Region != "nyc3" || o.RegionGroup != "us-east" {
				t.Errorf("G2 region mismatch: %s / %s", o.Region, o.RegionGroup)
			}
			if o.Attributes.VCPU != 2 || o.Attributes.RAMGB != 8 || o.Attributes.Family != "general_purpose" {
				t.Errorf("G2 attrs mismatch: %+v", o.Attributes)
			}
			expectedPrice := decimal.RequireFromString("0.09375")
			if !o.PriceAmount.Equal(expectedPrice) {
				t.Errorf("G2 price = %s, want %s", o.PriceAmount, expectedPrice)
			}
		}
	}

	if !foundBasicNYC1 || !foundBasicSFO3 || !foundC4 || !foundM4 || !foundSO2 || !foundG2 {
		t.Errorf("missing expected profiles: BasicNYC1=%v, BasicSFO3=%v, C4=%v, M4=%v, SO2=%v, G2=%v",
			foundBasicNYC1, foundBasicSFO3, foundC4, foundM4, foundSO2, foundG2)
	}
}

func TestNormalize_QuarantineUnmappedItems(t *testing.T) {
	jsonPayload := `{
		"sizes": [
			{
				"slug": "s-unknown-region",
				"memory": 1024,
				"vcpus": 1,
				"price_hourly": 0.01,
				"regions": ["unmapped-mars-region"]
			},
			{
				"slug": "s-zero-vcpu",
				"memory": 1024,
				"vcpus": 0,
				"price_hourly": 0.01,
				"regions": ["nyc1"]
			},
			{
				"slug": "s-zero-memory",
				"memory": 0,
				"vcpus": 1,
				"price_hourly": 0.01,
				"regions": ["nyc1"]
			},
			{
				"slug": "s-missing-regions",
				"memory": 1024,
				"vcpus": 1,
				"price_hourly": 0.01,
				"regions": []
			}
		]
	}`

	sink := &memoryQuarantineSink{}
	now := time.Now().UTC()
	res, err := Normalize(strings.NewReader(jsonPayload), now, sink)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	obs := res.Observations

	if len(obs) != 0 {
		t.Errorf("expected 0 valid observations, got %d", len(obs))
	}

	if sink.Count() != 4 {
		t.Fatalf("expected 4 quarantined items (unmapped region + zero vcpu + zero memory + missing regions), got %d", sink.Count())
	}
}

func TestNormalize_MonthlyPriceFallback(t *testing.T) {
	jsonPayload := `{
		"sizes": [
			{
				"slug": "s-monthly-only",
				"memory": 1024,
				"vcpus": 1,
				"price_monthly": 73.0,
				"price_hourly": 0.0,
				"regions": ["nyc1"]
			}
		]
	}`

	res, err := Normalize(strings.NewReader(jsonPayload), time.Now().UTC())
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	obs := res.Observations

	if len(obs) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(obs))
	}

	expectedPrice := decimal.RequireFromString("0.1") // 73 / 730 = 0.1
	if !obs[0].PriceAmount.Equal(expectedPrice) {
		t.Errorf("PriceAmount = %s, want %s", obs[0].PriceAmount, expectedPrice)
	}
}

func TestNormalize_InvalidJSON(t *testing.T) {
	_, err := Normalize(bytes.NewReader([]byte("{invalid json")), time.Now().UTC())
	if err == nil {
		t.Fatal("expected error on invalid JSON, got nil")
	}
}

func TestNormalize_EmptyResource_NoPanic(t *testing.T) {
	jsonPayload := `{"sizes": []}`
	res, err := Normalize(strings.NewReader(jsonPayload), time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error on empty resources: %v", err)
	}
	obs := res.Observations
	if len(obs) != 0 {
		t.Errorf("expected 0 observations, got %d", len(obs))
	}
}
