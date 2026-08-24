package transfertypemap

import (
	"fmt"
)

var digitalOceanTransferTypeMap = map[string]string{
	// Bandwidth Egress
	"Bandwidth":          "internet_egress",
	"bandwidth":          "internet_egress",
	"Bandwidth Transfer": "internet_egress",
	"bandwidth-transfer": "internet_egress",
	"Bandwidth Overage":  "internet_egress",
	"bandwidth-overage":  "internet_egress",
	"bandwidth-egress":   "internet_egress",
	"Data Transfer Out":  "internet_egress",
	"Internet Egress":    "internet_egress",
	"Internet":           "internet_egress",

	// Intra-Region and Inter-Region
	"Intra-Region Data Transfer": "intra_region",
	"Intra-Region":               "intra_region",
	"Inter-Region Data Transfer": "inter_region",
	"Inter-Region":               "inter_region",
}

// MapDigitalOceanTransferType resolves a DigitalOcean transfer type to a canonical transfer type.
func MapDigitalOceanTransferType(rawType string) (string, error) {
	canonical, ok := digitalOceanTransferTypeMap[rawType]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedTransferType, rawType)
	}
	return canonical, nil
}

// KnownDigitalOceanTransferTypes returns a copy of known DigitalOcean transfer type mappings.
func KnownDigitalOceanTransferTypes() map[string]string {
	m := make(map[string]string, len(digitalOceanTransferTypeMap))
	for k, v := range digitalOceanTransferTypeMap {
		m[k] = v
	}
	return m
}
