package gcp_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/gcp"
)

func TestNormalize_GCPDatabaseNoSQLGolden(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	f, err := os.Open("../../../../testdata/golden/gcp/database_nosql.json")
	if err != nil {
		t.Fatalf("failed to open golden file: %v", err)
	}
	defer func() { _ = f.Close() }()

	res, _, err := gcp.Normalize(f, fixedTime)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	obs := res.Observations

	if len(obs) != 3 {
		t.Fatalf("expected 3 observations, got %d", len(obs))
	}

	for _, o := range obs {
		if o.Provider != "gcp" {
			t.Errorf("expected Provider gcp, got %s", o.Provider)
		}
		if o.ServiceCategory != "database_nosql" {
			t.Errorf("expected ServiceCategory database_nosql, got %s", o.ServiceCategory)
		}
		if o.PriceCurrency != "USD" {
			t.Errorf("expected PriceCurrency USD, got %s", o.PriceCurrency)
		}
		if o.Region != "us-east4" || o.RegionGroup != "us-east" {
			t.Errorf("unexpected region/group: %s / %s", o.Region, o.RegionGroup)
		}
	}
}

func TestNormalize_GCPBigtable(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	jsonInput := `{
		"skus": [
			{
				"name": "services/C802-861C-2155/skus/SKU-BIGTABLE-NODE",
				"skuId": "SKU-BIGTABLE-NODE",
				"description": "Cloud Bigtable: Node in Virginia",
				"category": {
					"serviceDisplayName": "Cloud Bigtable",
					"resourceFamily": "ApplicationServices",
					"resourceGroup": "Node",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["us-east4"],
				"pricingInfo": [{
					"pricingExpression": {
						"usageUnit": "h",
						"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 650000000}}]
					}
				}]
			},
			{
				"name": "services/C802-861C-2155/skus/SKU-BIGTABLE-SSD",
				"skuId": "SKU-BIGTABLE-SSD",
				"description": "Cloud Bigtable: SSD Storage in Virginia",
				"category": {
					"serviceDisplayName": "Cloud Bigtable",
					"resourceFamily": "ApplicationServices",
					"resourceGroup": "SSD",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["us-east4"],
				"pricingInfo": [{
					"pricingExpression": {
						"usageUnit": "GiBy.mo",
						"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 170000000}}]
					}
				}]
			},
			{
				"name": "services/C802-861C-2155/skus/SKU-BIGTABLE-HDD",
				"skuId": "SKU-BIGTABLE-HDD",
				"description": "Cloud Bigtable: HDD Storage in Virginia",
				"category": {
					"serviceDisplayName": "Cloud Bigtable",
					"resourceFamily": "ApplicationServices",
					"resourceGroup": "HDD",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["us-east4"],
				"pricingInfo": [{
					"pricingExpression": {
						"usageUnit": "GiBy.mo",
						"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 26000000}}]
					}
				}]
			}
		]
	}`

	res, _, err := gcp.Normalize(strings.NewReader(jsonInput), fixedTime)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	obs := res.Observations
	if len(obs) != 3 {
		t.Fatalf("expected 3 observations, got %d", len(obs))
	}

	for _, o := range obs {
		if o.Provider != "gcp" {
			t.Errorf("expected Provider gcp, got %s", o.Provider)
		}
		if o.ServiceCategory != "database_nosql" {
			t.Errorf("expected ServiceCategory database_nosql, got %s", o.ServiceCategory)
		}
		if o.DatabaseNoSQLAttributes.DataModel != "wide_column" {
			t.Errorf("expected DataModel wide_column, got %s", o.DatabaseNoSQLAttributes.DataModel)
		}
		if o.RegionGroup != "us-east" {
			t.Errorf("expected RegionGroup us-east, got %s", o.RegionGroup)
		}
	}

	// Verify node observation
	nodeObs := obs[0]
	if nodeObs.SkuID != "SKU-BIGTABLE-NODE" {
		t.Errorf("expected SkuID SKU-BIGTABLE-NODE, got %s", nodeObs.SkuID)
	}
	if nodeObs.DatabaseNoSQLAttributes.ComponentType != "throughput" {
		t.Errorf("expected ComponentType throughput, got %s", nodeObs.DatabaseNoSQLAttributes.ComponentType)
	}
	if nodeObs.DatabaseNoSQLAttributes.PricingMode != "provisioned" {
		t.Errorf("expected PricingMode provisioned, got %s", nodeObs.DatabaseNoSQLAttributes.PricingMode)
	}
	if nodeObs.Unit != "Hrs" {
		t.Errorf("expected Unit Hrs, got %s", nodeObs.Unit)
	}

	// Verify SSD storage observation
	ssdObs := obs[1]
	if ssdObs.DatabaseNoSQLAttributes.ComponentType != "storage" {
		t.Errorf("expected ComponentType storage, got %s", ssdObs.DatabaseNoSQLAttributes.ComponentType)
	}
	if ssdObs.DatabaseNoSQLAttributes.StorageClass != "standard" {
		t.Errorf("expected StorageClass standard, got %s", ssdObs.DatabaseNoSQLAttributes.StorageClass)
	}

	// Verify HDD storage observation
	hddObs := obs[2]
	if hddObs.DatabaseNoSQLAttributes.ComponentType != "storage" {
		t.Errorf("expected ComponentType storage, got %s", hddObs.DatabaseNoSQLAttributes.ComponentType)
	}
	if hddObs.DatabaseNoSQLAttributes.StorageClass != "infrequent_access" {
		t.Errorf("expected StorageClass infrequent_access, got %s", hddObs.DatabaseNoSQLAttributes.StorageClass)
	}
}
