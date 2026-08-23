package azure

import (
	"os"
	"testing"
	"time"
)

func TestAzureNormalize_Database(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	f, err := os.Open("../../../../testdata/golden/azure/database.json")
	if err != nil {
		t.Fatalf("failed to open database golden file: %v", err)
	}
	defer func() { _ = f.Close() }()

	obs, _, err := Normalize(f, fixedTime)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if len(obs) != 5 {
		t.Fatalf("expected 5 observations, got %d", len(obs))
	}

	for _, o := range obs {
		if o.ServiceCategory != "database_rdbms" {
			t.Errorf("expected ServiceCategory database_rdbms, got %s", o.ServiceCategory)
		}
		if o.Provider != "azure" {
			t.Errorf("expected Provider azure, got %s", o.Provider)
		}
	}

	// Check PG instance observation
	var pgInst *struct {
		engine  string
		vcpu    float64
		ram     float64
		multiAZ bool
	}
	for _, o := range obs {
		if o.SkuID == "SKU-AZURE-PG-GP-4VCORE" {
			pgInst = &struct {
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
	if pgInst == nil {
		t.Fatalf("missing SKU-AZURE-PG-GP-4VCORE")
	}
	if pgInst.engine != "postgresql" || pgInst.vcpu != 4 || pgInst.ram != 16 || pgInst.multiAZ != false {
		t.Errorf("unexpected instance attrs: %+v", pgInst)
	}

	// Check PG storage observation
	var pgStor *struct {
		multiAZ bool
	}
	for _, o := range obs {
		if o.SkuID == "SKU-AZURE-PG-STORAGE-HA" {
			pgStor = &struct {
				multiAZ bool
			}{
				multiAZ: o.DatabaseRDBMSAttributes.MultiAZ,
			}
			if o.DatabaseRDBMSAttributes.ComponentType != "storage" {
				t.Errorf("expected component_type storage, got %s", o.DatabaseRDBMSAttributes.ComponentType)
			}
		}
	}
	if pgStor == nil {
		t.Fatalf("missing SKU-AZURE-PG-STORAGE-HA")
	}
	if pgStor.multiAZ != true {
		t.Errorf("expected multiAZ true, got false")
	}
}
