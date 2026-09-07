package service_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/aws"
	"github.com/thatengineerguy21/CloudVitta/internal/config"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	urlStr := os.Getenv("CLOUDVITTA_TEST_DATABASE_URL")
	if urlStr == "" {
		t.Skip("CLOUDVITTA_TEST_DATABASE_URL not set, skipping integration test")
	}
	return urlStr
}

func TestIngestionOrchestrator_RunAWSComputeIngestion(t *testing.T) {
	dbURL := testDatabaseURL(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Load fixture data
	fixturePath := filepath.Join("..", "adapter", "provider", "aws", "testdata", "ec2-us-east-1-sample.json")
	fixtureBytes, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read test fixture at %s: %v", fixturePath, err)
	}

	// Mock AWS HTTP Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixtureBytes)
	}))
	defer ts.Close()

	// Setup Database Connection
	u, err := url.Parse(dbURL)
	if err != nil {
		t.Fatalf("parse db url: %v", err)
	}
	pwd, _ := u.User.Password()
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 5432
	}

	cfg := config.DatabaseConfig{
		Host:            u.Hostname(),
		Port:            port,
		User:            u.User.Username(),
		Password:        pwd,
		Name:            strings.TrimPrefix(u.Path, "/"),
		SSLMode:         u.Query().Get("sslmode"),
		MaxOpenConns:    2,
		MaxIdleConns:    1,
		ConnMaxLifetime: 60,
		ConnMaxIdleTime: 30,
	}

	pool, err := store.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	queries := store.New(pool)
	memStorage := storage.NewMemoryRawStorage()
	awsClient := aws.NewClient(aws.WithURL(ts.URL), aws.WithHTTPClient(ts.Client()))
	awsAdapter := aws.NewAdapter(awsClient, memStorage, aws.WithCategory("compute"))

	factory := provider.NewFactory()
	factory.Register(provider.ProviderConfig{
		Provider: "aws",
		Category: "compute",
	}, awsAdapter)

	orch := service.NewOrchestrator(queries, nil, nil, factory, service.DefaultOrchestratorConfig())

	// Clean up any prior test rows before starting and after finishing
	_, _ = pool.Exec(ctx, "DELETE FROM price_observations WHERE provider = 'aws' AND sku_id IN ('SKU-C5-XLARGE', 'SKU-M5-LARGE')")
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM price_observations WHERE provider = 'aws' AND sku_id IN ('SKU-C5-XLARGE', 'SKU-M5-LARGE')")
	}()

	// First ingestion run: inserts new price observations
	results1 := orch.RunAll(ctx)
	if len(results1) != 1 {
		t.Fatalf("expected 1 job result, got %d", len(results1))
	}
	if results1[0].Err != nil {
		t.Fatalf("run 1 failed: %v", results1[0].Err)
	}
	if results1[0].InsertedCount != 2 {
		t.Fatalf("run 1 InsertedCount = %d, want 2", results1[0].InsertedCount)
	}
	if results1[0].UpdatedCount != 0 {
		t.Fatalf("run 1 UpdatedCount = %d, want 0", results1[0].UpdatedCount)
	}

	// Verify rows land in DB
	rows1, err := queries.GetPriceObservations(ctx, store.GetPriceObservationsParams{
		Provider:        "aws",
		ServiceCategory: "compute",
		RegionGroup:     "us-east",
	})
	if err != nil {
		t.Fatalf("GetPriceObservations: %v", err)
	}
	if len(rows1) < 2 {
		t.Fatalf("expected at least 2 rows in DB, got %d", len(rows1))
	}

	// Second ingestion run with identical prices: should bump last_seen_at idempotently without creating duplicate rows
	results2 := orch.RunAll(ctx)
	if len(results2) != 1 {
		t.Fatalf("expected 1 job result, got %d", len(results2))
	}
	if results2[0].Err != nil {
		t.Fatalf("run 2 failed: %v", results2[0].Err)
	}
	if results2[0].InsertedCount != 0 {
		t.Errorf("run 2 InsertedCount = %d, want 0 (idempotent deduplication)", results2[0].InsertedCount)
	}
	if results2[0].UpdatedCount != 2 {
		t.Errorf("run 2 UpdatedCount = %d, want 2 (bumped last_seen_at)", results2[0].UpdatedCount)
	}

	rows2, err := queries.GetPriceObservations(ctx, store.GetPriceObservationsParams{
		Provider:        "aws",
		ServiceCategory: "compute",
		RegionGroup:     "us-east",
	})
	if err != nil {
		t.Fatalf("GetPriceObservations after run 2: %v", err)
	}

	if len(rows2) != len(rows1) {
		t.Errorf("DB rows count after run 2 = %d, want %d (idempotent update violated)", len(rows2), len(rows1))
	}

	// Verify memory raw storage recorded raw files
	files := memStorage.GetFiles()
	if len(files) != 2 {
		t.Errorf("MemoryStorage recorded %d files, want 2 (one per fetch)", len(files))
	}
}
