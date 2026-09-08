package nosqldatamodelmap

import (
	"fmt"
	"strings"
)

var gcpDataModelMap = map[string]string{
	"cloud firestore":       DataModelDocument,
	"firestore":             DataModelDocument,
	"cloud datastore":       DataModelDocument,
	"datastore":             DataModelDocument,
	"document":              DataModelDocument,
	"doc":                   DataModelDocument,
	"json":                  DataModelDocument,
	"key_value":             DataModelKeyValue,
	"key-value":             DataModelKeyValue,
	"keyvalue":              DataModelKeyValue,
	"kv":                    DataModelKeyValue,
	"bigtable":              DataModelWideColumn,
	"cloud bigtable":        DataModelWideColumn,
	"google cloud bigtable": DataModelWideColumn,
	"wide_column":           DataModelWideColumn,
	"wide-column":           DataModelWideColumn,
	"widecolumn":            DataModelWideColumn,
	"graph":                 DataModelGraph,
	"multi_model":           DataModelMultiModel,
	"multi-model":           DataModelMultiModel,
	"multimodel":            DataModelMultiModel,
}

// MapGCPDataModel maps a GCP Firestore / Datastore / NoSQL model string to a canonical data model identifier.
func MapGCPDataModel(rawModel string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(rawModel))
	if canonical, ok := gcpDataModelMap[key]; ok {
		return canonical, nil
	}
	return "", fmt.Errorf("%w: gcp data model %q", ErrUnmappedDataModel, rawModel)
}
