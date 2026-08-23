package nosqldatamodelmap

import (
	"errors"
	"fmt"
	"strings"
)

// Canonical NoSQL database data model taxonomy constants.
const (
	DataModelDocument   = "document"
	DataModelKeyValue   = "key_value"
	DataModelWideColumn = "wide_column"
	DataModelGraph      = "graph"
	DataModelMultiModel = "multi_model"
)

// ErrUnmappedDataModel is returned when a raw data model value cannot be mapped to canonical taxonomy.
var ErrUnmappedDataModel = errors.New("nosqldatamodelmap: unmapped data model")

// SupportedCanonicalDataModels returns the list of canonical NoSQL data models active in the current stage.
func SupportedCanonicalDataModels() []string {
	return []string{
		DataModelDocument,
		DataModelKeyValue,
		DataModelWideColumn,
		DataModelGraph,
		DataModelMultiModel,
	}
}

// IsSupportedStageDataModel returns true if the data model is active in the current stage.
func IsSupportedStageDataModel(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case DataModelDocument, DataModelKeyValue, DataModelWideColumn, DataModelGraph, DataModelMultiModel:
		return true
	default:
		return false
	}
}

// ResolveCanonicalDataModel maps a user-supplied data model string to a canonical NoSQL data model.
// It checks canonical constants first, then tries provider-specific mappings across AWS, Azure, and GCP.
// If rawModel cannot be mapped, ErrUnmappedDataModel is returned.
func ResolveCanonicalDataModel(rawModel string) (string, error) {
	trimmed := strings.TrimSpace(rawModel)
	if trimmed == "" {
		return "", nil
	}

	lower := strings.ToLower(trimmed)
	switch lower {
	case DataModelDocument, DataModelKeyValue, DataModelWideColumn, DataModelGraph, DataModelMultiModel:
		return lower, nil
	}

	for _, prov := range []string{"aws", "azure", "gcp"} {
		if norm, err := NormalizeDataModel(prov, trimmed); err == nil {
			return norm, nil
		}
	}

	return "", fmt.Errorf("%w: %q", ErrUnmappedDataModel, rawModel)
}

// NormalizeDataModel resolves a provider-specific raw data model name to a canonical NoSQL data model.
func NormalizeDataModel(provider, rawModel string) (string, error) {
	switch provider {
	case "aws":
		return MapAWSDataModel(rawModel)
	case "azure":
		return MapAzureDataModel(rawModel)
	case "gcp":
		return MapGCPDataModel(rawModel)
	default:
		return "", fmt.Errorf("nosqldatamodelmap: unmapped provider %q", provider)
	}
}
