package catalogmap

import (
	"errors"
	"testing"
)

func TestMapGCPProduct(t *testing.T) {
	tests := []struct {
		name         string
		serviceName  string
		wantCategory string
		wantErr      error
	}{
		{
			name:         "valid Compute Engine service display name",
			serviceName:  "Compute Engine",
			wantCategory: "compute",
			wantErr:      nil,
		},
		{
			name:         "valid Compute Engine service ID",
			serviceName:  "6F81-5844-456A",
			wantCategory: "compute",
			wantErr:      nil,
		},
		{
			name:         "valid Cloud Storage service display name",
			serviceName:  "Cloud Storage",
			wantCategory: "storage",
			wantErr:      nil,
		},
		{
			name:         "valid Cloud Storage service ID",
			serviceName:  "95FF-2EF5-5EA1",
			wantCategory: "storage",
			wantErr:      nil,
		},
		{
			name:         "valid Cloud Interconnect service display name",
			serviceName:  "Cloud Interconnect",
			wantCategory: "network",
			wantErr:      nil,
		},
		{
			name:         "valid Networking service display name",
			serviceName:  "Networking",
			wantCategory: "network",
			wantErr:      nil,
		},
		{
			name:         "valid Network service ID",
			serviceName:  "E89B-A08C-8A2D",
			wantCategory: "network",
			wantErr:      nil,
		},
		{
			name:         "valid Cloud SQL service display name",
			serviceName:  "Cloud SQL",
			wantCategory: "database_rdbms",
			wantErr:      nil,
		},
		{
			name:         "valid Cloud SQL service ID",
			serviceName:  "9662-B51E-5089",
			wantCategory: "database_rdbms",
			wantErr:      nil,
		},
		{
			name:         "valid AlloyDB service display name",
			serviceName:  "AlloyDB",
			wantCategory: "database_rdbms",
			wantErr:      nil,
		},
		{
			name:         "valid Cloud Firestore service display name",
			serviceName:  "Cloud Firestore",
			wantCategory: "database_nosql",
			wantErr:      nil,
		},
		{
			name:         "valid Cloud Datastore service display name",
			serviceName:  "Cloud Datastore",
			wantCategory: "database_nosql",
			wantErr:      nil,
		},
		{
			name:         "valid Kubernetes Engine service display name",
			serviceName:  "Kubernetes Engine",
			wantCategory: "kubernetes",
			wantErr:      nil,
		},
		{
			name:         "valid Kubernetes Engine service ID",
			serviceName:  "44CD-3C5E-2A4B",
			wantCategory: "kubernetes",
			wantErr:      nil,
		},
		{
			name:         "valid Cloud Functions service display name",
			serviceName:  "Cloud Functions",
			wantCategory: "serverless",
			wantErr:      nil,
		},
		{
			name:         "valid Cloud Run functions service display name",
			serviceName:  "Cloud Run functions",
			wantCategory: "serverless",
			wantErr:      nil,
		},
		{
			name:         "valid Cloud Functions service ID",
			serviceName:  "29E7-DA93-CA13",
			wantCategory: "serverless",
			wantErr:      nil,
		},
		{
			name:         "valid AlloyDB service ID",
			serviceName:  "C49F-B7F2-7416",
			wantCategory: "database_rdbms",
			wantErr:      nil,
		},
		{
			name:         "valid Cloud Bigtable service display name",
			serviceName:  "Cloud Bigtable",
			wantCategory: "database_nosql",
			wantErr:      nil,
		},
		{
			name:         "valid Cloud Bigtable service ID",
			serviceName:  "C802-861C-2155",
			wantCategory: "database_nosql",
			wantErr:      nil,
		},
		{
			name:         "valid Cloud Run service display name",
			serviceName:  "Cloud Run",
			wantCategory: "serverless",
			wantErr:      nil,
		},
		{
			name:         "valid Cloud Run service ID",
			serviceName:  "152E-C115-5142",
			wantCategory: "serverless",
			wantErr:      nil,
		},
		{
			name:         "unmapped service fails loudly",
			serviceName:  "Cloud Spanner",
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
			gotCategory, err := MapGCPProduct(tt.serviceName)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("MapGCPProduct(%q) expected error %v, got nil", tt.serviceName, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("MapGCPProduct(%q) error = %v, wantErr %v", tt.serviceName, err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("MapGCPProduct(%q) unexpected error: %v", tt.serviceName, err)
				}
				if gotCategory != tt.wantCategory {
					t.Errorf("MapGCPProduct(%q) = %q, want %q", tt.serviceName, gotCategory, tt.wantCategory)
				}
			}
		})
	}
}
