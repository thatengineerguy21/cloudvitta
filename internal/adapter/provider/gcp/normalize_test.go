package gcp

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

func TestParseGCPDatabaseAttributes_NoFabricatedFallback(t *testing.T) {
	tests := []struct {
		name        string
		description string
		skuName     string
		wantOK      bool
	}{
		{
			name:        "real vCPU meter SKU from Virginia",
			description: "Cloud SQL for PostgreSQL: Zonal - Enterprise vCPU in Northern Virginia",
			skuName:     "services/9662-B51E-5089/skus/0001-0118-796B",
			wantOK:      false,
		},
		{
			name:        "real RAM meter SKU from Virginia",
			description: "Cloud SQL for PostgreSQL: Zonal - Enterprise RAM in Northern Virginia",
			skuName:     "services/9662-B51E-5089/skus/0002-0118-796C",
			wantOK:      false,
		},
		{
			name:        "real MySQL vCPU meter SKU from London",
			description: "Cloud SQL for MySQL: Regional - Enterprise Plus vCPU in London",
			skuName:     "services/9662-B51E-5089/skus/0003-0118-796D",
			wantOK:      false,
		},
		{
			name:        "real MySQL RAM meter SKU from London",
			description: "Cloud SQL for MySQL: Regional - Enterprise Plus RAM in London",
			skuName:     "services/9662-B51E-5089/skus/0004-0118-796E",
			wantOK:      false,
		},
		{
			name:        "unparseable arbitrary text",
			description: "Cloud SQL for PostgreSQL: Some unparseable custom resource",
			skuName:     "services/9662-B51E-5089/skus/0005-0118-796F",
			wantOK:      false,
		},
		{
			name:        "valid db-custom description",
			description: "Cloud SQL for PostgreSQL: DB instance - db-custom-4-15360 running in Virginia",
			skuName:     "services/9662-B51E-5089/skus/0006-0118-796G",
			wantOK:      true,
		},
		{
			name:        "valid vCPU and GB description",
			description: "AlloyDB for PostgreSQL: Instance 8 vCPU, 64 GB running in Virginia",
			skuName:     "services/9662-B51E-5089/skus/0007-0118-796H",
			wantOK:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vcpu, ram, _, ok := parseGCPDatabaseAttributes(tc.description, tc.skuName)
			if ok != tc.wantOK {
				t.Fatalf("parseGCPDatabaseAttributes(%q) ok = %v, want %v", tc.description, ok, tc.wantOK)
			}
			if !tc.wantOK && (vcpu == 2 && ram == 8) {
				t.Errorf("parseGCPDatabaseAttributes(%q) returned fabricated fallback (2, 8)", tc.description)
			}
		})
	}
}

func TestGCPNormalize_Database_UnparseableSKUsRouteToQuarantine(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	unparseableSKUs := []struct {
		skuID string
		desc  string
	}{
		{
			skuID: "SKU-TEST-UNKNOWN-CLOUDSQL-1",
			desc:  "Cloud SQL for PostgreSQL: Unparseable Custom Line Item",
		},
		{
			skuID: "SKU-TEST-UNKNOWN-CLOUDSQL-2",
			desc:  "Cloud SQL for MySQL: Arbitrary Unrecognized Database Feature",
		},
	}

	for _, item := range unparseableSKUs {
		t.Run(item.skuID, func(t *testing.T) {
			jsonBody := fmt.Sprintf(`{
				"skus": [
					{
						"skuId": "%s",
						"description": "%s",
						"category": {
							"serviceDisplayName": "Cloud SQL",
							"resourceFamily": "ApplicationServices",
							"resourceGroup": "PostgreSQL",
							"usageType": "OnDemand"
						},
						"serviceRegions": ["us-east4"],
						"pricingInfo": [
							{
								"pricingExpression": {
									"usageUnit": "h",
									"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 50000000}}]
								}
							}
						]
					}
				]
			}`, item.skuID, item.desc)

			memSink := quarantine.NewMemorySink()
			res, _, err := Normalize(strings.NewReader(jsonBody), fixedTime, memSink)
			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}

			// Must not emit fabricated instance observations
			for _, obs := range res.Observations {
				if obs.DatabaseRDBMSAttributes.VCPU == 2 && obs.DatabaseRDBMSAttributes.RAMGB == 8 {
					t.Fatalf("emitted observation with fabricated vCPU=2, RAM=8: %+v", obs)
				}
			}

			// Genuinely unparseable non-component SKUs must route to quarantine
			if memSink.Count() == 0 {
				t.Fatalf("expected SKU %s to route to quarantine sink, got 0 items", item.skuID)
			}

			items := memSink.Items()
			found := false
			for _, qItem := range items {
				if qItem.SkuID == item.skuID {
					found = true
					if qItem.Kind != "database_attributes" {
						t.Errorf("expected Kind database_attributes, got %s", qItem.Kind)
					}
				}
			}
			if !found {
				t.Errorf("quarantine sink does not contain SKU %s", item.skuID)
			}
		})
	}
}

func TestGCPNormalize_Database_ComponentComposition(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	jsonBody := `{
		"skus": [
			{
				"skuId": "SKU-TEST-VCPU-METER",
				"description": "Cloud SQL for PostgreSQL: Zonal - Enterprise vCPU in Northern Virginia",
				"category": {
					"serviceDisplayName": "Cloud SQL",
					"resourceFamily": "ApplicationServices",
					"resourceGroup": "CPU",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["us-east4"],
				"pricingInfo": [
					{
						"pricingExpression": {
							"usageUnit": "h",
							"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 50000000}}]
						}
					}
				]
			},
			{
				"skuId": "SKU-TEST-RAM-METER",
				"description": "Cloud SQL for PostgreSQL: Zonal - Enterprise RAM in Northern Virginia",
				"category": {
					"serviceDisplayName": "Cloud SQL",
					"resourceFamily": "ApplicationServices",
					"resourceGroup": "RAM",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["us-east4"],
				"pricingInfo": [
					{
						"pricingExpression": {
							"usageUnit": "GiBy.h",
							"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 7000000}}]
						}
					}
				]
			}
		]
	}`

	memSink := quarantine.NewMemorySink()
	res, _, err := Normalize(strings.NewReader(jsonBody), fixedTime, memSink)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if memSink.Count() != 0 {
		t.Errorf("expected 0 quarantine items for valid component meters, got %d", memSink.Count())
	}

	if len(res.Observations) != len(knownGCPCloudSQLSpecs) {
		t.Fatalf("got %d synthesized observations, want %d", len(res.Observations), len(knownGCPCloudSQLSpecs))
	}

	// Find the 4 vCPU, 16 GB observation
	var found16GB bool
	for _, obs := range res.Observations {
		if obs.SkuID == "SKU-GCP-CLOUDSQL-POSTGRESQL-STANDARD-4VCPU-16GB" {
			found16GB = true
			if obs.DatabaseRDBMSAttributes.Engine != "postgresql" {
				t.Errorf("expected engine postgresql, got %s", obs.DatabaseRDBMSAttributes.Engine)
			}
			if obs.DatabaseRDBMSAttributes.VCPU != 4 {
				t.Errorf("expected vCPU 4, got %f", obs.DatabaseRDBMSAttributes.VCPU)
			}
			if obs.DatabaseRDBMSAttributes.RAMGB != 16 {
				t.Errorf("expected RAM 16, got %f", obs.DatabaseRDBMSAttributes.RAMGB)
			}
			if obs.DatabaseRDBMSAttributes.MultiAZ != false {
				t.Errorf("expected multiAZ false, got true")
			}
			// 4 * 0.05 + 16 * 0.007 = 0.20 + 0.112 = 0.312
			expectedPrice := "0.312"
			if obs.PriceAmount.String() != expectedPrice {
				t.Errorf("expected PriceAmount %s, got %s", expectedPrice, obs.PriceAmount.String())
			}
		}
	}
	if !found16GB {
		t.Errorf("missing synthesized observation SKU-GCP-CLOUDSQL-POSTGRESQL-STANDARD-4VCPU-16GB")
	}
}
