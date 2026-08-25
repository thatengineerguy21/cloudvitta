package aws

import (
	"os"
	"testing"
	"time"
)

func TestAWSNormalize_GoldenCorpus(t *testing.T) {
	fixedTime := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		goldenFile    string
		wantObsCount  int
		expectedSKUs  []string
		expectedPrice string
	}{
		{
			name:          "AWS Compute Golden",
			goldenFile:    "../../../../testdata/golden/aws/compute.json",
			wantObsCount:  3, // C5.xlarge, Local Zone NYC, Wavelength Boston (Windows ignored)
			expectedSKUs:  []string{"SKU-C5-XLARGE", "SKU-LOCAL-ZONE-NYC", "SKU-WAVELENGTH-BOSTON"},
			expectedPrice: "0.1700000000",
		},
		{
			name:          "AWS Storage Golden",
			goldenFile:    "../../../../testdata/golden/aws/storage.json",
			wantObsCount:  2, // S3 Standard, S3 Glacier
			expectedSKUs:  []string{"SKU-S3-STANDARD", "SKU-S3-GLACIER"},
			expectedPrice: "0.0230000000",
		},
		{
			name:          "AWS Network Golden",
			goldenFile:    "../../../../testdata/golden/aws/network.json",
			wantObsCount:  2, // US East, Vodafone Dortmund
			expectedSKUs:  []string{"SKU-TRANSFER-US-EAST", "SKU-TRANSFER-VODAFONE-DORTMUND"},
			expectedPrice: "0.0900000000",
		},
		{
			name:          "AWS Database Golden",
			goldenFile:    "../../../../testdata/golden/aws/database.json",
			wantObsCount:  6, // RDS PG M6g xlarge (single & HA), GP3 (single & HA), Aurora PG R6g 2xlarge, Aurora Storage
			expectedSKUs:  []string{"SKU-RDS-PG-M6G-XLARGE", "SKU-RDS-PG-M6G-XLARGE-HA", "SKU-RDS-STORAGE-GP3", "SKU-RDS-STORAGE-GP3-HA", "SKU-AURORA-PG-R6G-2XLARGE", "SKU-AURORA-STORAGE"},
			expectedPrice: "0.2600000000",
		},
		{
			name:          "AWS NoSQL Database Golden",
			goldenFile:    "../../../../testdata/golden/aws/database_nosql.json",
			wantObsCount:  6, // Read provisioned, Write provisioned, Read on-demand, Write on-demand, Standard storage, IA storage
			expectedSKUs:  []string{"SKU-DDB-READ-PROVISIONED", "SKU-DDB-WRITE-PROVISIONED", "SKU-DDB-READ-ONDEMAND", "SKU-DDB-WRITE-ONDEMAND", "SKU-DDB-STORAGE-STANDARD", "SKU-DDB-STORAGE-IA"},
			expectedPrice: "0.0001300000",
		},
		{
			name:          "AWS Kubernetes Golden",
			goldenFile:    "../../../../testdata/golden/aws/kubernetes.json",
			wantObsCount:  2, // Standard, Extended Support
			expectedSKUs:  []string{"SKU-AWS-EKS-STANDARD", "SKU-AWS-EKS-EXTENDED"},
			expectedPrice: "0.1000000000",
		},
		{
			name:          "AWS Serverless Golden",
			goldenFile:    "../../../../testdata/golden/aws/serverless.json",
			wantObsCount:  4, // x86 Request, x86 Duration, ARM Request, ARM Duration
			expectedSKUs:  []string{"SKU-AWS-LAMBDA-REQ-X86", "SKU-AWS-LAMBDA-DUR-X86", "SKU-AWS-LAMBDA-REQ-ARM", "SKU-AWS-LAMBDA-DUR-ARM"},
			expectedPrice: "0.2000000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(tt.goldenFile)
			if err != nil {
				t.Fatalf("failed to open golden file %s: %v", tt.goldenFile, err)
			}
			defer func() { _ = f.Close() }()

			res, err := Normalize(f, fixedTime)
			if err != nil {
				t.Fatalf("Normalize() unexpected error: %v", err)
			}
			obs := res.Observations

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
