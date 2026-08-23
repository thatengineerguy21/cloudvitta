package gcp

import (
	"os"
	"testing"
	"time"
)

func TestGCPNormalize_GoldenCorpus(t *testing.T) {
	fixedTime := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		goldenFile   string
		wantObsCount int
		expectedSKUs []string
	}{
		{
			name:         "GCP Compute Golden",
			goldenFile:   "../../../../testdata/golden/gcp/compute.json",
			wantObsCount: 1, // N2 predefined (RAM skipped)
			expectedSKUs: []string{"SKU-GCP-N2-STD-4"},
		},
		{
			name:         "GCP Storage Golden",
			goldenFile:   "../../../../testdata/golden/gcp/storage.json",
			wantObsCount: 1, // Standard Storage (TagBinding skipped)
			expectedSKUs: []string{"SKU-GCP-STORAGE-STD"},
		},
		{
			name:         "GCP Network Golden",
			goldenFile:   "../../../../testdata/golden/gcp/network.json",
			wantObsCount: 1, // Premium Tier Internet Egress
			expectedSKUs: []string{"SKU-GCP-NETWORK-PREMIUM"},
		},
		{
			name:         "GCP Database Golden",
			goldenFile:   "../../../../testdata/golden/gcp/database.json",
			wantObsCount: 5, // Cloud SQL PG 4vCore (single & HA), Storage (single & HA), AlloyDB PG 8vCore
			expectedSKUs: []string{"SKU-GCP-CLOUDSQL-PG-4VCORE", "SKU-GCP-CLOUDSQL-PG-4VCORE-HA", "SKU-GCP-CLOUDSQL-STORAGE-SSD", "SKU-GCP-CLOUDSQL-STORAGE-SSD-HA", "SKU-GCP-ALLOYDB-PG-8VCORE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(tt.goldenFile)
			if err != nil {
				t.Fatalf("failed to open golden file %s: %v", tt.goldenFile, err)
			}
			defer func() { _ = f.Close() }()

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
