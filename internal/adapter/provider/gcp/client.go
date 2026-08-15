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

// DefaultBillingCatalogURL is the base URL for the GCP Cloud Billing Catalog API.
const DefaultBillingCatalogURL = "https://cloudbilling.googleapis.com/v1/services/" + DefaultComputeServiceID + "/skus"

// Client is an HTTP client for fetching GCP Cloud Billing Catalog API data.
type Client struct {
	httpClient *http.Client
	url        string
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

// WithURL sets a custom endpoint URL for testing.
func WithURL(rawURL string) Option {
	return func(c *Client) {
		c.url = rawURL
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
		url:        DefaultBillingCatalogURL,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FetchPriceList fetches the raw price list JSON response stream.
// The caller is responsible for closing the returned io.ReadCloser.
func (c *Client) FetchPriceList(ctx context.Context) (io.ReadCloser, error) {
	reqURL := c.url
	if c.apiKey != "" {
		u, err := url.Parse(reqURL)
		if err != nil {
			return nil, fmt.Errorf("gcp client: parse url: %w", err)
		}
		q := u.Query()
		q.Set("key", c.apiKey)
		u.RawQuery = q.Encode()
		reqURL = u.String()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("gcp client: create request: %w", err)
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
