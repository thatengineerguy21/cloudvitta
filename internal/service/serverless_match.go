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
	dec1769 = decimal.RequireFromString("1769")
	dec400k = decimal.RequireFromString("400000")
	dec200k = decimal.RequireFromString("200000")
	dec360k = decimal.RequireFromString("360000")
	dec180k = decimal.RequireFromString("180000")
	dec2M   = decimal.RequireFromString("2000000")

	// Monthly free tier allowances
	awsFreeRequests    = decimal.RequireFromString("1000000")
	awsFreeGBSeconds   = decimal.RequireFromString("400000")
	azureFreeRequests  = decimal.RequireFromString("1000000")
	azureFreeGBSeconds = decimal.RequireFromString("400000")

	// GCP 1st Gen free tier allowances
	gcp1stGenFreeRequests   = decimal.RequireFromString("2000000")
	gcp1stGenFreeGBSeconds  = decimal.RequireFromString("400000")
	gcp1stGenFreeGHzSeconds = decimal.RequireFromString("200000")

	// GCP 2nd Gen free tier allowances
	gcp2ndGenFreeRequests    = decimal.RequireFromString("2000000")
	gcp2ndGenFreeGiBSeconds  = decimal.RequireFromString("360000")
	gcp2ndGenFreeVCPUSeconds = decimal.RequireFromString("180000")
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

	targetTier := target.ServerlessWorkload.Tier
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

// RawServerlessParams bundles unparsed query strings for serverless workload definition.
type RawServerlessParams struct {
	Architecture        string
	Tier                string
	RequestsPerMonth    string
	MemoryMB            string
	ExecutionDurationMS string
}

// ParseRawServerlessWorkload validates and converts bundled raw string parameters into a ServerlessWorkload.
func ParseRawServerlessWorkload(params RawServerlessParams) (ServerlessWorkload, error) {
	arch := domain.ArchitectureX86_64
	if strings.TrimSpace(params.Architecture) != "" {
		resolved, err := serverlessarchmap.ResolveCanonicalArchitecture(params.Architecture)
		if err != nil || !serverlessarchmap.IsSupportedStageArchitecture(resolved) {
			return ServerlessWorkload{}, fmt.Errorf("%w: architecture must be a supported CPU architecture (x86_64, arm64)", ErrInvalidParameters)
		}
		arch = resolved
	}

	tier := strings.ToLower(strings.TrimSpace(params.Tier))
	if tier == "" {
		tier = domain.ServerlessTierConsumption
	}

	requestsPerMonth := decimal.RequireFromString("1000000")
	if strings.TrimSpace(params.RequestsPerMonth) != "" {
		val, err := decimal.NewFromString(strings.TrimSpace(params.RequestsPerMonth))
		if err != nil || val.IsNegative() {
			return ServerlessWorkload{}, fmt.Errorf("%w: requests_per_month must be a non-negative number", ErrInvalidParameters)
		}
		requestsPerMonth = val
	}

	memoryMB := decimal.RequireFromString("512")
	if strings.TrimSpace(params.MemoryMB) != "" {
		val, err := decimal.NewFromString(strings.TrimSpace(params.MemoryMB))
		if err != nil || val.LessThan(decimal.RequireFromString("128")) || val.GreaterThan(decimal.RequireFromString("10240")) {
			return ServerlessWorkload{}, fmt.Errorf("%w: memory_mb must be between 128 and 10240 MB", ErrInvalidParameters)
		}
		memoryMB = val
	}

	executionDurationMS := decimal.RequireFromString("200")
	if strings.TrimSpace(params.ExecutionDurationMS) != "" {
		val, err := decimal.NewFromString(strings.TrimSpace(params.ExecutionDurationMS))
		if err != nil || val.LessThan(decimal.RequireFromString("1")) || val.GreaterThan(decimal.RequireFromString("900000")) {
			return ServerlessWorkload{}, fmt.Errorf("%w: execution_duration_ms must be between 1 and 900000 ms", ErrInvalidParameters)
		}
		executionDurationMS = val
	}

	return ServerlessWorkload{
		Architecture:        arch,
		Tier:                tier,
		RequestsPerMonth:    requestsPerMonth,
		MemoryMB:            memoryMB,
		ExecutionDurationMS: executionDurationMS,
	}, nil
}

// ParseServerlessWorkload extracts and validates serverless workload parameters from raw string inputs.
func ParseServerlessWorkload(rawArch, rawTier, rawReqs, rawMem, rawDur string) (ServerlessWorkload, error) {
	return ParseRawServerlessWorkload(RawServerlessParams{
		Architecture:        rawArch,
		Tier:                rawTier,
		RequestsPerMonth:    rawReqs,
		MemoryMB:            rawMem,
		ExecutionDurationMS: rawDur,
	})
}

// MatchServerlessObservations joins multi-component serverless observations (requests and compute duration),
// enforces hard CPU architecture pre-filtering, calculates workload costs with free-tier netting,
// scores candidates, and returns the lowest-distance match.
func MatchServerlessObservations(obsList []domain.PriceObservation, target MatchTarget, thresholds CategoryThresholds) (*MatchResult, error) {
	if len(obsList) == 0 {
		return nil, nil
	}

	workload := target.ServerlessWorkload
	if workload.Architecture == "" {
		workload.Architecture = domain.ArchitectureX86_64
	}
	if workload.Tier == "" {
		workload.Tier = domain.ServerlessTierConsumption
	}
	if workload.RequestsPerMonth.LessThanOrEqual(decZero) {
		workload.RequestsPerMonth = dec1M
	}
	if workload.MemoryMB.LessThanOrEqual(decZero) {
		workload.MemoryMB = decimal.RequireFromString("512")
	}
	if workload.ExecutionDurationMS.LessThanOrEqual(decZero) {
		workload.ExecutionDurationMS = decimal.RequireFromString("200")
	}

	provider := obsList[0].Provider

	// Hard Architecture Pre-filter:
	// If caller requested arm64 and provider is Azure or GCP (which do not support arm64 serverless),
	// fail fast with ErrArchitectureUnsupported.
	if strings.EqualFold(workload.Architecture, domain.ArchitectureARM64) && (provider == "azure" || provider == "gcp") {
		return nil, ErrArchitectureUnsupported
	}

	// Filter observations by requested architecture
	var archObs []domain.PriceObservation
	for _, o := range obsList {
		if strings.EqualFold(o.ServerlessRateAttributes.Architecture, workload.Architecture) {
			archObs = append(archObs, o)
		}
	}

	if len(archObs) == 0 {
		if strings.EqualFold(workload.Architecture, domain.ArchitectureARM64) {
			return nil, ErrArchitectureUnsupported
		}
		return nil, ErrNoMatchFound
	}

	effectiveTarget := target
	effectiveTarget.ServerlessWorkload = workload

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

// ServerlessRateComponents encapsulates normalized provider rate observations for a serverless offering.
type ServerlessRateComponents struct {
	Provider string
	Tier     string

	// Request fee component
	ReqPrice decimal.Decimal
	ReqUnit  string

	// Undifferentiated duration fee component (AWS, Azure)
	DurPrice decimal.Decimal
	DurUnit  string

	// GCP separate duration fee components
	DurCPUPrice decimal.Decimal
	DurCPUUnit  string
	DurMemPrice decimal.Decimal
	DurMemUnit  string
}

func gcp1stGenGHz(memoryMB decimal.Decimal) decimal.Decimal {
	switch {
	case memoryMB.LessThanOrEqual(decimal.NewFromInt(128)):
		return decimal.RequireFromString("0.200")
	case memoryMB.LessThanOrEqual(decimal.NewFromInt(256)):
		return decimal.RequireFromString("0.400")
	case memoryMB.LessThanOrEqual(decimal.NewFromInt(512)):
		return decimal.RequireFromString("0.800")
	case memoryMB.LessThanOrEqual(decimal.NewFromInt(1024)):
		return decimal.RequireFromString("1.400")
	case memoryMB.LessThanOrEqual(decimal.NewFromInt(2048)):
		return decimal.RequireFromString("2.400")
	default:
		return decimal.RequireFromString("4.800")
	}
}

// CalculateCost computes authoritative monthly cost and derived hourly cost using decimal arithmetic.
func (c ServerlessRateComponents) CalculateCost(workload ServerlessWorkload) (monthlyCost, hourlyCost decimal.Decimal) {
	requests := workload.RequestsPerMonth
	if requests.IsNegative() {
		requests = decZero
	}
	memoryMB := workload.MemoryMB
	if memoryMB.LessThanOrEqual(decZero) {
		memoryMB = decimal.RequireFromString("512")
	}
	durationMS := workload.ExecutionDurationMS
	if durationMS.LessThanOrEqual(decZero) {
		durationMS = decimal.RequireFromString("200")
	}

	durationSec := durationMS.Div(dec1000)
	memoryGB := memoryMB.Div(dec1024)
	monthlyGBSeconds := requests.Mul(memoryGB).Mul(durationSec)

	tier := c.Tier
	if tier == "" {
		tier = domain.ServerlessTierConsumption
	}

	switch c.Provider {
	case "aws":
		billableRequests := decimal.Max(decZero, requests.Sub(awsFreeRequests))
		billableGBSeconds := decimal.Max(decZero, monthlyGBSeconds.Sub(awsFreeGBSeconds))

		var reqCost decimal.Decimal
		if !c.ReqPrice.IsZero() && !billableRequests.IsZero() {
			switch c.ReqUnit {
			case domain.UnitPerMillionRequests:
				reqCost = billableRequests.Div(dec1M).Mul(c.ReqPrice)
			case domain.UnitPerRequest:
				reqCost = billableRequests.Mul(c.ReqPrice)
			case domain.UnitPer10Requests:
				reqCost = billableRequests.Div(dec10).Mul(c.ReqPrice)
			default:
				reqCost = billableRequests.Div(dec1M).Mul(c.ReqPrice)
			}
		}

		var durCost decimal.Decimal
		if !c.DurPrice.IsZero() && !billableGBSeconds.IsZero() {
			durCost = billableGBSeconds.Mul(c.DurPrice)
		}

		monthlyCost = reqCost.Add(durCost)

	case "azure":
		var freeRequests, freeGBSeconds decimal.Decimal
		if tier != domain.ServerlessTierFlexConsumption {
			freeRequests = azureFreeRequests
			freeGBSeconds = azureFreeGBSeconds
		}

		billableRequests := decimal.Max(decZero, requests.Sub(freeRequests))
		billableGBSeconds := decimal.Max(decZero, monthlyGBSeconds.Sub(freeGBSeconds))

		var reqCost decimal.Decimal
		if !c.ReqPrice.IsZero() && !billableRequests.IsZero() {
			switch c.ReqUnit {
			case domain.UnitPer10Requests:
				reqCost = billableRequests.Div(dec10).Mul(c.ReqPrice)
			case domain.UnitPerMillionRequests:
				reqCost = billableRequests.Div(dec1M).Mul(c.ReqPrice)
			default:
				reqCost = billableRequests.Mul(c.ReqPrice)
			}
		}

		var durCost decimal.Decimal
		if !c.DurPrice.IsZero() && !billableGBSeconds.IsZero() {
			durCost = billableGBSeconds.Mul(c.DurPrice)
		}

		monthlyCost = reqCost.Add(durCost)

	case "gcp":
		var freeRequests decimal.Decimal
		var freeMemSeconds decimal.Decimal
		var freeCPUSeconds decimal.Decimal

		var monthlyCPUSeconds decimal.Decimal
		if tier == domain.ServerlessTier2ndGen {
			freeRequests = gcp2ndGenFreeRequests
			freeMemSeconds = gcp2ndGenFreeGiBSeconds
			freeCPUSeconds = gcp2ndGenFreeVCPUSeconds

			vcpu := memoryMB.Div(dec1769)
			monthlyCPUSeconds = requests.Mul(vcpu).Mul(durationSec)
		} else {
			freeRequests = gcp1stGenFreeRequests
			freeMemSeconds = gcp1stGenFreeGBSeconds
			freeCPUSeconds = gcp1stGenFreeGHzSeconds

			ghz := gcp1stGenGHz(memoryMB)
			monthlyCPUSeconds = requests.Mul(ghz).Mul(durationSec)
		}

		billableRequests := decimal.Max(decZero, requests.Sub(freeRequests))
		billableMemSeconds := decimal.Max(decZero, monthlyGBSeconds.Sub(freeMemSeconds))
		billableCPUSeconds := decimal.Max(decZero, monthlyCPUSeconds.Sub(freeCPUSeconds))

		var reqCost decimal.Decimal
		if !c.ReqPrice.IsZero() && !billableRequests.IsZero() {
			switch c.ReqUnit {
			case domain.UnitPerRequest:
				reqCost = billableRequests.Mul(c.ReqPrice)
			case domain.UnitPerMillionRequests:
				reqCost = billableRequests.Div(dec1M).Mul(c.ReqPrice)
			default:
				reqCost = billableRequests.Mul(c.ReqPrice)
			}
		}

		var memCost decimal.Decimal
		if !c.DurMemPrice.IsZero() && !billableMemSeconds.IsZero() {
			memCost = billableMemSeconds.Mul(c.DurMemPrice)
		}

		var cpuCost decimal.Decimal
		if !c.DurCPUPrice.IsZero() && !billableCPUSeconds.IsZero() {
			cpuCost = billableCPUSeconds.Mul(c.DurCPUPrice)
		}

		// If both separate duration prices are zero, fallback to undifferentiated duration price
		if c.DurCPUPrice.IsZero() && c.DurMemPrice.IsZero() && !c.DurPrice.IsZero() && !billableMemSeconds.IsZero() {
			memCost = billableMemSeconds.Mul(c.DurPrice)
		}

		monthlyCost = reqCost.Add(memCost).Add(cpuCost)

	default:
		// Generic calculation without provider-specific free tiers
		durationCost := monthlyGBSeconds.Mul(c.DurPrice)
		reqCost := requests.Mul(c.ReqPrice)
		monthlyCost = reqCost.Add(durationCost)
	}

	hourlyCost = monthlyCost.Div(HoursInMonth)
	return monthlyCost, hourlyCost
}

// CalculateServerlessCost calculates monthly and hourly serverless costs using decimal arithmetic and free-tier netting.
func CalculateServerlessCost(provider, tier string, reqFeePrice decimal.Decimal, reqUnit string, durationFeePrice decimal.Decimal, workload ServerlessWorkload) (monthlyCost, hourlyCost decimal.Decimal) {
	comp := ServerlessRateComponents{
		Provider: provider,
		Tier:     tier,
		ReqPrice: reqFeePrice,
		ReqUnit:  reqUnit,
		DurPrice: durationFeePrice,
	}
	return comp.CalculateCost(workload)
}

type serverlessComponentRows struct {
	reqRow    *domain.PriceObservation
	durRow    *domain.PriceObservation
	durCPURow *domain.PriceObservation
	durMemRow *domain.PriceObservation
}

func extractServerlessComponentRows(obsList []domain.PriceObservation) serverlessComponentRows {
	var rows serverlessComponentRows
	for i := range obsList {
		o := &obsList[i]
		switch o.ServerlessRateAttributes.ComponentType {
		case domain.ComponentTypeRequestFee:
			rows.reqRow = o
		case domain.ComponentTypeDurationFee:
			rows.durRow = o
		case domain.ComponentTypeDurationFeeCPU:
			rows.durCPURow = o
		case domain.ComponentTypeDurationFeeMemory:
			rows.durMemRow = o
		}
	}
	return rows
}

func buildAWSServerlessCandidates(obsList []domain.PriceObservation, target MatchTarget) []domain.PriceObservation {
	rows := extractServerlessComponentRows(obsList)
	if rows.reqRow == nil && rows.durRow == nil {
		return nil
	}

	var reqPrice decimal.Decimal
	var reqUnit string
	if rows.reqRow != nil {
		reqPrice = rows.reqRow.PriceAmount
		reqUnit = rows.reqRow.ServerlessRateAttributes.Unit
	}

	var durPrice decimal.Decimal
	var durUnit string
	if rows.durRow != nil {
		durPrice = rows.durRow.PriceAmount
		durUnit = rows.durRow.ServerlessRateAttributes.Unit
	}

	components := ServerlessRateComponents{
		Provider: "aws",
		Tier:     domain.ServerlessTierConsumption,
		ReqPrice: reqPrice,
		ReqUnit:  reqUnit,
		DurPrice: durPrice,
		DurUnit:  durUnit,
	}

	monthlyCost, _ := components.CalculateCost(target.ServerlessWorkload)

	baseObs := obsList[0]
	if rows.reqRow != nil {
		baseObs = *rows.reqRow
	} else if rows.durRow != nil {
		baseObs = *rows.durRow
	}

	sku := fmt.Sprintf("AWS-LAMBDA-%s", strings.ToUpper(target.ServerlessWorkload.Architecture))
	displayName := fmt.Sprintf("AWS Lambda (%s)", target.ServerlessWorkload.Architecture)

	cand := baseObs
	cand.SkuID = sku
	cand.DisplayName = displayName
	cand.Unit = "month"
	cand.PriceAmount = monthlyCost
	cand.ServerlessRateAttributes = domain.ServerlessRateAttributes{
		Architecture:  target.ServerlessWorkload.Architecture,
		Tier:          domain.ServerlessTierConsumption,
		ComponentType: "composite",
	}

	return []domain.PriceObservation{cand}
}

func buildAzureServerlessCandidates(obsList []domain.PriceObservation, target MatchTarget) []domain.PriceObservation {
	tierMap := make(map[string][]domain.PriceObservation)
	for i := range obsList {
		o := obsList[i]
		tier := o.ServerlessRateAttributes.Tier
		if tier == "" {
			tier = domain.ServerlessTierConsumption
		}
		tierMap[tier] = append(tierMap[tier], o)
	}

	var candidates []domain.PriceObservation
	for tier, tierObs := range tierMap {
		rows := extractServerlessComponentRows(tierObs)
		if rows.reqRow == nil && rows.durRow == nil {
			continue
		}

		var reqPrice decimal.Decimal
		var reqUnit string
		if rows.reqRow != nil {
			reqPrice = rows.reqRow.PriceAmount
			reqUnit = rows.reqRow.ServerlessRateAttributes.Unit
		}

		var durPrice decimal.Decimal
		var durUnit string
		if rows.durRow != nil {
			durPrice = rows.durRow.PriceAmount
			durUnit = rows.durRow.ServerlessRateAttributes.Unit
		}

		components := ServerlessRateComponents{
			Provider: "azure",
			Tier:     tier,
			ReqPrice: reqPrice,
			ReqUnit:  reqUnit,
			DurPrice: durPrice,
			DurUnit:  durUnit,
		}

		monthlyCost, _ := components.CalculateCost(target.ServerlessWorkload)

		baseObs := tierObs[0]
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
		cand.Unit = "month"
		cand.PriceAmount = monthlyCost
		cand.ServerlessRateAttributes = domain.ServerlessRateAttributes{
			Architecture:  target.ServerlessWorkload.Architecture,
			Tier:          tier,
			ComponentType: "composite",
		}

		candidates = append(candidates, cand)
	}

	return candidates
}

func buildGCPServerlessCandidates(obsList []domain.PriceObservation, target MatchTarget) []domain.PriceObservation {
	rows := extractServerlessComponentRows(obsList)
	if rows.reqRow == nil && rows.durRow == nil && rows.durCPURow == nil && rows.durMemRow == nil {
		return nil
	}

	var reqPrice decimal.Decimal
	var reqUnit string
	if rows.reqRow != nil {
		reqPrice = rows.reqRow.PriceAmount
		reqUnit = rows.reqRow.ServerlessRateAttributes.Unit
	}

	var durPrice decimal.Decimal
	var durUnit string
	if rows.durRow != nil {
		durPrice = rows.durRow.PriceAmount
		durUnit = rows.durRow.ServerlessRateAttributes.Unit
	}

	var durCPUPrice decimal.Decimal
	var durCPUUnit string
	if rows.durCPURow != nil {
		durCPUPrice = rows.durCPURow.PriceAmount
		durCPUUnit = rows.durCPURow.ServerlessRateAttributes.Unit
	}

	var durMemPrice decimal.Decimal
	var durMemUnit string
	if rows.durMemRow != nil {
		durMemPrice = rows.durMemRow.PriceAmount
		durMemUnit = rows.durMemRow.ServerlessRateAttributes.Unit
	}

	tier := domain.ServerlessTierConsumption
	if target.ServerlessWorkload.Tier != "" {
		tier = target.ServerlessWorkload.Tier
	} else if rows.reqRow != nil && rows.reqRow.ServerlessRateAttributes.Tier != "" {
		tier = rows.reqRow.ServerlessRateAttributes.Tier
	}

	components := ServerlessRateComponents{
		Provider:    "gcp",
		Tier:        tier,
		ReqPrice:    reqPrice,
		ReqUnit:     reqUnit,
		DurPrice:    durPrice,
		DurUnit:     durUnit,
		DurCPUPrice: durCPUPrice,
		DurCPUUnit:  durCPUUnit,
		DurMemPrice: durMemPrice,
		DurMemUnit:  durMemUnit,
	}

	monthlyCost, _ := components.CalculateCost(target.ServerlessWorkload)

	baseObs := obsList[0]
	if rows.reqRow != nil {
		baseObs = *rows.reqRow
	} else if rows.durMemRow != nil {
		baseObs = *rows.durMemRow
	} else if rows.durCPURow != nil {
		baseObs = *rows.durCPURow
	} else if rows.durRow != nil {
		baseObs = *rows.durRow
	}

	cand := baseObs
	cand.SkuID = "GCP-CLOUD-FUNCTIONS"
	cand.DisplayName = "Cloud Functions"
	cand.Unit = "month"
	cand.PriceAmount = monthlyCost
	cand.ServerlessRateAttributes = domain.ServerlessRateAttributes{
		Architecture:  target.ServerlessWorkload.Architecture,
		Tier:          tier,
		ComponentType: "composite",
	}

	return []domain.PriceObservation{cand}
}

func buildGenericServerlessCandidates(obsList []domain.PriceObservation, target MatchTarget) []domain.PriceObservation {
	var candidates []domain.PriceObservation
	for _, o := range obsList {
		cand := o
		cand.ServerlessRateAttributes.Architecture = target.ServerlessWorkload.Architecture
		cand.ServerlessRateAttributes.Tier = target.ServerlessWorkload.Tier
		candidates = append(candidates, cand)
	}
	return candidates
}
