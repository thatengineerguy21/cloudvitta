package storageclassmap

import (
	"fmt"
)

var providerResolvers = map[string]func(string) (string, error){
	"aws":          MapAWSStorageClass,
	"azure":        MapAzureStorageClass,
	"gcp":          MapGCPStorageClass,
	"oracle":       MapOracleStorageClass,
	"ibm":          MapIBMStorageClass,
	"alibaba":      MapAlibabaStorageClass,
	"digitalocean": MapDigitalOceanStorageClass,
}

// MapStorageClass resolves a provider-specific raw storage class name to a canonical storage class.
// It fails loudly with ErrUnmappedStorageClass if the raw class is not recognized for that provider.
func MapStorageClass(provider, rawClass string) (string, error) {
	resolver, ok := providerResolvers[provider]
	if !ok {
		return "", fmt.Errorf("storageclassmap: unmapped provider %q", provider)
	}
	return resolver(rawClass)
}
