package alibaba

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// DefaultECSEndpoint is the standard Alibaba Cloud ECS RPC API endpoint.
const DefaultECSEndpoint = "https://ecs.aliyuncs.com"

// Client is an HTTP client for fetching Alibaba Cloud ECS pricing data.
type Client struct {
	httpClient      *http.Client
	url             string
	accessKeyID     string
	accessKeySecret string
	regionID        string
}

// Option allows customizing the Alibaba client.
type Option func(*Client)

// WithHTTPClient sets a custom http.Client for the Alibaba client.
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

// WithCredentials sets Alibaba Cloud AccessKeyId and AccessKeySecret for HMAC-SHA1 RPC signing.
func WithCredentials(accessKeyID, accessKeySecret string) Option {
	return func(c *Client) {
		c.accessKeyID = accessKeyID
		c.accessKeySecret = accessKeySecret
	}
}

// WithRegionID sets the default region ID for RPC calls.
func WithRegionID(regionID string) Option {
	return func(c *Client) {
		c.regionID = regionID
	}
}

// NewClient constructs a new Alibaba Cloud client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{},
		url:        DefaultECSEndpoint,
		regionID:   "us-east-1",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FetchPriceList fetches the raw Alibaba ECS pricing JSON response stream.
// If credentials are configured, it signs the request with HMAC-SHA1 before execution.
// The caller is responsible for closing the returned io.ReadCloser.
func (c *Client) FetchPriceList(ctx context.Context) (io.ReadCloser, error) {
	targetURL := c.url

	if c.accessKeyID != "" && c.accessKeySecret != "" {
		params := url.Values{
			"Action":   {"DescribePrice"},
			"RegionId": {c.regionID},
		}
		signedURL, err := SignRequest(http.MethodGet, c.url, params, c.accessKeyID, c.accessKeySecret)
		if err != nil {
			return nil, fmt.Errorf("alibaba client: sign request: %w", err)
		}
		targetURL = signedURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("alibaba client: create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("alibaba client: execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("alibaba client: unexpected HTTP status %d", resp.StatusCode)
	}

	return resp.Body, nil
}
