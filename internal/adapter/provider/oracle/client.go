package oracle

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// DefaultComputePriceListURL is the standard Oracle CE Tools API endpoint for Virtual Machines in USD.
const DefaultComputePriceListURL = "https://apexapps.oracle.com/pls/apex/cetools/api/v1/products/?serviceCategory=Compute%20-%20Virtual%20Machine&currencyCode=USD"

// DefaultStoragePriceListURL is the standard Oracle CE Tools API endpoint for all Storage (Object & Block) in USD.
const DefaultStoragePriceListURL = "https://apexapps.oracle.com/pls/apex/cetools/api/v1/products/?serviceCategory=Storage&currencyCode=USD"

// DefaultBlockStoragePriceListURL is the standard Oracle CE Tools API endpoint for Block Volumes in USD.
const DefaultBlockStoragePriceListURL = "https://apexapps.oracle.com/pls/apex/cetools/api/v1/products/?serviceCategory=Storage%20-%20Block%20Volume&currencyCode=USD"

// DefaultNetworkPriceListURL is the standard Oracle CE Tools API endpoint for Networking in USD.
const DefaultNetworkPriceListURL = "https://apexapps.oracle.com/pls/apex/cetools/api/v1/products/?serviceCategory=Networking%20-%20Virtual%20Cloud%20Network&currencyCode=USD"

// Client is an HTTP client for fetching Oracle OCI CE Tools pricing data.
type Client struct {
	httpClient *http.Client
	url        string
}

// Option allows customizing the Oracle client.
type Option func(*Client)

// WithHTTPClient sets a custom http.Client for the Oracle client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithURL sets a custom endpoint URL for testing.
func WithURL(url string) Option {
	return func(c *Client) {
		c.url = url
	}
}

// NewClient constructs a new Oracle client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{},
		url:        DefaultComputePriceListURL,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FetchPriceList fetches the raw price list JSON response stream.
// The caller is responsible for closing the returned io.ReadCloser.
func (c *Client) FetchPriceList(ctx context.Context) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, fmt.Errorf("oracle client: create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oracle client: execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("oracle client: unexpected HTTP status %d", resp.StatusCode)
	}

	return resp.Body, nil
}
