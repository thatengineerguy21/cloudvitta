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
		{"Direct Connect Data Transfer Out", "direct_connect_egress", nil},
		{"VPN Data Transfer Out", "vpn_egress", nil},
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
		{"ExpressRoute", "direct_connect_egress", nil},
		{"VPN Gateway", "vpn_egress", nil},
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
		{"Cloud Interconnect Egress", "direct_connect_egress", nil},
		{"Cloud VPN Egress", "vpn_egress", nil},
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

func TestMapOracleTransferType(t *testing.T) {
	tests := []struct {
		rawType  string
		wantType string
		wantErr  error
	}{
		{"Outbound Data Transfer", "internet_egress", nil},
		{"Outbound Data Transfer (Internet)", "internet_egress", nil},
		{"Data Transfer Out", "internet_egress", nil},
		{"Internet", "internet_egress", nil},
		{"Intra-Region Data Transfer", "intra_region", nil},
		{"Inter-Region Data Transfer", "inter_region", nil},
		{"UnknownTransferType", "", ErrUnmappedTransferType},
	}

	for _, tt := range tests {
		got, err := MapOracleTransferType(tt.rawType)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapOracleTransferType(%q) error = %v, want %v", tt.rawType, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantType {
				t.Errorf("MapOracleTransferType(%q) = (%q, %v), want (%q, nil)", tt.rawType, got, err, tt.wantType)
			}
		}
	}
}

func TestMapIBMTransferType(t *testing.T) {
	tests := []struct {
		rawType  string
		wantType string
		wantErr  error
	}{
		{"VPC Public Egress", "internet_egress", nil},
		{"Public Egress", "internet_egress", nil},
		{"public-egress", "internet_egress", nil},
		{"Public Gateway", "internet_egress", nil},
		{"Floating IP", "internet_egress", nil},
		{"Internet", "internet_egress", nil},
		{"Intra-Region Data Transfer", "intra_region", nil},
		{"Inter-Region Data Transfer", "inter_region", nil},
		{"UnknownTransferType", "", ErrUnmappedTransferType},
	}

	for _, tt := range tests {
		got, err := MapIBMTransferType(tt.rawType)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapIBMTransferType(%q) error = %v, want %v", tt.rawType, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantType {
				t.Errorf("MapIBMTransferType(%q) = (%q, %v), want (%q, nil)", tt.rawType, got, err, tt.wantType)
			}
		}
	}
}

func TestMapAlibabaTransferType(t *testing.T) {
	tests := []struct {
		rawType  string
		wantType string
		wantErr  error
	}{
		{"Pay-By-Traffic Internet Egress", "internet_egress", nil},
		{"Internet Data Transfer", "internet_egress", nil},
		{"Data Transfer Out", "internet_egress", nil},
		{"data-transfer-out", "internet_egress", nil},
		{"EIP", "internet_egress", nil},
		{"Elastic IP", "internet_egress", nil},
		{"Internet", "internet_egress", nil},
		{"Intra-Region Data Transfer", "intra_region", nil},
		{"Inter-Region Data Transfer", "inter_region", nil},
		{"UnknownTransferType", "", ErrUnmappedTransferType},
	}

	for _, tt := range tests {
		got, err := MapAlibabaTransferType(tt.rawType)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapAlibabaTransferType(%q) error = %v, want %v", tt.rawType, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantType {
				t.Errorf("MapAlibabaTransferType(%q) = (%q, %v), want (%q, nil)", tt.rawType, got, err, tt.wantType)
			}
		}
	}
}

func TestMapDigitalOceanTransferType(t *testing.T) {
	tests := []struct {
		rawType  string
		wantType string
		wantErr  error
	}{
		{"Bandwidth", "internet_egress", nil},
		{"bandwidth", "internet_egress", nil},
		{"Bandwidth Transfer", "internet_egress", nil},
		{"bandwidth-overage", "internet_egress", nil},
		{"Data Transfer Out", "internet_egress", nil},
		{"Internet", "internet_egress", nil},
		{"Intra-Region Data Transfer", "intra_region", nil},
		{"Inter-Region Data Transfer", "inter_region", nil},
		{"UnknownTransferType", "", ErrUnmappedTransferType},
	}

	for _, tt := range tests {
		got, err := MapDigitalOceanTransferType(tt.rawType)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapDigitalOceanTransferType(%q) error = %v, want %v", tt.rawType, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantType {
				t.Errorf("MapDigitalOceanTransferType(%q) = (%q, %v), want (%q, nil)", tt.rawType, got, err, tt.wantType)
			}
		}
	}
}

func TestMapTransferType(t *testing.T) {
	providers := []struct {
		provider string
		rawType  string
		wantType string
	}{
		{"aws", "AWS Data Transfer Out", "internet_egress"},
		{"azure", "Data Transfer Out", "internet_egress"},
		{"gcp", "Network Internet Egress", "internet_egress"},
		{"oracle", "Outbound Data Transfer", "internet_egress"},
		{"ibm", "VPC Public Egress", "internet_egress"},
		{"alibaba", "Pay-By-Traffic Internet Egress", "internet_egress"},
		{"digitalocean", "Bandwidth Transfer", "internet_egress"},
	}

	for _, tc := range providers {
		got, err := MapTransferType(tc.provider, tc.rawType)
		if err != nil || got != tc.wantType {
			t.Errorf("MapTransferType(%s, %s) = (%q, %v), want (%q, nil)", tc.provider, tc.rawType, got, err, tc.wantType)
		}
	}

	_, err := MapTransferType("unmapped_provider", "Internet")
	if err == nil {
		t.Errorf("MapTransferType(unmapped_provider, Internet) expected error, got nil")
	}
}

func TestSupportedTransferTypes(t *testing.T) {
	types := SupportedTransferTypes()
	if len(types) != 5 {
		t.Fatalf("expected 5 supported transfer types, got %d", len(types))
	}
	expected := map[string]bool{
		"internet_egress":       true,
		"inter_region":          true,
		"intra_region":          true,
		"direct_connect_egress": true,
		"vpn_egress":            true,
	}
	for _, tt := range types {
		if !expected[tt] {
			t.Errorf("unexpected transfer type in SupportedTransferTypes: %q", tt)
		}
		if !IsValidTransferType(tt) {
			t.Errorf("IsValidTransferType(%q) = false, want true", tt)
		}
	}
	if IsValidTransferType("unsupported_transfer_type") {
		t.Errorf("IsValidTransferType(unsupported_transfer_type) = true, want false")
	}
}
