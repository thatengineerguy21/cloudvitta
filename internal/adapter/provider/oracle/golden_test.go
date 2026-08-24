package oracle

import (
	"os"
	"testing"
	"time"
)

func TestOracleNormalize_GoldenCorpus(t *testing.T) {
	fixedTime := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		goldenFile   string
		wantObsCount int
		expectedSKUs []string
	}{
		{
			name:         "Oracle Compute Golden",
			goldenFile:   "../../../../testdata/golden/oracle/compute.json",
			wantObsCount: 12,
			expectedSKUs: []string{
				"SKU-OCI-VM-STANDARD-E4-FLEX-2VCPU-8GB",
				"SKU-OCI-VM-STANDARD-E4-FLEX-4VCPU-16GB",
				"SKU-OCI-VM-STANDARD-E4-FLEX-8VCPU-32GB",
				"SKU-OCI-VM-STANDARD-E4-FLEX-16VCPU-64GB",
				"SKU-OCI-VM-STANDARD-E4-FLEX-32VCPU-128GB",
				"SKU-OCI-VM-STANDARD-A1-FLEX-1VCPU-6GB",
				"SKU-OCI-VM-STANDARD-A1-FLEX-2VCPU-12GB",
				"SKU-OCI-VM-STANDARD-A1-FLEX-4VCPU-24GB",
				"SKU-OCI-VM-STANDARD-A1-FLEX-8VCPU-48GB",
				"SKU-OCI-VM-STANDARD-A1-FLEX-16VCPU-64GB",
				"SKU-OCI-B88317-STANDARD2-1",
				"SKU-OCI-B88318-STANDARD2-2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(tt.goldenFile)
			if err != nil {
				t.Fatalf("failed to open golden file %s: %v", tt.goldenFile, err)
			}
			defer func() { _ = f.Close() }()

			obs, err := Normalize(f, fixedTime)
			if err != nil {
				t.Fatalf("Normalize() unexpected error: %v", err)
			}

			if len(obs) != tt.wantObsCount {
				t.Fatalf("got %d observations, want %d", len(obs), tt.wantObsCount)
			}

			skuMap := make(map[string]bool)
			for _, o := range obs {
				skuMap[o.SkuID] = true
				if o.FetchedAt != fixedTime {
					t.Errorf("expected FetchedAt %v, got %v", fixedTime, o.FetchedAt)
				}
				if o.Provider != "oracle" {
					t.Errorf("expected Provider oracle, got %q", o.Provider)
				}
				if o.ServiceCategory != "compute" {
					t.Errorf("expected ServiceCategory compute, got %q", o.ServiceCategory)
				}
				if o.RegionGroup != "us-east" {
					t.Errorf("expected RegionGroup us-east, got %q", o.RegionGroup)
				}
				if o.PriceAmount.IsZero() {
					t.Errorf("expected non-zero PriceAmount for SKU %s", o.SkuID)
				}
			}

			for _, expectedSKU := range tt.expectedSKUs {
				if !skuMap[expectedSKU] {
					t.Errorf("missing expected SKU %s in normalized output", expectedSKU)
				}
			}
		})
	}
}
