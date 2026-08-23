package serverlessunitmap

import (
	"errors"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

func TestMapAWSUnit(t *testing.T) {
	tests := []struct {
		rawUnit   string
		compType  string
		wantUnit  string
		wantErrIs error
	}{
		{"Requests", domain.ComponentTypeRequestFee, domain.UnitPerMillionRequests, nil},
		{"Request", domain.ComponentTypeRequestFee, domain.UnitPerMillionRequests, nil},
		{"AWS-Lambda-Requests", domain.ComponentTypeRequestFee, domain.UnitPerMillionRequests, nil},
		{"Seconds", domain.ComponentTypeDurationFee, domain.UnitPerGBSecond, nil},
		{"GB-Second", domain.ComponentTypeDurationFee, domain.UnitPerGBSecond, nil},
		{"Lambda-GB-Second", domain.ComponentTypeDurationFee, domain.UnitPerGBSecond, nil},
		{"unknown_unit", domain.ComponentTypeRequestFee, "", ErrUnmappedUnit},
		{"unknown_unit", domain.ComponentTypeDurationFee, "", ErrUnmappedUnit},
	}

	for _, tt := range tests {
		got, err := MapAWSUnit(tt.rawUnit, tt.compType)
		if tt.wantErrIs != nil {
			if !errors.Is(err, tt.wantErrIs) {
				t.Errorf("MapAWSUnit(%q, %q) error = %v, want %v", tt.rawUnit, tt.compType, err, tt.wantErrIs)
			}
		} else {
			if err != nil || got != tt.wantUnit {
				t.Errorf("MapAWSUnit(%q, %q) = (%q, %v), want (%q, nil)", tt.rawUnit, tt.compType, got, err, tt.wantUnit)
			}
		}
	}
}

func TestMapAzureUnit(t *testing.T) {
	tests := []struct {
		rawUnit   string
		compType  string
		wantUnit  string
		wantErrIs error
	}{
		{"10", domain.ComponentTypeRequestFee, domain.UnitPer10Requests, nil},
		{"10 Executions", domain.ComponentTypeRequestFee, domain.UnitPer10Requests, nil},
		{"10x Executions", domain.ComponentTypeRequestFee, domain.UnitPer10Requests, nil},
		{"1 GB Second", domain.ComponentTypeDurationFee, domain.UnitPerGBSecond, nil},
		{"1 GB-s", domain.ComponentTypeDurationFee, domain.UnitPerGBSecond, nil},
		{"GB-Second", domain.ComponentTypeDurationFee, domain.UnitPerGBSecond, nil},
		{"unknown_unit", domain.ComponentTypeRequestFee, "", ErrUnmappedUnit},
	}

	for _, tt := range tests {
		got, err := MapAzureUnit(tt.rawUnit, tt.compType)
		if tt.wantErrIs != nil {
			if !errors.Is(err, tt.wantErrIs) {
				t.Errorf("MapAzureUnit(%q, %q) error = %v, want %v", tt.rawUnit, tt.compType, err, tt.wantErrIs)
			}
		} else {
			if err != nil || got != tt.wantUnit {
				t.Errorf("MapAzureUnit(%q, %q) = (%q, %v), want (%q, nil)", tt.rawUnit, tt.compType, got, err, tt.wantUnit)
			}
		}
	}
}

func TestMapGCPUnit(t *testing.T) {
	tests := []struct {
		rawUnit   string
		compType  string
		wantUnit  string
		wantErrIs error
	}{
		{"Calls", domain.ComponentTypeRequestFee, domain.UnitPerRequest, nil},
		{"Invocations", domain.ComponentTypeRequestFee, domain.UnitPerRequest, nil},
		{"s", domain.ComponentTypeDurationFeeMemory, domain.UnitPerGBSecond, nil},
		{"GiBy.s", domain.ComponentTypeDurationFeeMemory, domain.UnitPerGBSecond, nil},
		{"GB-Second", domain.ComponentTypeDurationFeeMemory, domain.UnitPerGBSecond, nil},
		{"GHz.s", domain.ComponentTypeDurationFeeCPU, domain.UnitPerGHzSecond, nil},
		{"vCPU.s", domain.ComponentTypeDurationFeeCPU, domain.UnitPerVCPUSecond, nil},
		{"s", domain.ComponentTypeDurationFeeCPU, domain.UnitPerGHzSecond, nil},
		{"unknown_unit", domain.ComponentTypeRequestFee, "", ErrUnmappedUnit},
	}

	for _, tt := range tests {
		got, err := MapGCPUnit(tt.rawUnit, tt.compType)
		if tt.wantErrIs != nil {
			if !errors.Is(err, tt.wantErrIs) {
				t.Errorf("MapGCPUnit(%q, %q) error = %v, want %v", tt.rawUnit, tt.compType, err, tt.wantErrIs)
			}
		} else {
			if err != nil || got != tt.wantUnit {
				t.Errorf("MapGCPUnit(%q, %q) = (%q, %v), want (%q, nil)", tt.rawUnit, tt.compType, got, err, tt.wantUnit)
			}
		}
	}
}

func TestNormalizeUnit(t *testing.T) {
	got, err := NormalizeUnit("aws", "Requests", domain.ComponentTypeRequestFee)
	if err != nil || got != domain.UnitPerMillionRequests {
		t.Errorf("NormalizeUnit(aws, Requests) = (%q, %v), want (%q, nil)", got, err, domain.UnitPerMillionRequests)
	}

	gotAzure, err := NormalizeUnit("azure", "10", domain.ComponentTypeRequestFee)
	if err != nil || gotAzure != domain.UnitPer10Requests {
		t.Errorf("NormalizeUnit(azure, 10) = (%q, %v), want (%q, nil)", gotAzure, err, domain.UnitPer10Requests)
	}

	gotGCP, err := NormalizeUnit("gcp", "Calls", domain.ComponentTypeRequestFee)
	if err != nil || gotGCP != domain.UnitPerRequest {
		t.Errorf("NormalizeUnit(gcp, Calls) = (%q, %v), want (%q, nil)", gotGCP, err, domain.UnitPerRequest)
	}

	_, err = NormalizeUnit("unmapped_provider", "Calls", domain.ComponentTypeRequestFee)
	if err == nil {
		t.Errorf("NormalizeUnit(unmapped_provider) expected error, got nil")
	}
}

func TestSupportedCanonicalUnits(t *testing.T) {
	units := SupportedCanonicalUnits()
	if len(units) < 5 {
		t.Fatalf("expected at least 5 supported canonical units, got %d", len(units))
	}
}
