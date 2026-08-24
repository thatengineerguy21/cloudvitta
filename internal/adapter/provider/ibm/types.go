package ibm

import "github.com/shopspring/decimal"

// IAMTokenResponse represents the JSON response from IBM Cloud IAM OAuth 2.0 token endpoint.
type IAMTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Expiration   int64  `json:"expiration"`
}

// PriceTier represents a quantity tier and price in IBM Cloud Global Catalog pricing.
type PriceTier struct {
	QuantityTier int             `json:"quantity_tier"`
	Price        decimal.Decimal `json:"price"`
}

// PricingAmount represents pricing amounts for a country and currency in IBM Cloud Global Catalog.
type PricingAmount struct {
	Country  string      `json:"country,omitempty"`
	Currency string      `json:"currency"`
	Prices   []PriceTier `json:"prices"`
}

// PricingMetric represents a billing metric entry in IBM Cloud Global Catalog pricing.
type PricingMetric struct {
	MetricID       string          `json:"metric_id"`
	TierModel      string          `json:"tier_model,omitempty"`
	ChargeUnit     string          `json:"charge_unit,omitempty"`
	ChargeUnitName string          `json:"charge_unit_name,omitempty"`
	Amounts        []PricingAmount `json:"amounts"`
}

// Pricing represents pricing metadata and metrics for an IBM Cloud service or plan.
type Pricing struct {
	Type    string          `json:"type,omitempty"`
	Origin  string          `json:"origin,omitempty"`
	Metrics []PricingMetric `json:"metrics"`
}

// OverviewUIDetails holds localized display information in IBM Cloud Global Catalog.
type OverviewUIDetails struct {
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
}

// Resource represents a service, plan, or deployment resource in IBM Cloud Global Catalog.
type Resource struct {
	ID         string                       `json:"id"`
	Name       string                       `json:"name"`
	Kind       string                       `json:"kind"`
	GeoTags    []string                     `json:"geo_tags,omitempty"`
	OverviewUI map[string]OverviewUIDetails `json:"overview_ui,omitempty"`
	Pricing    *Pricing                     `json:"pricing,omitempty"`
	Children   []Resource                   `json:"children,omitempty"`
}

// CatalogResponse represents the top-level payload returned by IBM Cloud Global Catalog API.
type CatalogResponse struct {
	Offset     int        `json:"offset,omitempty"`
	Limit      int        `json:"limit,omitempty"`
	TotalCount int        `json:"total_count,omitempty"`
	Resources  []Resource `json:"resources"`
}
