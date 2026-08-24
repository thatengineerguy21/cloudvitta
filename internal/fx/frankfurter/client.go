package frankfurter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const (
	// DefaultBaseURL is the public Frankfurter API endpoint.
	DefaultBaseURL = "https://api.frankfurter.dev"
	// DefaultTimeout is the default HTTP client timeout.
	DefaultTimeout = 10 * time.Second
)

// RatesResponse represents the JSON response returned by the Frankfurter API.
type RatesResponse struct {
	Amount decimal.Decimal            `json:"amount"`
	Base   string                     `json:"base"`
	Date   string                     `json:"date"`
	Rates  map[string]decimal.Decimal `json:"rates"`
}

// Client queries the Frankfurter Foreign Exchange API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Option configures the Frankfurter Client.
type Option func(*Client)

// WithBaseURL overrides the base URL (useful for testing).
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithHTTPClient overrides the underlying HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithTimeout configures the default HTTP client timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if c.httpClient == nil {
			c.httpClient = &http.Client{Timeout: timeout}
		} else {
			c.httpClient.Timeout = timeout
		}
	}
}

// NewClient creates a new Frankfurter API client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FetchLatestRates fetches the latest exchange rates for the given base currency.
func (c *Client) FetchLatestRates(ctx context.Context, baseCurrency string) (*RatesResponse, error) {
	if baseCurrency == "" {
		baseCurrency = "USD"
	}

	reqURL, err := url.Parse(fmt.Sprintf("%s/v1/latest", c.baseURL))
	if err != nil {
		return nil, fmt.Errorf("frankfurter: invalid base url: %w", err)
	}

	q := reqURL.Query()
	q.Set("base", baseCurrency)
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("frankfurter: create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("frankfurter: http get: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("frankfurter: upstream returned status %d", resp.StatusCode)
	}

	var result RatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("frankfurter: decode response: %w", err)
	}

	return &result, nil
}
