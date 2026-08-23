package service_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
)

func TestNoSQLScorer_ExactMatch(t *testing.T) {
	scorer := service.NoSQLScorer{}

	target := service.MatchTarget{
		DataModel:        "document",
		PricingMode:      "provisioned",
		ReadUnits:        100,
		WriteUnits:       20,
		NoSQLStorageGB:   50,
		NoSQLMultiRegion: false,
		Category:         "database_nosql",
	}

	cand := domain.PriceObservation{
		Provider:        "aws",
		ServiceCategory: "database_nosql",
		DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
			DataModel:     "document",
			PricingMode:   "provisioned",
			ReadUnits:     100,
			WriteUnits:    20,
			StorageGB:     50,
			MultiRegion:   false,
			ComponentType: "composite",
		},
	}

	dist, missing, eligible := scorer.Score(cand, target)
	if !eligible {
		t.Fatalf("expected eligible true, got false")
	}
	if dist != 0.0 {
		t.Errorf("expected distance 0.0, got %f", dist)
	}
	if len(missing) != 0 {
		t.Errorf("expected 0 missing attrs, got %v", missing)
	}
}

func TestNoSQLScorer_ModelCompatibility(t *testing.T) {
	scorer := service.NoSQLScorer{}

	target := service.MatchTarget{
		DataModel:        "document",
		PricingMode:      "provisioned",
		ReadUnits:        100,
		WriteUnits:       20,
		NoSQLStorageGB:   50,
		NoSQLMultiRegion: false,
	}

	// Key-Value should be compatible with document without penalty
	candKV := domain.PriceObservation{
		DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
			DataModel:     "key_value",
			PricingMode:   "provisioned",
			ReadUnits:     100,
			WriteUnits:    20,
			StorageGB:     50,
			MultiRegion:   false,
			ComponentType: "composite",
		},
	}
	distKV, _, eligible := scorer.Score(candKV, target)
	if !eligible || distKV != 0.0 {
		t.Errorf("expected key_value to be compatible with distance 0.0, got %f (eligible: %v)", distKV, eligible)
	}

	// Wide-column should incur model penalty (0.40)
	candWide := domain.PriceObservation{
		DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
			DataModel:     "wide_column",
			PricingMode:   "provisioned",
			ReadUnits:     100,
			WriteUnits:    20,
			StorageGB:     50,
			MultiRegion:   false,
			ComponentType: "composite",
		},
	}
	distWide, _, eligible := scorer.Score(candWide, target)
	if !eligible || distWide < 0.39 || distWide > 0.41 {
		t.Errorf("expected wide_column penalty 0.40, got %f", distWide)
	}
}

func TestNoSQLScorer_ModeAndHAPenalties(t *testing.T) {
	scorer := service.NoSQLScorer{}

	target := service.MatchTarget{
		DataModel:        "document",
		PricingMode:      "provisioned",
		ReadUnits:        100,
		WriteUnits:       20,
		NoSQLStorageGB:   50,
		NoSQLMultiRegion: false,
	}

	// Mode penalty: on_demand vs provisioned (0.30)
	candMode := domain.PriceObservation{
		DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
			DataModel:     "document",
			PricingMode:   "on_demand",
			ReadUnits:     100,
			WriteUnits:    20,
			StorageGB:     50,
			MultiRegion:   false,
			ComponentType: "composite",
		},
	}
	distMode, _, _ := scorer.Score(candMode, target)
	if distMode < 0.29 || distMode > 0.31 {
		t.Errorf("expected mode penalty 0.30, got %f", distMode)
	}

	// HA penalty: multi_region true vs target false (0.50)
	candHA := domain.PriceObservation{
		DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
			DataModel:     "document",
			PricingMode:   "provisioned",
			ReadUnits:     100,
			WriteUnits:    20,
			StorageGB:     50,
			MultiRegion:   true,
			ComponentType: "composite",
		},
	}
	distHA, _, _ := scorer.Score(candHA, target)
	if distHA < 0.49 || distHA > 0.51 {
		t.Errorf("expected HA penalty 0.50, got %f", distHA)
	}
}

func TestMatchNoSQLObservations_AWS(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	obsList := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-DDB-READ-PROV",
			PriceAmount:     decimal.NewFromFloat(0.00013),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				ReadUnits:     1,
				ComponentType: "throughput",
			},
			FetchedAt: fixedTime,
		},
		{
			Provider:        "aws",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-DDB-WRITE-PROV",
			PriceAmount:     decimal.NewFromFloat(0.00065),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				WriteUnits:    1,
				ComponentType: "throughput",
			},
			FetchedAt: fixedTime,
		},
		{
			Provider:        "aws",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-DDB-STORAGE",
			PriceAmount:     decimal.NewFromFloat(0.25),
			PriceCurrency:   "USD",
			Unit:            "GB-Mo",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				StorageGB:     1,
				StorageClass:  "standard",
				ComponentType: "storage",
			},
			FetchedAt: fixedTime,
		},
	}

	target := service.MatchTarget{
		DataModel:        "document",
		PricingMode:      "provisioned",
		ReadUnits:        400, // 400 reads/sec = 100 RCUs
		WriteUnits:       50,  // 50 writes/sec = 50 WCUs
		NoSQLStorageGB:   100, // 100 GB
		NoSQLMultiRegion: false,
		Category:         "database_nosql",
	}

	thresholds := service.ThresholdsForCategory("database_nosql")
	res, err := service.MatchNoSQLObservations(obsList, target, thresholds)
	if err != nil {
		t.Fatalf("MatchNoSQLObservations() failed: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil MatchResult")
	}

	if res.MatchQuality != "exact" {
		t.Errorf("expected exact match quality, got %s", res.MatchQuality)
	}
	if res.MatchDeltaPct != 0.0 {
		t.Errorf("expected 0.0 delta pct, got %f", res.MatchDeltaPct)
	}

	// Hourly cost calculation verification:
	// 100 RCU * 0.00013 = 0.013
	// 50 WCU * 0.00065 = 0.0325
	// 100 GB * 0.25 / 730 = 0.0342465753...
	// Total Hourly = 0.013 + 0.0325 + 0.0342465753 = ~0.0797465753
	expectedHourly := decimal.NewFromFloat(100 * 0.00013).
		Add(decimal.NewFromFloat(50 * 0.00065)).
		Add(decimal.NewFromFloat(100 * 0.25).Div(service.HoursInMonth))

	if !res.Observation.PriceAmount.Equal(expectedHourly) {
		t.Errorf("expected hourly price %s, got %s", expectedHourly, res.Observation.PriceAmount)
	}
}

func TestMatchNoSQLObservations_Azure(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	obsList := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AZURE-100RU",
			PriceAmount:     decimal.NewFromFloat(0.008),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			Region:          "eastus",
			RegionGroup:     "us-east",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				ReadUnits:     100,
				WriteUnits:    20,
				ComponentType: "throughput",
			},
			FetchedAt: fixedTime,
		},
		{
			Provider:        "azure",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AZURE-STORAGE",
			PriceAmount:     decimal.NewFromFloat(0.25),
			PriceCurrency:   "USD",
			Unit:            "GB-Mo",
			Region:          "eastus",
			RegionGroup:     "us-east",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				StorageGB:     1,
				StorageClass:  "standard",
				ComponentType: "storage",
			},
			FetchedAt: fixedTime,
		},
	}

	target := service.MatchTarget{
		DataModel:        "document",
		PricingMode:      "provisioned",
		ReadUnits:        200, // 200 * 1 = 200 RU
		WriteUnits:       60,  // 60 * 5 = 300 RU => Total = 500 RU
		NoSQLStorageGB:   200,
		NoSQLMultiRegion: false,
		Category:         "database_nosql",
	}

	thresholds := service.ThresholdsForCategory("database_nosql")
	res, err := service.MatchNoSQLObservations(obsList, target, thresholds)
	if err != nil {
		t.Fatalf("MatchNoSQLObservations() failed: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil MatchResult")
	}

	if res.MatchQuality != "exact" {
		t.Errorf("expected exact match quality, got %s", res.MatchQuality)
	}

	// 500 RU / 100 * 0.008 = 0.04
	// 200 GB * 0.25 / 730 = 0.06849315...
	expectedHourly := decimal.NewFromFloat(5 * 0.008).
		Add(decimal.NewFromFloat(200 * 0.25).Div(service.HoursInMonth))

	if !res.Observation.PriceAmount.Equal(expectedHourly) {
		t.Errorf("expected hourly price %s, got %s", expectedHourly, res.Observation.PriceAmount)
	}
}

func TestMatchNoSQLObservations_GCP(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	obsList := []domain.PriceObservation{
		{
			Provider:        "gcp",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-GCP-READS",
			PriceAmount:     decimal.NewFromFloat(0.03), // $0.03 per 100k
			PriceCurrency:   "USD",
			Unit:            "100k-ops",
			Region:          "us-east4",
			RegionGroup:     "us-east",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "on_demand",
				ReadUnits:     100000,
				ComponentType: "request_operations",
			},
			FetchedAt: fixedTime,
		},
		{
			Provider:        "gcp",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-GCP-WRITES",
			PriceAmount:     decimal.NewFromFloat(0.09), // $0.09 per 100k
			PriceCurrency:   "USD",
			Unit:            "100k-ops",
			Region:          "us-east4",
			RegionGroup:     "us-east",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "on_demand",
				WriteUnits:    100000,
				ComponentType: "request_operations",
			},
			FetchedAt: fixedTime,
		},
		{
			Provider:        "gcp",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-GCP-STORAGE",
			PriceAmount:     decimal.NewFromFloat(0.18),
			PriceCurrency:   "USD",
			Unit:            "GB-Mo",
			Region:          "us-east4",
			RegionGroup:     "us-east",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				StorageGB:     1,
				StorageClass:  "standard",
				ComponentType: "storage",
			},
			FetchedAt: fixedTime,
		},
	}

	target := service.MatchTarget{
		DataModel:        "document",
		PricingMode:      "on_demand",
		ReadUnits:        50, // 50 * 3600 = 180,000 reads/hr => 1.8 * 0.03 = 0.054
		WriteUnits:       10, // 10 * 3600 = 36,000 writes/hr => 0.36 * 0.09 = 0.0324
		NoSQLStorageGB:   50, // 50 * 0.18 / 730
		NoSQLMultiRegion: false,
		Category:         "database_nosql",
	}

	thresholds := service.ThresholdsForCategory("database_nosql")
	res, err := service.MatchNoSQLObservations(obsList, target, thresholds)
	if err != nil {
		t.Fatalf("MatchNoSQLObservations() failed: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil MatchResult")
	}

	if res.MatchQuality != "exact" {
		t.Errorf("expected exact match quality, got %s", res.MatchQuality)
	}

	expectedHourly := decimal.NewFromFloat(1.8 * 0.03).
		Add(decimal.NewFromFloat(0.36 * 0.09)).
		Add(decimal.NewFromFloat(50 * 0.18).Div(service.HoursInMonth))

	if !res.Observation.PriceAmount.Equal(expectedHourly) {
		t.Errorf("expected hourly price %s, got %s", expectedHourly, res.Observation.PriceAmount)
	}
}
