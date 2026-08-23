package service

import (
	"math"
	"sort"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

// Weights and penalties for NoSQL database distance scoring formula.
const (
	WeightNoSQLReads      = 1.0
	WeightNoSQLWrites     = 1.0
	WeightNoSQLStorage    = 0.5
	PenaltyNoSQLMode      = 0.30
	PenaltyNoSQLHighAvail = 0.50
	PenaltyNoSQLModel     = 0.40
)

// NoSQLScorer implements CategoryScorer for NoSQL database resources.
// Distance formula:
//
//	Distance = w_reads * Δreads + w_writes * Δwrites + w_storage * Δstorage + Penalty_mode + Penalty_ha + Penalty_model
//	where:
//	  Δreads    = |C.read_units - T.read_units| / T.read_units (if T.read_units > 0)
//	  Δwrites   = |C.write_units - T.write_units| / T.write_units (if T.write_units > 0)
//	  Δstorage  = |C.storage_gb - T.storage_gb| / T.storage_gb (if T.storage_gb > 0)
//	  Penalty_mode = 0.30 (if C.pricing_mode != T.pricing_mode)
//	  Penalty_ha   = 0.50 (if C.multi_region != T.multi_region)
//	  Penalty_model = 0.40 (if incompatible data models)
type NoSQLScorer struct{}

// Score computes weighted distance between a NoSQL candidate and the target spec.
func (s NoSQLScorer) Score(candidate domain.PriceObservation, target MatchTarget) (float64, []string, bool) {
	candAttrs := candidate.DatabaseNoSQLAttributes

	// Minimum comparable dimensions guard: candidate must provide positive capacity
	if candAttrs.ReadUnits <= 0 && candAttrs.WriteUnits <= 0 && candAttrs.StorageGB <= 0 {
		return 0, nil, false
	}

	var distance float64
	var missingAttrs []string

	// Data model compatibility penalty
	if target.DataModel != "" && candAttrs.DataModel != "" {
		if !areNoSQLModelsCompatible(candAttrs.DataModel, target.DataModel) {
			distance += PenaltyNoSQLModel
		}
	}

	// Pricing mode penalty (provisioned vs on_demand / serverless)
	if target.PricingMode != "" && candAttrs.PricingMode != "" {
		tMode := normalizePricingMode(target.PricingMode)
		cMode := normalizePricingMode(candAttrs.PricingMode)
		if tMode != cMode {
			distance += PenaltyNoSQLMode
		}
	}

	// High Availability / Multi-Region penalty
	if candAttrs.MultiRegion != target.NoSQLMultiRegion {
		distance += PenaltyNoSQLHighAvail
	}

	// Read throughput term (weight = 1.0)
	if target.ReadUnits > 0 {
		if candAttrs.ReadUnits > 0 {
			distance += WeightNoSQLReads * math.Abs(candAttrs.ReadUnits-target.ReadUnits) / target.ReadUnits
		} else {
			missingAttrs = append(missingAttrs, "read_units")
		}
	}

	// Write throughput term (weight = 1.0)
	if target.WriteUnits > 0 {
		if candAttrs.WriteUnits > 0 {
			distance += WeightNoSQLWrites * math.Abs(candAttrs.WriteUnits-target.WriteUnits) / target.WriteUnits
		} else {
			missingAttrs = append(missingAttrs, "write_units")
		}
	}

	// Storage capacity term (weight = 0.5)
	if target.NoSQLStorageGB > 0 {
		if candAttrs.StorageGB > 0 {
			distance += WeightNoSQLStorage * math.Abs(candAttrs.StorageGB-target.NoSQLStorageGB) / target.NoSQLStorageGB
		} else {
			missingAttrs = append(missingAttrs, "storage_gb")
		}
	}

	return distance, missingAttrs, true
}

func areNoSQLModelsCompatible(m1, m2 string) bool {
	s1 := strings.ToLower(strings.TrimSpace(m1))
	s2 := strings.ToLower(strings.TrimSpace(m2))

	if s1 == s2 || s1 == "multi_model" || s2 == "multi_model" {
		return true
	}

	// Document and key_value models are compatible without penalty
	if (s1 == "document" || s1 == "key_value") && (s2 == "document" || s2 == "key_value") {
		return true
	}

	return false
}

func normalizePricingMode(mode string) string {
	m := strings.ToLower(strings.TrimSpace(mode))
	if m == "serverless" || m == "on_demand" || m == "ondemand" || m == "pay_per_request" {
		return "on_demand"
	}
	return "provisioned"
}

// MatchNoSQLObservations orchestrates dynamic query-time assembly of NoSQL throughput and storage
// observations, calculates hourly cost according to cross-cloud unit conversions, scores composite
// candidates, and selects the lowest-distance match.
func MatchNoSQLObservations(obsList []domain.PriceObservation, target MatchTarget, thresholds CategoryThresholds) (*MatchResult, error) {
	if len(obsList) == 0 {
		return nil, nil
	}

	reqReads := target.ReadUnits
	reqWrites := target.WriteUnits
	reqStorage := target.NoSQLStorageGB

	// Fallback to baseline default values if no capacity dimensions were provided
	if reqReads <= 0 && reqWrites <= 0 && reqStorage <= 0 {
		reqReads = 100
		reqWrites = 20
		reqStorage = 10
	}

	effectiveTarget := target
	effectiveTarget.ReadUnits = reqReads
	effectiveTarget.WriteUnits = reqWrites
	effectiveTarget.NoSQLStorageGB = reqStorage

	provider := obsList[0].Provider

	var candidates []domain.PriceObservation

	switch provider {
	case "aws":
		candidates = buildAWSCandidates(obsList, effectiveTarget)
	case "azure":
		candidates = buildAzureCandidates(obsList, effectiveTarget)
	case "gcp":
		candidates = buildGCPCandidates(obsList, effectiveTarget)
	default:
		candidates = buildGenericCandidates(obsList, effectiveTarget)
	}

	if len(candidates) == 0 {
		return nil, ErrNoMatchFound
	}

	type scoredCandidate struct {
		obs          domain.PriceObservation
		distance     float64
		missingAttrs []string
	}

	scorer := NoSQLScorer{}
	var scoredList []scoredCandidate

	for _, cand := range candidates {
		dist, missing, eligible := scorer.Score(cand, effectiveTarget)
		if !eligible {
			continue
		}
		scoredList = append(scoredList, scoredCandidate{
			obs:          cand,
			distance:     dist,
			missingAttrs: missing,
		})
	}

	if len(scoredList) == 0 {
		return nil, ErrNoMatchFound
	}

	// Deterministic sort: lowest distance first, tie-break by SkuID
	sort.Slice(scoredList, func(i, j int) bool {
		if scoredList[i].distance != scoredList[j].distance {
			return scoredList[i].distance < scoredList[j].distance
		}
		return scoredList[i].obs.SkuID < scoredList[j].obs.SkuID
	})

	best := scoredList[0]
	quality := classifyTier(best.distance, thresholds)
	if quality == "none" {
		return nil, ErrNoMatchFound
	}

	missing := best.missingAttrs
	if missing == nil {
		missing = []string{}
	}

	return &MatchResult{
		Observation:       best.obs,
		MatchQuality:      quality,
		MatchDeltaPct:     roundTo2(best.distance * 100),
		MissingAttributes: missing,
	}, nil
}

// buildAWSCandidates constructs composite provisioned and on-demand DynamoDB candidates from raw observations.
func buildAWSCandidates(obsList []domain.PriceObservation, target MatchTarget) []domain.PriceObservation {
	var rcuRow, wcuRow, rruRow, wruRow, storageRow, storageIARow *domain.PriceObservation

	for i := range obsList {
		o := &obsList[i]
		attrs := o.DatabaseNoSQLAttributes
		switch attrs.ComponentType {
		case "throughput":
			if attrs.ReadUnits > 0 {
				rcuRow = o
			}
			if attrs.WriteUnits > 0 {
				wcuRow = o
			}
		case "request_operations":
			if attrs.ReadUnits > 0 {
				rruRow = o
			}
			if attrs.WriteUnits > 0 {
				wruRow = o
			}
		case "storage":
			if attrs.StorageClass == "infrequent_access" {
				storageIARow = o
			} else {
				storageRow = o
			}
		}
	}

	var candidates []domain.PriceObservation

	// 1. Provisioned candidate:
	// 1 RCU = 4 reads/sec (1 KB payload); 1 WCU = 1 write/sec (1 KB payload).
	if rcuRow != nil || wcuRow != nil || storageRow != nil {
		rcuCount := math.Ceil(target.ReadUnits / 4.0)
		if rcuCount < 1 && target.ReadUnits > 0 {
			rcuCount = 1
		}
		wcuCount := target.WriteUnits
		if wcuCount < 1 && target.WriteUnits > 0 {
			wcuCount = 1
		}

		var throughputHourly decimal.Decimal
		if rcuRow != nil && rcuCount > 0 {
			throughputHourly = throughputHourly.Add(rcuRow.PriceAmount.Mul(decimal.NewFromFloat(rcuCount)))
		}
		if wcuRow != nil && wcuCount > 0 {
			throughputHourly = throughputHourly.Add(wcuRow.PriceAmount.Mul(decimal.NewFromFloat(wcuCount)))
		}

		selectedStorage := storageRow
		if target.StorageClass == "infrequent_access" && storageIARow != nil {
			selectedStorage = storageIARow
		}

		var storageHourly decimal.Decimal
		if selectedStorage != nil && target.NoSQLStorageGB > 0 {
			storageHourly = CalculateStorageHourlyCost(selectedStorage.PriceAmount, decimal.NewFromFloat(target.NoSQLStorageGB))
		}

		totalHourly := throughputHourly.Add(storageHourly)

		baseObs := obsList[0]
		if rcuRow != nil {
			baseObs = *rcuRow
		} else if storageRow != nil {
			baseObs = *storageRow
		}

		cand := baseObs
		cand.SkuID = "AWS-DYNAMODB-PROVISIONED"
		cand.DisplayName = "Amazon DynamoDB (Provisioned)"
		cand.Unit = "hour"
		cand.PriceAmount = totalHourly
		cand.DatabaseNoSQLAttributes = domain.DatabaseNoSQLAttributes{
			DataModel:     "document",
			PricingMode:   "provisioned",
			ReadUnits:     target.ReadUnits,
			WriteUnits:    target.WriteUnits,
			StorageGB:     target.NoSQLStorageGB,
			StorageClass:  "standard",
			MultiRegion:   target.NoSQLMultiRegion,
			ComponentType: "composite",
		}
		candidates = append(candidates, cand)
	}

	// 2. On-Demand candidate:
	// Read/Write Request Units billed per million operations. Hourly operations = units/sec * 3600.
	if rruRow != nil || wruRow != nil {
		hourlyReads := target.ReadUnits * 3600.0
		hourlyWrites := target.WriteUnits * 3600.0

		var throughputHourly decimal.Decimal
		if rruRow != nil && hourlyReads > 0 {
			rruCost := decimal.NewFromFloat(hourlyReads / 1_000_000.0).Mul(rruRow.PriceAmount)
			throughputHourly = throughputHourly.Add(rruCost)
		}
		if wruRow != nil && hourlyWrites > 0 {
			wruCost := decimal.NewFromFloat(hourlyWrites / 1_000_000.0).Mul(wruRow.PriceAmount)
			throughputHourly = throughputHourly.Add(wruCost)
		}

		selectedStorage := storageRow
		if target.StorageClass == "infrequent_access" && storageIARow != nil {
			selectedStorage = storageIARow
		}

		var storageHourly decimal.Decimal
		if selectedStorage != nil && target.NoSQLStorageGB > 0 {
			storageHourly = CalculateStorageHourlyCost(selectedStorage.PriceAmount, decimal.NewFromFloat(target.NoSQLStorageGB))
		}

		totalHourly := throughputHourly.Add(storageHourly)

		baseObs := obsList[0]
		if rruRow != nil {
			baseObs = *rruRow
		}

		cand := baseObs
		cand.SkuID = "AWS-DYNAMODB-ON-DEMAND"
		cand.DisplayName = "Amazon DynamoDB (On-Demand)"
		cand.Unit = "hour"
		cand.PriceAmount = totalHourly
		cand.DatabaseNoSQLAttributes = domain.DatabaseNoSQLAttributes{
			DataModel:     "document",
			PricingMode:   "on_demand",
			ReadUnits:     target.ReadUnits,
			WriteUnits:    target.WriteUnits,
			StorageGB:     target.NoSQLStorageGB,
			StorageClass:  "standard",
			MultiRegion:   target.NoSQLMultiRegion,
			ComponentType: "composite",
		}
		candidates = append(candidates, cand)
	}

	return candidates
}

// buildAzureCandidates constructs composite provisioned and serverless Cosmos DB candidates from raw observations.
func buildAzureCandidates(obsList []domain.PriceObservation, target MatchTarget) []domain.PriceObservation {
	var prov100RURow, serverlessRow, storageRow, storageAnalyticalRow *domain.PriceObservation

	for i := range obsList {
		o := &obsList[i]
		attrs := o.DatabaseNoSQLAttributes
		switch attrs.ComponentType {
		case "throughput":
			prov100RURow = o
		case "request_operations":
			serverlessRow = o
		case "storage":
			if attrs.StorageClass == "analytical" {
				storageAnalyticalRow = o
			} else {
				storageRow = o
			}
		}
	}

	var candidates []domain.PriceObservation

	// 1. Provisioned RU/s candidate:
	// 1 KB baseline: 1 read = 1 RU, 1 write = 5 RU. Total RU/s = R * 1 + W * 5. Billed per 100 RU/s-hour.
	if prov100RURow != nil || storageRow != nil {
		neededRUs := target.ReadUnits*1.0 + target.WriteUnits*5.0
		if neededRUs < 1 && (target.ReadUnits > 0 || target.WriteUnits > 0) {
			neededRUs = 100
		}

		var throughputHourly decimal.Decimal
		if prov100RURow != nil && neededRUs > 0 {
			hundredRUs := neededRUs / 100.0
			throughputHourly = decimal.NewFromFloat(hundredRUs).Mul(prov100RURow.PriceAmount)
		}

		selectedStorage := storageRow
		if target.StorageClass == "analytical" && storageAnalyticalRow != nil {
			selectedStorage = storageAnalyticalRow
		}

		var storageHourly decimal.Decimal
		if selectedStorage != nil && target.NoSQLStorageGB > 0 {
			storageHourly = CalculateStorageHourlyCost(selectedStorage.PriceAmount, decimal.NewFromFloat(target.NoSQLStorageGB))
		}

		totalHourly := throughputHourly.Add(storageHourly)

		baseObs := obsList[0]
		if prov100RURow != nil {
			baseObs = *prov100RURow
		}

		cand := baseObs
		cand.SkuID = "AZURE-COSMOS-PROVISIONED"
		cand.DisplayName = "Azure Cosmos DB (Provisioned)"
		cand.Unit = "hour"
		cand.PriceAmount = totalHourly
		cand.DatabaseNoSQLAttributes = domain.DatabaseNoSQLAttributes{
			DataModel:     "document",
			PricingMode:   "provisioned",
			ReadUnits:     target.ReadUnits,
			WriteUnits:    target.WriteUnits,
			StorageGB:     target.NoSQLStorageGB,
			StorageClass:  "standard",
			MultiRegion:   target.NoSQLMultiRegion,
			ComponentType: "composite",
		}
		candidates = append(candidates, cand)
	}

	// 2. Serverless consumption candidate:
	// Consumed RU/hour = (R * 1 + W * 5) * 3600. Billed per 1M RUs.
	if serverlessRow != nil {
		neededRUsPerHour := (target.ReadUnits*1.0 + target.WriteUnits*5.0) * 3600.0

		var throughputHourly decimal.Decimal
		if neededRUsPerHour > 0 {
			millionRUs := neededRUsPerHour / 1_000_000.0
			throughputHourly = decimal.NewFromFloat(millionRUs).Mul(serverlessRow.PriceAmount)
		}

		selectedStorage := storageRow
		if target.StorageClass == "analytical" && storageAnalyticalRow != nil {
			selectedStorage = storageAnalyticalRow
		}

		var storageHourly decimal.Decimal
		if selectedStorage != nil && target.NoSQLStorageGB > 0 {
			storageHourly = CalculateStorageHourlyCost(selectedStorage.PriceAmount, decimal.NewFromFloat(target.NoSQLStorageGB))
		}

		totalHourly := throughputHourly.Add(storageHourly)

		cand := *serverlessRow
		cand.SkuID = "AZURE-COSMOS-SERVERLESS"
		cand.DisplayName = "Azure Cosmos DB (Serverless)"
		cand.Unit = "hour"
		cand.PriceAmount = totalHourly
		cand.DatabaseNoSQLAttributes = domain.DatabaseNoSQLAttributes{
			DataModel:     "document",
			PricingMode:   "on_demand",
			ReadUnits:     target.ReadUnits,
			WriteUnits:    target.WriteUnits,
			StorageGB:     target.NoSQLStorageGB,
			StorageClass:  "standard",
			MultiRegion:   target.NoSQLMultiRegion,
			ComponentType: "composite",
		}
		candidates = append(candidates, cand)
	}

	return candidates
}

// buildGCPCandidates constructs composite Firestore on-demand candidates from raw operational observations.
func buildGCPCandidates(obsList []domain.PriceObservation, target MatchTarget) []domain.PriceObservation {
	var readRow, writeRow, storageRow *domain.PriceObservation

	for i := range obsList {
		o := &obsList[i]
		attrs := o.DatabaseNoSQLAttributes
		switch attrs.ComponentType {
		case "request_operations":
			if attrs.ReadUnits > 0 {
				readRow = o
			}
			if attrs.WriteUnits > 0 {
				writeRow = o
			}
		case "storage":
			storageRow = o
		}
	}

	var candidates []domain.PriceObservation

	hourlyReads := target.ReadUnits * 3600.0
	hourlyWrites := target.WriteUnits * 3600.0

	var throughputHourly decimal.Decimal
	if readRow != nil && hourlyReads > 0 {
		hundredKReads := hourlyReads / 100_000.0
		throughputHourly = throughputHourly.Add(decimal.NewFromFloat(hundredKReads).Mul(readRow.PriceAmount))
	}
	if writeRow != nil && hourlyWrites > 0 {
		hundredKWrites := hourlyWrites / 100_000.0
		throughputHourly = throughputHourly.Add(decimal.NewFromFloat(hundredKWrites).Mul(writeRow.PriceAmount))
	}

	var storageHourly decimal.Decimal
	if storageRow != nil && target.NoSQLStorageGB > 0 {
		storageHourly = CalculateStorageHourlyCost(storageRow.PriceAmount, decimal.NewFromFloat(target.NoSQLStorageGB))
	}

	totalHourly := throughputHourly.Add(storageHourly)

	baseObs := obsList[0]
	if readRow != nil {
		baseObs = *readRow
	} else if storageRow != nil {
		baseObs = *storageRow
	}

	cand := baseObs
	cand.SkuID = "GCP-FIRESTORE-ON-DEMAND"
	cand.DisplayName = "Cloud Firestore"
	cand.Unit = "hour"
	cand.PriceAmount = totalHourly
	cand.DatabaseNoSQLAttributes = domain.DatabaseNoSQLAttributes{
		DataModel:     "document",
		PricingMode:   "on_demand",
		ReadUnits:     target.ReadUnits,
		WriteUnits:    target.WriteUnits,
		StorageGB:     target.NoSQLStorageGB,
		StorageClass:  "standard",
		MultiRegion:   target.NoSQLMultiRegion,
		ComponentType: "composite",
	}
	candidates = append(candidates, cand)

	return candidates
}

func buildGenericCandidates(obsList []domain.PriceObservation, target MatchTarget) []domain.PriceObservation {
	var candidates []domain.PriceObservation
	for _, o := range obsList {
		cand := o
		cand.DatabaseNoSQLAttributes.ReadUnits = target.ReadUnits
		cand.DatabaseNoSQLAttributes.WriteUnits = target.WriteUnits
		cand.DatabaseNoSQLAttributes.StorageGB = target.NoSQLStorageGB
		candidates = append(candidates, cand)
	}
	return candidates
}
