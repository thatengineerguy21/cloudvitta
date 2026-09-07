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

// DefaultDynamoDBPriceListURL is the standard AWS Price List API URL for AmazonDynamoDB us-east-1.
const DefaultDynamoDBPriceListURL = "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonDynamoDB/current/us-east-1/index.json"

// DefaultEKSPriceListURL is the standard AWS Price List API URL for AmazonEKS us-east-1.
const DefaultEKSPriceListURL = "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEKS/current/us-east-1/index.json"

// DefaultLambdaPriceListURL is the standard AWS Price List API URL for AWS Lambda in us-east-1.
const DefaultLambdaPriceListURL = "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AWSLambda/current/us-east-1/index.json"

// BuildRegionalURL constructs the standard AWS Price List API URL for an offerCode and region.
func BuildRegionalURL(offerCode, region string) string {
	return fmt.Sprintf("https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/%s/current/%s/index.json", offerCode, region)
}

// OfferCodeForCategory maps a service category to the AWS Price List offerCode.
func OfferCodeForCategory(category string) string {
	switch category {
	case "storage":
		return "AmazonS3"
	case "network":
		return "AWSDataTransfer"
	case "database_rdbms":
		return "AmazonRDS"
	case "database_nosql":
		return "AmazonDynamoDB"
	case "kubernetes":
		return "AmazonEKS"
	case "serverless":
		return "AWSLambda"
	default:
		return "AmazonEC2"
	}
}

// Client is an HTTP client for fetching AWS Price List API data.
type Client struct {
	httpClient   *http.Client
	url          string
	hasCustomURL bool
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
		c.hasCustomURL = true
	}
}

// HasCustomURL reports whether a custom endpoint URL was set on the client.
func (c *Client) HasCustomURL() bool {
	return c.hasCustomURL
}

// URL returns the base endpoint URL.
func (c *Client) URL() string {
	return c.url
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

// FetchPriceList fetches the raw price list JSON response stream from the default URL.
// The caller is responsible for closing the returned io.ReadCloser.
func (c *Client) FetchPriceList(ctx context.Context) (io.ReadCloser, error) {
	return c.FetchPriceListURL(ctx, c.url)
}

// FetchPriceListURL fetches the raw price list JSON response stream from a specific URL.
// The caller is responsible for closing the returned io.ReadCloser.
func (c *Client) FetchPriceListURL(ctx context.Context, u string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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
