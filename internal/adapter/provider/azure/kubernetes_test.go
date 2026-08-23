package azure_test

import (
	"os"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/azure"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/kubernetestieremap"
)

func TestNormalize_AzureKubernetesGolden(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	f, err := os.Open("../../../../testdata/golden/azure/kubernetes.json")
	if err != nil {
		t.Fatalf("failed to open golden file: %v", err)
	}
	defer func() { _ = f.Close() }()

	obs, _, err := azure.Normalize(f, fixedTime)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}

	if len(obs) != 3 {
		t.Fatalf("expected 3 observations, got %d", len(obs))
	}

	tiersFound := make(map[string]bool)
	for _, o := range obs {
		if o.Provider != "azure" {
			t.Errorf("expected Provider azure, got %s", o.Provider)
		}
		if o.ServiceCategory != "kubernetes" {
			t.Errorf("expected ServiceCategory kubernetes, got %s", o.ServiceCategory)
		}
		if o.PriceCurrency != "USD" {
			t.Errorf("expected PriceCurrency USD, got %s", o.PriceCurrency)
		}
		if o.Region != "eastus" || o.RegionGroup != "us-east" {
			t.Errorf("unexpected region/group: %s / %s", o.Region, o.RegionGroup)
		}
		tiersFound[o.KubernetesAttributes.Tier] = true
	}

	if !tiersFound[kubernetestieremap.TierFree] {
		t.Errorf("expected to find free tier observation")
	}
	if !tiersFound[kubernetestieremap.TierStandard] {
		t.Errorf("expected to find standard tier observation")
	}
	if !tiersFound[kubernetestieremap.TierExtendedSupport] {
		t.Errorf("expected to find extended_support tier observation")
	}
}
