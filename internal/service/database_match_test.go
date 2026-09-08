package service

import (
	"errors"
	"math"
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
		PriceAmount:     decimal.RequireFromString("0.26"),
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
			PriceAmount:     decimal.RequireFromString("0.2600"), // $0.26 / hr
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
			PriceAmount:     decimal.RequireFromString("0.115"), // $0.115 / GB-mo
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
	res, err := MatchDatabaseObservations(obsList, target, thresholds)
	if err != nil || res == nil {
		t.Fatalf("expected match result, got nil (err: %v)", err)
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
	expectedInstance := decimal.RequireFromString("0.26")
	expectedStorage := decimal.RequireFromString("0.115").Mul(decimal.RequireFromString("100")).Div(HoursInMonth)
	expectedHourly := expectedInstance.Add(expectedStorage)
	if !res.Observation.PriceAmount.Equal(expectedHourly) {
		t.Errorf("expected combined hourly cost %s, got %s", expectedHourly, res.Observation.PriceAmount)
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
		PriceAmount:     decimal.RequireFromString("0.26"),
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
	resExact, err := MatchDatabaseObservations([]domain.PriceObservation{inst}, targetExact, thresholds)
	if err != nil || resExact == nil || resExact.MatchQuality != "exact" {
		t.Errorf("expected exact match, got %+v (err: %v)", resExact, err)
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
	resClose, err := MatchDatabaseObservations([]domain.PriceObservation{inst}, targetClose, thresholds)
	if err != nil || resClose == nil || resClose.MatchQuality != "close" {
		t.Errorf("expected close match, got %+v (err: %v)", resClose, err)
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
	resApprox, err := MatchDatabaseObservations([]domain.PriceObservation{inst}, targetApprox, thresholds)
	if err != nil || resApprox == nil || resApprox.MatchQuality != "approximate" {
		t.Errorf("expected approximate match, got %+v (err: %v)", resApprox, err)
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
	resNone, err := MatchDatabaseObservations([]domain.PriceObservation{inst}, targetNone, thresholds)
	if resNone != nil {
		t.Errorf("expected candidate exceeding cutoff to be omitted (nil), got %+v", resNone)
	}
	if !errors.Is(err, ErrNoMatchFound) {
		t.Errorf("expected ErrNoMatchFound, got %v", err)
	}
}

// TestDatabaseRDBMSScorer_IOPSDistanceScoring verifies that candidate IOPS is scored against target IOPS
// without being overwritten, properly computing Δiops and missing_attributes (Item 1).
func TestDatabaseRDBMSScorer_IOPSDistanceScoring(t *testing.T) {
	scorer := DatabaseRDBMSScorer{}
	thresholds := ThresholdsForCategory("database_rdbms")

	candIOPS := 1000
	reqIOPS := 3000

	cand := domain.PriceObservation{
		Provider:        "aws",
		ServiceCategory: "database_rdbms",
		SkuID:           "AWS-RDS-PG-M6G-XLARGE-IOPS1000",
		PriceAmount:     decimal.RequireFromString("0.26"),
		DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
			Engine:         "postgresql",
			VCPU:           4,
			RAMGB:          16,
			StorageGB:      100,
			IOPS:           &candIOPS,
			MultiAZ:        false,
			DeploymentTier: "standard",
			ComponentType:  "instance",
		},
	}

	target := MatchTarget{
		Engine:            "postgresql",
		VCPU:              4,
		RAMGB:             16,
		DatabaseStorageGB: 100,
		DatabaseIOPS:      &reqIOPS,
		MultiAZ:           false,
		Category:          "database_rdbms",
	}

	// Score directly
	dist, missing, eligible := scorer.Score(cand, target)
	if !eligible {
		t.Fatalf("expected candidate to be eligible")
	}

	// Expected distance: w_iops * |1000 - 3000| / 3000 = 0.5 * 2000 / 3000 = 0.3333333333333333
	expectedDist := 0.5 * (2000.0 / 3000.0)
	if math.Abs(dist-expectedDist) > 1e-6 {
		t.Errorf("expected distance %f, got %f (IOPS distance scoring must be active)", expectedDist, dist)
	}
	if len(missing) != 0 {
		t.Errorf("expected 0 missing attributes when candidate has IOPS, got %v", missing)
	}

	// Match through MatchDatabaseObservations
	res, err := MatchDatabaseObservations([]domain.PriceObservation{cand}, target, thresholds)
	if err != nil || res == nil {
		t.Fatalf("expected match result, got nil (err: %v)", err)
	}
	// Distance is ~0.3333, which is > 0.20 (close threshold) and <= 0.50 (approximate threshold)
	if res.MatchQuality != "approximate" {
		t.Errorf("expected match quality 'approximate' for IOPS mismatch, got %s", res.MatchQuality)
	}
	if res.MatchDeltaPct == 0.0 {
		t.Errorf("expected non-zero match_delta_pct, got 0.0")
	}

	// Test missing attribute when candidate has nil IOPS
	candNoIOPS := cand
	candNoIOPS.DatabaseRDBMSAttributes.IOPS = nil
	distNoIOPS, missingNoIOPS, eligibleNoIOPS := scorer.Score(candNoIOPS, target)
	if !eligibleNoIOPS {
		t.Fatalf("expected candidate without IOPS to be eligible")
	}
	if distNoIOPS != 0.0 {
		t.Errorf("expected 0 distance when candidate has nil IOPS, got %f", distNoIOPS)
	}
	if len(missingNoIOPS) != 1 || missingNoIOPS[0] != "iops" {
		t.Errorf("expected missing_attributes ['iops'], got %v", missingNoIOPS)
	}
}

// TestMatchDatabaseObservations_CombinedIOPSPriced verifies that the joined hourly price
// includes all three terms: Price_instance + (StorageGB * Price_storage_hourly) + (IOPS * Price_iops_hourly) (Item 2).
func TestMatchDatabaseObservations_CombinedIOPSPriced(t *testing.T) {
	now := time.Now().UTC()
	reqIOPS := 3000

	obsList := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "database_rdbms",
			SkuID:           "AWS-RDS-PG-M6G-XLARGE",
			Region:          "us-east-1",
			PriceAmount:     decimal.RequireFromString("0.2600"), // Instance: $0.26 / hr
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
			PriceAmount:     decimal.RequireFromString("0.115"), // Storage: $0.115 / GB-mo -> 100 GB = $0.0157534 / hr
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:        "any",
				StorageGB:     1,
				MultiAZ:       false,
				StorageFamily: "gp3",
				ComponentType: "storage",
			},
			FetchedAt: now,
		},
		{
			Provider:        "aws",
			ServiceCategory: "database_rdbms",
			SkuID:           "AWS-RDS-IOPS-GP3",
			Region:          "us-east-1",
			PriceAmount:     decimal.RequireFromString("0.010"), // IOPS: $0.010 / IOPS-mo -> 3000 IOPS = $30/mo = $0.04109589 / hr
			Unit:            "IOPS-Mo",
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:        "any",
				MultiAZ:       false,
				StorageFamily: "gp3",
				ComponentType: "iops",
			},
			FetchedAt: now,
		},
	}

	target := MatchTarget{
		Engine:            "postgresql",
		VCPU:              4,
		RAMGB:             16,
		DatabaseStorageGB: 100,
		DatabaseIOPS:      &reqIOPS,
		MultiAZ:           false,
		Category:          "database_rdbms",
	}

	thresholds := ThresholdsForCategory("database_rdbms")
	res, err := MatchDatabaseObservations(obsList, target, thresholds)
	if err != nil || res == nil {
		t.Fatalf("expected match result, got nil (err: %v)", err)
	}

	// Expected three-term total:
	// Instance: $0.26
	// Storage: 100 * 0.115 / 730 = ~$0.0157534
	// IOPS: 3000 * 0.010 / 730 = ~$0.0410959
	// Total: ~$0.3168493 / hr
	expectedInstance := decimal.RequireFromString("0.26")
	expectedStorage := decimal.RequireFromString("0.115").Mul(decimal.RequireFromString("100")).Div(HoursInMonth)
	expectedIOPS := decimal.RequireFromString("0.010").Mul(decimal.RequireFromString("3000")).Div(HoursInMonth)
	expectedHourly := expectedInstance.Add(expectedStorage).Add(expectedIOPS)
	if !res.Observation.PriceAmount.Equal(expectedHourly) {
		t.Fatalf("expected combined price %s (including IOPS rate), got %s",
			expectedHourly, res.Observation.PriceAmount)
	}
}

// TestMatchDatabaseObservations_EngineMismatchExcluded verifies that when candidates exist
// but none match the requested engine, ErrEngineMismatch is returned (Item 3).
func TestMatchDatabaseObservations_EngineMismatchExcluded(t *testing.T) {
	now := time.Now().UTC()
	thresholds := ThresholdsForCategory("database_rdbms")

	obsList := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "database_rdbms",
			SkuID:           "AWS-RDS-PG-M6G-XLARGE",
			Region:          "us-east-1",
			PriceAmount:     decimal.RequireFromString("0.26"),
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:        "postgresql",
				VCPU:          4,
				RAMGB:         16,
				ComponentType: "instance",
			},
			FetchedAt: now,
		},
	}

	targetMySQL := MatchTarget{
		Engine:            "mysql",
		VCPU:              4,
		RAMGB:             16,
		DatabaseStorageGB: 100,
		Category:          "database_rdbms",
	}

	res, err := MatchDatabaseObservations(obsList, targetMySQL, thresholds)
	if res != nil {
		t.Errorf("expected res to be nil on engine mismatch, got %+v", res)
	}
	if !errors.Is(err, ErrEngineMismatch) {
		t.Fatalf("expected ErrEngineMismatch, got %v", err)
	}
}

// TestMatchDatabaseObservations_GCPCloudSQL_SynthesizedInstance_ExactMatch verifies
// that synthesized GCP Cloud SQL observations (4 vCPU / 16 GB shape from Task 2)
// match exact specifications within acceptable thresholds instead of returning ErrNoMatchFound (Research Finding 1).
func TestMatchDatabaseObservations_GCPCloudSQL_SynthesizedInstance_ExactMatch(t *testing.T) {
	now := time.Now().UTC()
	thresholds := ThresholdsForCategory("database_rdbms")

	obsList := []domain.PriceObservation{
		// Synthesized GCP Cloud SQL instance observation (4 vCPU, 16 GB, PostgreSQL, us-east4 / us-east)
		{
			Provider:        "gcp",
			ServiceCategory: "database_rdbms",
			SkuID:           "SKU-GCP-CLOUDSQL-POSTGRESQL-STANDARD-4VCPU-16GB",
			DisplayName:     "Cloud SQL for PostgreSQL: db-custom-4-16384 (4 vCPU, 16 GB RAM)",
			Region:          "us-east4",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.RequireFromString("0.312"),
			Unit:            "Hrs",
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:         "postgresql",
				VCPU:           4,
				RAMGB:          16,
				StorageGB:      0,
				MultiAZ:        false,
				DeploymentTier: "standard",
				ComponentType:  "instance",
			},
			FetchedAt: now,
		},
		// GCP Cloud SQL storage observation (SSD, us-east4 / us-east)
		{
			Provider:        "gcp",
			ServiceCategory: "database_rdbms",
			SkuID:           "SKU-GCP-CLOUDSQL-STORAGE-SSD",
			DisplayName:     "Cloud SQL for PostgreSQL: Storage PD SSD in Virginia",
			Region:          "us-east4",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.RequireFromString("0.170"),
			Unit:            "GB-Mo",
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:        "postgresql",
				VCPU:          0,
				RAMGB:         0,
				StorageGB:     1,
				MultiAZ:       false,
				StorageFamily: "ssd",
				ComponentType: "storage",
			},
			FetchedAt: now,
		},
	}

	target := MatchTarget{
		Engine:            "postgresql",
		VCPU:              4,
		RAMGB:             16,
		DatabaseStorageGB: 100,
		MultiAZ:           false,
		Category:          "database_rdbms",
	}

	res, err := MatchDatabaseObservations(obsList, target, thresholds)
	if err != nil {
		t.Fatalf("expected successful match for GCP Cloud SQL, got error: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil match result")
	}
	if res.Observation.Provider != "gcp" {
		t.Errorf("expected provider gcp, got %s", res.Observation.Provider)
	}
	if res.MatchQuality != "exact" {
		t.Errorf("expected match_quality exact, got %s (delta_pct=%f)", res.MatchQuality, res.MatchDeltaPct)
	}
	if res.Observation.SkuID != "SKU-GCP-CLOUDSQL-POSTGRESQL-STANDARD-4VCPU-16GB" {
		t.Errorf("expected SkuID SKU-GCP-CLOUDSQL-POSTGRESQL-STANDARD-4VCPU-16GB, got %s", res.Observation.SkuID)
	}
}
