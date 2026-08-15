package catalogmap

import (
	"fmt"
)

var azureCatalogMap = map[string]string{
	"Virtual Machines": "compute",
	"Storage":          "storage",
	"Bandwidth":        "network",
}

// MapAzureProduct resolves an Azure service name to a normalized service category.
// It fails loudly with ErrUnmappedProduct if the service name is not explicitly mapped.
func MapAzureProduct(serviceName string) (string, error) {
	category, ok := azureCatalogMap[serviceName]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedProduct, serviceName)
	}
	return category, nil
}
