package kubernetestieremap

import (
	"errors"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

func TestMapAWSTier(t *testing.T) {
	tests := []struct {
		rawTier  string
		wantTier domain.KubernetesTier
		wantErr  error
	}{
		{"AmazonEKS", TierStandard, nil},
		{"standard", TierStandard, nil},
		{"AmazonEKS-Hours:perCluster", TierStandard, nil},
		{"CreateCluster", TierStandard, nil},
		{"extended_support", TierExtendedSupport, nil},
		{"extended-support", TierExtendedSupport, nil},
		{"AmazonEKS-ExtendedSupport-Hours:perCluster", TierExtendedSupport, nil},
		{"ClusterSupport", TierExtendedSupport, nil},
		{"unknown_tier", "", ErrUnmappedTier},
	}

	for _, tt := range tests {
		got, err := MapAWSTier(tt.rawTier)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapAWSTier(%q) error = %v, want %v", tt.rawTier, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantTier {
				t.Errorf("MapAWSTier(%q) = (%q, %v), want (%q, nil)", tt.rawTier, got, err, tt.wantTier)
			}
		}
	}
}

func TestMapAzureTier(t *testing.T) {
	tests := []struct {
		rawTier  string
		wantTier domain.KubernetesTier
		wantErr  error
	}{
		{"Free", TierFree, nil},
		{"free tier", TierFree, nil},
		{"standard", TierStandard, nil},
		{"Standard Tier", TierStandard, nil},
		{"Uptime SLA", TierStandard, nil},
		{"extended_support", TierExtendedSupport, nil},
		{"Extended Support", TierExtendedSupport, nil},
		{"Long Term Support", TierExtendedSupport, nil},
		{"LTS", TierExtendedSupport, nil},
		{"Premium", TierExtendedSupport, nil},
		{"unknown_tier", "", ErrUnmappedTier},
	}

	for _, tt := range tests {
		got, err := MapAzureTier(tt.rawTier)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapAzureTier(%q) error = %v, want %v", tt.rawTier, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantTier {
				t.Errorf("MapAzureTier(%q) = (%q, %v), want (%q, nil)", tt.rawTier, got, err, tt.wantTier)
			}
		}
	}
}

func TestMapGCPTier(t *testing.T) {
	tests := []struct {
		rawTier  string
		wantTier domain.KubernetesTier
		wantErr  error
	}{
		{"standard", TierStandard, nil},
		{"Standard Tier", TierStandard, nil},
		{"Cluster Management", TierStandard, nil},
		{"Cluster Management Fee", TierStandard, nil},
		{"Kubernetes Engine Cluster Management Fee", TierStandard, nil},
		{"GKE Cluster Management Fee", TierStandard, nil},
		{"GKE Standard", TierStandard, nil},
		{"GKE", TierStandard, nil},
		{"Kubernetes Engine", TierStandard, nil},
		{"unknown_tier", "", ErrUnmappedTier},
	}

	for _, tt := range tests {
		got, err := MapGCPTier(tt.rawTier)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapGCPTier(%q) error = %v, want %v", tt.rawTier, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantTier {
				t.Errorf("MapGCPTier(%q) = (%q, %v), want (%q, nil)", tt.rawTier, got, err, tt.wantTier)
			}
		}
	}
}

func TestNormalizeTier(t *testing.T) {
	got, err := NormalizeTier("aws", "AmazonEKS")
	if err != nil || got != TierStandard {
		t.Errorf("NormalizeTier(aws, AmazonEKS) = (%q, %v), want (%q, nil)", got, err, TierStandard)
	}

	gotAzure, err := NormalizeTier("azure", "Free")
	if err != nil || gotAzure != TierFree {
		t.Errorf("NormalizeTier(azure, Free) = (%q, %v), want (%q, nil)", gotAzure, err, TierFree)
	}

	gotGCP, err := NormalizeTier("gcp", "Cluster Management Fee")
	if err != nil || gotGCP != TierStandard {
		t.Errorf("NormalizeTier(gcp, Cluster Management Fee) = (%q, %v), want (%q, nil)", gotGCP, err, TierStandard)
	}

	_, err = NormalizeTier("unmapped_provider", "standard")
	if err == nil {
		t.Errorf("NormalizeTier(unmapped_provider, standard) expected error, got nil")
	}
}

func TestResolveCanonicalTier(t *testing.T) {
	tests := []struct {
		input       string
		wantTier    domain.KubernetesTier
		wantStageOk bool
		wantErr     bool
	}{
		{"standard", TierStandard, true, false},
		{"free", TierFree, true, false},
		{"extended_support", TierExtendedSupport, true, false},
		{"Uptime SLA", TierStandard, true, false},
		{"Free Tier", TierFree, true, false},
		{"Cluster Management", TierStandard, true, false},
		{"Extended Support", TierExtendedSupport, true, false},
		{"", "", false, false},
		{"invalid_tier_name", "", false, true},
	}

	for _, tt := range tests {
		got, err := ResolveCanonicalTier(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ResolveCanonicalTier(%q) expected error, got nil", tt.input)
			}
		} else {
			if err != nil || got != tt.wantTier {
				t.Errorf("ResolveCanonicalTier(%q) = (%q, %v), want (%q, nil)", tt.input, got, err, tt.wantTier)
			}
			if tt.input != "" {
				stageOk := IsSupportedStageTier(got)
				if stageOk != tt.wantStageOk {
					t.Errorf("IsSupportedStageTier(%q) = %v, want %v", got, stageOk, tt.wantStageOk)
				}
			}
		}
	}
}

func TestSupportedCanonicalTiers(t *testing.T) {
	tiers := SupportedCanonicalTiers()
	if len(tiers) != 3 {
		t.Fatalf("expected 3 supported canonical tiers, got %d", len(tiers))
	}
	expected := map[domain.KubernetesTier]bool{
		TierFree:            true,
		TierStandard:        true,
		TierExtendedSupport: true,
	}
	for _, tier := range tiers {
		if !expected[tier] {
			t.Errorf("unexpected supported canonical tier: %s", tier)
		}
	}
}
