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
