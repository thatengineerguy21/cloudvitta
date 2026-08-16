package service

import "github.com/shopspring/decimal"

// CalculateStorageMonthlyCost calculates the estimated monthly storage cost given unit price and size in GB.
// This encapsulates pricing arithmetic in the service layer (08-CONSISTENCY-RULES.md).
func CalculateStorageMonthlyCost(unitPrice, sizeGB decimal.Decimal) decimal.Decimal {
	return unitPrice.Mul(sizeGB)
}

// CalculateNetworkMonthlyCost calculates the estimated monthly network egress cost given unit price and egress in GB.
// This encapsulates pricing arithmetic in the service layer (08-CONSISTENCY-RULES.md).
func CalculateNetworkMonthlyCost(unitPrice, egressGB decimal.Decimal) decimal.Decimal {
	return unitPrice.Mul(egressGB)
}

// HoursInMonth defines the standard average hours in a month (365 days * 24 hours / 12 months = 730 hours).
var HoursInMonth = decimal.NewFromInt(730)

// CalculateStorageHourlyCost calculates normalized hourly storage cost from unit price ($/GB-mo) and size in GB.
// Formula: (unitPrice * sizeGB) / 730
func CalculateStorageHourlyCost(unitPrice, sizeGB decimal.Decimal) decimal.Decimal {
	return CalculateStorageMonthlyCost(unitPrice, sizeGB).Div(HoursInMonth)
}

// CalculateNetworkHourlyCost calculates normalized hourly network egress cost from unit price ($/GB) and egress in GB.
// Formula: (unitPrice * egressGB) / 730
func CalculateNetworkHourlyCost(unitPrice, egressGB decimal.Decimal) decimal.Decimal {
	return CalculateNetworkMonthlyCost(unitPrice, egressGB).Div(HoursInMonth)
}
