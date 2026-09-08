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
		{
			name:          "AlloyDB endpoint",
			serviceID:     gcp.AlloyDBServiceID,
			wantServiceID: "C49F-B7F2-7416",
			catalogURL:    gcp.BuildBillingCatalogURL(gcp.AlloyDBServiceID),
			wantURL:       "https://cloudbilling.googleapis.com/v1/services/C49F-B7F2-7416/skus",
		},
		{
			name:          "Bigtable endpoint",
			serviceID:     gcp.BigtableServiceID,
			wantServiceID: "C802-861C-2155",
			catalogURL:    gcp.BuildBillingCatalogURL(gcp.BigtableServiceID),
			wantURL:       "https://cloudbilling.googleapis.com/v1/services/C802-861C-2155/skus",
		},
		{
			name:          "Cloud Run endpoint",
			serviceID:     gcp.CloudRunServiceID,
			wantServiceID: "152E-C115-5142",
			catalogURL:    gcp.BuildBillingCatalogURL(gcp.CloudRunServiceID),
			wantURL:       "https://cloudbilling.googleapis.com/v1/services/152E-C115-5142/skus",
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

func TestGCPClient_CategoryServiceIDs(t *testing.T) {
	tests := []struct {
		category string
		wantIDs  []string
	}{
		{"database_rdbms", []string{"9662-B51E-5089", "C49F-B7F2-7416"}},
		{"database_nosql", []string{"EE2C-7FAC-5E08", "C802-861C-2155"}},
		{"serverless", []string{"29E7-DA93-CA13", "152E-C115-5142"}},
		{"compute", []string{"6F81-5844-456A"}},
		{"storage", []string{"95FF-2EF5-5EA1"}},
		{"network", []string{"6F81-5844-456A"}},
		{"kubernetes", []string{"CCD8-9BF1-090E"}},
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			gotIDs := gcp.CategoryServiceIDs(tt.category)
			if len(gotIDs) != len(tt.wantIDs) {
				t.Fatalf("got %d IDs, want %d", len(gotIDs), len(tt.wantIDs))
			}
			for i := range gotIDs {
				if gotIDs[i] != tt.wantIDs[i] {
					t.Errorf("ID[%d] = %q, want %q", i, gotIDs[i], tt.wantIDs[i])
				}
			}

			urls := gcp.CategoryBillingCatalogURLs(tt.category)
			if len(urls) != len(tt.wantIDs) {
				t.Fatalf("got %d URLs, want %d", len(urls), len(tt.wantIDs))
			}
			for i, sid := range tt.wantIDs {
				wantURL := "https://cloudbilling.googleapis.com/v1/services/" + sid + "/skus"
				if urls[i] != wantURL {
					t.Errorf("URL[%d] = %q, want %q", i, urls[i], wantURL)
				}
			}
		})
	}
}

func TestGCPClient_WithURLs(t *testing.T) {
	customURLs := []string{
		"https://example.com/api1",
		"https://example.com/api2",
	}
	client := gcp.NewClient(gcp.WithURLs(customURLs...))
	gotURLs := client.URLs()
	if len(gotURLs) != 2 {
		t.Fatalf("expected 2 URLs, got %d", len(gotURLs))
	}
	if gotURLs[0] != customURLs[0] || gotURLs[1] != customURLs[1] {
		t.Errorf("URLs mismatch: got %v, want %v", gotURLs, customURLs)
	}
}
