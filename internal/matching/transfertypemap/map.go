package transfertypemap

import (
	"fmt"
)

var providerResolvers = map[string]func(string) (string, error){
	"aws":          MapAWSTransferType,
	"azure":        MapAzureTransferType,
	"gcp":          MapGCPTransferType,
	"oracle":       MapOracleTransferType,
	"ibm":          MapIBMTransferType,
	"alibaba":      MapAlibabaTransferType,
	"digitalocean": MapDigitalOceanTransferType,
}

// MapTransferType resolves a provider-specific raw network transfer type to a canonical transfer type.
// It fails loudly with ErrUnmappedTransferType if the raw type is not recognized for that provider.
func MapTransferType(provider, rawType string) (string, error) {
	resolver, ok := providerResolvers[provider]
	if !ok {
		return "", fmt.Errorf("transfertypemap: unmapped provider %q", provider)
	}
	return resolver(rawType)
}
