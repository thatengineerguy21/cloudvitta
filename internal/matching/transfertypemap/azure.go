package transfertypemap

import (
	"fmt"
)

var azureTransferTypeMap = map[string]string{
	"Bandwidth - Data Transfer Out":                "internet_egress",
	"Bandwidth Data Transfer Out":                  "internet_egress",
	"Data Transfer Out":                            "internet_egress",
	"Inter-Region":                                 "inter_region",
	"Intra-Region":                                 "intra_region",
	"Internet Egress":                              "internet_egress",
	"Internet":                                     "internet_egress",
	"Rtn Preference: MGN":                          "internet_egress",
	"Rtn Preference: Transit":                      "internet_egress",
	"Routing Preference: Microsoft Global Network": "internet_egress",
	"Routing Preference: Transit Provider":         "internet_egress",
	"Routing Preference":                           "internet_egress",
	"Microsoft Global Network":                     "internet_egress",
	"Standard Internet Egress":                     "internet_egress",
	"Routing Preference Internet Egress":           "internet_egress",
	"Routing Preference: Transit / ISP":            "internet_egress",
	"Routing Preference: Transit":                  "internet_egress",
	"Routing Preference: ISP":                      "internet_egress",
	"ExpressRoute":                                 "internet_egress",
	"Global":                                       "internet_egress",
}

// MapAzureTransferType resolves an Azure transfer type to a canonical transfer type.
func MapAzureTransferType(rawType string) (string, error) {
	canonical, ok := azureTransferTypeMap[rawType]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedTransferType, rawType)
	}
	return canonical, nil
}

// KnownAzureTransferTypes returns a copy of known Azure transfer type mappings.
func KnownAzureTransferTypes() map[string]string {
	m := make(map[string]string, len(azureTransferTypeMap))
	for k, v := range azureTransferTypeMap {
		m[k] = v
	}
	return m
}
