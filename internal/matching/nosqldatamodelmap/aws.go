package nosqldatamodelmap

import (
	"fmt"
	"strings"
)

var awsDataModelMap = map[string]string{
	"amazondynamodb": DataModelDocument,
	"dynamodb":       DataModelDocument,
	"document":       DataModelDocument,
	"doc":            DataModelDocument,
	"json":           DataModelDocument,
	"key_value":      DataModelKeyValue,
	"key-value":      DataModelKeyValue,
	"keyvalue":       DataModelKeyValue,
	"kv":             DataModelKeyValue,
	"wide_column":    DataModelWideColumn,
	"wide-column":    DataModelWideColumn,
	"widecolumn":     DataModelWideColumn,
	"graph":          DataModelGraph,
	"multi_model":    DataModelMultiModel,
	"multi-model":    DataModelMultiModel,
	"multimodel":     DataModelMultiModel,
}

// MapAWSDataModel maps an AWS NoSQL data model string to a canonical data model identifier.
func MapAWSDataModel(rawModel string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(rawModel))
	canonical, ok := awsDataModelMap[key]
	if !ok {
		return "", fmt.Errorf("%w: aws data model %q", ErrUnmappedDataModel, rawModel)
	}
	return canonical, nil
}
