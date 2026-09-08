//go:build live_gcp_catalog

package gcp_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/gcp"
)

type gcpServiceItem struct {
	Name        string `json:"name"`
	ServiceID   string `json:"serviceId"`
	DisplayName string `json:"displayName"`
}

type gcpServicesListResponse struct {
	Services      []gcpServiceItem `json:"services"`
	NextPageToken string           `json:"nextPageToken"`
}

// TestGCPServiceIDs_MatchLiveCatalog validates that all nine configured service constants in client.go
// match the live Google Cloud Billing Catalog API. Gated behind the live_gcp_catalog build tag.
func TestGCPServiceIDs_MatchLiveCatalog(t *testing.T) {
	apiKey := os.Getenv("GCP_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("API_KEY")
	}
	if apiKey == "" {
		t.Skip("skipping live catalog test: neither GCP_API_KEY nor API_KEY environment variable is set")
	}

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	liveServices := make(map[string]string) // displayName -> serviceID
	pageToken := ""

	for {
		reqURL := "https://cloudbilling.googleapis.com/v1/services"
		u, err := url.Parse(reqURL)
		if err != nil {
			t.Fatalf("failed to parse url: %v", err)
		}
		q := u.Query()
		q.Set("key", apiKey)
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}
		u.RawQuery = q.Encode()

		resp, err := httpClient.Get(u.String())
		if err != nil {
			t.Fatalf("failed to fetch live services: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			t.Fatalf("live services request returned status %d", resp.StatusCode)
		}

		var page gcpServicesListResponse
		err = json.NewDecoder(resp.Body).Decode(&page)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("failed to decode live services response: %v", err)
		}

		for _, s := range page.Services {
			liveServices[s.DisplayName] = s.ServiceID
		}

		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}

	expectedIDs := []struct {
		displayName   string
		configuredID  string
		alternateName string
	}{
		{displayName: "Compute Engine", configuredID: gcp.DefaultComputeServiceID},
		{displayName: "Cloud Storage", configuredID: gcp.DefaultStorageServiceID},
		{displayName: "Cloud SQL", configuredID: gcp.DefaultDatabaseServiceID},
		{displayName: "AlloyDB", configuredID: gcp.AlloyDBServiceID, alternateName: "AlloyDB for PostgreSQL"},
		{displayName: "Cloud Firestore", configuredID: gcp.DefaultNoSQLDatabaseServiceID},
		{displayName: "Cloud Bigtable", configuredID: gcp.BigtableServiceID},
		{displayName: "Kubernetes Engine", configuredID: gcp.DefaultKubernetesServiceID},
		{displayName: "Cloud Run Functions", configuredID: gcp.DefaultServerlessServiceID, alternateName: "Cloud Functions"},
		{displayName: "Cloud Run", configuredID: gcp.CloudRunServiceID},
	}

	for _, tc := range expectedIDs {
		t.Run(tc.displayName, func(t *testing.T) {
			liveID, ok := liveServices[tc.displayName]
			if !ok && tc.alternateName != "" {
				liveID, ok = liveServices[tc.alternateName]
			}
			if !ok {
				t.Errorf("service %q not found in live catalog. Available services matching pattern: %v",
					tc.displayName, findMatchingServices(liveServices, tc.displayName))
				return
			}
			if liveID != tc.configuredID {
				t.Errorf("mismatch for %q: configured=%q, live=%q", tc.displayName, tc.configuredID, liveID)
			}
		})
	}
}

func findMatchingServices(services map[string]string, term string) map[string]string {
	res := make(map[string]string)
	termLower := strings.ToLower(term)
	for name, id := range services {
		if strings.Contains(strings.ToLower(name), termLower) {
			res[name] = id
		}
	}
	return res
}
