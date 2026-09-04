package store

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

// UUIDToPg converts a standard google uuid.UUID to a pgtype.UUID.
func UUIDToPg(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

// PgToUUID converts a pgtype.UUID to a standard google uuid.UUID.
func PgToUUID(u pgtype.UUID) uuid.UUID {
	if u.Valid {
		return uuid.UUID(u.Bytes)
	}
	return uuid.Nil
}

// TimestamptzFromTime converts a time.Time to a pgtype.Timestamptz.
func TimestamptzFromTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}

// DateFromTime converts a time.Time to a pgtype.Date.
func DateFromTime(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: !t.IsZero()}
}

// DateFromString parses an ISO 8601 date string (YYYY-MM-DD) to a pgtype.Date.
func DateFromString(s string) (pgtype.Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return pgtype.Date{}, fmt.Errorf("parse date string %q: %w", s, err)
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

// TextFromString converts a Go string to a pgtype.Text.
func TextFromString(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

// NumericToDecimal converts a pgtype.Numeric to a shopspring/decimal.Decimal.
func NumericToDecimal(n pgtype.Numeric) (decimal.Decimal, error) {
	if !n.Valid {
		return decimal.Zero, nil
	}
	val, err := n.Value()
	if err != nil {
		return decimal.Zero, fmt.Errorf("read numeric value: %w", err)
	}
	if val == nil {
		return decimal.Zero, nil
	}
	strVal, ok := val.(string)
	if !ok {
		return decimal.Zero, fmt.Errorf("unexpected numeric driver type: %T", val)
	}
	return decimal.NewFromString(strVal)
}

// DecimalToNumeric converts a shopspring/decimal.Decimal to a pgtype.Numeric.
func DecimalToNumeric(d decimal.Decimal) (pgtype.Numeric, error) {
	var num pgtype.Numeric
	if err := num.Scan(d.String()); err != nil {
		return pgtype.Numeric{}, fmt.Errorf("scan decimal to numeric: %w", err)
	}
	return num, nil
}

// ToInsertPriceObservationParams constructs InsertPriceObservationParams from a domain PriceObservation.
func ToInsertPriceObservationParams(obs domain.PriceObservation, rawGCSPath string, anomalyStatus string) (InsertPriceObservationParams, error) {
	attrBytes, err := domain.MarshalAttributes(obs)
	if err != nil {
		return InsertPriceObservationParams{}, fmt.Errorf("marshal attributes for sku %s: %w", obs.SkuID, err)
	}

	priceAmt, err := DecimalToNumeric(obs.PriceAmount)
	if err != nil {
		return InsertPriceObservationParams{}, fmt.Errorf("convert price amount for sku %s: %w", obs.SkuID, err)
	}

	return InsertPriceObservationParams{
		Provider:        obs.Provider,
		ServiceCategory: obs.ServiceCategory,
		SkuID:           obs.SkuID,
		DisplayName:     obs.DisplayName,
		Region:          obs.Region,
		RegionGroup:     obs.RegionGroup,
		Unit:            obs.Unit,
		PriceAmount:     priceAmt,
		PriceCurrency:   obs.PriceCurrency,
		PricingModel:    obs.PricingModel,
		Attributes:      attrBytes,
		RawResponseRef:  TextFromString(rawGCSPath),
		FetchedAt:       TimestamptzFromTime(obs.FetchedAt),
		LastSeenAt:      TimestamptzFromTime(obs.FetchedAt),
		AnomalyStatus:   TextFromString(anomalyStatus),
	}, nil
}

// ToComputeCatalogItem converts a store.ComputeInstanceCatalog row to a domain.ComputeCatalogItem.
func ToComputeCatalogItem(row ComputeInstanceCatalog) (domain.ComputeCatalogItem, error) {
	vcpuDec, err := NumericToDecimal(row.Vcpu)
	if err != nil {
		return domain.ComputeCatalogItem{}, fmt.Errorf("convert vcpu for %s: %w", row.InstanceTypeID, err)
	}
	memDec, err := NumericToDecimal(row.MemoryGib)
	if err != nil {
		return domain.ComputeCatalogItem{}, fmt.Errorf("convert memory_gib for %s: %w", row.InstanceTypeID, err)
	}

	var attrs domain.ComputeAttributes
	if len(row.Attributes) > 0 {
		if err := json.Unmarshal(row.Attributes, &attrs); err != nil {
			return domain.ComputeCatalogItem{}, fmt.Errorf("unmarshal attributes for %s: %w", row.InstanceTypeID, err)
		}
	}

	var gpuType *string
	if row.GpuType.Valid {
		gpuType = &row.GpuType.String
	}

	return domain.ComputeCatalogItem{
		ID:              row.ID,
		Provider:        row.Provider,
		InstanceTypeID:  row.InstanceTypeID,
		DisplayName:     row.DisplayName,
		InstanceFamily:  row.InstanceFamily,
		Category:        row.Category,
		VCPU:            vcpuDec.InexactFloat64(),
		MemoryGiB:       memDec.InexactFloat64(),
		CPUArchitecture: row.CpuArchitecture,
		GPUCount:        row.GpuCount,
		GPUType:         gpuType,
		IsBurstable:     row.IsBurstable,
		IsCurrentGen:    row.IsCurrentGen,
		FirstSeenAt:     row.FirstSeenAt.Time,
		LastSeenAt:      row.LastSeenAt.Time,
		Attributes:      attrs,
	}, nil
}

// ToUpsertComputeCatalogItemParams constructs UpsertComputeCatalogItemParams from domain.ComputeCatalogItem.
func ToUpsertComputeCatalogItemParams(item domain.ComputeCatalogItem) (UpsertComputeCatalogItemParams, error) {
	vcpuNum, err := DecimalToNumeric(decimal.NewFromFloat(item.VCPU))
	if err != nil {
		return UpsertComputeCatalogItemParams{}, fmt.Errorf("convert vcpu for %s: %w", item.InstanceTypeID, err)
	}
	memNum, err := DecimalToNumeric(decimal.NewFromFloat(item.MemoryGiB))
	if err != nil {
		return UpsertComputeCatalogItemParams{}, fmt.Errorf("convert memory_gib for %s: %w", item.InstanceTypeID, err)
	}

	attrBytes, err := json.Marshal(item.Attributes)
	if err != nil {
		return UpsertComputeCatalogItemParams{}, fmt.Errorf("marshal attributes for %s: %w", item.InstanceTypeID, err)
	}

	var gpuType pgtype.Text
	if item.GPUType != nil {
		gpuType = TextFromString(*item.GPUType)
	}

	firstSeen := item.FirstSeenAt
	if firstSeen.IsZero() {
		firstSeen = time.Now().UTC()
	}
	lastSeen := item.LastSeenAt
	if lastSeen.IsZero() {
		lastSeen = time.Now().UTC()
	}

	return UpsertComputeCatalogItemParams{
		Provider:        item.Provider,
		InstanceTypeID:  item.InstanceTypeID,
		DisplayName:     item.DisplayName,
		InstanceFamily:  item.InstanceFamily,
		Category:        item.Category,
		Vcpu:            vcpuNum,
		MemoryGib:       memNum,
		CpuArchitecture: item.CPUArchitecture,
		GpuCount:        item.GPUCount,
		GpuType:         gpuType,
		IsBurstable:     item.IsBurstable,
		IsCurrentGen:    item.IsCurrentGen,
		FirstSeenAt:     TimestamptzFromTime(firstSeen),
		LastSeenAt:      TimestamptzFromTime(lastSeen),
		Attributes:      attrBytes,
	}, nil
}

// ToListComputeCatalogItemsParams constructs ListComputeCatalogItemsParams from domain.CatalogFilter.
func ToListComputeCatalogItemsParams(filter domain.CatalogFilter) (ListComputeCatalogItemsParams, error) {
	params := ListComputeCatalogItemsParams{
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}
	if filter.Provider != nil && *filter.Provider != "" {
		params.Provider = TextFromString(*filter.Provider)
	}
	if filter.Category != nil && *filter.Category != "" {
		params.Category = TextFromString(*filter.Category)
	}
	if filter.InstanceFamily != nil && *filter.InstanceFamily != "" {
		params.InstanceFamily = TextFromString(*filter.InstanceFamily)
	}
	if filter.MinVCPU != nil {
		num, err := DecimalToNumeric(decimal.NewFromFloat(*filter.MinVCPU))
		if err != nil {
			return params, err
		}
		params.MinVcpu = num
	}
	if filter.MaxVCPU != nil {
		num, err := DecimalToNumeric(decimal.NewFromFloat(*filter.MaxVCPU))
		if err != nil {
			return params, err
		}
		params.MaxVcpu = num
	}
	if filter.MinMemoryGiB != nil {
		num, err := DecimalToNumeric(decimal.NewFromFloat(*filter.MinMemoryGiB))
		if err != nil {
			return params, err
		}
		params.MinMemoryGib = num
	}
	if filter.MaxMemoryGiB != nil {
		num, err := DecimalToNumeric(decimal.NewFromFloat(*filter.MaxMemoryGiB))
		if err != nil {
			return params, err
		}
		params.MaxMemoryGib = num
	}
	return params, nil
}

// ToCountComputeCatalogItemsParams constructs CountComputeCatalogItemsParams from domain.CatalogFilter.
func ToCountComputeCatalogItemsParams(filter domain.CatalogFilter) (CountComputeCatalogItemsParams, error) {
	params := CountComputeCatalogItemsParams{}
	if filter.Provider != nil && *filter.Provider != "" {
		params.Provider = TextFromString(*filter.Provider)
	}
	if filter.Category != nil && *filter.Category != "" {
		params.Category = TextFromString(*filter.Category)
	}
	if filter.InstanceFamily != nil && *filter.InstanceFamily != "" {
		params.InstanceFamily = TextFromString(*filter.InstanceFamily)
	}
	if filter.MinVCPU != nil {
		num, err := DecimalToNumeric(decimal.NewFromFloat(*filter.MinVCPU))
		if err != nil {
			return params, err
		}
		params.MinVcpu = num
	}
	if filter.MaxVCPU != nil {
		num, err := DecimalToNumeric(decimal.NewFromFloat(*filter.MaxVCPU))
		if err != nil {
			return params, err
		}
		params.MaxVcpu = num
	}
	if filter.MinMemoryGiB != nil {
		num, err := DecimalToNumeric(decimal.NewFromFloat(*filter.MinMemoryGiB))
		if err != nil {
			return params, err
		}
		params.MinMemoryGib = num
	}
	if filter.MaxMemoryGiB != nil {
		num, err := DecimalToNumeric(decimal.NewFromFloat(*filter.MaxMemoryGiB))
		if err != nil {
			return params, err
		}
		params.MaxMemoryGib = num
	}
	return params, nil
}
