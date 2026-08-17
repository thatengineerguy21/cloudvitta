package storageclassmap

import (
	"fmt"
)

var azureStorageClassMap = map[string]string{
	"Hot":              "standard",
	"Standard":         "standard",
	"Premium":          "standard",
	"Premium LRS":      "standard",
	"Premium ZRS":      "standard",
	"SSD":              "standard",
	"SSD LRS":          "standard",
	"SSD ZRS":          "standard",
	"Standard SSD":     "standard",
	"Standard SSD LRS": "standard",
	"Standard SSD ZRS": "standard",
	"Premium SSD":      "standard",
	"Premium SSD LRS":  "standard",
	"Premium SSD ZRS":  "standard",
	"Ultra SSD":        "standard",
	"Ultra SSD LRS":    "standard",
	"HDD":              "standard",
	"Standard HDD":     "standard",
	"Standard HDD LRS": "standard",
	"Standard LRS":     "standard",
	"Standard ZRS":     "standard",
	"Standard GRS":     "standard",
	"Hot LRS":          "standard",
	"Hot ZRS":          "standard",
	"Cool":             "infrequent_access",
	"Cool LRS":         "infrequent_access",
	"Cool ZRS":         "infrequent_access",
	"Cold":             "infrequent_access",
	"Cold LRS":         "infrequent_access",
	"Cold ZRS":         "infrequent_access",
	"Archive":          "archive",
	"Archive LRS":      "archive",
	"Archive ZRS":      "archive",
}

// MapAzureStorageClass resolves an Azure storage class to a canonical class.
func MapAzureStorageClass(rawClass string) (string, error) {
	canonical, ok := azureStorageClassMap[rawClass]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedStorageClass, rawClass)
	}
	return canonical, nil
}
