package storageclassmap

import (
	"fmt"
)

var azureStorageClassMap = map[string]string{
	"Hot":         "standard",
	"Standard":    "standard",
	"Premium":     "standard",
	"Premium LRS": "standard",
	"Premium ZRS": "standard",
	"Cool":        "infrequent_access",
	"Cold":        "infrequent_access",
	"Archive":     "archive",
}

// MapAzureStorageClass resolves an Azure storage class to a canonical class.
func MapAzureStorageClass(rawClass string) (string, error) {
	canonical, ok := azureStorageClassMap[rawClass]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedStorageClass, rawClass)
	}
	return canonical, nil
}
