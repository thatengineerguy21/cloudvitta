package azure_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/azure"
)

func TestAzureClient_DefaultEndpoints(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "Compute endpoint",
			raw:  azure.DefaultRetailPricesURL,
			want: "https://prices.azure.com/api/retail/prices?$filter=serviceName%20eq%20'Virtual%20Machines'%20and%20armRegionName%20eq%20'eastus'%20and%20priceType%20eq%20'Consumption'",
		},
		{
			name: "Storage endpoint",
			raw:  azure.DefaultStorageRetailPricesURL,
			want: "https://prices.azure.com/api/retail/prices?$filter=serviceName%20eq%20'Storage'%20and%20armRegionName%20eq%20'eastus'%20and%20priceType%20eq%20'Consumption'",
		},
		{
			name: "Network endpoint",
			raw:  azure.DefaultNetworkRetailPricesURL,
			want: "https://prices.azure.com/api/retail/prices?$filter=serviceName%20eq%20'Bandwidth'%20and%20priceType%20eq%20'Consumption'",
		},
		{
			name: "Database endpoint",
			raw:  azure.DefaultDatabaseRetailPricesURL,
			want: "https://prices.azure.com/api/retail/prices?$filter=(serviceName%20eq%20'Azure%20Database%20for%20PostgreSQL'%20or%20serviceName%20eq%20'Azure%20Database%20for%20MySQL'%20or%20serviceName%20eq%20'SQL%20Database')%20and%20armRegionName%20eq%20'eastus'%20and%20priceType%20eq%20'Consumption'",
		},
		{
			name: "CosmosDB endpoint",
			raw:  azure.DefaultCosmosDBRetailPricesURL,
			want: "https://prices.azure.com/api/retail/prices?$filter=serviceName%20eq%20'Azure%20Cosmos%20DB'%20and%20armRegionName%20eq%20'eastus'%20and%20priceType%20eq%20'Consumption'",
		},
		{
			name: "Kubernetes endpoint",
			raw:  azure.DefaultKubernetesRetailPricesURL,
			want: "https://prices.azure.com/api/retail/prices?$filter=serviceName%20eq%20'Azure%20Kubernetes%20Service'%20and%20armRegionName%20eq%20'eastus'%20and%20priceType%20eq%20'Consumption'",
		},
		{
			name: "Functions serverless endpoint",
			raw:  azure.DefaultFunctionsRetailPricesURL,
			want: "https://prices.azure.com/api/retail/prices?$filter=(serviceName%20eq%20'Azure%20Functions'%20or%20serviceName%20eq%20'Functions')%20and%20armRegionName%20eq%20'eastus'%20and%20priceType%20eq%20'Consumption'",
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
			if parsed.Host != "prices.azure.com" {
				t.Errorf("expected prices.azure.com host, got %q", parsed.Host)
			}
			if !strings.HasPrefix(parsed.Path, "/api/retail/prices") {
				t.Errorf("expected path /api/retail/prices, got %q", parsed.Path)
			}
		})
	}
}
