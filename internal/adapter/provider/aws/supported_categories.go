package aws

// SupportedCategories maps service categories supported by the AWS provider adapter.
var SupportedCategories = map[string]bool{
	"compute": true,
	"storage": true,
	"network": true,
}

// IsCategorySupported reports whether the specified category is supported by the AWS adapter.
func IsCategorySupported(category string) bool {
	return SupportedCategories[category]
}
