package digitalocean

import (
	"os"
	"testing"
	"time"
)

func TestDigitalOceanNormalize_GoldenCorpus(t *testing.T) {
	fixedTime := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		goldenFile   string
		wantObsCount int
		expectedSKUs []string
	}{
		{
			name:         "DigitalOcean Compute Golden",
			goldenFile:   "../../../../testdata/golden/digitalocean/compute.json",
			wantObsCount: 58, // 10 regions for s-1vcpu-1gb + 12*4 regions for others
			expectedSKUs: []string{
				"SKU-DO-DROPLET-S-1VCPU-1GB",
				"SKU-DO-DROPLET-S-1VCPU-2GB",
				"SKU-DO-DROPLET-S-2VCPU-2GB",
				"SKU-DO-DROPLET-S-2VCPU-4GB",
				"SKU-DO-DROPLET-S-4VCPU-8GB",
				"SKU-DO-DROPLET-S-8VCPU-16GB",
				"SKU-DO-DROPLET-C-2",
				"SKU-DO-DROPLET-C-4",
				"SKU-DO-DROPLET-C-8",
				"SKU-DO-DROPLET-M-2VCPU-16GB",
				"SKU-DO-DROPLET-M-4VCPU-32GB",
				"SKU-DO-DROPLET-SO-2VCPU-16GB",
				"SKU-DO-DROPLET-G-2VCPU-8GB",
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
				if o.Provider != "digitalocean" {
					t.Errorf("expected Provider digitalocean, got %q", o.Provider)
				}
				if o.ServiceCategory != "compute" {
					t.Errorf("expected ServiceCategory compute, got %q", o.ServiceCategory)
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
