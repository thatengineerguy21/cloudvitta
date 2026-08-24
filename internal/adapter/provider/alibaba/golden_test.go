package alibaba

import (
	"os"
	"testing"
	"time"
)

func TestAlibabaNormalize_GoldenCorpus(t *testing.T) {
	fixedTime := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		goldenFile   string
		category     string
		wantObsCount int
		expectedSKUs []string
	}{
		{
			name:       "Alibaba Cloud Compute Golden",
			goldenFile: "../../../../testdata/golden/alibaba/compute.json",
			category:   "compute",
			// 13 items * 4 regions (52) + 1 item * 2 regions (2) + 1 item * 1 region (1) = 55 observations
			wantObsCount: 55,
			expectedSKUs: []string{
				"SKU-ALI-ECS-G7-LARGE",
				"SKU-ALI-ECS-G7-XLARGE",
				"SKU-ALI-ECS-G7-2XLARGE",
				"SKU-ALI-ECS-G6-LARGE",
				"SKU-ALI-ECS-C7-LARGE",
				"SKU-ALI-ECS-C7-XLARGE",
				"SKU-ALI-ECS-C7-2XLARGE",
				"SKU-ALI-ECS-C6-LARGE",
				"SKU-ALI-ECS-R7-LARGE",
				"SKU-ALI-ECS-R7-XLARGE",
				"SKU-ALI-ECS-R7-2XLARGE",
				"SKU-ALI-ECS-T6-C1M1-LARGE",
				"SKU-ALI-ECS-T6-C1M2-LARGE",
				"SKU-ALI-ECS-I3-XLARGE",
			},
		},
		{
			name:         "Alibaba Cloud Storage Golden",
			goldenFile:   "../../../../testdata/golden/alibaba/storage.json",
			category:     "storage",
			wantObsCount: 6, // oss-standard * 3 regions (3) + oss-ia, oss-archive, cloud_essd * 1 region (3) = 6
			expectedSKUs: []string{
				"SKU-ALI-STORAGE-STANDARD",
				"SKU-ALI-STORAGE-IA",
				"SKU-ALI-STORAGE-ARCHIVE",
				"SKU-ALI-STORAGE-CLOUD_ESSD",
			},
		},
		{
			name:         "Alibaba Cloud Network Golden",
			goldenFile:   "../../../../testdata/golden/alibaba/network.json",
			category:     "network",
			wantObsCount: 3, // data-transfer-out * 2 regions (2) + eip * 1 region (1) = 3
			expectedSKUs: []string{
				"SKU-ALI-NETWORK-DATA-TRANSFER-OUT",
				"SKU-ALI-NETWORK-EIP",
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
			var foundCNY bool
			for _, o := range obs {
				skuMap[o.SkuID] = true
				if o.FetchedAt != fixedTime {
					t.Errorf("expected FetchedAt %v, got %v", fixedTime, o.FetchedAt)
				}
				if o.Provider != "alibaba" {
					t.Errorf("expected Provider alibaba, got %q", o.Provider)
				}
				if o.ServiceCategory != tt.category {
					t.Errorf("expected ServiceCategory %q, got %q", tt.category, o.ServiceCategory)
				}
				if o.PriceAmount.IsZero() {
					t.Errorf("expected non-zero PriceAmount for SKU %s", o.SkuID)
				}
				if o.PriceCurrency == "CNY" {
					foundCNY = true
					if o.Region != "cn-hangzhou" || o.RegionGroup != "cn-east" {
						t.Errorf("expected CNY observation to be in cn-hangzhou/cn-east, got %s/%s", o.Region, o.RegionGroup)
					}
				}
			}

			if tt.category == "compute" && !foundCNY {
				t.Errorf("expected at least one observation with CNY currency")
			}

			for _, expectedSKU := range tt.expectedSKUs {
				if !skuMap[expectedSKU] {
					t.Errorf("missing expected SKU %s in normalized output", expectedSKU)
				}
			}
		})
	}
}
