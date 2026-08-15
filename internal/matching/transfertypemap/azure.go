package transfertypemap

import (
	"fmt"
)

var azureTransferTypeMap = map[string]string{
	"Bandwidth - Data Transfer Out": "internet_egress",
	"Bandwidth Data Transfer Out":   "internet_egress",
	"Data Transfer Out":             "internet_egress",
	"Inter-Region":                  "inter_region",
	"Intra-Region":                  "intra_region",
	"Internet Egress":               "internet_egress",
	"Internet":                      "internet_egress",
}

// MapAzureTransferType resolves an Azure transfer type to a canonical transfer type.
func MapAzureTransferType(rawType string) (string, error) {
	canonical, ok := azureTransferTypeMap[rawType]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedTransferType, rawType)
	}
	return canonical, nil
}
