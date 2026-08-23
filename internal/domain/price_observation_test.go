package domain_test

import (
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

func TestMarshalUnmarshalDatabaseRDBMSAttributes(t *testing.T) {
	iops := 3000
	orig := domain.PriceObservation{
		Provider:        "aws",
		ServiceCategory: "database_rdbms",
		SkuID:           "AWS-RDS-PG-DB-M6G-XLARGE",
		DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
			Engine:         "postgresql",
			VCPU:           4,
			RAMGB:          16,
			StorageGB:      100,
			IOPS:           &iops,
			MultiAZ:        true,
			DeploymentTier: "standard",
			StorageFamily:  "gp3",
			ComponentType:  "instance",
		},
	}

	data, err := domain.MarshalAttributes(orig)
	if err != nil {
		t.Fatalf("MarshalAttributes() failed: %v", err)
	}

	_, _, _, dbAttrs, err := domain.UnmarshalAttributes("database_rdbms", data)
	if err != nil {
		t.Fatalf("UnmarshalAttributes() failed: %v", err)
	}

	if dbAttrs.Engine != "postgresql" {
		t.Errorf("expected Engine postgresql, got %s", dbAttrs.Engine)
	}
	if dbAttrs.VCPU != 4 || dbAttrs.RAMGB != 16 || dbAttrs.StorageGB != 100 {
		t.Errorf("unexpected specs: %+v", dbAttrs)
	}
	if dbAttrs.IOPS == nil || *dbAttrs.IOPS != 3000 {
		t.Errorf("expected IOPS 3000, got %v", dbAttrs.IOPS)
	}
	if !dbAttrs.MultiAZ {
		t.Errorf("expected MultiAZ true, got false")
	}
	if dbAttrs.ComponentType != "instance" {
		t.Errorf("expected ComponentType instance, got %s", dbAttrs.ComponentType)
	}
}
