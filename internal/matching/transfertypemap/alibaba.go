package transfertypemap

import (
	"fmt"
)

var alibabaTransferTypeMap = map[string]string{
	// Pay-By-Traffic Internet Egress
	"Pay-By-Traffic Internet Egress": "internet_egress",
	"Pay-By-Traffic":                 "internet_egress",
	"pay-by-traffic":                 "internet_egress",
	"pay-by-traffic-egress":          "internet_egress",
	"Internet Data Transfer":         "internet_egress",
	"Data Transfer Out":              "internet_egress",
	"data-transfer-out":              "internet_egress",
	"data_transfer":                  "internet_egress",
	"EIP":                            "internet_egress",
	"eip":                            "internet_egress",
	"Elastic IP":                     "internet_egress",
	"Internet Egress":                "internet_egress",
	"Internet":                       "internet_egress",

	// Intra-Region and Inter-Region
	"Intra-Region Data Transfer": "intra_region",
	"Intra-Region":               "intra_region",
	"intra-region":               "intra_region",
	"Inter-Region Data Transfer": "inter_region",
	"Inter-Region":               "inter_region",
	"inter-region":               "inter_region",
}

// MapAlibabaTransferType resolves an Alibaba Cloud transfer type to a canonical transfer type.
func MapAlibabaTransferType(rawType string) (string, error) {
	canonical, ok := alibabaTransferTypeMap[rawType]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedTransferType, rawType)
	}
	return canonical, nil
}

// KnownAlibabaTransferTypes returns a copy of known Alibaba Cloud transfer type mappings.
func KnownAlibabaTransferTypes() map[string]string {
	m := make(map[string]string, len(alibabaTransferTypeMap))
	for k, v := range alibabaTransferTypeMap {
		m[k] = v
	}
	return m
}
