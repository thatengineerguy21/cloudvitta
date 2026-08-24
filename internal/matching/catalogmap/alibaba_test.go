package catalogmap

import (
	"errors"
	"testing"
)

func TestMapAlibabaProduct(t *testing.T) {
	tests := []struct {
		name         string
		productCode  string
		wantCategory string
		wantErr      error
	}{
		{
			name:         "valid ecs product",
			productCode:  "ecs",
			wantCategory: "compute",
			wantErr:      nil,
		},
		{
			name:         "valid ecs.instance product",
			productCode:  "ecs.instance",
			wantCategory: "compute",
			wantErr:      nil,
		},
		{
			name:         "valid Elastic Compute Service product",
			productCode:  "Elastic Compute Service",
			wantCategory: "compute",
			wantErr:      nil,
		},
		{
			name:         "valid oss product",
			productCode:  "oss",
			wantCategory: "storage",
			wantErr:      nil,
		},
		{
			name:         "valid disk product",
			productCode:  "disk",
			wantCategory: "storage",
			wantErr:      nil,
		},
		{
			name:         "valid ebs product",
			productCode:  "ebs",
			wantCategory: "storage",
			wantErr:      nil,
		},
		{
			name:         "valid data_transfer product",
			productCode:  "data_transfer",
			wantCategory: "network",
			wantErr:      nil,
		},
		{
			name:         "valid eip product",
			productCode:  "eip",
			wantCategory: "network",
			wantErr:      nil,
		},
		{
			name:         "valid rds product",
			productCode:  "rds",
			wantCategory: "database_rdbms",
			wantErr:      nil,
		},
		{
			name:         "valid polardb product",
			productCode:  "polardb",
			wantCategory: "database_rdbms",
			wantErr:      nil,
		},
		{
			name:         "valid lindorm product",
			productCode:  "lindorm",
			wantCategory: "database_nosql",
			wantErr:      nil,
		},
		{
			name:         "valid mongodb product",
			productCode:  "mongodb",
			wantCategory: "database_nosql",
			wantErr:      nil,
		},
		{
			name:         "valid ack product",
			productCode:  "ack",
			wantCategory: "kubernetes",
			wantErr:      nil,
		},
		{
			name:         "valid fc product",
			productCode:  "fc",
			wantCategory: "serverless",
			wantErr:      nil,
		},
		{
			name:         "unmapped product fails loudly",
			productCode:  "AlibabaAIUnknownService",
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
			gotCategory, err := MapAlibabaProduct(tt.productCode)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("MapAlibabaProduct(%q) expected error %v, got nil", tt.productCode, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("MapAlibabaProduct(%q) error = %v, wantErr %v", tt.productCode, err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("MapAlibabaProduct(%q) unexpected error: %v", tt.productCode, err)
				}
				if gotCategory != tt.wantCategory {
					t.Errorf("MapAlibabaProduct(%q) = %q, want %q", tt.productCode, gotCategory, tt.wantCategory)
				}
			}
		})
	}
}

func TestKnownAlibabaProducts(t *testing.T) {
	known := KnownAlibabaProducts()
	if len(known) == 0 {
		t.Fatal("KnownAlibabaProducts() returned empty map")
	}
	if known["ecs"] != "compute" {
		t.Errorf("expected ecs -> compute, got %q", known["ecs"])
	}
	if known["oss"] != "storage" {
		t.Errorf("expected oss -> storage, got %q", known["oss"])
	}
}
