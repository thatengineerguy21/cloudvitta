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

	if len(obs) != 5 {
		t.Fatalf("expected 5 observations, got %d", len(obs))
	}

	for _, o := range obs {
		if o.ServiceCategory != "database_rdbms" {
			t.Errorf("expected ServiceCategory database_rdbms, got %s", o.ServiceCategory)
		}
		if o.Provider != "gcp" {
			t.Errorf("expected Provider gcp, got %s", o.Provider)
		}
	}

	// Check Cloud SQL PG instance observation
	var cloudSQLInst *struct {
		engine  string
		vcpu    float64
		ram     float64
		multiAZ bool
	}
	for _, o := range obs {
		if o.SkuID == "SKU-GCP-CLOUDSQL-PG-4VCORE" {
			cloudSQLInst = &struct {
				engine  string
				vcpu    float64
				ram     float64
				multiAZ bool
			}{
				engine:  o.DatabaseRDBMSAttributes.Engine,
				vcpu:    o.DatabaseRDBMSAttributes.VCPU,
				ram:     o.DatabaseRDBMSAttributes.RAMGB,
				multiAZ: o.DatabaseRDBMSAttributes.MultiAZ,
			}
			if o.DatabaseRDBMSAttributes.ComponentType != "instance" {
				t.Errorf("expected component_type instance, got %s", o.DatabaseRDBMSAttributes.ComponentType)
			}
		}
	}
	if cloudSQLInst == nil {
		t.Fatalf("missing SKU-GCP-CLOUDSQL-PG-4VCORE")
	}
	if cloudSQLInst.engine != "postgresql" || cloudSQLInst.vcpu != 4 || cloudSQLInst.ram != 15 || cloudSQLInst.multiAZ != false {
		t.Errorf("unexpected instance attrs: %+v", cloudSQLInst)
	}

	// Check Cloud SQL storage observation
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
