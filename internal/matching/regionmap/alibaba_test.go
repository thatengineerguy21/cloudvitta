package regionmap

import (
	"errors"
	"testing"
)

func TestMapAlibabaRegion(t *testing.T) {
	tests := []struct {
		name      string
		region    string
		wantGroup string
		wantErr   error
	}{
		{
			name:      "valid us-east-1",
			region:    "us-east-1",
			wantGroup: "us-east",
			wantErr:   nil,
		},
		{
			name:      "valid US (Virginia)",
			region:    "US (Virginia)",
			wantGroup: "us-east",
			wantErr:   nil,
		},
		{
			name:      "valid us-west-1",
			region:    "us-west-1",
			wantGroup: "us-west",
			wantErr:   nil,
		},
		{
			name:      "valid eu-central-1",
			region:    "eu-central-1",
			wantGroup: "eu-central",
			wantErr:   nil,
		},
		{
			name:      "valid Germany (Frankfurt)",
			region:    "Germany (Frankfurt)",
			wantGroup: "eu-central",
			wantErr:   nil,
		},
		{
			name:      "valid eu-west-1",
			region:    "eu-west-1",
			wantGroup: "uk-south",
			wantErr:   nil,
		},
		{
			name:      "valid ap-southeast-1",
			region:    "ap-southeast-1",
			wantGroup: "ap-southeast",
			wantErr:   nil,
		},
		{
			name:      "valid ap-southeast-2",
			region:    "ap-southeast-2",
			wantGroup: "ap-southeast",
			wantErr:   nil,
		},
		{
			name:      "valid ap-northeast-1",
			region:    "ap-northeast-1",
			wantGroup: "ap-northeast",
			wantErr:   nil,
		},
		{
			name:      "valid ap-south-1",
			region:    "ap-south-1",
			wantGroup: "ap-south",
			wantErr:   nil,
		},
		{
			name:      "valid me-east-1",
			region:    "me-east-1",
			wantGroup: "me-central",
			wantErr:   nil,
		},
		{
			name:      "valid cn-hangzhou",
			region:    "cn-hangzhou",
			wantGroup: "cn-east",
			wantErr:   nil,
		},
		{
			name:      "valid cn-shanghai",
			region:    "cn-shanghai",
			wantGroup: "cn-east",
			wantErr:   nil,
		},
		{
			name:      "valid cn-beijing",
			region:    "cn-beijing",
			wantGroup: "cn-north",
			wantErr:   nil,
		},
		{
			name:      "valid cn-shenzhen",
			region:    "cn-shenzhen",
			wantGroup: "cn-south",
			wantErr:   nil,
		},
		{
			name:      "valid cn-hongkong",
			region:    "cn-hongkong",
			wantGroup: "ap-east",
			wantErr:   nil,
		},
		{
			name:      "unmapped region fails loudly",
			region:    "moon-crater-1",
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
			gotGroup, err := MapAlibabaRegion(tt.region)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("MapAlibabaRegion(%q) expected error %v, got nil", tt.region, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("MapAlibabaRegion(%q) error = %v, wantErr %v", tt.region, err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("MapAlibabaRegion(%q) unexpected error: %v", tt.region, err)
				}
				if gotGroup != tt.wantGroup {
					t.Errorf("MapAlibabaRegion(%q) = %q, want %q", tt.region, gotGroup, tt.wantGroup)
				}
			}
		})
	}
}

func TestResolveAlibabaNativeRegion(t *testing.T) {
	tests := []struct {
		group   string
		want    string
		wantErr bool
	}{
		{"us-east", "us-east-1", false},
		{"us-west", "us-west-1", false},
		{"eu-central", "eu-central-1", false},
		{"uk-south", "eu-west-1", false},
		{"ap-southeast", "ap-southeast-1", false},
		{"ap-northeast", "ap-northeast-1", false},
		{"ap-south", "ap-south-1", false},
		{"me-central", "me-east-1", false},
		{"cn-east", "cn-hangzhou", false},
		{"cn-north", "cn-beijing", false},
		{"cn-south", "cn-shenzhen", false},
		{"ap-east", "cn-hongkong", false},
		{"invalid-group", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.group, func(t *testing.T) {
			got, err := ResolveAlibabaNativeRegion(tt.group)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveAlibabaNativeRegion(%q) expected error, got nil", tt.group)
				}
			} else {
				if err != nil {
					t.Errorf("ResolveAlibabaNativeRegion(%q) unexpected error: %v", tt.group, err)
				}
				if got != tt.want {
					t.Errorf("ResolveAlibabaNativeRegion(%q) = %q, want %q", tt.group, got, tt.want)
				}
			}
		})
	}
}

func TestKnownAlibabaRegions(t *testing.T) {
	known := KnownAlibabaRegions()
	if len(known) == 0 {
		t.Fatal("KnownAlibabaRegions() returned empty map")
	}
	if known["us-east-1"] != "us-east" {
		t.Errorf("expected us-east-1 -> us-east, got %q", known["us-east-1"])
	}
}
