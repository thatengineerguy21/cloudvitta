package aws

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// DefaultEC2PriceListURL is the standard AWS Price List API URL for EC2 us-east-1.
const DefaultEC2PriceListURL = "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/current/us-east-1/index.json"

// DefaultS3PriceListURL is the standard AWS Price List API URL for S3 us-east-1.
const DefaultS3PriceListURL = "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonS3/current/us-east-1/index.json"

// DefaultDataTransferPriceListURL is the standard AWS Price List API URL for AWSDataTransfer us-east-1.
const DefaultDataTransferPriceListURL = "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AWSDataTransfer/current/us-east-1/index.json"

// DefaultRDSPriceListURL is the standard AWS Price List API URL for AmazonRDS us-east-1.
const DefaultRDSPriceListURL = "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonRDS/current/us-east-1/index.json"

// Client is an HTTP client for fetching AWS Price List API data.
type Client struct {
	httpClient *http.Client
	url        string
}

// Option allows customizing the AWS client.
type Option func(*Client)

// WithHTTPClient sets a custom http.Client for the AWS client.
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

// NewClient constructs a new AWS client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{},
		url:        DefaultEC2PriceListURL,
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
		return nil, fmt.Errorf("aws client: create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("aws client: execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("aws client: unexpected HTTP status %d", resp.StatusCode)
	}

	return resp.Body, nil
}
