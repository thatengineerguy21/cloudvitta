package digitalocean

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// DefaultSizesURL is the standard DigitalOcean API endpoint for Droplet sizes.
const DefaultSizesURL = "https://api.digitalocean.com/v2/sizes?per_page=200"

// DefaultRegionsURL is the standard DigitalOcean API endpoint for regions.
const DefaultRegionsURL = "https://api.digitalocean.com/v2/regions?per_page=100"

// Client is an HTTP client for querying DigitalOcean API v2 pricing data.
type Client struct {
	httpClient *http.Client
	sizesURL   string
	regionsURL string
	token      string
}

// Option allows customizing the DigitalOcean client.
type Option func(*Client)

// WithHTTPClient sets a custom http.Client for the DigitalOcean client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithURL sets a custom sizes endpoint URL for testing.
func WithURL(url string) Option {
	return func(c *Client) {
		c.sizesURL = url
	}
}

// WithRegionsURL sets a custom regions endpoint URL for testing.
func WithRegionsURL(url string) Option {
	return func(c *Client) {
		c.regionsURL = url
	}
}

// WithToken sets the DigitalOcean Personal Access Token.
func WithToken(token string) Option {
	return func(c *Client) {
		c.token = token
	}
}

// NewClient constructs a new DigitalOcean client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{},
		sizesURL:   DefaultSizesURL,
		regionsURL: DefaultRegionsURL,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FetchPriceList fetches the raw Droplet sizes JSON response stream.
// The caller is responsible for closing the returned io.ReadCloser.
func (c *Client) FetchPriceList(ctx context.Context) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.sizesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("digitalocean client: create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("digitalocean client: execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("digitalocean client: unexpected HTTP status %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// FetchRegions fetches the raw DigitalOcean regions JSON response stream.
// The caller is responsible for closing the returned io.ReadCloser.
func (c *Client) FetchRegions(ctx context.Context) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.regionsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("digitalocean client: create regions request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("digitalocean client: execute regions request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("digitalocean client: unexpected regions HTTP status %d", resp.StatusCode)
	}

	return resp.Body, nil
}
