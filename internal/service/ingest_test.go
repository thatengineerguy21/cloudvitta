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

func TestIngestionService_RunAWSComputeIngestion(t *testing.T) {
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
	awsAdapter := aws.NewAdapter(awsClient, memStorage)

	ingestSvc := service.NewIngestionService(queries, awsAdapter, nil)

	// First ingestion run
	count1, err := ingestSvc.RunAWSComputeIngestion(ctx)
	if err != nil {
		t.Fatalf("RunAWSComputeIngestion run 1 failed: %v", err)
	}
	if count1 != 2 {
		t.Fatalf("run 1 count = %d, want 2", count1)
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

	// Verify "always insert, never diff-and-skip" by running ingestion a second time
	count2, err := ingestSvc.RunAWSComputeIngestion(ctx)
	if err != nil {
		t.Fatalf("RunAWSComputeIngestion run 2 failed: %v", err)
	}
	if count2 != 2 {
		t.Fatalf("run 2 count = %d, want 2", count2)
	}

	rows2, err := queries.GetPriceObservations(ctx, store.GetPriceObservationsParams{
		Provider:        "aws",
		ServiceCategory: "compute",
		RegionGroup:     "us-east",
	})
	if err != nil {
		t.Fatalf("GetPriceObservations after run 2: %v", err)
	}

	if len(rows2) != len(rows1)+2 {
		t.Errorf("DB rows count after run 2 = %d, want %d (always insert property violated)", len(rows2), len(rows1)+2)
	}

	// Verify memory raw storage recorded raw files
	files := memStorage.GetFiles()
	if len(files) != 2 {
		t.Errorf("MemoryStorage recorded %d files, want 2 (one per fetch)", len(files))
	}

	// Clean up inserted test rows from DB
	_, _ = pool.Exec(ctx, "DELETE FROM price_observations WHERE provider = 'aws' AND sku_id IN ('SKU-C5-XLARGE', 'SKU-M5-LARGE')")
}
