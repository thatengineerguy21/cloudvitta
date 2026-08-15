package transfertypemap

import (
	"errors"
	"fmt"
)

// ErrUnmappedTransferType is returned when a provider transfer type has no entry in the map.
var ErrUnmappedTransferType = errors.New("transfertypemap: unmapped transfer type")

var awsTransferTypeMap = map[string]string{
	"AWS Data Transfer Out":            "internet_egress",
	"Data Transfer Out (Internet)":     "internet_egress",
	"Data Transfer Out (Inter-Region)": "inter_region",
	"Data Transfer Out (Intra-Region)": "intra_region",
	"Internet":                         "internet_egress",
	"Inter-Region":                     "inter_region",
	"Intra-Region":                     "intra_region",
}

// MapAWSTransferType resolves an AWS transfer type to a canonical transfer type.
func MapAWSTransferType(rawType string) (string, error) {
	canonical, ok := awsTransferTypeMap[rawType]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedTransferType, rawType)
	}
	return canonical, nil
}
