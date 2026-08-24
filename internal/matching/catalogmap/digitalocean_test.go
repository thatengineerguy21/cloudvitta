package catalogmap

import (
	"errors"
	"testing"
)

func TestMapDigitalOceanProduct(t *testing.T) {
	tests := []struct {
		name         string
		productCode  string
		wantCategory string
		wantErr      error
	}{
		{
			name:         "valid droplet product",
			productCode:  "droplet",
			wantCategory: "compute",
			wantErr:      nil,
		},
		{
			name:         "valid droplets product",
			productCode:  "droplets",
			wantCategory: "compute",
			wantErr:      nil,
		},
		{
			name:         "valid Droplet product",
			productCode:  "Droplet",
			wantCategory: "compute",
			wantErr:      nil,
		},
		{
			name:         "valid spaces product",
			productCode:  "spaces",
			wantCategory: "storage",
			wantErr:      nil,
		},
		{
			name:         "valid volume product",
			productCode:  "volume",
			wantCategory: "storage",
			wantErr:      nil,
		},
		{
			name:         "valid bandwidth product",
			productCode:  "bandwidth",
			wantCategory: "network",
			wantErr:      nil,
		},
		{
			name:         "valid database product",
			productCode:  "database",
			wantCategory: "database_rdbms",
			wantErr:      nil,
		},
		{
			name:         "valid doks product",
			productCode:  "doks",
			wantCategory: "kubernetes",
			wantErr:      nil,
		},
		{
			name:         "valid functions product",
			productCode:  "functions",
			wantCategory: "serverless",
			wantErr:      nil,
		},
		{
			name:         "unmapped product fails loudly",
			productCode:  "DigitalOceanAIUnknownService",
			wantCategory: "",
			wantErr:      ErrUnmappedProduct,
		},
		{
			name:         "empty product code fails loudly",
			productCode:  "",
			wantCategory: "",
			wantErr:      ErrUnmappedProduct,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCategory, err := MapDigitalOceanProduct(tt.productCode)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("MapDigitalOceanProduct(%q) expected error %v, got nil", tt.productCode, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("MapDigitalOceanProduct(%q) error = %v, wantErr %v", tt.productCode, err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("MapDigitalOceanProduct(%q) unexpected error: %v", tt.productCode, err)
				}
				if gotCategory != tt.wantCategory {
					t.Errorf("MapDigitalOceanProduct(%q) = %q, want %q", tt.productCode, gotCategory, tt.wantCategory)
				}
			}
		})
	}
}

func TestKnownDigitalOceanProducts(t *testing.T) {
	known := KnownDigitalOceanProducts()
	if len(known) == 0 {
		t.Fatal("KnownDigitalOceanProducts() returned empty map")
	}
	if known["droplet"] != "compute" {
		t.Errorf("expected droplet -> compute, got %q", known["droplet"])
	}
	if known["spaces"] != "storage" {
		t.Errorf("expected spaces -> storage, got %q", known["spaces"])
	}
}
