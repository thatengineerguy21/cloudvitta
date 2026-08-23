package catalogmap

import (
	"errors"
	"testing"
)

func TestMapAzureProduct(t *testing.T) {
	tests := []struct {
		name         string
		serviceName  string
		wantCategory string
		wantErr      error
	}{
		{
			name:         "valid Virtual Machines service",
			serviceName:  "Virtual Machines",
			wantCategory: "compute",
			wantErr:      nil,
		},
		{
			name:         "valid Storage service",
			serviceName:  "Storage",
			wantCategory: "storage",
			wantErr:      nil,
		},
		{
			name:         "valid Bandwidth service",
			serviceName:  "Bandwidth",
			wantCategory: "network",
			wantErr:      nil,
		},
		{
			name:         "valid SQL Database service",
			serviceName:  "SQL Database",
			wantCategory: "database_rdbms",
			wantErr:      nil,
		},
		{
			name:         "unmapped service fails loudly",
			serviceName:  "Cosmos DB",
			wantCategory: "",
			wantErr:      ErrUnmappedProduct,
		},
		{
			name:         "empty service name fails loudly",
			serviceName:  "",
			wantCategory: "",
			wantErr:      ErrUnmappedProduct,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCategory, err := MapAzureProduct(tt.serviceName)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("MapAzureProduct(%q) expected error %v, got nil", tt.serviceName, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("MapAzureProduct(%q) error = %v, wantErr %v", tt.serviceName, err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("MapAzureProduct(%q) unexpected error: %v", tt.serviceName, err)
				}
				if gotCategory != tt.wantCategory {
					t.Errorf("MapAzureProduct(%q) = %q, want %q", tt.serviceName, gotCategory, tt.wantCategory)
				}
			}
		})
	}
}
