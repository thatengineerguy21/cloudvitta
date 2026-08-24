package frankfurter_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/fx/frankfurter"
)

func TestClient_FetchLatestRates_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/latest" {
			t.Errorf("expected /v1/latest, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("base") != "USD" {
			t.Errorf("expected base=USD, got %s", r.URL.Query().Get("base"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"amount": 1.0,
			"base": "USD",
			"date": "2026-08-21",
			"rates": {
				"EUR": 0.9038,
				"CNY": 7.1852,
				"GBP": 0.7745,
				"JPY": 147.25
			}
		}`))
	}))
	defer ts.Close()

	client := frankfurter.NewClient(
		frankfurter.WithBaseURL(ts.URL),
		frankfurter.WithTimeout(2*time.Second),
	)

	resp, err := client.FetchLatestRates(context.Background(), "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Base != "USD" {
		t.Errorf("expected base USD, got %s", resp.Base)
	}
	if resp.Date != "2026-08-21" {
		t.Errorf("expected date 2026-08-21, got %s", resp.Date)
	}
	if len(resp.Rates) != 4 {
		t.Errorf("expected 4 rates, got %d", len(resp.Rates))
	}

	expectedEUR := decimal.RequireFromString("0.9038")
	if !resp.Rates["EUR"].Equal(expectedEUR) {
		t.Errorf("expected EUR %s, got %s", expectedEUR, resp.Rates["EUR"])
	}

	expectedCNY := decimal.RequireFromString("7.1852")
	if !resp.Rates["CNY"].Equal(expectedCNY) {
		t.Errorf("expected CNY %s, got %s", expectedCNY, resp.Rates["CNY"])
	}
}

func TestClient_FetchLatestRates_Non200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"message": "upstream service unavailable"}`))
	}))
	defer ts.Close()

	client := frankfurter.NewClient(
		frankfurter.WithBaseURL(ts.URL),
	)

	_, err := client.FetchLatestRates(context.Background(), "USD")
	if err == nil {
		t.Fatal("expected error on 503 status, got nil")
	}
}

func TestClient_FetchLatestRates_MalformedJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid-json`))
	}))
	defer ts.Close()

	client := frankfurter.NewClient(
		frankfurter.WithBaseURL(ts.URL),
	)

	_, err := client.FetchLatestRates(context.Background(), "USD")
	if err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}
}

func TestClient_FetchLatestRates_ContextTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := frankfurter.NewClient(
		frankfurter.WithBaseURL(ts.URL),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.FetchLatestRates(ctx, "USD")
	if err == nil {
		t.Fatal("expected error on context timeout, got nil")
	}
}
