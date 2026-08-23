package azure

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// DefaultRetailPricesURL is the standard Azure Retail Prices API URL for Virtual Machines in eastus.
const DefaultRetailPricesURL = "https://prices.azure.com/api/retail/prices?$filter=serviceName%20eq%20'Virtual%20Machines'%20and%20armRegionName%20eq%20'eastus'%20and%20priceType%20eq%20'Consumption'"

// DefaultStorageRetailPricesURL is the standard Azure Retail Prices API URL for Storage in eastus.
const DefaultStorageRetailPricesURL = "https://prices.azure.com/api/retail/prices?$filter=serviceName%20eq%20'Storage'%20and%20armRegionName%20eq%20'eastus'%20and%20priceType%20eq%20'Consumption'"

// DefaultNetworkRetailPricesURL is the standard Azure Retail Prices API URL for Bandwidth / Network.
const DefaultNetworkRetailPricesURL = "https://prices.azure.com/api/retail/prices?$filter=serviceName%20eq%20'Bandwidth'%20and%20priceType%20eq%20'Consumption'"

// DefaultDatabaseRetailPricesURL is the standard Azure Retail Prices API URL for relational database services in eastus.
const DefaultDatabaseRetailPricesURL = "https://prices.azure.com/api/retail/prices?$filter=(serviceName%20eq%20'Azure%20Database%20for%20PostgreSQL'%20or%20serviceName%20eq%20'Azure%20Database%20for%20MySQL'%20or%20serviceName%20eq%20'SQL%20Database')%20and%20armRegionName%20eq%20'eastus'%20and%20priceType%20eq%20'Consumption'"

// DefaultCosmosDBRetailPricesURL is the standard Azure Retail Prices API URL for Azure Cosmos DB in eastus.
const DefaultCosmosDBRetailPricesURL = "https://prices.azure.com/api/retail/prices?$filter=serviceName%20eq%20'Azure%20Cosmos%20DB'%20and%20armRegionName%20eq%20'eastus'%20and%20priceType%20eq%20'Consumption'"

// DefaultKubernetesRetailPricesURL is the standard Azure Retail Prices API URL for Azure Kubernetes Service in eastus.
const DefaultKubernetesRetailPricesURL = "https://prices.azure.com/api/retail/prices?$filter=serviceName%20eq%20'Azure%20Kubernetes%20Service'%20and%20armRegionName%20eq%20'eastus'%20and%20priceType%20eq%20'Consumption'"

// Client is an HTTP client for fetching Azure Retail Prices API data.
type Client struct {
	httpClient *http.Client
	url        string
}

// Option allows customizing the Azure client.
type Option func(*Client)

// WithHTTPClient sets a custom http.Client for the Azure client.
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

// NewClient constructs a new Azure client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{},
		url:        DefaultRetailPricesURL,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FetchPriceList fetches the raw price list JSON response stream.
// If urlOverride is not empty, it fetches from that URL instead of the default.
// The caller is responsible for closing the returned io.ReadCloser.
func (c *Client) FetchPriceList(ctx context.Context, urlOverride string) (io.ReadCloser, error) {
	u := c.url
	if urlOverride != "" {
		u = urlOverride
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("azure client: create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure client: execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("azure client: unexpected HTTP status %d", resp.StatusCode)
	}

	return resp.Body, nil
}
