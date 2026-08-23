package serverlessarchmap

import (
	"errors"
	"testing"
)

func TestMapAWSArchitecture(t *testing.T) {
	tests := []struct {
		rawArch  string
		wantArch string
		wantErr  error
	}{
		{"x86_64", ArchX86_64, nil},
		{"x86", ArchX86_64, nil},
		{"AWS-Lambda-Requests", ArchX86_64, nil},
		{"AWS-Lambda-Duration", ArchX86_64, nil},
		{"Request", ArchX86_64, nil},
		{"Lambda-GB-Second", ArchX86_64, nil},
		{"USE2-Request", ArchX86_64, nil},
		{"arm64", ArchARM64, nil},
		{"arm", ArchARM64, nil},
		{"graviton", ArchARM64, nil},
		{"AWS-Lambda-Requests-ARM", ArchARM64, nil},
		{"AWS-Lambda-Duration-ARM", ArchARM64, nil},
		{"Request-ARM", ArchARM64, nil},
		{"Lambda-GB-Second-ARM", ArchARM64, nil},
		{"unknown_arch", "", ErrUnmappedArchitecture},
	}

	for _, tt := range tests {
		got, err := MapAWSArchitecture(tt.rawArch)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapAWSArchitecture(%q) error = %v, want %v", tt.rawArch, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantArch {
				t.Errorf("MapAWSArchitecture(%q) = (%q, %v), want (%q, nil)", tt.rawArch, got, err, tt.wantArch)
			}
		}
	}
}

func TestMapAzureArchitecture(t *testing.T) {
	tests := []struct {
		rawArch  string
		wantArch string
		wantErr  error
	}{
		{"x86_64", ArchX86_64, nil},
		{"Standard Total Executions", ArchX86_64, nil},
		{"Standard Execution Time", ArchX86_64, nil},
		{"Consumption", ArchX86_64, nil},
		{"On Demand Total Executions", ArchX86_64, nil},
		{"arm64", "", ErrUnmappedArchitecture},
		{"arm", "", ErrUnmappedArchitecture},
		{"unknown_arch", "", ErrUnmappedArchitecture},
	}

	for _, tt := range tests {
		got, err := MapAzureArchitecture(tt.rawArch)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapAzureArchitecture(%q) error = %v, want %v", tt.rawArch, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantArch {
				t.Errorf("MapAzureArchitecture(%q) = (%q, %v), want (%q, nil)", tt.rawArch, got, err, tt.wantArch)
			}
		}
	}
}

func TestMapGCPArchitecture(t *testing.T) {
	tests := []struct {
		rawArch  string
		wantArch string
		wantErr  error
	}{
		{"x86_64", ArchX86_64, nil},
		{"Cloud Functions", ArchX86_64, nil},
		{"Cloud Run functions", ArchX86_64, nil},
		{"1st_gen", ArchX86_64, nil},
		{"2nd_gen", ArchX86_64, nil},
		{"Invocations", ArchX86_64, nil},
		{"Execution Time", ArchX86_64, nil},
		{"GB-Seconds", ArchX86_64, nil},
		{"arm64", "", ErrUnmappedArchitecture},
		{"arm", "", ErrUnmappedArchitecture},
		{"unknown_arch", "", ErrUnmappedArchitecture},
	}

	for _, tt := range tests {
		got, err := MapGCPArchitecture(tt.rawArch)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapGCPArchitecture(%q) error = %v, want %v", tt.rawArch, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantArch {
				t.Errorf("MapGCPArchitecture(%q) = (%q, %v), want (%q, nil)", tt.rawArch, got, err, tt.wantArch)
			}
		}
	}
}

func TestNormalizeArchitecture(t *testing.T) {
	got, err := NormalizeArchitecture("aws", "Request-ARM")
	if err != nil || got != ArchARM64 {
		t.Errorf("NormalizeArchitecture(aws, Request-ARM) = (%q, %v), want (%q, nil)", got, err, ArchARM64)
	}

	gotAzure, err := NormalizeArchitecture("azure", "Standard Total Executions")
	if err != nil || gotAzure != ArchX86_64 {
		t.Errorf("NormalizeArchitecture(azure, Standard Total Executions) = (%q, %v), want (%q, nil)", gotAzure, err, ArchX86_64)
	}

	gotGCP, err := NormalizeArchitecture("gcp", "Cloud Functions")
	if err != nil || gotGCP != ArchX86_64 {
		t.Errorf("NormalizeArchitecture(gcp, Cloud Functions) = (%q, %v), want (%q, nil)", gotGCP, err, ArchX86_64)
	}

	_, err = NormalizeArchitecture("unmapped_provider", "x86_64")
	if err == nil {
		t.Errorf("NormalizeArchitecture(unmapped_provider, x86_64) expected error, got nil")
	}
}

func TestResolveCanonicalArchitecture(t *testing.T) {
	tests := []struct {
		input       string
		wantArch    string
		wantStageOk bool
		wantErr     bool
	}{
		{"x86_64", ArchX86_64, true, false},
		{"arm64", ArchARM64, true, false},
		{"x86", ArchX86_64, true, false},
		{"arm", ArchARM64, true, false},
		{"graviton2", ArchARM64, true, false},
		{"amd64", ArchX86_64, true, false},
		{"AWS-Lambda-Requests-ARM", ArchARM64, true, false},
		{"Standard Total Executions", ArchX86_64, true, false},
		{"Cloud Functions", ArchX86_64, true, false},
		{"", "", false, false},
		{"quantum_arch_invalid", "", false, true},
	}

	for _, tt := range tests {
		got, err := ResolveCanonicalArchitecture(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ResolveCanonicalArchitecture(%q) expected error, got nil", tt.input)
			}
		} else {
			if err != nil || got != tt.wantArch {
				t.Errorf("ResolveCanonicalArchitecture(%q) = (%q, %v), want (%q, nil)", tt.input, got, err, tt.wantArch)
			}
			if tt.input != "" {
				stageOk := IsSupportedStageArchitecture(got)
				if stageOk != tt.wantStageOk {
					t.Errorf("IsSupportedStageArchitecture(%q) = %v, want %v", got, stageOk, tt.wantStageOk)
				}
			}
		}
	}
}

func TestSupportedCanonicalArchitectures(t *testing.T) {
	archs := SupportedCanonicalArchitectures()
	if len(archs) != 2 {
		t.Fatalf("expected 2 supported canonical architectures, got %d", len(archs))
	}
	expected := map[string]bool{
		ArchX86_64: true,
		ArchARM64:  true,
	}
	for _, arch := range archs {
		if !expected[arch] {
			t.Errorf("unexpected supported canonical architecture: %s", arch)
		}
	}
}
