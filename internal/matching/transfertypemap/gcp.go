package transfertypemap

import (
	"fmt"
)

var gcpTransferTypeMap = map[string]string{
	// Standard/Premium Tiers
	"Premium Tier Internet Egress":                "internet_egress",
	"Standard Tier Internet Egress":               "internet_egress",

	// Internet Egress specific routes
	"Network Internet Egress":                     "internet_egress",
	"Internet Egress":                             "internet_egress",
	"Network Internet Egress from US to Americas": "internet_egress",
	"Network Internet Egress from APAC to APAC":   "internet_egress",
	"Network Internet Egress from EMEA to EMEA":   "internet_egress",
	"Network Internet Egress from NA to NA":       "internet_egress",
	"Network Internet Egress from Australia to":   "internet_egress",
	"Network Internet Egress from China to":       "internet_egress",

	// Inter-region and Intra-region
	"Network Inter Region Egress":                 "inter_region",
	"Inter Region Egress":                         "inter_region",
	"Network Intra Region Egress":                 "intra_region",
	"Intra Region Egress":                         "intra_region",

	// Interconnect/VPN
	"Cloud Interconnect Egress":                   "internet_egress",
	"Interconnect Egress":                         "internet_egress",
	"Cloud VPN Egress":                            "internet_egress",
	"VPN Egress":                                  "internet_egress",

	// Inter-zone
	"Network Inter-zone Egress":                   "intra_region",
	"Inter-zone Egress":                           "intra_region",
}

// MapGCPTransferType resolves a GCP transfer type to a canonical transfer type.
func MapGCPTransferType(rawType string) (string, error) {
	canonical, ok := gcpTransferTypeMap[rawType]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedTransferType, rawType)
	}
	return canonical, nil
}
