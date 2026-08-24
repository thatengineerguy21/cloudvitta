package storageclassmap

import (
	"errors"
	"testing"
)

func TestMapAWSStorageClass(t *testing.T) {
	tests := []struct {
		rawClass  string
		wantClass string
		wantErr   error
	}{
		{"Standard", "standard", nil},
		{"General Purpose", "standard", nil},
		{"Standard - Infrequent Access", "infrequent_access", nil},
		{"Infrequent Access", "infrequent_access", nil},
		{"Archive Instant Retrieval", "archive", nil},
		{"Glacier Deep Archive", "archive", nil},
		{"gp3", "standard", nil},
		{"io1", "standard", nil},
		{"Express One Zone", "standard", nil},
		{"High Performance", "standard", nil},
		{"Non-Critical Data", "standard", nil},
		{"Tags", "standard", nil},
		{"Analytics", "standard", nil},
		{"Files", "standard", nil},
		{"Vectors", "standard", nil},
		{"UnknownClass", "", ErrUnmappedStorageClass},
	}

	for _, tt := range tests {
		got, err := MapAWSStorageClass(tt.rawClass)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapAWSStorageClass(%q) error = %v, want %v", tt.rawClass, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantClass {
				t.Errorf("MapAWSStorageClass(%q) = (%q, %v), want (%q, nil)", tt.rawClass, got, err, tt.wantClass)
			}
		}
	}
}

func TestMapAzureStorageClass(t *testing.T) {
	tests := []struct {
		rawClass  string
		wantClass string
		wantErr   error
	}{
		{"Hot", "standard", nil},
		{"Standard", "standard", nil},
		{"Premium", "standard", nil},
		{"Premium LRS", "standard", nil},
		{"Premium ZRS", "standard", nil},
		{"SSD ZRS", "standard", nil},
		{"SSD LRS", "standard", nil},
		{"Blob", "standard", nil},
		{"Account Encrypted GZRS", "standard", nil},
		{"Cool", "infrequent_access", nil},
		{"Archive", "archive", nil},
		{"UnknownClass", "", ErrUnmappedStorageClass},
	}

	for _, tt := range tests {
		got, err := MapAzureStorageClass(tt.rawClass)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapAzureStorageClass(%q) error = %v, want %v", tt.rawClass, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantClass {
				t.Errorf("MapAzureStorageClass(%q) = (%q, %v), want (%q, nil)", tt.rawClass, got, err, tt.wantClass)
			}
		}
	}
}

func TestMapGCPStorageClass(t *testing.T) {
	tests := []struct {
		rawClass  string
		wantClass string
		wantErr   error
	}{
		{"Standard", "standard", nil},
		{"Regional", "standard", nil},
		{"Multi-Regional", "standard", nil},
		{"Dual-Region", "standard", nil},
		{"Nearline", "infrequent_access", nil},
		{"Coldline", "archive", nil},
		{"Archive", "archive", nil},
		{"DRAStorage", "infrequent_access", nil},
		{"DRA", "infrequent_access", nil},
		{"UnknownClass", "", ErrUnmappedStorageClass},
	}

	for _, tt := range tests {
		got, err := MapGCPStorageClass(tt.rawClass)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapGCPStorageClass(%q) error = %v, want %v", tt.rawClass, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantClass {
				t.Errorf("MapGCPStorageClass(%q) = (%q, %v), want (%q, nil)", tt.rawClass, got, err, tt.wantClass)
			}
		}
	}
}

func TestMapOracleStorageClass(t *testing.T) {
	tests := []struct {
		rawClass  string
		wantClass string
		wantErr   error
	}{
		{"standard", "standard", nil},
		{"Standard", "standard", nil},
		{"Object Storage Standard", "standard", nil},
		{"Object Storage - Storage", "standard", nil},
		{"infrequent_access", "infrequent_access", nil},
		{"Infrequent Access", "infrequent_access", nil},
		{"Object Storage - Infrequent Access Storage", "infrequent_access", nil},
		{"archive", "archive", nil},
		{"Archive", "archive", nil},
		{"Archive Storage", "archive", nil},
		{"Object Storage - Archive Storage", "archive", nil},
		{"Block Volume", "standard", nil},
		{"Block Volume - Storage", "standard", nil},
		{"Block Volume - Balanced", "standard", nil},
		{"UnknownClass", "", ErrUnmappedStorageClass},
	}

	for _, tt := range tests {
		got, err := MapOracleStorageClass(tt.rawClass)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapOracleStorageClass(%q) error = %v, want %v", tt.rawClass, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantClass {
				t.Errorf("MapOracleStorageClass(%q) = (%q, %v), want (%q, nil)", tt.rawClass, got, err, tt.wantClass)
			}
		}
	}
}

func TestMapIBMStorageClass(t *testing.T) {
	tests := []struct {
		rawClass  string
		wantClass string
		wantErr   error
	}{
		{"standard", "standard", nil},
		{"Standard", "standard", nil},
		{"COS Standard", "standard", nil},
		{"standard-storage", "standard", nil},
		{"Smart Tier", "standard", nil},
		{"vault", "infrequent_access", nil},
		{"Vault", "infrequent_access", nil},
		{"COS Vault", "infrequent_access", nil},
		{"cold_vault", "archive", nil},
		{"Cold Vault", "archive", nil},
		{"COS Cold Vault", "archive", nil},
		{"is.volume", "standard", nil},
		{"general-purpose", "standard", nil},
		{"tier-5iops-storage", "standard", nil},
		{"UnknownClass", "", ErrUnmappedStorageClass},
	}

	for _, tt := range tests {
		got, err := MapIBMStorageClass(tt.rawClass)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapIBMStorageClass(%q) error = %v, want %v", tt.rawClass, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantClass {
				t.Errorf("MapIBMStorageClass(%q) = (%q, %v), want (%q, nil)", tt.rawClass, got, err, tt.wantClass)
			}
		}
	}
}

func TestMapAlibabaStorageClass(t *testing.T) {
	tests := []struct {
		rawClass  string
		wantClass string
		wantErr   error
	}{
		{"standard", "standard", nil},
		{"Standard", "standard", nil},
		{"oss-standard", "standard", nil},
		{"Standard (LRS)", "standard", nil},
		{"ia", "infrequent_access", nil},
		{"IA", "infrequent_access", nil},
		{"oss-ia", "infrequent_access", nil},
		{"Infrequent Access", "infrequent_access", nil},
		{"archive", "archive", nil},
		{"Archive", "archive", nil},
		{"oss-archive", "archive", nil},
		{"Cold Archive", "archive", nil},
		{"cloud_essd", "standard", nil},
		{"cloud_ssd", "standard", nil},
		{"UnknownClass", "", ErrUnmappedStorageClass},
	}

	for _, tt := range tests {
		got, err := MapAlibabaStorageClass(tt.rawClass)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapAlibabaStorageClass(%q) error = %v, want %v", tt.rawClass, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantClass {
				t.Errorf("MapAlibabaStorageClass(%q) = (%q, %v), want (%q, nil)", tt.rawClass, got, err, tt.wantClass)
			}
		}
	}
}

func TestMapDigitalOceanStorageClass(t *testing.T) {
	tests := []struct {
		rawClass  string
		wantClass string
		wantErr   error
	}{
		{"spaces", "standard", nil},
		{"Spaces", "standard", nil},
		{"Spaces Object Storage", "standard", nil},
		{"volume", "standard", nil},
		{"volumes", "standard", nil},
		{"Block Storage", "standard", nil},
		{"UnknownClass", "", ErrUnmappedStorageClass},
	}

	for _, tt := range tests {
		got, err := MapDigitalOceanStorageClass(tt.rawClass)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapDigitalOceanStorageClass(%q) error = %v, want %v", tt.rawClass, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantClass {
				t.Errorf("MapDigitalOceanStorageClass(%q) = (%q, %v), want (%q, nil)", tt.rawClass, got, err, tt.wantClass)
			}
		}
	}
}

func TestMapStorageClass(t *testing.T) {
	providers := []struct {
		provider  string
		rawClass  string
		wantClass string
	}{
		{"aws", "Standard", "standard"},
		{"azure", "Hot", "standard"},
		{"gcp", "Standard", "standard"},
		{"oracle", "Object Storage Standard", "standard"},
		{"ibm", "COS Standard", "standard"},
		{"alibaba", "oss-standard", "standard"},
		{"digitalocean", "spaces", "standard"},
	}

	for _, tc := range providers {
		got, err := MapStorageClass(tc.provider, tc.rawClass)
		if err != nil || got != tc.wantClass {
			t.Errorf("MapStorageClass(%s, %s) = (%q, %v), want (%q, nil)", tc.provider, tc.rawClass, got, err, tc.wantClass)
		}
	}

	_, err := MapStorageClass("unmapped_provider", "Standard")
	if err == nil {
		t.Errorf("MapStorageClass(unmapped_provider, Standard) expected error, got nil")
	}
}
