package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/redis/go-redis/v9"
	"github.com/thatengineerguy21/CloudVitta/internal/auth"
	"github.com/thatengineerguy21/CloudVitta/internal/middleware/authmw"
	"github.com/thatengineerguy21/CloudVitta/internal/middleware/ratelimit"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/mcp"
)

var (
	testJWTSecret = []byte("super-secret-jwt-signing-key-32b-length!")
	testFixedTime = time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
)

type bearerAuthTransport struct {
	token string
	base  http.RoundTripper
}

func (b *bearerAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	if b.token != "" {
		cloned.Header.Set("Authorization", "Bearer "+b.token)
	}
	base := b.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(cloned)
}

func setupTestServer(t *testing.T) (http.Handler, *miniredis.Miniredis, *redis.Client, *service.PricingService, *service.FreshnessService) {
	return setupTestServerWithClock(t, func() time.Time { return testFixedTime })
}

func setupTestServerWithClock(t *testing.T, clock func() time.Time) (http.Handler, *miniredis.Miniredis, *redis.Client, *service.PricingService, *service.FreshnessService) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	freshnessSvc := service.NewFreshnessService(nil, nil)
	pricingSvc := service.NewPricingService(nil, rdb, service.WithFreshnessService(freshnessSvc))

	mcpServer := mcp.NewServer(pricingSvc, freshnessSvc)
	mcpHandler := mcp.NewStreamableHandler(mcpServer)

	limiter := ratelimit.NewRateLimiter(rdb, ratelimit.Config{
		StandardTierRate: 120,
		FreeTierRate:     20,
		IPCeilingRate:    60,
		CookieSecret:     testJWTSecret,
		Clock:            clock,
	})

	authMw := authmw.NewAuthMiddleware(testJWTSecret, clock)

	mux := http.NewServeMux()
	mux.Handle("/mcp", authmw.RequireAuth(limiter.Handler(mcpHandler)))

	handler := authMw(mux)

	return handler, mr, rdb, pricingSvc, freshnessSvc
}

func TestStreamableHTTP_UnauthenticatedRejection(t *testing.T) {
	handler, mr, rdb, _, _ := setupTestServer(t)
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	ts := httptest.NewServer(handler)
	defer ts.Close()

	reqBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/mcp", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401 Unauthorized for unauthenticated MCP request, got %d", resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("expected Content-Type application/problem+json, got %s", ct)
	}

	var problem map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&problem); err != nil {
		t.Fatalf("failed to decode problem details JSON: %v", err)
	}

	if problem["title"] != "Unauthorized" {
		t.Errorf("expected title 'Unauthorized', got %v", problem["title"])
	}
}

func TestStreamableHTTP_Authenticated_ToolsList(t *testing.T) {
	handler, mr, rdb, _, _ := setupTestServer(t)
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	ts := httptest.NewServer(handler)
	defer ts.Close()

	userID := uuid.New()
	token, err := auth.GenerateAccessToken(userID, "standard", testJWTSecret, testFixedTime, 15*time.Minute)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	ctx := context.Background()
	transport := &sdk.StreamableClientTransport{
		Endpoint: ts.URL + "/mcp",
		HTTPClient: &http.Client{
			Transport: &bearerAuthTransport{token: token},
		},
		DisableStandaloneSSE: true,
	}

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("client.Connect over Streamable HTTP failed: %v", err)
	}
	defer func() { _ = session.Close() }()

	toolsList, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	if len(toolsList.Tools) < 5 {
		t.Errorf("expected at least 5 tools, got %d", len(toolsList.Tools))
	}
}

func TestStreamableHTTP_Authenticated_ToolCall(t *testing.T) {
	handler, mr, rdb, _, _ := setupTestServer(t)
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	seedComputeObservations(ctx, t, rdb)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	userID := uuid.New()
	token, err := auth.GenerateAccessToken(userID, "standard", testJWTSecret, testFixedTime, 15*time.Minute)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	transport := &sdk.StreamableClientTransport{
		Endpoint: ts.URL + "/mcp",
		HTTPClient: &http.Client{
			Transport: &bearerAuthTransport{token: token},
		},
		DisableStandaloneSSE: true,
	}

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("client.Connect over Streamable HTTP failed: %v", err)
	}
	defer func() { _ = session.Close() }()

	vcpu := 2.0
	ramGB := 4.0
	res, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name: "compare_compute",
		Arguments: map[string]any{
			"vcpu":   vcpu,
			"ram_gb": ramGB,
			"region": "us-east",
		},
	})
	if err != nil {
		t.Fatalf("CallTool compare_compute over Streamable HTTP failed: %v", err)
	}

	if res.IsError {
		t.Fatalf("expected CallToolResult success, got error: %+v", res)
	}

	if len(res.Content) == 0 {
		t.Fatal("expected non-empty tool response content")
	}
}

func TestStreamableHTTP_RateLimiting_StandardTier(t *testing.T) {
	handler, mr, rdb, _, _ := setupTestServer(t)
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	ts := httptest.NewServer(handler)
	defer ts.Close()

	userID := uuid.New()
	token, err := auth.GenerateAccessToken(userID, "standard", testJWTSecret, testFixedTime, 15*time.Minute)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	ctx := context.Background()
	transport := &sdk.StreamableClientTransport{
		Endpoint: ts.URL + "/mcp",
		HTTPClient: &http.Client{
			Transport: &bearerAuthTransport{token: token},
		},
		DisableStandaloneSSE: true,
	}

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("client.Connect failed: %v", err)
	}
	defer func() { _ = session.Close() }()

	// Initial connect consumes 1 request (initialize)
	// We make 119 more requests to exhaust the 120 quota
	for i := 0; i < 119; i++ {
		_, err := session.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
	}

	// 121st request must fail due to rate limit exhaustion
	_, err121 := session.ListTools(ctx, nil)
	if err121 == nil {
		t.Fatalf("expected rate limit error on 121st request, got success")
	}
}
