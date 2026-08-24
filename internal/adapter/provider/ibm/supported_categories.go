package ibm

// supportedCategories maps service categories supported by the IBM Cloud provider adapter.
var supportedCategories = map[string]bool{
	"compute": true,
	"storage": true,
	"network": true,
}

// IsCategorySupported reports whether the specified category is supported by the IBM Cloud adapter.
func IsCategorySupported(category string) bool {
	return supportedCategories[category]
}

// SupportedCategories returns a copy of the supported category names for IBM Cloud.
func SupportedCategories() []string {
	return []string{"compute", "storage", "network"}
}
