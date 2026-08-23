package service

import (
	"fmt"
	"sort"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/serverlessarchmap"
)

// Penalty for serverless plan tier mismatch.
const (
	PenaltyServerlessTier = 0.20
)

var (
	decZero = decimal.Zero
	dec1000 = decimal.RequireFromString("1000")
	dec1024 = decimal.RequireFromString("1024")
	dec1M   = decimal.RequireFromString("1000000")
	dec10   = decimal.RequireFromString("10")

	// Monthly free tier allowances
	awsFreeRequests    = decimal.RequireFromString("1000000")
	awsFreeGBSeconds   = decimal.RequireFromString("400000")
	azureFreeRequests  = decimal.RequireFromString("1000000")
	azureFreeGBSeconds = decimal.RequireFromString("400000")
	gcpFreeRequests    = decimal.RequireFromString("2000000")
	gcpFreeGBSeconds   = decimal.RequireFromString("400000")
)

// ServerlessScorer implements CategoryScorer for serverless compute resources.
// Distance formula evaluates categorical tier differences:
//
//	Distance = Penalty_tier
//	where:
//	  Penalty_tier = 0.0  (if candidate.Tier == target.Tier)
//	  Penalty_tier = 0.20 (if candidate.Tier != target.Tier)
//
// Continuous hardware dimensions (MemoryMB, ExecutionDurationMS, RequestsPerMonth)
// are cost inputs only and are omitted from distance scoring.
type ServerlessScorer struct{}

// Score computes the categorical tier distance between a serverless candidate and target spec.
func (s ServerlessScorer) Score(candidate domain.PriceObservation, target MatchTarget) (float64, []string, bool) {
	if candidate.ServiceCategory != "serverless" {
		return 0, nil, false
	}

	targetTier := target.ServerlessTier
	if targetTier == "" {
		targetTier = domain.ServerlessTierConsumption
	}

	candTier := candidate.ServerlessRateAttributes.Tier
	if candTier == "" {
		candTier = domain.ServerlessTierConsumption
	}

	var distance float64
	if !strings.EqualFold(candTier, targetTier) {
		distance = PenaltyServerlessTier
	}

	return distance, []string{}, true
}

// MatchServerlessObservations joins multi-component serverless observations (requests and compute duration),
// enforces hard CPU architecture pre-filtering, calculates workload costs with free-tier netting,
// scores candidates, and returns the lowest-distance match.
func MatchServerlessObservations(obsList []domain.PriceObservation, target MatchTarget, thresholds CategoryThresholds) (*MatchResult, error) {
	if len(obsList) == 0 {
		return nil, nil
	}

	reqArch := target.ServerlessArchitecture
	if reqArch == "" {
		reqArch = serverlessarchmap.ArchX86_64
	}

	provider := obsList[0].Provider

	// Hard Architecture Pre-filter:
	// If caller requested arm64 and provider is Azure or GCP (which do not support arm64 serverless),
	// fail fast with ErrArchitectureUnsupported.
	if strings.EqualFold(reqArch, serverlessarchmap.ArchARM64) && (provider == "azure" || provider == "gcp") {
		return nil, ErrArchitectureUnsupported
	}

	// Filter observations by requested architecture
	var archObs []domain.PriceObservation
	for _, o := range obsList {
		if strings.EqualFold(o.ServerlessRateAttributes.Architecture, reqArch) {
			archObs = append(archObs, o)
		}
	}

	if len(archObs) == 0 {
		if strings.EqualFold(reqArch, serverlessarchmap.ArchARM64) {
			return nil, ErrArchitectureUnsupported
		}
		return nil, ErrNoMatchFound
	}

	// Workload dimensions defaults: 1M requests/mo, 512 MB memory, 200 ms duration
	reqRequests := target.RequestsPerMonth
	if reqRequests <= 0 {
		reqRequests = 1_000_000
	}
	reqMemory := target.MemoryMB
	if reqMemory <= 0 {
		reqMemory = 512
	}
	reqDuration := target.ExecutionDurationMS
	if reqDuration <= 0 {
		reqDuration = 200
	}

	effectiveTarget := target
	effectiveTarget.ServerlessArchitecture = reqArch
	effectiveTarget.RequestsPerMonth = reqRequests
	effectiveTarget.MemoryMB = reqMemory
	effectiveTarget.ExecutionDurationMS = reqDuration

	var candidates []domain.PriceObservation
	switch provider {
	case "aws":
		candidates = buildAWSServerlessCandidates(archObs, effectiveTarget)
	case "azure":
		candidates = buildAzureServerlessCandidates(archObs, effectiveTarget)
	case "gcp":
		candidates = buildGCPServerlessCandidates(archObs, effectiveTarget)
	default:
		candidates = buildGenericServerlessCandidates(archObs, effectiveTarget)
	}

	if len(candidates) == 0 {
		return nil, ErrNoMatchFound
	}

	type scoredCandidate struct {
		obs          domain.PriceObservation
		distance     float64
		missingAttrs []string
	}

	scorer := ServerlessScorer{}
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

	// Sort: lowest distance first, tie-break by SkuID lexicographically.
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

// CalculateServerlessCost calculates monthly and hourly serverless costs using decimal arithmetic and free-tier netting.
func CalculateServerlessCost(provider, tier string, reqFeePrice decimal.Decimal, reqUnit string, durationFeePrice decimal.Decimal, requestsPerMonth, memoryMB, durationMS float64) (monthlyCost, hourlyCost decimal.Decimal) {
	decRequests := decimal.NewFromFloat(requestsPerMonth)
	decMemoryMB := decimal.NewFromFloat(memoryMB)
	decDurationMS := decimal.NewFromFloat(durationMS)

	// Hardware dimension derivations
	durationSec := decDurationMS.Div(dec1000)
	memoryGB := decMemoryMB.Div(dec1024)
	monthlyGBSeconds := decRequests.Mul(memoryGB).Mul(durationSec)

	var freeRequests decimal.Decimal
	var freeGBSeconds decimal.Decimal

	switch provider {
	case "aws":
		freeRequests = awsFreeRequests
		freeGBSeconds = awsFreeGBSeconds
	case "azure":
		if tier != domain.ServerlessTierFlexConsumption {
			freeRequests = azureFreeRequests
			freeGBSeconds = azureFreeGBSeconds
		}
	case "gcp":
		freeRequests = gcpFreeRequests
		freeGBSeconds = gcpFreeGBSeconds
	}

	billableRequests := decimal.Max(decZero, decRequests.Sub(freeRequests))
	billableGBSeconds := decimal.Max(decZero, monthlyGBSeconds.Sub(freeGBSeconds))

	// Request fee calculation
	var reqCost decimal.Decimal
	if !reqFeePrice.IsZero() && !billableRequests.IsZero() {
		switch {
		case strings.Contains(reqUnit, "10"):
			// Azure unit of 10 executions
			reqCost = billableRequests.Div(dec10).Mul(reqFeePrice)
		case reqFeePrice.GreaterThanOrEqual(decimal.NewFromFloat(0.01)):
			// Price given per 1,000,000 requests (e.g. $0.20 or $0.40)
			reqCost = billableRequests.Div(dec1M).Mul(reqFeePrice)
		default:
			// Price given per individual request / call (e.g. $0.00000020 or $0.00000040)
			reqCost = billableRequests.Mul(reqFeePrice)
		}
	}

	// Duration fee calculation
	var durCost decimal.Decimal
	if !durationFeePrice.IsZero() && !billableGBSeconds.IsZero() {
		durCost = billableGBSeconds.Mul(durationFeePrice)
	}

	monthlyCost = reqCost.Add(durCost)
	hourlyCost = monthlyCost.Div(HoursInMonth)

	return monthlyCost, hourlyCost
}

func buildAWSServerlessCandidates(obsList []domain.PriceObservation, target MatchTarget) []domain.PriceObservation {
	var reqRow, durRow *domain.PriceObservation

	for i := range obsList {
		o := &obsList[i]
		if o.ServerlessRateAttributes.ComponentType == domain.ComponentTypeRequestFee {
			reqRow = o
		} else if o.ServerlessRateAttributes.ComponentType == domain.ComponentTypeDurationFee {
			durRow = o
		}
	}

	if reqRow == nil && durRow == nil {
		return nil
	}

	var reqPrice decimal.Decimal
	var reqUnit string
	if reqRow != nil {
		reqPrice = reqRow.PriceAmount
		reqUnit = reqRow.Unit
	}

	var durPrice decimal.Decimal
	if durRow != nil {
		durPrice = durRow.PriceAmount
	}

	_, hourlyCost := CalculateServerlessCost("aws", domain.ServerlessTierConsumption, reqPrice, reqUnit, durPrice, target.RequestsPerMonth, target.MemoryMB, target.ExecutionDurationMS)

	baseObs := obsList[0]
	if reqRow != nil {
		baseObs = *reqRow
	} else if durRow != nil {
		baseObs = *durRow
	}

	sku := fmt.Sprintf("AWS-LAMBDA-%s", strings.ToUpper(target.ServerlessArchitecture))
	displayName := fmt.Sprintf("AWS Lambda (%s)", target.ServerlessArchitecture)

	cand := baseObs
	cand.SkuID = sku
	cand.DisplayName = displayName
	cand.Unit = "hour"
	cand.PriceAmount = hourlyCost
	cand.ServerlessRateAttributes = domain.ServerlessRateAttributes{
		Architecture:  target.ServerlessArchitecture,
		Tier:          domain.ServerlessTierConsumption,
		ComponentType: "composite",
	}

	return []domain.PriceObservation{cand}
}

func buildAzureServerlessCandidates(obsList []domain.PriceObservation, target MatchTarget) []domain.PriceObservation {
	type tierRows struct {
		reqRow *domain.PriceObservation
		durRow *domain.PriceObservation
	}

	tierMap := make(map[string]*tierRows)
	for i := range obsList {
		o := &obsList[i]
		tier := o.ServerlessRateAttributes.Tier
		if tier == "" {
			tier = domain.ServerlessTierConsumption
		}
		if _, ok := tierMap[tier]; !ok {
			tierMap[tier] = &tierRows{}
		}
		if o.ServerlessRateAttributes.ComponentType == domain.ComponentTypeRequestFee {
			tierMap[tier].reqRow = o
		} else if o.ServerlessRateAttributes.ComponentType == domain.ComponentTypeDurationFee {
			tierMap[tier].durRow = o
		}
	}

	var candidates []domain.PriceObservation
	for tier, rows := range tierMap {
		var reqPrice decimal.Decimal
		var reqUnit string
		if rows.reqRow != nil {
			reqPrice = rows.reqRow.PriceAmount
			reqUnit = rows.reqRow.Unit
		}

		var durPrice decimal.Decimal
		if rows.durRow != nil {
			durPrice = rows.durRow.PriceAmount
		}

		_, hourlyCost := CalculateServerlessCost("azure", tier, reqPrice, reqUnit, durPrice, target.RequestsPerMonth, target.MemoryMB, target.ExecutionDurationMS)

		baseObs := obsList[0]
		if rows.reqRow != nil {
			baseObs = *rows.reqRow
		} else if rows.durRow != nil {
			baseObs = *rows.durRow
		}

		sku := fmt.Sprintf("AZURE-FUNCTIONS-%s", strings.ToUpper(tier))
		displayName := fmt.Sprintf("Azure Functions (%s)", tier)

		cand := baseObs
		cand.SkuID = sku
		cand.DisplayName = displayName
		cand.Unit = "hour"
		cand.PriceAmount = hourlyCost
		cand.ServerlessRateAttributes = domain.ServerlessRateAttributes{
			Architecture:  target.ServerlessArchitecture,
			Tier:          tier,
			ComponentType: "composite",
		}

		candidates = append(candidates, cand)
	}

	return candidates
}

func buildGCPServerlessCandidates(obsList []domain.PriceObservation, target MatchTarget) []domain.PriceObservation {
	var reqRow, durRow *domain.PriceObservation

	for i := range obsList {
		o := &obsList[i]
		if o.ServerlessRateAttributes.ComponentType == domain.ComponentTypeRequestFee {
			reqRow = o
		} else if o.ServerlessRateAttributes.ComponentType == domain.ComponentTypeDurationFee {
			durRow = o
		}
	}

	if reqRow == nil && durRow == nil {
		return nil
	}

	var reqPrice decimal.Decimal
	var reqUnit string
	if reqRow != nil {
		reqPrice = reqRow.PriceAmount
		reqUnit = reqRow.Unit
	}

	var durPrice decimal.Decimal
	if durRow != nil {
		durPrice = durRow.PriceAmount
	}

	tier := domain.ServerlessTierConsumption
	if reqRow != nil && reqRow.ServerlessRateAttributes.Tier != "" {
		tier = reqRow.ServerlessRateAttributes.Tier
	}

	_, hourlyCost := CalculateServerlessCost("gcp", tier, reqPrice, reqUnit, durPrice, target.RequestsPerMonth, target.MemoryMB, target.ExecutionDurationMS)

	baseObs := obsList[0]
	if reqRow != nil {
		baseObs = *reqRow
	} else if durRow != nil {
		baseObs = *durRow
	}

	cand := baseObs
	cand.SkuID = "GCP-CLOUD-FUNCTIONS"
	cand.DisplayName = "Cloud Functions"
	cand.Unit = "hour"
	cand.PriceAmount = hourlyCost
	cand.ServerlessRateAttributes = domain.ServerlessRateAttributes{
		Architecture:  target.ServerlessArchitecture,
		Tier:          tier,
		ComponentType: "composite",
	}

	return []domain.PriceObservation{cand}
}

func buildGenericServerlessCandidates(obsList []domain.PriceObservation, target MatchTarget) []domain.PriceObservation {
	var candidates []domain.PriceObservation
	for _, o := range obsList {
		cand := o
		cand.ServerlessRateAttributes.Architecture = target.ServerlessArchitecture
		cand.ServerlessRateAttributes.Tier = target.ServerlessTier
		candidates = append(candidates, cand)
	}
	return candidates
}
