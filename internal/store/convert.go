package store

import (
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
