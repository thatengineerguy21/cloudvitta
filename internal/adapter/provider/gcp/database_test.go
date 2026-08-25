package gcp

import (
	"os"
	"testing"
	"time"
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
