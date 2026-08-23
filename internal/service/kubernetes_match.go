package service

import (
	"strings"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/kubernetestieremap"
)

// Penalty for Kubernetes tier mismatch.
const (
	PenaltyKubernetesTier = 0.20
)

// KubernetesScorer implements CategoryScorer for managed Kubernetes control-plane resources.
// Distance formula:
//
//	Distance = Penalty_tier
//	where:
//	  Penalty_tier = 0.0 (if candidate.Tier == target.Tier)
//	  Penalty_tier = 0.20 (if candidate.Tier != target.Tier)
type KubernetesScorer struct{}

// Score computes the categorical tier distance between a Kubernetes control plane candidate and target spec.
func (s KubernetesScorer) Score(candidate domain.PriceObservation, target MatchTarget) (float64, []string, bool) {
	if candidate.ServiceCategory != "kubernetes" {
		return 0, nil, false
	}

	targetTier := target.KubernetesTier
	if targetTier == "" {
		targetTier = kubernetestieremap.TierStandard
	}

	candTier := candidate.KubernetesAttributes.Tier
	if candTier == "" {
		candTier = kubernetestieremap.TierStandard
	}

	var distance float64
	if !strings.EqualFold(string(candTier), string(targetTier)) {
		distance = PenaltyKubernetesTier
	}

	return distance, []string{}, true
}

// MatchKubernetesObservations matches candidate Kubernetes control plane observations against the target spec.
func MatchKubernetesObservations(obsList []domain.PriceObservation, target MatchTarget, thresholds CategoryThresholds) (*MatchResult, error) {
	if len(obsList) == 0 {
		return nil, nil
	}

	res := MatchObservations(KubernetesScorer{}, obsList, target, thresholds)
	if res == nil {
		return nil, ErrNoMatchFound
	}

	return res, nil
}

// GKEMonthlyCredit is the exact $74.40/month billing credit applied per Google Cloud billing account.
var GKEMonthlyCredit = decimal.RequireFromString("74.40")

// KubernetesCostAdjuster is a strategy function that calculates provider-specific pricing adjustments or discounts.
type KubernetesCostAdjuster func(baseHourlyCost decimal.Decimal, target MatchTarget) (decimal.Decimal, []CalculateWarning)

// gcpKubernetesCostAdjuster implements GCP's conditional GKE control plane credit.
// If cluster_topology is "zonal" or "autopilot", the monthly credit is amortized hourly and deducted.
// If cluster_topology is empty (""), no credit is applied and a warning is added.
// If cluster_topology is "regional" (or any other topology), no credit is applied.
func gcpKubernetesCostAdjuster(baseHourlyCost decimal.Decimal, target MatchTarget) (decimal.Decimal, []CalculateWarning) {
	var warnings []CalculateWarning
	topology := domain.ClusterTopology(strings.ToLower(strings.TrimSpace(string(target.ClusterTopology))))

	switch topology {
	case domain.ClusterTopologyZonal, domain.ClusterTopologyAutopilot:
		hourlyCredit := GKEMonthlyCredit.Div(HoursInMonth)
		adjusted := decimal.Max(decimal.Zero, baseHourlyCost.Sub(hourlyCredit))
		return adjusted, nil
	case "":
		warnings = append(warnings, CalculateWarning{
			Provider: "gcp",
			Code:     "cluster_topology_unspecified",
			Message:  "Cluster topology was not specified; assuming regional deployment without monthly management credit.",
		})
		return baseHourlyCost, warnings
	default:
		return baseHourlyCost, nil
	}
}

var kubernetesCostAdjusters = map[string]KubernetesCostAdjuster{
	"gcp": gcpKubernetesCostAdjuster,
}

// AdjustKubernetesCost applies any registered provider-specific cost adjustments.
func AdjustKubernetesCost(provider string, baseHourlyCost decimal.Decimal, target MatchTarget) (decimal.Decimal, []CalculateWarning) {
	if adjuster, ok := kubernetesCostAdjusters[strings.ToLower(strings.TrimSpace(provider))]; ok {
		return adjuster(baseHourlyCost, target)
	}
	return baseHourlyCost, nil
}
