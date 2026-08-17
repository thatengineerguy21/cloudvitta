package regionmap

import (
	"errors"
	"testing"
)

func TestMapAWSRegion(t *testing.T) {
	tests := []struct {
		name      string
		region    string
		wantGroup string
		wantErr   error
	}{
		{
			name:      "valid region code us-east-1",
			region:    "us-east-1",
			wantGroup: "us-east",
			wantErr:   nil,
		},
		{
			name:      "valid location US East (N. Virginia)",
			region:    "US East (N. Virginia)",
			wantGroup: "us-east",
			wantErr:   nil,
		},
		{
			name:      "valid region code us-west-2",
			region:    "us-west-2",
			wantGroup: "us-west",
			wantErr:   nil,
		},
		{
			name:      "valid location Nigeria (Lagos)",
			region:    "Nigeria (Lagos)",
			wantGroup: "af-south",
			wantErr:   nil,
		},
		{
			name:      "valid location External",
			region:    "External",
			wantGroup: "global",
			wantErr:   nil,
		},
		{
			name:      "valid location AWS GovCloud (US-West)",
			region:    "AWS GovCloud (US-West)",
			wantGroup: "us-west",
			wantErr:   nil,
		},
		{
			name:      "valid location Asia Pacific (New Zealand)",
			region:    "Asia Pacific (New Zealand)",
			wantGroup: "ap-southeast",
			wantErr:   nil,
		},
		{
			name:      "valid location EU (Ireland)",
			region:    "EU (Ireland)",
			wantGroup: "eu-west",
			wantErr:   nil,
		},
		{
			name:      "valid location US East (New York City)",
			region:    "US East (New York City)",
			wantGroup: "us-east",
			wantErr:   nil,
		},
		{
			name:      "valid location Mexico (Central)",
			region:    "Mexico (Central)",
			wantGroup: "us-central",
			wantErr:   nil,
		},
		{
			name:      "valid location US East (Verizon) - Nashville",
			region:    "US East (Verizon) - Nashville",
			wantGroup: "us-east",
			wantErr:   nil,
		},
		{
			name:      "valid location Philippines (Manila)",
			region:    "Philippines (Manila)",
			wantGroup: "ap-southeast",
			wantErr:   nil,
		},
		{
			name:      "valid location Denmark (Copenhagen)",
			region:    "Denmark (Copenhagen)",
			wantGroup: "eu-north",
			wantErr:   nil,
		},
		{
			name:      "valid region code ap-east-2",
			region:    "ap-east-2",
			wantGroup: "ap-east",
			wantErr:   nil,
		},
		{
			name:      "valid location Europe (Vodafone) - Dortmund",
			region:    "Europe (Vodafone) - Dortmund",
			wantGroup: "eu-central",
			wantErr:   nil,
		},
		{
			name:      "valid location Amazon CloudFront",
			region:    "Amazon CloudFront",
			wantGroup: "global",
			wantErr:   nil,
		},
		{
			name:      "valid location Argentina (Buenos Aires)",
			region:    "Argentina (Buenos Aires)",
			wantGroup: "sa-east",
			wantErr:   nil,
		},
		{
			name:      "valid location Chile (Santiago)",
			region:    "Chile (Santiago)",
			wantGroup: "sa-east",
			wantErr:   nil,
		},
		{
			name:      "valid location Peru (Lima)",
			region:    "Peru (Lima)",
			wantGroup: "sa-east",
			wantErr:   nil,
		},
		{
			name:      "valid location Poland (Warsaw)",
			region:    "Poland (Warsaw)",
			wantGroup: "eu-central",
			wantErr:   nil,
		},
		{
			name:      "valid location Germany (Hamburg)",
			region:    "Germany (Hamburg)",
			wantGroup: "eu-central",
			wantErr:   nil,
		},
		{
			name:      "valid location Finland (Helsinki)",
			region:    "Finland (Helsinki)",
			wantGroup: "eu-north",
			wantErr:   nil,
		},
		{
			name:      "valid location Greece (Athens)",
			region:    "Greece (Athens)",
			wantGroup: "eu-south",
			wantErr:   nil,
		},
		{
			name:      "valid location Turkey (Istanbul)",
			region:    "Turkey (Istanbul)",
			wantGroup: "eu-south",
			wantErr:   nil,
		},
		{
			name:      "valid location US West (Honolulu)",
			region:    "US West (Honolulu)",
			wantGroup: "us-west",
			wantErr:   nil,
		},
		{
			name:      "valid location US East (Lenexa)",
			region:    "US East (Lenexa)",
			wantGroup: "us-central",
			wantErr:   nil,
		},
		{
			name:      "valid location Morocco (Casablanca)",
			region:    "Morocco (Casablanca)",
			wantGroup: "af-south",
			wantErr:   nil,
		},
		{
			name:      "valid location Senegal (Dakar)",
			region:    "Senegal (Dakar)",
			wantGroup: "af-south",
			wantErr:   nil,
		},
		{
			name:      "unmapped region code fails loudly",
			region:    "unknown-aws-region-99",
			wantGroup: "",
			wantErr:   ErrUnmappedRegion,
		},
		{
			name:      "empty region fails loudly",
			region:    "",
			wantGroup: "",
			wantErr:   ErrUnmappedRegion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotGroup, err := MapAWSRegion(tt.region)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("MapAWSRegion(%q) expected error %v, got nil", tt.region, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("MapAWSRegion(%q) error = %v, wantErr %v", tt.region, err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("MapAWSRegion(%q) unexpected error: %v", tt.region, err)
				}
				if gotGroup != tt.wantGroup {
					t.Errorf("MapAWSRegion(%q) = %q, want %q", tt.region, gotGroup, tt.wantGroup)
				}
			}
		})
	}
}
