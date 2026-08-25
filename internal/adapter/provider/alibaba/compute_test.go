package alibaba

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

func TestNormalize_ECSProfiles(t *testing.T) {
	jsonPayload := `{
		"InstanceTypes": [
			{
				"InstanceTypeId": "ecs.g7.large",
				"CpuCoreCount": 2,
				"MemorySize": 8.0,
				"InstanceTypeFamily": "ecs.g7",
				"Price": {
					"TradePrice": 0.096,
					"Currency": "USD"
				},
				"Regions": ["us-east-1", "us-west-1"]
			},
			{
				"InstanceTypeId": "ecs.c7.xlarge",
				"CpuCoreCount": 4,
				"MemorySize": 8.0,
				"InstanceTypeFamily": "ecs.c7",
				"Price": {
					"TradePrice": 0.152,
					"Currency": "USD"
				},
				"Regions": ["us-east-1"]
			},
			{
				"InstanceTypeId": "ecs.r7.2xlarge",
				"CpuCoreCount": 8,
				"MemorySize": 64.0,
				"InstanceTypeFamily": "ecs.r7",
				"Price": {
					"TradePrice": 0.536,
					"Currency": "USD"
				},
				"Regions": ["eu-central-1"]
			},
			{
				"InstanceTypeId": "ecs.t6-c1m1.large",
				"CpuCoreCount": 2,
				"MemorySize": 2.0,
				"InstanceTypeFamily": "ecs.t6",
				"Price": {
					"TradePrice": 0.038,
					"Currency": "USD"
				},
				"Regions": ["ap-southeast-1"]
			},
			{
				"InstanceTypeId": "ecs.g7.large",
				"CpuCoreCount": 2,
				"MemorySize": 8.0,
				"InstanceTypeFamily": "ecs.g7",
				"Price": {
					"TradePrice": 0.650,
					"Currency": "CNY"
				},
				"Regions": ["cn-hangzhou"]
			}
		]
	}`

	now := time.Now().UTC()
	res, err := Normalize(strings.NewReader(jsonPayload), now)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	obs := res.Observations

	// 2 + 1 + 1 + 1 + 1 = 6 observations
	if len(obs) != 6 {
		t.Fatalf("expected 6 observations, got %d", len(obs))
	}

	var foundG7_USD, foundG7_CNY, foundC7, foundR7, foundT6 bool
	for _, o := range obs {
		switch o.SkuID {
		case "SKU-ALI-ECS-G7-LARGE":
			if o.PriceCurrency == "USD" && o.Region == "us-east-1" {
				foundG7_USD = true
				if o.RegionGroup != "us-east" {
					t.Errorf("expected RegionGroup us-east, got %s", o.RegionGroup)
				}
				if o.Attributes.VCPU != 2 || o.Attributes.RAMGB != 8 || o.Attributes.Family != "general_purpose" {
					t.Errorf("G7 attrs mismatch: %+v", o.Attributes)
				}
				expectedPrice := decimal.RequireFromString("0.096")
				if !o.PriceAmount.Equal(expectedPrice) {
					t.Errorf("G7 USD price = %s, want %s", o.PriceAmount, expectedPrice)
				}
			} else if o.PriceCurrency == "CNY" && o.Region == "cn-hangzhou" {
				foundG7_CNY = true
				if o.RegionGroup != "cn-east" {
					t.Errorf("expected RegionGroup cn-east, got %s", o.RegionGroup)
				}
				expectedPrice := decimal.RequireFromString("0.650")
				if !o.PriceAmount.Equal(expectedPrice) {
					t.Errorf("G7 CNY price = %s, want %s", o.PriceAmount, expectedPrice)
				}
			}
		case "SKU-ALI-ECS-C7-XLARGE":
			foundC7 = true
			if o.Attributes.VCPU != 4 || o.Attributes.RAMGB != 8 || o.Attributes.Family != "compute_optimized" {
				t.Errorf("C7 attrs mismatch: %+v", o.Attributes)
			}
			expectedPrice := decimal.RequireFromString("0.152")
			if !o.PriceAmount.Equal(expectedPrice) {
				t.Errorf("C7 price = %s, want %s", o.PriceAmount, expectedPrice)
			}
		case "SKU-ALI-ECS-R7-2XLARGE":
			foundR7 = true
			if o.Attributes.VCPU != 8 || o.Attributes.RAMGB != 64 || o.Attributes.Family != "memory_optimized" {
				t.Errorf("R7 attrs mismatch: %+v", o.Attributes)
			}
			expectedPrice := decimal.RequireFromString("0.536")
			if !o.PriceAmount.Equal(expectedPrice) {
				t.Errorf("R7 price = %s, want %s", o.PriceAmount, expectedPrice)
			}
		case "SKU-ALI-ECS-T6-C1M1-LARGE":
			foundT6 = true
			if o.Attributes.VCPU != 2 || o.Attributes.RAMGB != 2 || o.Attributes.Family != "burstable" {
				t.Errorf("T6 attrs mismatch: %+v", o.Attributes)
			}
			expectedPrice := decimal.RequireFromString("0.038")
			if !o.PriceAmount.Equal(expectedPrice) {
				t.Errorf("T6 price = %s, want %s", o.PriceAmount, expectedPrice)
			}
		}
	}

	if !foundG7_USD || !foundG7_CNY || !foundC7 || !foundR7 || !foundT6 {
		t.Errorf("missing expected profiles: G7_USD=%v, G7_CNY=%v, C7=%v, R7=%v, T6=%v", foundG7_USD, foundG7_CNY, foundC7, foundR7, foundT6)
	}
}

func TestNormalize_QuarantineUnmappedItems(t *testing.T) {
	jsonPayload := `{
		"InstanceTypes": [
			{
				"ProductCode": "unmapped.alibaba.service",
				"InstanceTypeId": "ecs.unknown.large",
				"CpuCoreCount": 2,
				"MemorySize": 8.0,
				"TradePrice": 0.10,
				"Regions": ["us-east-1"]
			},
			{
				"InstanceTypeId": "ecs.g7.large",
				"CpuCoreCount": 2,
				"MemorySize": 8.0,
				"TradePrice": 0.10,
				"Regions": ["unmapped-mars-region"]
			},
			{
				"InstanceTypeId": "ecs.unknownfamily.large",
				"CpuCoreCount": 2,
				"MemorySize": 8.0,
				"TradePrice": 0.10,
				"Regions": ["us-east-1"]
			},
			{
				"InstanceTypeId": "ecs.g7.large",
				"CpuCoreCount": 0,
				"MemorySize": 0,
				"TradePrice": 0.10,
				"Regions": ["us-east-1"]
			},
			{
				"InstanceTypeId": "ecs.g7.large",
				"CpuCoreCount": 2,
				"MemorySize": 8.0,
				"TradePrice": 0.10,
				"Regions": []
			}
		]
	}`

	sink := &memoryQuarantineSink{}
	now := time.Now().UTC()
	res, err := Normalize(strings.NewReader(jsonPayload), now, sink)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	obs := res.Observations

	if len(obs) != 0 {
		t.Errorf("expected 0 valid observations, got %d", len(obs))
	}

	if sink.Count() != 5 {
		t.Fatalf("expected 5 quarantined items (unmapped product + unmapped region + unknown family + zero core/ram + missing region), got %d", sink.Count())
	}
}

func TestNormalize_InvalidJSON(t *testing.T) {
	_, err := Normalize(bytes.NewReader([]byte("{invalid json")), time.Now().UTC())
	if err == nil {
		t.Fatal("expected error on invalid JSON, got nil")
	}
}

func TestNormalize_EmptyResource_NoPanic(t *testing.T) {
	jsonPayload := `{"InstanceTypes": []}`
	res, err := Normalize(strings.NewReader(jsonPayload), time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error on empty resources: %v", err)
	}
	obs := res.Observations
	if len(obs) != 0 {
		t.Errorf("expected 0 observations, got %d", len(obs))
	}
}
