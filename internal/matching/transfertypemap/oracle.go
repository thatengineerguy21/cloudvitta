package transfertypemap

import (
	"fmt"
)

var oracleTransferTypeMap = map[string]string{
	// Internet Egress
	"Outbound Data Transfer":            "internet_egress",
	"Outbound Data Transfer (Internet)": "internet_egress",
	"Data Transfer Out":                 "internet_egress",
	"Data Transfer Out (Internet)":      "internet_egress",
	"Internet Egress":                   "internet_egress",
	"Internet Data Transfer":            "internet_egress",
	"Internet":                          "internet_egress",

	// Intra-Region and Inter-Region
	"Intra-Region Data Transfer": "intra_region",
	"Intra-Region":               "intra_region",
	"Inter-Region Data Transfer": "inter_region",
	"Inter-Region":               "inter_region",
}

// MapOracleTransferType resolves an Oracle OCI transfer type to a canonical transfer type.
func MapOracleTransferType(rawType string) (string, error) {
	canonical, ok := oracleTransferTypeMap[rawType]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedTransferType, rawType)
	}
	return canonical, nil
}

// KnownOracleTransferTypes returns a copy of known Oracle transfer type mappings.
func KnownOracleTransferTypes() map[string]string {
	m := make(map[string]string, len(oracleTransferTypeMap))
	for k, v := range oracleTransferTypeMap {
		m[k] = v
	}
	return m
}
