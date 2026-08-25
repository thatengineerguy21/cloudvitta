package aws_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/aws"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/kubernetestieremap"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

type mockQuarantineSink struct {
	items []quarantine.UnmappedItem
}

func (m *mockQuarantineSink) Record(ctx context.Context, item quarantine.UnmappedItem) error {
	m.items = append(m.items, item)
	return nil
}

func TestNormalize_AWSKubernetesGolden(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	f, err := os.Open("../../../../testdata/golden/aws/kubernetes.json")
	if err != nil {
		t.Fatalf("failed to open golden file: %v", err)
	}
	defer func() { _ = f.Close() }()

	res, err := aws.Normalize(f, fixedTime)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	obs := res.Observations

	if len(obs) != 2 {
		t.Fatalf("expected 2 observations, got %d", len(obs))
	}

	for _, o := range obs {
		if o.Provider != "aws" {
			t.Errorf("expected Provider aws, got %s", o.Provider)
		}
		if o.ServiceCategory != "kubernetes" {
			t.Errorf("expected ServiceCategory kubernetes, got %s", o.ServiceCategory)
		}
		if o.PriceCurrency != "USD" {
			t.Errorf("expected PriceCurrency USD, got %s", o.PriceCurrency)
		}
		if o.Region != "us-east-1" || o.RegionGroup != "us-east" {
			t.Errorf("unexpected region/group: %s / %s", o.Region, o.RegionGroup)
		}
		if o.KubernetesAttributes.Tier != kubernetestieremap.TierStandard &&
			o.KubernetesAttributes.Tier != kubernetestieremap.TierExtendedSupport {
			t.Errorf("unexpected tier: %s", o.KubernetesAttributes.Tier)
		}
	}
}

func TestNormalize_AWSKubernetes_UnmappedTierQuarantine(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	rawJSON := `{
		"formatVersion": "v1.0",
		"offerCode": "AmazonEKS",
		"products": {
			"SKU-AWS-EKS-UNKNOWN": {
				"sku": "SKU-AWS-EKS-UNKNOWN",
				"productFamily": "Compute",
				"attributes": {
					"servicecode": "AmazonEKS",
					"location": "US East (N. Virginia)",
					"regionCode": "us-east-1",
					"group": "AmazonEKS-Quantum-Hours",
					"usagetype": "UnknownEKSUsageType",
					"operation": "UnknownOperation",
					"description": "Amazon EKS Quantum Management"
				}
			}
		},
		"terms": {
			"OnDemand": {
				"SKU-AWS-EKS-UNKNOWN": {
					"SKU-AWS-EKS-UNKNOWN.TERM": {
						"offerTermCode": "TERM",
						"sku": "SKU-AWS-EKS-UNKNOWN",
						"priceDimensions": {
							"SKU-AWS-EKS-UNKNOWN.TERM.DIM": {
								"unit": "Hrs",
								"pricePerUnit": {
									"USD": "0.5000000000"
								}
							}
						}
					}
				}
			}
		}
	}`

	sink := &mockQuarantineSink{}
	res, err := aws.Normalize(strings.NewReader(rawJSON), fixedTime, sink)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	obs := res.Observations

	if len(obs) != 0 {
		t.Fatalf("expected 0 normalized observations for unmapped tier, got %d", len(obs))
	}

	if len(sink.items) != 1 {
		t.Fatalf("expected 1 quarantine sink item, got %d", len(sink.items))
	}

	item := sink.items[0]
	if item.Provider != "aws" || item.Category != "kubernetes" || item.Kind != "kubernetes_tier" || item.RawValue != "UnknownEKSUsageType" || item.SkuID != "SKU-AWS-EKS-UNKNOWN" {
		t.Errorf("unexpected quarantine item: %+v", item)
	}
}
