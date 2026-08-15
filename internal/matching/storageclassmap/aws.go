package storageclassmap

import (
	"errors"
	"fmt"
)

// ErrUnmappedStorageClass is returned when a provider storage class has no entry in the map.
var ErrUnmappedStorageClass = errors.New("storageclassmap: unmapped storage class")

var awsStorageClassMap = map[string]string{
	"Standard":                     "standard",
	"General Purpose":              "standard",
	"Standard - Infrequent Access": "infrequent_access",
	"One Zone - Infrequent Access": "infrequent_access",
	"Glacier Flexible Retrieval":   "archive",
	"Glacier Deep Archive":         "archive",
}

// MapAWSStorageClass resolves an AWS storage class to a canonical class.
func MapAWSStorageClass(rawClass string) (string, error) {
	canonical, ok := awsStorageClassMap[rawClass]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedStorageClass, rawClass)
	}
	return canonical, nil
}
