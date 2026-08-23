package service

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

func TestDatabaseRDBMSScorer_ExactMatch(t *testing.T) {
	scorer := DatabaseRDBMSScorer{}
	iops := 3000

	candidate := domain.PriceObservation{
		Provider:        "aws",
		ServiceCategory: "database_rdbms",
		SkuID:           "AWS-RDS-PG-M6G-XLARGE",
		PriceAmount:     decimal.NewFromFloat(0.26),
		DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
			Engine:         "postgresql",
			VCPU:           4,
			RAMGB:          16,
			StorageGB:      100,
			IOPS:           &iops,
			MultiAZ:        true,
			DeploymentTier: "standard",
			ComponentType:  "instance",
		},
	}

	target := MatchTarget{
		Engine:            "postgresql",
		VCPU:              4,
		RAMGB:             16,
		DatabaseStorageGB: 100,
		DatabaseIOPS:      &iops,
		MultiAZ:           true,
		Category:          "database_rdbms",
	}

	dist, missing, eligible := scorer.Score(candidate, target)
	if !eligible {
		t.Fatalf("expected candidate to be eligible, got false")
	}
	if dist != 0.0 {
		t.Errorf("expected distance 0.0 for exact match, got %f", dist)
	}
	if len(missing) != 0 {
		t.Errorf("expected 0 missing attributes, got %v", missing)
	}
}

func TestDatabaseRDBMSScorer_EngineMismatch(t *testing.T) {
	scorer := DatabaseRDBMSScorer{}

	candidate := domain.PriceObservation{
		Provider:        "aws",
		ServiceCategory: "database_rdbms",
		SkuID:           "AWS-RDS-MYSQL-M6G-XLARGE",
		DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
			Engine:        "mysql",
			VCPU:          4,
			RAMGB:         16,
			StorageGB:     100,
			MultiAZ:       false,
			ComponentType: "instance",
		},
	}

	target := MatchTarget{
		Engine:            "postgresql",
		VCPU:              4,
		RAMGB:             16,
		DatabaseStorageGB: 100,
		Category:          "database_rdbms",
	}

	_, _, eligible := scorer.Score(candidate, target)
	if eligible {
		t.Errorf("expected candidate with mismatched engine to be ineligible (hard filter), got eligible=true")
	}
}

func TestDatabaseRDBMSScorer_HighAvailabilityPenalty(t *testing.T) {
	scorer := DatabaseRDBMSScorer{}

	candidateSingleAZ := domain.PriceObservation{
		Provider:        "aws",
		ServiceCategory: "database_rdbms",
		SkuID:           "AWS-RDS-PG-M6G-XLARGE-SINGLE",
		DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
			Engine:        "postgresql",
			VCPU:          4,
			RAMGB:         16,
			StorageGB:     100,
			MultiAZ:       false,
			ComponentType: "instance",
		},
	}

	targetMultiAZ := MatchTarget{
		Engine:            "postgresql",
		VCPU:              4,
		RAMGB:             16,
		DatabaseStorageGB: 100,
		MultiAZ:           true,
		Category:          "database_rdbms",
	}

	dist, _, eligible := scorer.Score(candidateSingleAZ, targetMultiAZ)
	if !eligible {
		t.Fatalf("expected candidate to be eligible")
	}
	// Distance should be 0.50 (HA penalty only)
	if dist != 0.50 {
		t.Errorf("expected distance 0.50 for HA penalty, got %f", dist)
	}
}

func TestMatchDatabaseObservations_QueryTimeJoin(t *testing.T) {
	now := time.Now().UTC()

	// Provider data with separate instance and storage rows
	obsList := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "database_rdbms",
			SkuID:           "AWS-RDS-PG-M6G-XLARGE",
			Region:          "us-east-1",
			PriceAmount:     decimal.NewFromFloat(0.2600), // $0.26 / hr
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:        "postgresql",
				VCPU:          4,
				RAMGB:         16,
				MultiAZ:       false,
				ComponentType: "instance",
			},
			FetchedAt: now,
		},
		{
			Provider:        "aws",
			ServiceCategory: "database_rdbms",
			SkuID:           "AWS-RDS-STORAGE-GP3",
			Region:          "us-east-1",
			PriceAmount:     decimal.NewFromFloat(0.115), // $0.115 / GB-mo
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:        "any",
				StorageGB:     1,
				MultiAZ:       false,
				StorageFamily: "gp3",
				ComponentType: "storage",
			},
			FetchedAt: now,
		},
	}

	target := MatchTarget{
		Engine:            "postgresql",
		VCPU:              4,
		RAMGB:             16,
		DatabaseStorageGB: 100, // 100 GB
		MultiAZ:           false,
		Category:          "database_rdbms",
	}

	thresholds := ThresholdsForCategory("database_rdbms")
	res := MatchDatabaseObservations(obsList, target, thresholds)
	if res == nil {
		t.Fatalf("expected match result, got nil")
	}

	if res.MatchQuality != "exact" {
		t.Errorf("expected match_quality 'exact', got %s", res.MatchQuality)
	}
	if res.MatchDeltaPct != 0.0 {
		t.Errorf("expected match_delta_pct 0.0, got %f", res.MatchDeltaPct)
	}

	// Joined price calculation:
	// Instance: $0.26 / hr
	// Storage: 100 GB * $0.115 / 730 = $0.0157534246575342 / hr
	// Total: ~$0.275753... / hr
	expectedMinHourly := decimal.NewFromFloat(0.27)
	expectedMaxHourly := decimal.NewFromFloat(0.28)
	if res.Observation.PriceAmount.LessThan(expectedMinHourly) || res.Observation.PriceAmount.GreaterThan(expectedMaxHourly) {
		t.Errorf("expected combined hourly cost between %s and %s, got %s", expectedMinHourly, expectedMaxHourly, res.Observation.PriceAmount)
	}
	if res.Observation.DatabaseRDBMSAttributes.StorageGB != 100 {
		t.Errorf("expected joined storage 100 GB, got %f", res.Observation.DatabaseRDBMSAttributes.StorageGB)
	}
	if res.Observation.DatabaseRDBMSAttributes.StorageFamily != "gp3" {
		t.Errorf("expected joined storage family gp3, got %s", res.Observation.DatabaseRDBMSAttributes.StorageFamily)
	}
}

func TestMatchDatabaseObservations_ThresholdsTiers(t *testing.T) {
	thresholds := ThresholdsForCategory("database_rdbms")

	// Instance with 4 vCPU, 16 GB RAM
	inst := domain.PriceObservation{
		Provider:        "aws",
		ServiceCategory: "database_rdbms",
		SkuID:           "AWS-RDS-PG-M6G-XLARGE",
		PriceAmount:     decimal.NewFromFloat(0.26),
		DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
			Engine:        "postgresql",
			VCPU:          4,
			RAMGB:         16,
			MultiAZ:       false,
			ComponentType: "instance",
		},
	}

	// 1. Exact match (4 vCPU, 16 GB RAM)
	targetExact := MatchTarget{
		Engine:            "postgresql",
		VCPU:              4,
		RAMGB:             16,
		DatabaseStorageGB: 100,
		MultiAZ:           false,
		Category:          "database_rdbms",
	}
	resExact := MatchDatabaseObservations([]domain.PriceObservation{inst}, targetExact, thresholds)
	if resExact == nil || resExact.MatchQuality != "exact" {
		t.Errorf("expected exact match, got %+v", resExact)
	}

	// 2. Close match: Requested 4 vCPU, 14 GB RAM -> RAM delta = |16-14|/14 = 0.1428 <= 0.20
	targetClose := MatchTarget{
		Engine:            "postgresql",
		VCPU:              4,
		RAMGB:             14,
		DatabaseStorageGB: 100,
		MultiAZ:           false,
		Category:          "database_rdbms",
	}
	resClose := MatchDatabaseObservations([]domain.PriceObservation{inst}, targetClose, thresholds)
	if resClose == nil || resClose.MatchQuality != "close" {
		t.Errorf("expected close match, got %+v", resClose)
	}

	// 3. Approximate match: Requested 4 vCPU, 11 GB RAM -> RAM delta = |16-11|/11 = 0.4545 <= 0.50
	targetApprox := MatchTarget{
		Engine:            "postgresql",
		VCPU:              4,
		RAMGB:             11,
		DatabaseStorageGB: 100,
		MultiAZ:           false,
		Category:          "database_rdbms",
	}
	resApprox := MatchDatabaseObservations([]domain.PriceObservation{inst}, targetApprox, thresholds)
	if resApprox == nil || resApprox.MatchQuality != "approximate" {
		t.Errorf("expected approximate match, got %+v", resApprox)
	}

	// 4. Exceeding cutoff: Requested 4 vCPU, 8 GB RAM -> RAM delta = |16-8|/8 = 1.0 > 0.50 -> dropped
	targetNone := MatchTarget{
		Engine:            "postgresql",
		VCPU:              4,
		RAMGB:             8,
		DatabaseStorageGB: 100,
		MultiAZ:           false,
		Category:          "database_rdbms",
	}
	resNone := MatchDatabaseObservations([]domain.PriceObservation{inst}, targetNone, thresholds)
	if resNone != nil {
		t.Errorf("expected candidate exceeding cutoff to be omitted (nil), got %+v", resNone)
	}
}
