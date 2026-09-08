package gcp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// DefaultComputeServiceID is the well-known GCP Cloud Billing service ID for Compute Engine.
const DefaultComputeServiceID = "6F81-5844-456A"

// DefaultStorageServiceID is the well-known GCP Cloud Billing service ID for Cloud Storage.
const DefaultStorageServiceID = "95FF-2EF5-5EA1"

// DefaultBillingCatalogURL is the base URL for the GCP Cloud Billing Catalog API.
const DefaultBillingCatalogURL = "https://cloudbilling.googleapis.com/v1/services/" + DefaultComputeServiceID + "/skus"

// DefaultStorageBillingCatalogURL is the base URL for the GCP Cloud Storage Billing Catalog API.
const DefaultStorageBillingCatalogURL = "https://cloudbilling.googleapis.com/v1/services/" + DefaultStorageServiceID + "/skus"

// DefaultNetworkServiceID is the well-known GCP Cloud Billing service ID for Networking/Data Transfer SKUs.
// In GCP Billing Catalog, Data Transfer and Egress SKUs are published under the Compute Engine service ID.
const DefaultNetworkServiceID = DefaultComputeServiceID

// DefaultNetworkBillingCatalogURL is the base URL for the GCP Network Billing Catalog API.
const DefaultNetworkBillingCatalogURL = "https://cloudbilling.googleapis.com/v1/services/" + DefaultNetworkServiceID + "/skus"

// DefaultDatabaseServiceID is the well-known GCP Cloud Billing service ID for Cloud SQL.
const DefaultDatabaseServiceID = "9662-B51E-5089"

// AlloyDBServiceID is the official GCP Cloud Billing service ID for AlloyDB for PostgreSQL.
const AlloyDBServiceID = "C49F-B7F2-7416"

// DefaultDatabaseBillingCatalogURL is the base URL for the GCP Cloud SQL Billing Catalog API.
const DefaultDatabaseBillingCatalogURL = "https://cloudbilling.googleapis.com/v1/services/" + DefaultDatabaseServiceID + "/skus"

// DefaultNoSQLDatabaseServiceID is the official GCP Cloud Billing service ID for Cloud Firestore.
const DefaultNoSQLDatabaseServiceID = "EE2C-7FAC-5E08"

// BigtableServiceID is the official GCP Cloud Billing service ID for Cloud Bigtable.
const BigtableServiceID = "C3BE-24A5-0975"

// DefaultNoSQLDatabaseBillingCatalogURL is the base URL for the GCP Cloud Firestore Billing Catalog API.
const DefaultNoSQLDatabaseBillingCatalogURL = "https://cloudbilling.googleapis.com/v1/services/" + DefaultNoSQLDatabaseServiceID + "/skus"

// DefaultKubernetesServiceID is the official GCP Cloud Billing service ID for Kubernetes Engine (GKE).
const DefaultKubernetesServiceID = "CCD8-9BF1-090E"

// DefaultKubernetesBillingCatalogURL is the base URL for the GCP Kubernetes Engine Billing Catalog API.
const DefaultKubernetesBillingCatalogURL = "https://cloudbilling.googleapis.com/v1/services/" + DefaultKubernetesServiceID + "/skus"

// DefaultServerlessServiceID is the official GCP Cloud Billing service ID for Cloud Functions / Cloud Run functions.
const DefaultServerlessServiceID = "29E7-DA93-CA13"

// CloudRunServiceID is the official GCP Cloud Billing service ID for Cloud Run container services and jobs.
const CloudRunServiceID = "152E-C115-5142"

// DefaultServerlessBillingCatalogURL is the base URL for the GCP Cloud Functions Billing Catalog API.
const DefaultServerlessBillingCatalogURL = "https://cloudbilling.googleapis.com/v1/services/" + DefaultServerlessServiceID + "/skus"

// BuildBillingCatalogURL constructs a GCP Cloud Billing Catalog API URL for a service ID.
func BuildBillingCatalogURL(serviceID string) string {
	return "https://cloudbilling.googleapis.com/v1/services/" + serviceID + "/skus"
}

// CategoryServiceIDs returns the canonical GCP Billing Catalog service IDs for a category.
func CategoryServiceIDs(category string) []string {
	switch category {
	case "compute":
		return []string{DefaultComputeServiceID}
	case "storage":
		return []string{DefaultStorageServiceID}
	case "network":
		return []string{DefaultNetworkServiceID}
	case "database_rdbms":
		return []string{DefaultDatabaseServiceID, AlloyDBServiceID}
	case "database_nosql":
		return []string{DefaultNoSQLDatabaseServiceID, BigtableServiceID}
	case "kubernetes":
		return []string{DefaultKubernetesServiceID}
	case "serverless":
		return []string{DefaultServerlessServiceID, CloudRunServiceID}
	default:
		return []string{DefaultComputeServiceID}
	}
}

// CategoryBillingCatalogURLs returns the base catalog endpoint URLs for a category.
func CategoryBillingCatalogURLs(category string) []string {
	serviceIDs := CategoryServiceIDs(category)
	urls := make([]string, len(serviceIDs))
	for i, sid := range serviceIDs {
		urls[i] = BuildBillingCatalogURL(sid)
	}
	return urls
}

// Client is an HTTP client for fetching GCP Cloud Billing Catalog API data.
type Client struct {
	httpClient *http.Client
	urls       []string
	apiKey     string
}

// Option allows customizing the GCP client.
type Option func(*Client)

// WithHTTPClient sets a custom http.Client for the GCP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithURL sets a single endpoint URL for testing.
func WithURL(rawURL string) Option {
	return func(c *Client) {
		c.urls = []string{rawURL}
	}
}

// WithURLs sets multiple endpoint URLs for multi-endpoint catalog ingestion.
func WithURLs(rawURLs ...string) Option {
	return func(c *Client) {
		c.urls = rawURLs
	}
}

// WithAPIKey sets the static GCP API key for authentication.
func WithAPIKey(key string) Option {
	return func(c *Client) {
		c.apiKey = key
	}
}

// NewClient constructs a new GCP client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{},
		urls:       []string{DefaultBillingCatalogURL},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// URLs returns a slice of all configured catalog endpoint URLs.
func (c *Client) URLs() []string {
	if len(c.urls) == 0 {
		return []string{DefaultBillingCatalogURL}
	}
	res := make([]string, len(c.urls))
	copy(res, c.urls)
	return res
}

// FetchPriceList fetches the raw price list JSON response stream from the primary/first configured URL.
// The caller is responsible for closing the returned io.ReadCloser.
func (c *Client) FetchPriceList(ctx context.Context, pageToken string) (io.ReadCloser, error) {
	primaryURL := DefaultBillingCatalogURL
	if len(c.urls) > 0 {
		primaryURL = c.urls[0]
	}
	return c.FetchPriceListURL(ctx, primaryURL, pageToken)
}

// FetchPriceListURL fetches the raw price list JSON response stream for a specific endpoint URL.
// The caller is responsible for closing the returned io.ReadCloser.
func (c *Client) FetchPriceListURL(ctx context.Context, targetURL, pageToken string) (io.ReadCloser, error) {
	reqURL := targetURL
	if pageToken != "" {
		u, err := url.Parse(reqURL)
		if err != nil {
			return nil, fmt.Errorf("gcp client: parse url: %w", err)
		}
		q := u.Query()
		q.Set("pageToken", pageToken)
		u.RawQuery = q.Encode()
		reqURL = u.String()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("gcp client: create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("X-Goog-Api-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gcp client: execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("gcp client: unexpected HTTP status %d", resp.StatusCode)
	}

	return resp.Body, nil
}
