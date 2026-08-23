package nosqldatamodelmap

import (
	"errors"
	"testing"
)

func TestMapAWSDataModel(t *testing.T) {
	tests := []struct {
		rawModel  string
		wantModel string
		wantErr   error
	}{
		{"AmazonDynamoDB", "document", nil},
		{"DynamoDB", "document", nil},
		{"document", "document", nil},
		{"doc", "document", nil},
		{"key_value", "key_value", nil},
		{"key-value", "key_value", nil},
		{"kv", "key_value", nil},
		{"wide_column", "wide_column", nil},
		{"graph", "graph", nil},
		{"multi_model", "multi_model", nil},
		{"unknown_model", "", ErrUnmappedDataModel},
	}

	for _, tt := range tests {
		got, err := MapAWSDataModel(tt.rawModel)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapAWSDataModel(%q) error = %v, want %v", tt.rawModel, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantModel {
				t.Errorf("MapAWSDataModel(%q) = (%q, %v), want (%q, nil)", tt.rawModel, got, err, tt.wantModel)
			}
		}
	}
}

func TestMapAzureDataModel(t *testing.T) {
	tests := []struct {
		rawModel  string
		wantModel string
		wantErr   error
	}{
		{"Azure Cosmos DB", "document", nil},
		{"Cosmos DB", "document", nil},
		{"SQL", "document", nil},
		{"NoSQL", "document", nil},
		{"Core", "document", nil},
		{"MongoDB", "document", nil},
		{"Table", "key_value", nil},
		{"Cassandra", "wide_column", nil},
		{"Gremlin", "graph", nil},
		{"multi_model", "multi_model", nil},
		{"unknown_model", "", ErrUnmappedDataModel},
	}

	for _, tt := range tests {
		got, err := MapAzureDataModel(tt.rawModel)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapAzureDataModel(%q) error = %v, want %v", tt.rawModel, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantModel {
				t.Errorf("MapAzureDataModel(%q) = (%q, %v), want (%q, nil)", tt.rawModel, got, err, tt.wantModel)
			}
		}
	}
}

func TestMapGCPDataModel(t *testing.T) {
	tests := []struct {
		rawModel  string
		wantModel string
		wantErr   error
	}{
		{"Cloud Firestore", "document", nil},
		{"Firestore", "document", nil},
		{"Cloud Datastore", "document", nil},
		{"Datastore", "document", nil},
		{"Bigtable", "wide_column", nil},
		{"key_value", "key_value", nil},
		{"graph", "graph", nil},
		{"multi_model", "multi_model", nil},
		{"unknown_model", "", ErrUnmappedDataModel},
	}

	for _, tt := range tests {
		got, err := MapGCPDataModel(tt.rawModel)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapGCPDataModel(%q) error = %v, want %v", tt.rawModel, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantModel {
				t.Errorf("MapGCPDataModel(%q) = (%q, %v), want (%q, nil)", tt.rawModel, got, err, tt.wantModel)
			}
		}
	}
}

func TestNormalizeDataModel(t *testing.T) {
	got, err := NormalizeDataModel("aws", "DynamoDB")
	if err != nil || got != "document" {
		t.Errorf("NormalizeDataModel(aws, DynamoDB) = (%q, %v), want (document, nil)", got, err)
	}

	gotAzure, err := NormalizeDataModel("azure", "MongoDB")
	if err != nil || gotAzure != "document" {
		t.Errorf("NormalizeDataModel(azure, MongoDB) = (%q, %v), want (document, nil)", gotAzure, err)
	}

	gotGCP, err := NormalizeDataModel("gcp", "Cloud Firestore")
	if err != nil || gotGCP != "document" {
		t.Errorf("NormalizeDataModel(gcp, Cloud Firestore) = (%q, %v), want (document, nil)", gotGCP, err)
	}

	_, err = NormalizeDataModel("unmapped_provider", "document")
	if err == nil {
		t.Errorf("NormalizeDataModel(unmapped_provider, document) expected error, got nil")
	}
}

func TestResolveCanonicalDataModel(t *testing.T) {
	tests := []struct {
		input       string
		wantModel   string
		wantStageOk bool
		wantErr     bool
	}{
		{"document", "document", true, false},
		{"doc", "document", true, false},
		{"json", "document", true, false},
		{"key_value", "key_value", true, false},
		{"kv", "key_value", true, false},
		{"wide_column", "wide_column", true, false},
		{"graph", "graph", true, false},
		{"multi_model", "multi_model", true, false},
		{"Cloud Firestore", "document", true, false},
		{"MongoDB", "document", true, false},
		{"Cassandra", "wide_column", true, false},
		{"Gremlin", "graph", true, false},
		{"", "", false, false},
		{"invalid_model_type", "", false, true},
	}

	for _, tt := range tests {
		got, err := ResolveCanonicalDataModel(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ResolveCanonicalDataModel(%q) expected error, got nil", tt.input)
			}
		} else {
			if err != nil || got != tt.wantModel {
				t.Errorf("ResolveCanonicalDataModel(%q) = (%q, %v), want (%q, nil)", tt.input, got, err, tt.wantModel)
			}
			if tt.input != "" {
				stageOk := IsSupportedStageDataModel(got)
				if stageOk != tt.wantStageOk {
					t.Errorf("IsSupportedStageDataModel(%q) = %v, want %v", got, stageOk, tt.wantStageOk)
				}
			}
		}
	}
}
