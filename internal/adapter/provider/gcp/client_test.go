package gcp_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/gcp"
)

func TestGCPClient_DefaultEndpoints(t *testing.T) {
	tests := []struct {
		name          string
		serviceID     string
		wantServiceID string
		catalogURL    string
		wantURL       string
	}{
		{
			name:          "Compute endpoint",
			serviceID:     gcp.DefaultComputeServiceID,
			wantServiceID: "6F81-5844-456A",
			catalogURL:    gcp.DefaultBillingCatalogURL,
			wantURL:       "https://cloudbilling.googleapis.com/v1/services/6F81-5844-456A/skus",
		},
		{
			name:          "Storage endpoint",
			serviceID:     gcp.DefaultStorageServiceID,
			wantServiceID: "95FF-2EF5-5EA1",
			catalogURL:    gcp.DefaultStorageBillingCatalogURL,
			wantURL:       "https://cloudbilling.googleapis.com/v1/services/95FF-2EF5-5EA1/skus",
		},
		{
			name:          "Network endpoint",
			serviceID:     gcp.DefaultNetworkServiceID,
			wantServiceID: "6F81-5844-456A",
			catalogURL:    gcp.DefaultNetworkBillingCatalogURL,
			wantURL:       "https://cloudbilling.googleapis.com/v1/services/6F81-5844-456A/skus",
		},
		{
			name:          "Database endpoint",
			serviceID:     gcp.DefaultDatabaseServiceID,
			wantServiceID: "9662-B51E-5089",
			catalogURL:    gcp.DefaultDatabaseBillingCatalogURL,
			wantURL:       "https://cloudbilling.googleapis.com/v1/services/9662-B51E-5089/skus",
		},
		{
			name:          "NoSQL database endpoint",
			serviceID:     gcp.DefaultNoSQLDatabaseServiceID,
			wantServiceID: "EE2C-7FAC-5E08",
			catalogURL:    gcp.DefaultNoSQLDatabaseBillingCatalogURL,
			wantURL:       "https://cloudbilling.googleapis.com/v1/services/EE2C-7FAC-5E08/skus",
		},
		{
			name:          "Kubernetes endpoint",
			serviceID:     gcp.DefaultKubernetesServiceID,
			wantServiceID: "CCD8-9BF1-090E",
			catalogURL:    gcp.DefaultKubernetesBillingCatalogURL,
			wantURL:       "https://cloudbilling.googleapis.com/v1/services/CCD8-9BF1-090E/skus",
		},
		{
			name:          "Serverless Cloud Functions endpoint",
			serviceID:     gcp.DefaultServerlessServiceID,
			wantServiceID: "29E7-DA93-CA13",
			catalogURL:    gcp.DefaultServerlessBillingCatalogURL,
			wantURL:       "https://cloudbilling.googleapis.com/v1/services/29E7-DA93-CA13/skus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.serviceID != tt.wantServiceID {
				t.Errorf("serviceID = %q, want %q", tt.serviceID, tt.wantServiceID)
			}
			if tt.catalogURL != tt.wantURL {
				t.Errorf("catalogURL = %q, want %q", tt.catalogURL, tt.wantURL)
			}
			parsed, err := url.Parse(tt.catalogURL)
			if err != nil {
				t.Fatalf("invalid url %q: %v", tt.catalogURL, err)
			}
			if parsed.Scheme != "https" {
				t.Errorf("expected https scheme, got %q", parsed.Scheme)
			}
			if parsed.Host != "cloudbilling.googleapis.com" {
				t.Errorf("expected cloudbilling.googleapis.com host, got %q", parsed.Host)
			}
			if !strings.HasSuffix(parsed.Path, "/skus") {
				t.Errorf("expected path to end in /skus, got %q", parsed.Path)
			}
		})
	}
}
