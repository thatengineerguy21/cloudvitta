package nosqldatamodelmap

import (
	"fmt"
	"strings"
)

var azureDataModelMap = map[string]string{
	"azure cosmos db":  DataModelDocument,
	"cosmos db":        DataModelDocument,
	"cosmos":           DataModelDocument,
	"sql":              DataModelDocument,
	"nosql":            DataModelDocument,
	"core":             DataModelDocument,
	"mongodb":          DataModelDocument,
	"mongo":            DataModelDocument,
	"document":         DataModelDocument,
	"doc":              DataModelDocument,
	"json":             DataModelDocument,
	"table":            DataModelKeyValue,
	"azure table":      DataModelKeyValue,
	"key_value":        DataModelKeyValue,
	"key-value":        DataModelKeyValue,
	"keyvalue":         DataModelKeyValue,
	"kv":               DataModelKeyValue,
	"cassandra":        DataModelWideColumn,
	"apache cassandra": DataModelWideColumn,
	"wide_column":      DataModelWideColumn,
	"wide-column":      DataModelWideColumn,
	"widecolumn":       DataModelWideColumn,
	"gremlin":          DataModelGraph,
	"apache gremlin":   DataModelGraph,
	"graph":            DataModelGraph,
	"multi_model":      DataModelMultiModel,
	"multi-model":      DataModelMultiModel,
	"multimodel":       DataModelMultiModel,
}

// MapAzureDataModel maps an Azure Cosmos DB API / model string to a canonical data model identifier.
func MapAzureDataModel(rawModel string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(rawModel))
	canonical, ok := azureDataModelMap[key]
	if !ok {
		return "", fmt.Errorf("%w: azure data model %q", ErrUnmappedDataModel, rawModel)
	}
	return canonical, nil
}
