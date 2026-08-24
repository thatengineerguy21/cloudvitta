package ibm

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

type memoryQuarantineSink struct {
	items []quarantine.UnmappedItem
}

func (s *memoryQuarantineSink) Record(ctx context.Context, item quarantine.UnmappedItem) error {
	s.items = append(s.items, item)
	return nil
}

func (s *memoryQuarantineSink) Count() int {
	return len(s.items)
}

func (s *memoryQuarantineSink) Flush(ctx context.Context) error {
	return nil
}

func TestNormalize_VPCProfiles(t *testing.T) {
	jsonPayload := `{
		"resources": [
			{
				"id": "is.instance",
				"name": "is.instance",
				"kind": "service",
				"geo_tags": ["us-east", "us-south"],
				"pricing": {
					"metrics": [
						{
							"metric_id": "bx2-2x8",
							"amounts": [
								{
									"currency": "USD",
									"prices": [{"quantity_tier": 1, "price": 0.096}]
								}
							]
						},
						{
							"metric_id": "cx2-4x8",
							"amounts": [
								{
									"currency": "USD",
									"prices": [{"quantity_tier": 1, "price": 0.152}]
								}
							]
						},
						{
							"metric_id": "mx2-8x64",
							"amounts": [
								{
									"currency": "USD",
									"prices": [{"quantity_tier": 1, "price": 0.536}]
								}
							]
						},
						{
							"metric_id": "vx2-4x56",
							"amounts": [
								{
									"currency": "USD",
									"prices": [{"quantity_tier": 1, "price": 0.420}]
								}
							]
						},
						{
							"metric_id": "ba2-2x8",
							"amounts": [
								{
									"currency": "USD",
									"prices": [{"quantity_tier": 1, "price": 0.086}]
								}
							]
						}
					]
				}
			}
		]
	}`

	now := time.Now().UTC()
	obs, err := Normalize(strings.NewReader(jsonPayload), now)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	// 5 profiles * 2 regions = 10 observations
	if len(obs) != 10 {
		t.Fatalf("expected 10 observations, got %d", len(obs))
	}

	var foundBX2, foundCX2, foundMX2, foundVX2, foundBA2 bool
	for _, o := range obs {
		if o.Region == "us-east" {
			if o.RegionGroup != "us-east" {
				t.Errorf("expected RegionGroup us-east for us-east, got %s", o.RegionGroup)
			}
		} else if o.Region == "us-south" {
			if o.RegionGroup != "us-central" {
				t.Errorf("expected RegionGroup us-central for us-south, got %s", o.RegionGroup)
			}
		}

		switch o.SkuID {
		case "SKU-IBM-VPC-BX2-2X8":
			foundBX2 = true
			if o.Attributes.VCPU != 2 || o.Attributes.RAMGB != 8 || o.Attributes.Family != "general_purpose" {
				t.Errorf("BX2 attrs mismatch: %+v", o.Attributes)
			}
			expectedPrice := decimal.RequireFromString("0.096")
			if !o.PriceAmount.Equal(expectedPrice) {
				t.Errorf("BX2 price = %s, want %s", o.PriceAmount, expectedPrice)
			}
		case "SKU-IBM-VPC-CX2-4X8":
			foundCX2 = true
			if o.Attributes.VCPU != 4 || o.Attributes.RAMGB != 8 || o.Attributes.Family != "compute_optimized" {
				t.Errorf("CX2 attrs mismatch: %+v", o.Attributes)
			}
			expectedPrice := decimal.RequireFromString("0.152")
			if !o.PriceAmount.Equal(expectedPrice) {
				t.Errorf("CX2 price = %s, want %s", o.PriceAmount, expectedPrice)
			}
		case "SKU-IBM-VPC-MX2-8X64":
			foundMX2 = true
			if o.Attributes.VCPU != 8 || o.Attributes.RAMGB != 64 || o.Attributes.Family != "memory_optimized" {
				t.Errorf("MX2 attrs mismatch: %+v", o.Attributes)
			}
			expectedPrice := decimal.RequireFromString("0.536")
			if !o.PriceAmount.Equal(expectedPrice) {
				t.Errorf("MX2 price = %s, want %s", o.PriceAmount, expectedPrice)
			}
		case "SKU-IBM-VPC-VX2-4X56":
			foundVX2 = true
			if o.Attributes.VCPU != 4 || o.Attributes.RAMGB != 56 || o.Attributes.Family != "memory_optimized" {
				t.Errorf("VX2 attrs mismatch: %+v", o.Attributes)
			}
			expectedPrice := decimal.RequireFromString("0.420")
			if !o.PriceAmount.Equal(expectedPrice) {
				t.Errorf("VX2 price = %s, want %s", o.PriceAmount, expectedPrice)
			}
		case "SKU-IBM-VPC-BA2-2X8":
			foundBA2 = true
			if o.Attributes.VCPU != 2 || o.Attributes.RAMGB != 8 || o.Attributes.Family != "general_purpose" {
				t.Errorf("BA2 attrs mismatch: %+v", o.Attributes)
			}
			expectedPrice := decimal.RequireFromString("0.086")
			if !o.PriceAmount.Equal(expectedPrice) {
				t.Errorf("BA2 price = %s, want %s", o.PriceAmount, expectedPrice)
			}
		}
	}

	if !foundBX2 || !foundCX2 || !foundMX2 || !foundVX2 || !foundBA2 {
		t.Errorf("missing expected profiles: BX2=%v, CX2=%v, MX2=%v, VX2=%v, BA2=%v", foundBX2, foundCX2, foundMX2, foundVX2, foundBA2)
	}
}

func TestNormalize_QuarantineUnmappedItems(t *testing.T) {
	jsonPayload := `{
		"resources": [
			{
				"id": "unmapped.service",
				"name": "unmapped.service",
				"pricing": {
					"metrics": [
						{
							"metric_id": "unmapped-metric",
							"amounts": [{"currency": "USD", "prices": [{"price": 1.0}]}]
						}
					]
				}
			},
			{
				"id": "is.instance",
				"name": "is.instance",
				"geo_tags": ["unmapped-mars-region"],
				"pricing": {
					"metrics": [
						{
							"metric_id": "bx2-2x8",
							"amounts": [{"currency": "USD", "prices": [{"price": 0.096}]}]
						}
					]
				}
			},
			{
				"id": "is.instance",
				"name": "is.instance",
				"geo_tags": ["us-east"],
				"pricing": {
					"metrics": [
						{
							"metric_id": "unparseable-profile-name",
							"amounts": [{"currency": "USD", "prices": [{"price": 0.50}]}]
						}
					]
				}
			}
		]
	}`

	sink := &memoryQuarantineSink{}
	now := time.Now().UTC()
	obs, err := Normalize(strings.NewReader(jsonPayload), now, sink)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if len(obs) != 0 {
		t.Errorf("expected 0 valid observations, got %d", len(obs))
	}

	if sink.Count() != 3 {
		t.Fatalf("expected 3 quarantined items (unmapped product + unmapped region + unrecognized profile), got %d", sink.Count())
	}
}

func TestNormalize_InvalidJSON(t *testing.T) {
	_, err := Normalize(bytes.NewReader([]byte("{invalid json")), time.Now().UTC())
	if err == nil {
		t.Fatal("expected error on invalid JSON, got nil")
	}
}

func TestNormalize_EmptyResource_NoPanic(t *testing.T) {
	jsonPayload := `{"resources": []}`
	obs, err := Normalize(strings.NewReader(jsonPayload), time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error on empty resources: %v", err)
	}
	if len(obs) != 0 {
		t.Errorf("expected 0 observations, got %d", len(obs))
	}
}
