package azure_test

import (
	"os"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/azure"
)

func TestNormalize_AzureDatabaseNoSQLGolden(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	f, err := os.Open("../../../../testdata/golden/azure/database_nosql.json")
	if err != nil {
		t.Fatalf("failed to open golden file: %v", err)
	}
	defer func() { _ = f.Close() }()

	res, _, err := azure.Normalize(f, fixedTime)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	obs := res.Observations

	if len(obs) != 4 {
		t.Fatalf("expected 4 observations, got %d", len(obs))
	}

	for _, o := range obs {
		if o.Provider != "azure" {
			t.Errorf("expected Provider azure, got %s", o.Provider)
		}
		if o.ServiceCategory != "database_nosql" {
			t.Errorf("expected ServiceCategory database_nosql, got %s", o.ServiceCategory)
		}
		if o.PriceCurrency != "USD" {
			t.Errorf("expected PriceCurrency USD, got %s", o.PriceCurrency)
		}
		if o.Region != "eastus" || o.RegionGroup != "us-east" {
			t.Errorf("unexpected region/group: %s / %s", o.Region, o.RegionGroup)
		}
	}
}
