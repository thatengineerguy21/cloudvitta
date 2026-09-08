package gcp

import (
	"os"
	"strings"
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
			wantObsCount: len(knownGCPCloudSQLSpecs)*2 + 3, // 16 shapes * 2 (Zonal + Regional HA), Storage (single & HA), AlloyDB PG 8vCore
			expectedSKUs: []string{
				"SKU-GCP-CLOUDSQL-POSTGRESQL-STANDARD-4VCPU-15GB",
				"SKU-GCP-CLOUDSQL-POSTGRESQL-STANDARD-4VCPU-16GB-HA",
				"SKU-GCP-CLOUDSQL-STORAGE-SSD",
				"SKU-GCP-CLOUDSQL-STORAGE-SSD-HA",
				"SKU-GCP-ALLOYDB-PG-8VCORE",
			},
		},
		{
			name:         "GCP NoSQL Database Golden",
			goldenFile:   "../../../../testdata/golden/gcp/database_nosql.json",
			wantObsCount: 3, // Firestore Reads, Firestore Writes, Firestore Storage
			expectedSKUs: []string{"SKU-GCP-FIRESTORE-READS", "SKU-GCP-FIRESTORE-WRITES", "SKU-GCP-FIRESTORE-STORAGE"},
		},
		{
			name:         "GCP Kubernetes Golden",
			goldenFile:   "../../../../testdata/golden/gcp/kubernetes.json",
			wantObsCount: 1, // GKE Cluster Management Fee
			expectedSKUs: []string{"SKU-GCP-GKE-CLUSTER-MGMT"},
		},
		{
			name:         "GCP Serverless Golden",
			goldenFile:   "../../../../testdata/golden/gcp/serverless.json",
			wantObsCount: 3, // Invocations, CPU Time, Memory Time
			expectedSKUs: []string{"SKU-GCP-CF-INVOCATIONS", "SKU-GCP-CF-CPU-TIME", "SKU-GCP-CF-MEM-TIME"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(tt.goldenFile)
			if err != nil {
				t.Fatalf("failed to open golden file %s: %v", tt.goldenFile, err)
			}
			defer func() { _ = f.Close() }()

			res, _, err := Normalize(f, fixedTime)
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

func TestComposeMachineTypePricing_MultiRegion(t *testing.T) {
	fixedTime := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

	jsonBody := `{
		"skus": [
			{
				"skuId": "SKU-GCP-N2-CORE-VIRGINIA",
				"description": "N2 Instance Core running in Virginia",
				"category": {
					"serviceDisplayName": "Compute Engine",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["us-east4"],
				"pricingInfo": [
					{
						"pricingExpression": {
							"usageUnit": "h",
							"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 31611000}}]
						}
					}
				]
			},
			{
				"skuId": "SKU-GCP-N2-RAM-VIRGINIA",
				"description": "N2 Instance Ram running in Virginia",
				"category": {
					"serviceDisplayName": "Compute Engine",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["us-east4"],
				"pricingInfo": [
					{
						"pricingExpression": {
							"usageUnit": "h",
							"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 4237000}}]
						}
					}
				]
			},
			{
				"skuId": "SKU-GCP-N2-CORE-FRANKFURT",
				"description": "N2 Instance Core running in Frankfurt",
				"category": {
					"serviceDisplayName": "Compute Engine",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["europe-west3"],
				"pricingInfo": [
					{
						"pricingExpression": {
							"usageUnit": "h",
							"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 34772000}}]
						}
					}
				]
			},
			{
				"skuId": "SKU-GCP-N2-RAM-FRANKFURT",
				"description": "N2 Instance Ram running in Frankfurt",
				"category": {
					"serviceDisplayName": "Compute Engine",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["europe-west3"],
				"pricingInfo": [
					{
						"pricingExpression": {
							"usageUnit": "h",
							"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 4661000}}]
						}
					}
				]
			}
		]
	}`

	res, _, err := Normalize(strings.NewReader(jsonBody), fixedTime)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	regionsFound := make(map[string]int)
	for _, o := range res.Observations {
		if o.ServiceCategory != "compute" {
			t.Errorf("expected compute observation, got %s", o.ServiceCategory)
		}
		regionsFound[o.Region]++
	}

	if regionsFound["us-east4"] == 0 {
		t.Errorf("expected observations in us-east4, got 0")
	}
	if regionsFound["europe-west3"] == 0 {
		t.Errorf("expected observations in europe-west3, got 0")
	}
}
