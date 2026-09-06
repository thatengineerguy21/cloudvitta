package aws_test

import (
	"net/url"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/aws"
)

func TestAWSClient_DefaultEndpoints(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "EC2 endpoint",
			raw:  aws.DefaultEC2PriceListURL,
			want: "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/current/us-east-1/index.json",
		},
		{
			name: "S3 endpoint",
			raw:  aws.DefaultS3PriceListURL,
			want: "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonS3/current/us-east-1/index.json",
		},
		{
			name: "DataTransfer endpoint",
			raw:  aws.DefaultDataTransferPriceListURL,
			want: "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AWSDataTransfer/current/us-east-1/index.json",
		},
		{
			name: "RDS endpoint",
			raw:  aws.DefaultRDSPriceListURL,
			want: "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonRDS/current/us-east-1/index.json",
		},
		{
			name: "DynamoDB endpoint",
			raw:  aws.DefaultDynamoDBPriceListURL,
			want: "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonDynamoDB/current/us-east-1/index.json",
		},
		{
			name: "EKS endpoint",
			raw:  aws.DefaultEKSPriceListURL,
			want: "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEKS/current/us-east-1/index.json",
		},
		{
			name: "Lambda serverless endpoint",
			raw:  aws.DefaultLambdaPriceListURL,
			want: "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AWSLambda/current/us-east-1/index.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.raw != tt.want {
				t.Errorf("got %q, want %q", tt.raw, tt.want)
			}
			parsed, err := url.Parse(tt.raw)
			if err != nil {
				t.Fatalf("invalid url %q: %v", tt.raw, err)
			}
			if parsed.Scheme != "https" {
				t.Errorf("expected https scheme, got %q", parsed.Scheme)
			}
			if parsed.Host != "pricing.us-east-1.amazonaws.com" {
				t.Errorf("expected pricing.us-east-1.amazonaws.com host, got %q", parsed.Host)
			}
		})
	}
}
