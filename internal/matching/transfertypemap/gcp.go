package transfertypemap

import (
	"fmt"
)

var gcpTransferTypeMap = map[string]string{
	"Network Internet Egress from US to Americas": "internet_egress",
	"Network Internet Egress":                     "internet_egress",
	"Network Inter Region Egress":                 "inter_region",
	"Network Intra Region Egress":                 "intra_region",
	"Internet Egress":                             "internet_egress",
	"Inter Region Egress":                         "inter_region",
	"Intra Region Egress":                         "intra_region",
}

// MapGCPTransferType resolves a GCP transfer type to a canonical transfer type.
func MapGCPTransferType(rawType string) (string, error) {
	canonical, ok := gcpTransferTypeMap[rawType]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedTransferType, rawType)
	}
	return canonical, nil
}
