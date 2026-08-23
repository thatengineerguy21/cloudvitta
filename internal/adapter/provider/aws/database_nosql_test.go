package aws_test

import (
	"os"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/aws"
)

func TestNormalize_AWSDatabaseNoSQLGolden(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	f, err := os.Open("../../../../testdata/golden/aws/database_nosql.json")
	if err != nil {
		t.Fatalf("failed to open golden file: %v", err)
	}
	defer func() { _ = f.Close() }()

	obs, err := aws.Normalize(f, fixedTime)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}

	if len(obs) != 6 {
		t.Fatalf("expected 6 observations, got %d", len(obs))
	}

	for _, o := range obs {
		if o.Provider != "aws" {
			t.Errorf("expected Provider aws, got %s", o.Provider)
		}
		if o.ServiceCategory != "database_nosql" {
			t.Errorf("expected ServiceCategory database_nosql, got %s", o.ServiceCategory)
		}
		if o.PriceCurrency != "USD" {
			t.Errorf("expected PriceCurrency USD, got %s", o.PriceCurrency)
		}
		if o.Region != "us-east-1" || o.RegionGroup != "us-east" {
			t.Errorf("unexpected region/group: %s / %s", o.Region, o.RegionGroup)
		}
	}
}
