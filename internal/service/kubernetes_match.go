package service

import (
	"strings"

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
	if !strings.EqualFold(candTier, targetTier) {
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
