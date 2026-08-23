package gcp_test

import (
	"os"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/gcp"
)

func TestNormalize_GCPDatabaseNoSQLGolden(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	f, err := os.Open("../../../../testdata/golden/gcp/database_nosql.json")
	if err != nil {
		t.Fatalf("failed to open golden file: %v", err)
	}
	defer func() { _ = f.Close() }()

	obs, _, err := gcp.Normalize(f, fixedTime)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}

	if len(obs) != 3 {
		t.Fatalf("expected 3 observations, got %d", len(obs))
	}

	for _, o := range obs {
		if o.Provider != "gcp" {
			t.Errorf("expected Provider gcp, got %s", o.Provider)
		}
		if o.ServiceCategory != "database_nosql" {
			t.Errorf("expected ServiceCategory database_nosql, got %s", o.ServiceCategory)
		}
		if o.PriceCurrency != "USD" {
			t.Errorf("expected PriceCurrency USD, got %s", o.PriceCurrency)
		}
		if o.Region != "us-east4" || o.RegionGroup != "us-east" {
			t.Errorf("unexpected region/group: %s / %s", o.Region, o.RegionGroup)
		}
	}
}
