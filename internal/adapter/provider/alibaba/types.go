package alibaba

import (
	"encoding/json"

	"github.com/shopspring/decimal"
)

// PriceItem represents price info in Alibaba Cloud ECS responses.
type PriceItem struct {
	OriginalPrice decimal.Decimal `json:"OriginalPrice"`
	DiscountPrice decimal.Decimal `json:"DiscountPrice"`
	TradePrice    decimal.Decimal `json:"TradePrice"`
	Currency      string          `json:"Currency"`
}

// InstanceTypeItem represents an ECS instance type specification and its pricing.
type InstanceTypeItem struct {
	InstanceTypeID     string          `json:"InstanceTypeId"`
	CPUCoreCount       float64         `json:"CpuCoreCount"`
	MemorySize         float64         `json:"MemorySize"`
	InstanceTypeFamily string          `json:"InstanceTypeFamily"`
	InstanceFamily     string          `json:"InstanceFamily,omitempty"`
	Price              *PriceItem      `json:"Price,omitempty"`
	TradePrice         decimal.Decimal `json:"TradePrice,omitempty"`
	OriginalPrice      decimal.Decimal `json:"OriginalPrice,omitempty"`
	Currency           string          `json:"Currency,omitempty"`
	RegionID           string          `json:"RegionId,omitempty"`
	Regions            []string        `json:"Regions,omitempty"`
	ProductCode        string          `json:"ProductCode,omitempty"`
}

// CatalogResponse represents the top-level Alibaba Cloud ECS catalog or DescribePrice/DescribeInstanceTypes response.
type CatalogResponse struct {
	RequestID     string              `json:"RequestId"`
	TotalCount    int                 `json:"TotalCount"`
	InstanceTypes *InstanceTypesField `json:"InstanceTypes,omitempty"`
	Prices        []InstanceTypeItem  `json:"Prices,omitempty"`
	Items         []InstanceTypeItem  `json:"Items,omitempty"`
}

// InstanceTypesField accommodates both a direct array or an object wrapper { "InstanceType": [...] }.
type InstanceTypesField struct {
	List []InstanceTypeItem
}

// UnmarshalJSON implements custom JSON unmarshaling for InstanceTypesField.
func (f *InstanceTypesField) UnmarshalJSON(data []byte) error {
	// Try unmarshaling as direct slice: [{"InstanceTypeId": ...}]
	var list []InstanceTypeItem
	if err := json.Unmarshal(data, &list); err == nil {
		f.List = list
		return nil
	}

	// Try unmarshaling as wrapped object: {"InstanceType": [{"InstanceTypeId": ...}]}
	var wrapper struct {
		InstanceType []InstanceTypeItem `json:"InstanceType"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return err
	}
	f.List = wrapper.InstanceType
	return nil
}
