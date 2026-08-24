package ibm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// DefaultIAMTokenURL is the official IBM Cloud IAM OAuth 2.0 token endpoint.
const DefaultIAMTokenURL = "https://iam.cloud.ibm.com/identity/token"

// ErrMissingAPIKey indicates an empty or invalid IBM API key was supplied.
var ErrMissingAPIKey = errors.New("ibm auth: api key is required")

// FetchIAMToken exchanges an IBM Cloud API key for an OAuth 2.0 access token.
// Per PRD §9.3.1, a fresh token is requested for each ingestion run.
func FetchIAMToken(ctx context.Context, httpClient *http.Client, iamURL, apiKey string) (string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return "", ErrMissingAPIKey
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if iamURL == "" {
		iamURL = DefaultIAMTokenURL
	}

	form := url.Values{}
	form.Set("grant_type", "urn:ibm:params:oauth:grant-type:apikey")
	form.Set("apikey", apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, iamURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("ibm auth: create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ibm auth: execute token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ibm auth: read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ibm auth: token request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenResp IAMTokenResponse
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		return "", fmt.Errorf("ibm auth: decode token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("ibm auth: empty access token in response")
	}

	return tokenResp.AccessToken, nil
}
