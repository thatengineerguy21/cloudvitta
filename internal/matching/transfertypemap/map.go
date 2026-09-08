package transfertypemap

import (
	"fmt"
)

// Canonical network transfer types across cloud providers.
const (
	TransferTypeInternetEgress      = "internet_egress"
	TransferTypeInterRegion         = "inter_region"
	TransferTypeIntraRegion         = "intra_region"
	TransferTypeDirectConnectEgress = "direct_connect_egress"
	TransferTypeVPNEgress           = "vpn_egress"
)

// SupportedTransferTypes returns the list of all canonical network transfer types.
func SupportedTransferTypes() []string {
	return []string{
		TransferTypeInternetEgress,
		TransferTypeInterRegion,
		TransferTypeIntraRegion,
		TransferTypeDirectConnectEgress,
		TransferTypeVPNEgress,
	}
}

// IsValidTransferType reports whether the specified transfer type is a recognized canonical type.
func IsValidTransferType(transferType string) bool {
	switch transferType {
	case TransferTypeInternetEgress,
		TransferTypeInterRegion,
		TransferTypeIntraRegion,
		TransferTypeDirectConnectEgress,
		TransferTypeVPNEgress:
		return true
	default:
		return false
	}
}

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
