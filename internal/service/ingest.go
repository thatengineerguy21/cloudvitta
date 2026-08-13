package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/aws"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

// IngestionService orchestrates fetching, normalizing, storage persistence,
// and database insertion of provider pricing observations.
type IngestionService struct {
	queries    *store.Queries
	awsAdapter *aws.Adapter
}

// NewIngestionService constructs a new IngestionService.
func NewIngestionService(queries *store.Queries, awsAdapter *aws.Adapter) *IngestionService {
	return &IngestionService{
		queries:    queries,
		awsAdapter: awsAdapter,
	}
}

// RunAWSComputeIngestion triggers the AWS EC2 compute pricing ingestion pipeline:
// fetches AWS price list, streams raw payload to storage, normalizes pricing records,
// and inserts each observation into Postgres via sqlc queries.
// Returns the total number of inserted records.
func (s *IngestionService) RunAWSComputeIngestion(ctx context.Context) (int, error) {
	observations, gcsPath, err := s.awsAdapter.Fetch(ctx)
	if err != nil {
		return 0, fmt.Errorf("ingest service: fetch aws compute: %w", err)
	}

	insertedCount := 0
	for _, obs := range observations {
		attrBytes, err := json.Marshal(obs.Attributes)
		if err != nil {
			return insertedCount, fmt.Errorf("ingest service: marshal attributes for sku %s: %w", obs.SkuID, err)
		}

		var priceAmt pgtype.Numeric
		if err := priceAmt.Scan(obs.PriceAmount.String()); err != nil {
			return insertedCount, fmt.Errorf("ingest service: scan price amount for sku %s: %w", obs.SkuID, err)
		}

		params := store.InsertPriceObservationParams{
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
			RawResponseRef:  pgtype.Text{String: gcsPath, Valid: gcsPath != ""},
			FetchedAt:       pgtype.Timestamptz{Time: obs.FetchedAt, Valid: !obs.FetchedAt.IsZero()},
			LastSeenAt:      pgtype.Timestamptz{Time: obs.FetchedAt, Valid: !obs.FetchedAt.IsZero()},
			AnomalyStatus:   pgtype.Text{Valid: false},
		}

		if _, err := s.queries.InsertPriceObservation(ctx, params); err != nil {
			return insertedCount, fmt.Errorf("ingest service: insert observation for sku %s: %w", obs.SkuID, err)
		}
		insertedCount++
	}

	return insertedCount, nil
}
