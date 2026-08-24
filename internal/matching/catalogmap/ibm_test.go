package catalogmap

import (
	"errors"
	"testing"
)

func TestMapIBMProduct(t *testing.T) {
	tests := []struct {
		name         string
		productCode  string
		wantCategory string
		wantErr      error
	}{
		{
			name:         "valid is.instance product",
			productCode:  "is.instance",
			wantCategory: "compute",
			wantErr:      nil,
		},
		{
			name:         "valid virtual-server-for-vpc product",
			productCode:  "virtual-server-for-vpc",
			wantCategory: "compute",
			wantErr:      nil,
		},
		{
			name:         "valid is.volume product",
			productCode:  "is.volume",
			wantCategory: "storage",
			wantErr:      nil,
		},
		{
			name:         "valid cloud-object-storage product",
			productCode:  "cloud-object-storage",
			wantCategory: "storage",
			wantErr:      nil,
		},
		{
			name:         "valid is.floating-ip product",
			productCode:  "is.floating-ip",
			wantCategory: "network",
			wantErr:      nil,
		},
		{
			name:         "valid is.public-gateway product",
			productCode:  "is.public-gateway",
			wantCategory: "network",
			wantErr:      nil,
		},
		{
			name:         "valid databases-for-postgresql product",
			productCode:  "databases-for-postgresql",
			wantCategory: "database_rdbms",
			wantErr:      nil,
		},
		{
			name:         "valid databases-for-mysql product",
			productCode:  "databases-for-mysql",
			wantCategory: "database_rdbms",
			wantErr:      nil,
		},
		{
			name:         "valid databases-for-mongodb product",
			productCode:  "databases-for-mongodb",
			wantCategory: "database_nosql",
			wantErr:      nil,
		},
		{
			name:         "valid containers-kubernetes product",
			productCode:  "containers-kubernetes",
			wantCategory: "kubernetes",
			wantErr:      nil,
		},
		{
			name:         "valid code-engine product",
			productCode:  "code-engine",
			wantCategory: "serverless",
			wantErr:      nil,
		},
		{
			name:         "unmapped product fails loudly",
			productCode:  "IBMAIUnknownService",
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
			gotCategory, err := MapIBMProduct(tt.productCode)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("MapIBMProduct(%q) expected error %v, got nil", tt.productCode, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("MapIBMProduct(%q) error = %v, wantErr %v", tt.productCode, err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("MapIBMProduct(%q) unexpected error: %v", tt.productCode, err)
				}
				if gotCategory != tt.wantCategory {
					t.Errorf("MapIBMProduct(%q) = %q, want %q", tt.productCode, gotCategory, tt.wantCategory)
				}
			}
		})
	}
}

func TestKnownIBMProducts(t *testing.T) {
	known := KnownIBMProducts()
	if len(known) == 0 {
		t.Fatal("KnownIBMProducts() returned empty map")
	}
	if known["is.instance"] != "compute" {
		t.Errorf("expected is.instance -> compute, got %q", known["is.instance"])
	}
}
