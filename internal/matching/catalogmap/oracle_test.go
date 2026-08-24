package catalogmap

import (
	"errors"
	"testing"
)

func TestMapOracleProduct(t *testing.T) {
	tests := []struct {
		name         string
		productCode  string
		wantCategory string
		wantErr      error
	}{
		{
			name:         "valid Compute - Virtual Machine product",
			productCode:  "Compute - Virtual Machine",
			wantCategory: "compute",
			wantErr:      nil,
		},
		{
			name:         "valid Compute category",
			productCode:  "Compute",
			wantCategory: "compute",
			wantErr:      nil,
		},
		{
			name:         "valid Virtual Machine category",
			productCode:  "Virtual Machine",
			wantCategory: "compute",
			wantErr:      nil,
		},
		{
			name:         "valid Storage - Block Volume product",
			productCode:  "Storage - Block Volume",
			wantCategory: "storage",
			wantErr:      nil,
		},
		{
			name:         "valid Storage - Object Storage product",
			productCode:  "Storage - Object Storage",
			wantCategory: "storage",
			wantErr:      nil,
		},
		{
			name:         "valid Networking - Virtual Cloud Network product",
			productCode:  "Networking - Virtual Cloud Network",
			wantCategory: "network",
			wantErr:      nil,
		},
		{
			name:         "valid Database product",
			productCode:  "Database",
			wantCategory: "database_rdbms",
			wantErr:      nil,
		},
		{
			name:         "valid Database - Cloud Service product",
			productCode:  "Database - Cloud Service",
			wantCategory: "database_rdbms",
			wantErr:      nil,
		},
		{
			name:         "valid Container Engine for Kubernetes product",
			productCode:  "Container Engine for Kubernetes",
			wantCategory: "kubernetes",
			wantErr:      nil,
		},
		{
			name:         "valid Functions product",
			productCode:  "Functions",
			wantCategory: "serverless",
			wantErr:      nil,
		},
		{
			name:         "unmapped product fails loudly",
			productCode:  "OracleAIUnknownService",
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
			gotCategory, err := MapOracleProduct(tt.productCode)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("MapOracleProduct(%q) expected error %v, got nil", tt.productCode, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("MapOracleProduct(%q) error = %v, wantErr %v", tt.productCode, err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("MapOracleProduct(%q) unexpected error: %v", tt.productCode, err)
				}
				if gotCategory != tt.wantCategory {
					t.Errorf("MapOracleProduct(%q) = %q, want %q", tt.productCode, gotCategory, tt.wantCategory)
				}
			}
		})
	}
}

func TestKnownOracleProducts(t *testing.T) {
	known := KnownOracleProducts()
	if len(known) == 0 {
		t.Fatal("KnownOracleProducts() returned empty map")
	}
	if known["Compute - Virtual Machine"] != "compute" {
		t.Errorf("expected Compute - Virtual Machine -> compute, got %q", known["Compute - Virtual Machine"])
	}
}
