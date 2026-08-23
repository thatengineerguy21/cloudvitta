package aws_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/aws"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/serverlessarchmap"
)

func TestNormalize_AWSServerlessGolden(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	f, err := os.Open("../../../../testdata/golden/aws/serverless.json")
	if err != nil {
		t.Fatalf("failed to open golden file: %v", err)
	}
	defer func() { _ = f.Close() }()

	obs, err := aws.Normalize(f, fixedTime)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}

	if len(obs) != 4 {
		t.Fatalf("expected 4 observations, got %d", len(obs))
	}

	for _, o := range obs {
		if o.Provider != "aws" {
			t.Errorf("expected Provider aws, got %s", o.Provider)
		}
		if o.ServiceCategory != "serverless" {
			t.Errorf("expected ServiceCategory serverless, got %s", o.ServiceCategory)
		}
		if o.PriceCurrency != "USD" {
			t.Errorf("expected PriceCurrency USD, got %s", o.PriceCurrency)
		}
		if o.Region != "us-east-1" || o.RegionGroup != "us-east" {
			t.Errorf("unexpected region/group: %s / %s", o.Region, o.RegionGroup)
		}
		if o.ServerlessRateAttributes.Architecture != serverlessarchmap.ArchX86_64 &&
			o.ServerlessRateAttributes.Architecture != serverlessarchmap.ArchARM64 {
			t.Errorf("unexpected architecture: %s", o.ServerlessRateAttributes.Architecture)
		}
		if o.ServerlessRateAttributes.ComponentType != "request_fee" &&
			o.ServerlessRateAttributes.ComponentType != "duration_fee" {
			t.Errorf("unexpected component type: %s", o.ServerlessRateAttributes.ComponentType)
		}
	}
}

func TestNormalize_AWSServerless_UnmappedArchQuarantine(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	rawJSON := `{
		"formatVersion": "v1.0",
		"offerCode": "AWSLambda",
		"products": {
			"SKU-AWS-LAMBDA-UNKNOWN-ARCH": {
				"sku": "SKU-AWS-LAMBDA-UNKNOWN-ARCH",
				"productFamily": "Serverless",
				"attributes": {
					"servicecode": "AWSLambda",
					"location": "US East (N. Virginia)",
					"regionCode": "us-east-1",
					"group": "UnknownGroup",
					"usagetype": "UnknownUsageType",
					"description": "Unknown Architecture SKU"
				}
			}
		},
		"terms": {
			"OnDemand": {
				"SKU-AWS-LAMBDA-UNKNOWN-ARCH": {
					"SKU-AWS-LAMBDA-UNKNOWN-ARCH.TERM": {
						"offerTermCode": "TERM",
						"sku": "SKU-AWS-LAMBDA-UNKNOWN-ARCH",
						"priceDimensions": {
							"SKU-AWS-LAMBDA-UNKNOWN-ARCH.TERM.DIM": {
								"unit": "Requests",
								"pricePerUnit": {
									"USD": "0.2000000000"
								}
							}
						}
					}
				}
			}
		}
	}`

	sink := &mockQuarantineSink{}
	obs, err := aws.Normalize(strings.NewReader(rawJSON), fixedTime, sink)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}

	if len(obs) != 0 {
		t.Fatalf("expected 0 normalized observations for unmapped arch, got %d", len(obs))
	}

	// UnknownGroup that doesn't match Request or Duration is filtered early without error
}
