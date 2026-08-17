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
		{"Glacier Deep Archive", "archive", nil},
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

func TestMapStorageClass(t *testing.T) {
	got, err := MapStorageClass("aws", "Standard")
	if err != nil || got != "standard" {
		t.Errorf("MapStorageClass(aws, Standard) = (%q, %v), want (standard, nil)", got, err)
	}

	_, err = MapStorageClass("unmapped_provider", "Standard")
	if err == nil {
		t.Errorf("MapStorageClass(unmapped_provider, Standard) expected error, got nil")
	}
}
