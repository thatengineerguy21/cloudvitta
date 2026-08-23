package aws

import (
	"os"
	"testing"
	"time"
)

func TestAWSNormalize_Database(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	f, err := os.Open("../../../../testdata/golden/aws/database.json")
	if err != nil {
		t.Fatalf("failed to open database golden file: %v", err)
	}
	defer func() { _ = f.Close() }()

	obs, err := Normalize(f, fixedTime)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if len(obs) != 6 {
		t.Fatalf("expected 6 observations, got %d", len(obs))
	}

	for _, o := range obs {
		if o.ServiceCategory != "database_rdbms" {
			t.Errorf("expected ServiceCategory database_rdbms, got %s", o.ServiceCategory)
		}
		if o.Provider != "aws" {
			t.Errorf("expected Provider aws, got %s", o.Provider)
		}
	}

	// Check instance observation
	var instObs *struct {
		engine  string
		vcpu    float64
		ram     float64
		multiAZ bool
	}
	for _, o := range obs {
		if o.SkuID == "SKU-RDS-PG-M6G-XLARGE" {
			instObs = &struct {
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
	if instObs == nil {
		t.Fatalf("missing SKU-RDS-PG-M6G-XLARGE")
	}
	if instObs.engine != "postgresql" || instObs.vcpu != 4 || instObs.ram != 16 || instObs.multiAZ != false {
		t.Errorf("unexpected instance attrs: %+v", instObs)
	}

	// Check storage observation
	var storObs *struct {
		family  string
		multiAZ bool
	}
	for _, o := range obs {
		if o.SkuID == "SKU-RDS-STORAGE-GP3" {
			storObs = &struct {
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
	if storObs == nil {
		t.Fatalf("missing SKU-RDS-STORAGE-GP3")
	}
	if storObs.family != "gp3" || storObs.multiAZ != false {
		t.Errorf("unexpected storage attrs: %+v", storObs)
	}
}
