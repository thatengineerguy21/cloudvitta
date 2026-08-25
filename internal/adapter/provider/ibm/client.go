package ibm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// DefaultGlobalCatalogURL is the standard IBM Cloud Global Catalog API endpoint for VPC instances.
const DefaultGlobalCatalogURL = "https://globalcatalog.cloud.ibm.com/api/v1?q=kind:service+name:is.instance&include=*"

// DefaultGlobalStorageCatalogURL is the standard IBM Cloud Global Catalog API endpoint for Cloud Object Storage.
const DefaultGlobalStorageCatalogURL = "https://globalcatalog.cloud.ibm.com/api/v1?q=kind:service+name:cloud-object-storage&include=*"

// DefaultGlobalStorageVolumeCatalogURL is the standard IBM Cloud Global Catalog API endpoint for VPC Block Storage Volumes.
const DefaultGlobalStorageVolumeCatalogURL = "https://globalcatalog.cloud.ibm.com/api/v1?q=kind:service+name:is.volume&include=*"

// DefaultGlobalNetworkCatalogURL is the standard IBM Cloud Global Catalog API endpoint for VPC Networking.
const DefaultGlobalNetworkCatalogURL = "https://globalcatalog.cloud.ibm.com/api/v1?q=kind:service+name:is.floating-ip&include=*"

// Client is an HTTP client for fetching IBM Cloud Global Catalog pricing data.
type Client struct {
	httpClient *http.Client
	iamURL     string
	catalogURL string
	apiKey     string
}

// Option allows customizing the IBM client.
type Option func(*Client)

// WithHTTPClient sets a custom http.Client for the IBM client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithIAMURL sets a custom IAM token endpoint URL for testing.
func WithIAMURL(url string) Option {
	return func(c *Client) {
		c.iamURL = url
	}
}

// WithCatalogURL sets a custom Global Catalog endpoint URL for testing.
func WithCatalogURL(url string) Option {
	return func(c *Client) {
		c.catalogURL = url
	}
}

// WithAPIKey sets the IBM Cloud API key for OAuth 2.0 IAM token exchange.
func WithAPIKey(apiKey string) Option {
	return func(c *Client) {
		c.apiKey = apiKey
	}
}

// NewClient constructs a new IBM Cloud client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{},
		iamURL:     DefaultIAMTokenURL,
		catalogURL: DefaultGlobalCatalogURL,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FetchPriceList fetches the raw Global Catalog pricing JSON response stream.
// If an API key is configured, it requests a fresh IAM OAuth 2.0 bearer token first.
// If querying default storage catalog, it fetches both object storage and volume endpoints and merges their resources.
// The caller is responsible for closing the returned io.ReadCloser.
func (c *Client) FetchPriceList(ctx context.Context) (io.ReadCloser, error) {
	if c.catalogURL == DefaultGlobalStorageCatalogURL {
		return c.fetchMergedStorage(ctx)
	}
	return c.fetchSingleURL(ctx, c.catalogURL)
}

func (c *Client) fetchSingleURL(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("ibm client: create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	if c.apiKey != "" {
		token, err := FetchIAMToken(ctx, c.httpClient, c.iamURL, c.apiKey)
		if err != nil {
			return nil, fmt.Errorf("ibm client: iam authentication: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ibm client: execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("ibm client: unexpected HTTP status %d", resp.StatusCode)
	}

	return resp.Body, nil
}

func (c *Client) fetchMergedStorage(ctx context.Context) (io.ReadCloser, error) {
	rc1, err := c.fetchSingleURL(ctx, DefaultGlobalStorageCatalogURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc1.Close() }()

	var resp1 CatalogResponse
	if err := json.NewDecoder(rc1).Decode(&resp1); err != nil {
		return nil, fmt.Errorf("ibm client: decode storage catalog 1: %w", err)
	}

	rc2, err := c.fetchSingleURL(ctx, DefaultGlobalStorageVolumeCatalogURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc2.Close() }()

	var resp2 CatalogResponse
	if err := json.NewDecoder(rc2).Decode(&resp2); err != nil {
		return nil, fmt.Errorf("ibm client: decode storage catalog 2: %w", err)
	}

	resp1.Resources = append(resp1.Resources, resp2.Resources...)

	pr, pw := io.Pipe()
	go func() {
		defer func() { _ = pw.Close() }()
		_ = json.NewEncoder(pw).Encode(resp1)
	}()
	return pr, nil
}
