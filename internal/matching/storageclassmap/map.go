package storageclassmap

import (
	"fmt"
)

// MapStorageClass resolves a provider-specific storage class to a canonical storage class.
func MapStorageClass(provider, rawClass string) (string, error) {
	switch provider {
	case "aws":
		return MapAWSStorageClass(rawClass)
	case "azure":
		return MapAzureStorageClass(rawClass)
	case "gcp":
		return MapGCPStorageClass(rawClass)
	default:
		return "", fmt.Errorf("storageclassmap: unmapped provider %q", provider)
	}
}
