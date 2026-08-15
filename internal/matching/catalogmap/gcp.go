package catalogmap

import (
	"fmt"
)

var gcpCatalogMap = map[string]string{
	"Compute Engine":     "compute",
	"6F81-5844-456A":     "compute",
	"Cloud Storage":      "storage",
	"95FF-2EF5-5EA1":     "storage",
	"Cloud Interconnect": "network",
	"Networking":         "network",
	"Bandwidth":          "network",
	"E89B-A08C-8A2D":     "network",
}

// MapGCPProduct resolves a GCP service display name or service ID to a normalized service category.
// It fails loudly with ErrUnmappedProduct if the service is not explicitly mapped.
func MapGCPProduct(serviceName string) (string, error) {
	category, ok := gcpCatalogMap[serviceName]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedProduct, serviceName)
	}
	return category, nil
}
