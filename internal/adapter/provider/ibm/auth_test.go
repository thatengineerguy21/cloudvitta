package ibm_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/ibm"
)

func TestFetchIAMToken_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("expected Content-Type application/x-www-form-urlencoded, got %s", ct)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		if gt := r.Form.Get("grant_type"); gt != "urn:ibm:params:oauth:grant-type:apikey" {
			t.Errorf("expected grant_type urn:ibm:params:oauth:grant-type:apikey, got %s", gt)
		}
		if key := r.Form.Get("apikey"); key != "test-api-key-123" {
			t.Errorf("expected apikey test-api-key-123, got %s", key)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"access_token": "mock-ibm-jwt-access-token",
			"refresh_token": "mock-refresh-token",
			"token_type": "Bearer",
			"expires_in": 3600,
			"expiration": 1700000000
		}`))
	}))
	defer ts.Close()

	token, err := ibm.FetchIAMToken(context.Background(), ts.Client(), ts.URL, "test-api-key-123")
	if err != nil {
		t.Fatalf("FetchIAMToken unexpected error: %v", err)
	}
	if token != "mock-ibm-jwt-access-token" {
		t.Errorf("expected token mock-ibm-jwt-access-token, got %s", token)
	}
}

func TestFetchIAMToken_MissingAPIKey(t *testing.T) {
	_, err := ibm.FetchIAMToken(context.Background(), nil, "", "")
	if err == nil {
		t.Fatal("expected error for empty API key, got nil")
	}
	if !errors.Is(err, ibm.ErrMissingAPIKey) {
		t.Errorf("expected ErrMissingAPIKey, got %v", err)
	}
}

func TestFetchIAMToken_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errorCode": "BXNIM0415E", "errorMessage": "Provided API key could not be found"}`))
	}))
	defer ts.Close()

	_, err := ibm.FetchIAMToken(context.Background(), ts.Client(), ts.URL, "invalid-key")
	if err == nil {
		t.Fatal("expected error for HTTP 401, got nil")
	}
}

func TestFetchIAMToken_InvalidJSONResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer ts.Close()

	_, err := ibm.FetchIAMToken(context.Background(), ts.Client(), ts.URL, "test-key")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestFetchIAMToken_EmptyAccessToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token_type": "Bearer", "expires_in": 3600}`))
	}))
	defer ts.Close()

	_, err := ibm.FetchIAMToken(context.Background(), ts.Client(), ts.URL, "test-key")
	if err == nil {
		t.Fatal("expected error for empty access token, got nil")
	}
}
