package transfertypemap

import (
	"fmt"
)

// MapTransferType resolves a provider-specific transfer type to a canonical transfer type.
func MapTransferType(provider, rawType string) (string, error) {
	switch provider {
	case "aws":
		return MapAWSTransferType(rawType)
	case "azure":
		return MapAzureTransferType(rawType)
	case "gcp":
		return MapGCPTransferType(rawType)
	default:
		return "", fmt.Errorf("transfertypemap: unmapped provider %q", provider)
	}
}
