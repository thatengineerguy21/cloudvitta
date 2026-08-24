package transfertypemap

import (
	"fmt"
)

var ibmTransferTypeMap = map[string]string{
	// Public Egress / Internet Egress
	"VPC Public Egress":     "internet_egress",
	"Public Egress":         "internet_egress",
	"public-egress":         "internet_egress",
	"Public Gateway":        "internet_egress",
	"public-gateway-egress": "internet_egress",
	"Floating IP":           "internet_egress",
	"floating-ip-egress":    "internet_egress",
	"Internet Egress":       "internet_egress",
	"Data Transfer Out":     "internet_egress",
	"Internet":              "internet_egress",

	// Intra-Region and Inter-Region
	"Intra-Region Data Transfer": "intra_region",
	"Intra-Region":               "intra_region",
	"intra-region-egress":        "intra_region",
	"Inter-Region Data Transfer": "inter_region",
	"Inter-Region":               "inter_region",
	"inter-region-egress":        "inter_region",
}

// MapIBMTransferType resolves an IBM Cloud transfer type to a canonical transfer type.
func MapIBMTransferType(rawType string) (string, error) {
	canonical, ok := ibmTransferTypeMap[rawType]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedTransferType, rawType)
	}
	return canonical, nil
}

// KnownIBMTransferTypes returns a copy of known IBM Cloud transfer type mappings.
func KnownIBMTransferTypes() map[string]string {
	m := make(map[string]string, len(ibmTransferTypeMap))
	for k, v := range ibmTransferTypeMap {
		m[k] = v
	}
	return m
}
