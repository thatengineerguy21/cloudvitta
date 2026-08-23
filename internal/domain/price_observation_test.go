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

	var target domain.PriceObservation
	target.ServiceCategory = "database_rdbms"
	if err := domain.UnmarshalAttributes(&target, data); err != nil {
		t.Fatalf("UnmarshalAttributes() failed: %v", err)
	}

	dbAttrs := target.DatabaseRDBMSAttributes
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

func TestMarshalUnmarshalOtherCategoryAttributes(t *testing.T) {
	// Compute
	comp := domain.PriceObservation{
		ServiceCategory: "compute",
		Attributes: domain.ComputeAttributes{
			VCPU:   8,
			RAMGB:  32,
			Family: "m6i",
		},
	}
	compBytes, err := domain.MarshalAttributes(comp)
	if err != nil {
		t.Fatalf("MarshalAttributes(compute) failed: %v", err)
	}
	var compTarget domain.PriceObservation
	compTarget.ServiceCategory = "compute"
	if err := domain.UnmarshalAttributes(&compTarget, compBytes); err != nil {
		t.Fatalf("UnmarshalAttributes(compute) failed: %v", err)
	}
	if compTarget.Attributes.VCPU != 8 || compTarget.Attributes.RAMGB != 32 || compTarget.Attributes.Family != "m6i" {
		t.Errorf("unexpected compute attributes: %+v", compTarget.Attributes)
	}

	// Storage
	stor := domain.PriceObservation{
		ServiceCategory: "storage",
		StorageAttributes: domain.StorageAttributes{
			SizeGB:       500,
			StorageClass: "standard",
		},
	}
	storBytes, err := domain.MarshalAttributes(stor)
	if err != nil {
		t.Fatalf("MarshalAttributes(storage) failed: %v", err)
	}
	var storTarget domain.PriceObservation
	storTarget.ServiceCategory = "storage"
	if err := domain.UnmarshalAttributes(&storTarget, storBytes); err != nil {
		t.Fatalf("UnmarshalAttributes(storage) failed: %v", err)
	}
	if storTarget.StorageAttributes.SizeGB != 500 || storTarget.StorageAttributes.StorageClass != "standard" {
		t.Errorf("unexpected storage attributes: %+v", storTarget.StorageAttributes)
	}

	// Network
	net := domain.PriceObservation{
		ServiceCategory: "network",
		NetworkAttributes: domain.NetworkAttributes{
			EgressGB:     1000,
			TransferType: "internet_egress",
		},
	}
	netBytes, err := domain.MarshalAttributes(net)
	if err != nil {
		t.Fatalf("MarshalAttributes(network) failed: %v", err)
	}
	var netTarget domain.PriceObservation
	netTarget.ServiceCategory = "network"
	if err := domain.UnmarshalAttributes(&netTarget, netBytes); err != nil {
		t.Fatalf("UnmarshalAttributes(network) failed: %v", err)
	}
	if netTarget.NetworkAttributes.EgressGB != 1000 || netTarget.NetworkAttributes.TransferType != "internet_egress" {
		t.Errorf("unexpected network attributes: %+v", netTarget.NetworkAttributes)
	}
}
