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
