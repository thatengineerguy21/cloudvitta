package oracle

// PriceValue represents a specific pricing rate entry in Oracle CE Tools API.
type PriceValue struct {
	Model string  `json:"model"`
	Value float64 `json:"value"`
}

// CurrencyPrice represents prices in a specific currency in Oracle CE Tools API.
type CurrencyPrice struct {
	CurrencyCode string       `json:"currencyCode"`
	Prices       []PriceValue `json:"prices"`
}

// ProductItem represents a product item in the Oracle CE Tools API response.
type ProductItem struct {
	PartNumber                 string          `json:"partNumber"`
	DisplayName                string          `json:"displayName"`
	Description                string          `json:"description,omitempty"`
	MetricName                 string          `json:"metricName"`
	ServiceCategory            string          `json:"serviceCategory"`
	ServiceCategoryDisplayName string          `json:"serviceCategoryDisplayName,omitempty"`
	Prices                     []CurrencyPrice `json:"prices"`
	Regions                    []string        `json:"regions,omitempty"`
}

// ProductResponse represents the top-level payload returned by Oracle CE Tools API.
type ProductResponse struct {
	Items []ProductItem `json:"items"`
}
