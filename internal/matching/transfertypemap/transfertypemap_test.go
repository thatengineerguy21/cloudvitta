package transfertypemap

import (
	"errors"
	"testing"
)

func TestMapAWSTransferType(t *testing.T) {
	tests := []struct {
		rawType  string
		wantType string
		wantErr  error
	}{
		{"AWS Data Transfer Out", "internet_egress", nil},
		{"Data Transfer Out (Internet)", "internet_egress", nil},
		{"Data Transfer Internet (Out)", "internet_egress", nil},
		{"Data Transfer Out (Inter-Region)", "inter_region", nil},
		{"Data Transfer Out (Intra-Region)", "intra_region", nil},
		{"Direct Connect Data Transfer Out", "internet_egress", nil},
		{"CloudFront Data Transfer Out", "internet_egress", nil},
		{"UnknownTransferType", "", ErrUnmappedTransferType},
	}

	for _, tt := range tests {
		got, err := MapAWSTransferType(tt.rawType)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapAWSTransferType(%q) error = %v, want %v", tt.rawType, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantType {
				t.Errorf("MapAWSTransferType(%q) = (%q, %v), want (%q, nil)", tt.rawType, got, err, tt.wantType)
			}
		}
	}
}

func TestMapAzureTransferType(t *testing.T) {
	tests := []struct {
		rawType  string
		wantType string
		wantErr  error
	}{
		{"Bandwidth Data Transfer Out", "internet_egress", nil},
		{"Data Transfer Out", "internet_egress", nil},
		{"Rtn Preference: MGN", "internet_egress", nil},
		{"Routing Preference: Microsoft Global Network", "internet_egress", nil},
		{"Routing Preference: Transit / ISP", "internet_egress", nil},
		{"ExpressRoute", "internet_egress", nil},
		{"Global", "internet_egress", nil},
		{"Inter-Region", "inter_region", nil},
		{"Intra-Region", "intra_region", nil},
		{"UnknownTransferType", "", ErrUnmappedTransferType},
	}

	for _, tt := range tests {
		got, err := MapAzureTransferType(tt.rawType)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapAzureTransferType(%q) error = %v, want %v", tt.rawType, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantType {
				t.Errorf("MapAzureTransferType(%q) = (%q, %v), want (%q, nil)", tt.rawType, got, err, tt.wantType)
			}
		}
	}
}

func TestMapGCPTransferType(t *testing.T) {
	tests := []struct {
		rawType  string
		wantType string
		wantErr  error
	}{
		{"Network Internet Egress", "internet_egress", nil},
		{"Premium Tier Internet Egress", "internet_egress", nil},
		{"Cloud Interconnect Egress", "internet_egress", nil},
		{"Network Inter Region Egress", "inter_region", nil},
		{"Network Intra Region Egress", "intra_region", nil},
		{"UnknownTransferType", "", ErrUnmappedTransferType},
	}

	for _, tt := range tests {
		got, err := MapGCPTransferType(tt.rawType)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapGCPTransferType(%q) error = %v, want %v", tt.rawType, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantType {
				t.Errorf("MapGCPTransferType(%q) = (%q, %v), want (%q, nil)", tt.rawType, got, err, tt.wantType)
			}
		}
	}
}

func TestMapTransferType(t *testing.T) {
	got, err := MapTransferType("aws", "AWS Data Transfer Out")
	if err != nil || got != "internet_egress" {
		t.Errorf("MapTransferType(aws, AWS Data Transfer Out) = (%q, %v), want (internet_egress, nil)", got, err)
	}

	_, err = MapTransferType("unmapped_provider", "Internet")
	if err == nil {
		t.Errorf("MapTransferType(unmapped_provider, Internet) expected error, got nil")
	}
}
