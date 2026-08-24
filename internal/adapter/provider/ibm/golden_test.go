package ibm

import (
	"os"
	"testing"
	"time"
)

func TestIBMNormalize_GoldenCorpus(t *testing.T) {
	fixedTime := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		goldenFile   string
		category     string
		wantObsCount int
		expectedSKUs []string
	}{
		{
			name:         "IBM Cloud Compute Golden",
			goldenFile:   "../../../../testdata/golden/ibm/compute.json",
			category:     "compute",
			wantObsCount: 60, // 12 metrics * 5 regions
			expectedSKUs: []string{
				"SKU-IBM-VPC-BX2-2X8",
				"SKU-IBM-VPC-BX2-4X16",
				"SKU-IBM-VPC-BX2-8X32",
				"SKU-IBM-VPC-CX2-2X4",
				"SKU-IBM-VPC-CX2-4X8",
				"SKU-IBM-VPC-CX2-8X16",
				"SKU-IBM-VPC-MX2-2X16",
				"SKU-IBM-VPC-MX2-4X32",
				"SKU-IBM-VPC-MX2-8X64",
				"SKU-IBM-VPC-VX2-2X28",
				"SKU-IBM-VPC-VX2-4X56",
				"SKU-IBM-VPC-BA2-2X8",
			},
		},
		{
			name:         "IBM Cloud Storage Golden",
			goldenFile:   "../../../../testdata/golden/ibm/storage.json",
			category:     "storage",
			wantObsCount: 10, // 3 COS metrics * 3 regions (9) + 1 Volume metric * 1 region (1) = 10
			expectedSKUs: []string{
				"SKU-IBM-STANDARD-STORAGE",
				"SKU-IBM-VAULT-STORAGE",
				"SKU-IBM-COLD-VAULT-STORAGE",
				"SKU-IBM-GENERAL-PURPOSE-STORAGE",
			},
		},
		{
			name:         "IBM Cloud Network Golden",
			goldenFile:   "../../../../testdata/golden/ibm/network.json",
			category:     "network",
			wantObsCount: 2, // 1 metric * 2 regions = 2
			expectedSKUs: []string{
				"SKU-IBM-PUBLIC-EGRESS",
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
				if o.Provider != "ibm" {
					t.Errorf("expected Provider ibm, got %q", o.Provider)
				}
				if o.ServiceCategory != tt.category {
					t.Errorf("expected ServiceCategory %q, got %q", tt.category, o.ServiceCategory)
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
