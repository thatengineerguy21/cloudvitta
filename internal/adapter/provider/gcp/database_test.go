package gcp

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

func TestGCPNormalize_Database(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	f, err := os.Open("../../../../testdata/golden/gcp/database.json")
	if err != nil {
		t.Fatalf("failed to open database golden file: %v", err)
	}
	defer func() { _ = f.Close() }()

	res, _, err := Normalize(f, fixedTime)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	obs := res.Observations

	// 16 shapes * 2 (Zonal + Regional HA) + 2 storage (Zonal + Regional HA) + 1 AlloyDB = 35
	expectedTotal := len(knownGCPCloudSQLSpecs)*2 + 3
	if len(obs) != expectedTotal {
		t.Fatalf("expected %d observations, got %d", expectedTotal, len(obs))
	}

	for _, o := range obs {
		if o.ServiceCategory != "database_rdbms" {
			t.Errorf("expected ServiceCategory database_rdbms, got %s", o.ServiceCategory)
		}
		if o.Provider != "gcp" {
			t.Errorf("expected Provider gcp, got %s", o.Provider)
		}
	}

	// Check Cloud SQL PG synthesized 4 vCPU, 15 GB instance observation (Zonal)
	var cloudSQLInst15 *struct {
		engine      string
		vcpu        float64
		ram         float64
		multiAZ     bool
		priceAmount string
	}
	for _, o := range obs {
		if o.SkuID == "SKU-GCP-CLOUDSQL-POSTGRESQL-STANDARD-4VCPU-15GB" {
			cloudSQLInst15 = &struct {
				engine      string
				vcpu        float64
				ram         float64
				multiAZ     bool
				priceAmount string
			}{
				engine:      o.DatabaseRDBMSAttributes.Engine,
				vcpu:        o.DatabaseRDBMSAttributes.VCPU,
				ram:         o.DatabaseRDBMSAttributes.RAMGB,
				multiAZ:     o.DatabaseRDBMSAttributes.MultiAZ,
				priceAmount: o.PriceAmount.String(),
			}
			if o.DatabaseRDBMSAttributes.ComponentType != "instance" {
				t.Errorf("expected component_type instance, got %s", o.DatabaseRDBMSAttributes.ComponentType)
			}
		}
	}
	if cloudSQLInst15 == nil {
		t.Fatalf("missing SKU-GCP-CLOUDSQL-POSTGRESQL-STANDARD-4VCPU-15GB")
	}
	// 4 * 0.05 + 15 * 0.007 = 0.20 + 0.105 = 0.305
	if cloudSQLInst15.engine != "postgresql" || cloudSQLInst15.vcpu != 4 || cloudSQLInst15.ram != 15 || cloudSQLInst15.multiAZ != false || cloudSQLInst15.priceAmount != "0.305" {
		t.Errorf("unexpected instance attrs: %+v", cloudSQLInst15)
	}

	// Check Cloud SQL PG synthesized 4 vCPU, 16 GB instance observation (Regional HA)
	var cloudSQLInst16HA *struct {
		engine      string
		vcpu        float64
		ram         float64
		multiAZ     bool
		priceAmount string
	}
	for _, o := range obs {
		if o.SkuID == "SKU-GCP-CLOUDSQL-POSTGRESQL-STANDARD-4VCPU-16GB-HA" {
			cloudSQLInst16HA = &struct {
				engine      string
				vcpu        float64
				ram         float64
				multiAZ     bool
				priceAmount string
			}{
				engine:      o.DatabaseRDBMSAttributes.Engine,
				vcpu:        o.DatabaseRDBMSAttributes.VCPU,
				ram:         o.DatabaseRDBMSAttributes.RAMGB,
				multiAZ:     o.DatabaseRDBMSAttributes.MultiAZ,
				priceAmount: o.PriceAmount.String(),
			}
			if o.DatabaseRDBMSAttributes.ComponentType != "instance" {
				t.Errorf("expected component_type instance, got %s", o.DatabaseRDBMSAttributes.ComponentType)
			}
		}
	}
	if cloudSQLInst16HA == nil {
		t.Fatalf("missing SKU-GCP-CLOUDSQL-POSTGRESQL-STANDARD-4VCPU-16GB-HA")
	}
	// 4 * 0.10 + 16 * 0.014 = 0.40 + 0.224 = 0.624
	if cloudSQLInst16HA.engine != "postgresql" || cloudSQLInst16HA.vcpu != 4 || cloudSQLInst16HA.ram != 16 || cloudSQLInst16HA.multiAZ != true || cloudSQLInst16HA.priceAmount != "0.624" {
		t.Errorf("unexpected HA instance attrs: %+v", cloudSQLInst16HA)
	}

	// Check Cloud SQL storage observation (Regional HA)
	var cloudSQLStor *struct {
		family  string
		multiAZ bool
	}
	for _, o := range obs {
		if o.SkuID == "SKU-GCP-CLOUDSQL-STORAGE-SSD-HA" {
			cloudSQLStor = &struct {
				family  string
				multiAZ bool
			}{
				family:  o.DatabaseRDBMSAttributes.StorageFamily,
				multiAZ: o.DatabaseRDBMSAttributes.MultiAZ,
			}
			if o.DatabaseRDBMSAttributes.ComponentType != "storage" {
				t.Errorf("expected component_type storage, got %s", o.DatabaseRDBMSAttributes.ComponentType)
			}
		}
	}
	if cloudSQLStor == nil {
		t.Fatalf("missing SKU-GCP-CLOUDSQL-STORAGE-SSD-HA")
	}
	if cloudSQLStor.family != "ssd" || cloudSQLStor.multiAZ != true {
		t.Errorf("unexpected storage attrs: %+v", cloudSQLStor)
	}
}

func TestGCPNormalize_Database_GenericStorageAndNonInstanceIgnored(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	jsonBody := `{
		"skus": [
			{
				"skuId": "SKU-GCP-GENERIC-STORAGE",
				"description": "Storage PD SSD in Virginia",
				"category": {
					"serviceDisplayName": "Cloud SQL",
					"resourceFamily": "ApplicationServices",
					"resourceGroup": "PDSSD",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["us-east4"],
				"pricingInfo": [
					{
						"pricingExpression": {
							"usageUnit": "GiBy.mo",
							"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 170000000}}]
						}
					}
				]
			},
			{
				"skuId": "SKU-GCP-CLOUDSQL-EGRESS",
				"description": "Cloud SQL: Network Egress - Worldwide",
				"category": {
					"serviceDisplayName": "Cloud SQL",
					"resourceFamily": "ApplicationServices",
					"resourceGroup": "Network",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["us-east4"],
				"pricingInfo": [
					{
						"pricingExpression": {
							"usageUnit": "GiBy",
							"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 120000000}}]
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

	if len(res.Observations) != 1 {
		t.Fatalf("expected 1 observation (generic storage), got %d", len(res.Observations))
	}
	obs := res.Observations[0]
	if obs.SkuID != "SKU-GCP-GENERIC-STORAGE" {
		t.Errorf("expected SKU-GCP-GENERIC-STORAGE, got %s", obs.SkuID)
	}
	if obs.DatabaseRDBMSAttributes.ComponentType != "storage" {
		t.Errorf("expected ComponentType storage, got %s", obs.DatabaseRDBMSAttributes.ComponentType)
	}
	if obs.DatabaseRDBMSAttributes.Engine != "" {
		t.Errorf("expected generic storage to have empty engine, got %s", obs.DatabaseRDBMSAttributes.Engine)
	}

	if res.IgnoredCount != 1 {
		t.Errorf("expected 1 ignored out-of-scope SKU (network egress), got %d", res.IgnoredCount)
	}

	if memSink.Count() != 0 {
		t.Errorf("expected 0 quarantine items, got %d", memSink.Count())
	}
}

func TestNormalize_GCPAlloyDB(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	jsonBody := `{
		"skus": [
			{
				"name": "services/C49F-B7F2-7416/skus/SKU-GCP-ALLOYDB-8VCPU",
				"skuId": "SKU-GCP-ALLOYDB-8VCPU",
				"description": "AlloyDB for PostgreSQL: Instance 8 vCPU, 64 GB in Virginia",
				"category": {
					"serviceDisplayName": "AlloyDB for PostgreSQL",
					"resourceFamily": "ApplicationServices",
					"resourceGroup": "AlloyDB",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["us-east4"],
				"pricingInfo": [
					{
						"pricingExpression": {
							"usageUnit": "h",
							"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 780000000}}]
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

	if len(res.Observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(res.Observations))
	}
	obs := res.Observations[0]
	if obs.Provider != "gcp" {
		t.Errorf("Provider = %q, want gcp", obs.Provider)
	}
	if obs.ServiceCategory != "database_rdbms" {
		t.Errorf("ServiceCategory = %q, want database_rdbms", obs.ServiceCategory)
	}
	if obs.DatabaseRDBMSAttributes.Engine != "postgresql" {
		t.Errorf("Engine = %q, want postgresql", obs.DatabaseRDBMSAttributes.Engine)
	}
	if obs.DatabaseRDBMSAttributes.VCPU != 8 {
		t.Errorf("VCPU = %v, want 8", obs.DatabaseRDBMSAttributes.VCPU)
	}
	if obs.DatabaseRDBMSAttributes.RAMGB != 64 {
		t.Errorf("RAMGB = %v, want 64", obs.DatabaseRDBMSAttributes.RAMGB)
	}
	if obs.DatabaseRDBMSAttributes.DeploymentTier != "alloydb" {
		t.Errorf("DeploymentTier = %q, want alloydb", obs.DatabaseRDBMSAttributes.DeploymentTier)
	}
	if obs.DatabaseRDBMSAttributes.ComponentType != "instance" {
		t.Errorf("ComponentType = %q, want instance", obs.DatabaseRDBMSAttributes.ComponentType)
	}
}
