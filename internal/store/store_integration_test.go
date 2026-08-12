package store_test

import (
	"context"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/thatengineerguy21/CloudVitta/internal/config"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

// testDatabaseURL returns the database URL from the environment.
// Integration tests are skipped when the variable is not set.
func testDatabaseURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("CLOUDVITTA_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CLOUDVITTA_TEST_DATABASE_URL not set, skipping integration test")
	}
	return url
}

// TestInsertAndQueryRoundTrip verifies that a price observation can be
// inserted and read back through the sqlc-generated Queries interface.
func TestInsertAndQueryRoundTrip(t *testing.T) {
	dbURL := testDatabaseURL(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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

	now := time.Now().Truncate(time.Microsecond)

	// Insert a price observation.
	insertParams := store.InsertPriceObservationParams{
		Provider:        "aws",
		ServiceCategory: "compute",
		SkuID:           "t3.micro-us-east-1",
		DisplayName:     "t3.micro",
		Region:          "us-east-1",
		RegionGroup:     "us-east",
		Unit:            "Hrs",
		PriceAmount:     pgtype.Numeric{},
		PriceCurrency:   "USD",
		PricingModel:    "OnDemand",
		Attributes:      nil,
		RawResponseRef:  pgtype.Text{String: "gs://bucket/aws/2024-01-01.json", Valid: true},
		FetchedAt:       pgtype.Timestamptz{Time: now, Valid: true},
		LastSeenAt:      pgtype.Timestamptz{Time: now, Valid: true},
		AnomalyStatus:   pgtype.Text{},
	}

	// Set price_amount to 0.0104
	if err := insertParams.PriceAmount.Scan("0.0104"); err != nil {
		t.Fatalf("scan price_amount: %v", err)
	}

	id, err := queries.InsertPriceObservation(ctx, insertParams)
	if err != nil {
		t.Fatalf("InsertPriceObservation: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	// Query back by (provider, service_category, region_group).
	rows, err := queries.GetPriceObservations(ctx, store.GetPriceObservationsParams{
		Provider:        "aws",
		ServiceCategory: "compute",
		RegionGroup:     "us-east",
	})
	if err != nil {
		t.Fatalf("GetPriceObservations: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected at least one row, got zero")
	}

	// Verify the inserted row is in the results.
	var found bool
	for _, row := range rows {
		if row.ID == id {
			found = true
			if row.Provider != "aws" {
				t.Errorf("provider: got %q, want %q", row.Provider, "aws")
			}
			if row.SkuID != "t3.micro-us-east-1" {
				t.Errorf("sku_id: got %q, want %q", row.SkuID, "t3.micro-us-east-1")
			}
			if row.DisplayName != "t3.micro" {
				t.Errorf("display_name: got %q, want %q", row.DisplayName, "t3.micro")
			}
			if row.Region != "us-east-1" {
				t.Errorf("region: got %q, want %q", row.Region, "us-east-1")
			}
			if row.RegionGroup != "us-east" {
				t.Errorf("region_group: got %q, want %q", row.RegionGroup, "us-east")
			}
			if row.PriceCurrency != "USD" {
				t.Errorf("price_currency: got %q, want %q", row.PriceCurrency, "USD")
			}
			if row.PricingModel != "OnDemand" {
				t.Errorf("pricing_model: got %q, want %q", row.PricingModel, "OnDemand")
			}
			break
		}
	}
	if !found {
		t.Errorf("inserted row (id=%d) not found in query results", id)
	}

	// Clean up: remove the test row.
	_, err = pool.Exec(ctx, "DELETE FROM price_observations WHERE id = $1", id)
	if err != nil {
		t.Logf("cleanup: failed to delete test row: %v", err)
	}
}
