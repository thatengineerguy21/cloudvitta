package transfertypemap

import (
	"errors"
	"fmt"
)

// ErrUnmappedTransferType is returned when a provider transfer type has no entry in the map.
var ErrUnmappedTransferType = errors.New("transfertypemap: unmapped transfer type")

var awsTransferTypeMap = map[string]string{
	"AWS Data Transfer Out":                "internet_egress",
	"AWS Data Transfer Out to Internet":    "internet_egress",
	"AWS Tiered Data Transfer Out":         "internet_egress",
	"Data Transfer Out":                    "internet_egress",
	"Data Transfer Out (Internet)":         "internet_egress",
	"Data Transfer Internet (Out)":         "internet_egress",
	"Data Transfer Out (Inter-Region)":     "inter_region",
	"Data Transfer Out (Intra-Region)":     "intra_region",
	"Region to Region Data Transfer":       "inter_region",
	"Intra-Region Data Transfer":           "intra_region",
	"Direct Connect Data Transfer Out":     TransferTypeDirectConnectEgress,
	"Direct Connect Egress":                TransferTypeDirectConnectEgress,
	"AWS Direct Connect Data Transfer Out": TransferTypeDirectConnectEgress,
	"VPN Data Transfer Out":                TransferTypeVPNEgress,
	"AWS VPN Data Transfer Out":            TransferTypeVPNEgress,
	"VPN Egress":                           TransferTypeVPNEgress,
	"CloudFront Data Transfer Out":         "internet_egress",
	"Internet":                             "internet_egress",
	"Inter-Region":                         "inter_region",
	"Intra-Region":                         "intra_region",
}

// MapAWSTransferType resolves an AWS transfer type to a canonical transfer type.
func MapAWSTransferType(rawType string) (string, error) {
	canonical, ok := awsTransferTypeMap[rawType]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedTransferType, rawType)
	}
	return canonical, nil
}

// KnownAWSTransferTypes returns a copy of known AWS transfer type mappings.
func KnownAWSTransferTypes() map[string]string {
	m := make(map[string]string, len(awsTransferTypeMap))
	for k, v := range awsTransferTypeMap {
		m[k] = v
	}
	return m
}
