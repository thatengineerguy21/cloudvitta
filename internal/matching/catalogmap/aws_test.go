package catalogmap

import (
	"errors"
	"testing"
)

func TestMapAWSProduct(t *testing.T) {
	tests := []struct {
		name         string
		productCode  string
		wantCategory string
		wantErr      error
	}{
		{
			name:         "valid AmazonEC2 product",
			productCode:  "AmazonEC2",
			wantCategory: "compute",
			wantErr:      nil,
		},
		{
			name:         "unmapped product fails loudly",
			productCode:  "AmazonS3",
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
			gotCategory, err := MapAWSProduct(tt.productCode)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("MapAWSProduct(%q) expected error %v, got nil", tt.productCode, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("MapAWSProduct(%q) error = %v, wantErr %v", tt.productCode, err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("MapAWSProduct(%q) unexpected error: %v", tt.productCode, err)
				}
				if gotCategory != tt.wantCategory {
					t.Errorf("MapAWSProduct(%q) = %q, want %q", tt.productCode, gotCategory, tt.wantCategory)
				}
			}
		})
	}
}
