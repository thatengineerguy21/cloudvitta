package azure

import (
	"os"
	"testing"
	"time"
)

func TestAzureNormalize_GoldenCorpus(t *testing.T) {
	fixedTime := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		goldenFile   string
		wantObsCount int
		expectedSKUs []string
	}{
		{
			name:         "Azure Compute Golden",
			goldenFile:   "../../../../testdata/golden/azure/compute.json",
			wantObsCount: 2, // Standard_D2s_v5 East US, Gov Virginia (Windows skipped)
			expectedSKUs: []string{"DZH318Z0BQ4C/0001", "DZH318Z0BQ4C/0002"},
		},
		{
			name:         "Azure Storage Golden",
			goldenFile:   "../../../../testdata/golden/azure/storage.json",
			wantObsCount: 2, // Hot LRS, Archive LRS
			expectedSKUs: []string{"DZH318Z0BNZ5/0001", "DZH318Z0BNZ5/0002"},
		},
		{
			name:         "Azure Network Golden",
			goldenFile:   "../../../../testdata/golden/azure/network.json",
			wantObsCount: 2, // East US egress, Intercontinental egress
			expectedSKUs: []string{"DZH318Z0BNZ7/0001", "DZH318Z0BNZ7/0002"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(tt.goldenFile)
			if err != nil {
				t.Fatalf("failed to open golden file %s: %v", tt.goldenFile, err)
			}
			defer f.Close()

			obs, _, err := Normalize(f, fixedTime)
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
			}

			for _, expectedSKU := range tt.expectedSKUs {
				if !skuMap[expectedSKU] {
					t.Errorf("missing expected SKU %s in normalized output", expectedSKU)
				}
			}
		})
	}
}
