package azure

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
	"golang.org/x/sync/errgroup"
)

// Adapter handles fetching, persisting raw response to GCS, and normalizing Azure compute pricing.
type Adapter struct {
	client  *Client
	storage storage.RawStorage
}

// NewAdapter constructs a new Azure provider adapter.
func NewAdapter(client *Client, st storage.RawStorage) *Adapter {
	return &Adapter{
		client:  client,
		storage: st,
	}
}

// Fetch retrieves the Azure price list, concurrently streams raw JSON to storage and normalizes it.
// Returns a domain.FetchResult containing the observations and raw GCS path, and any error.
func (a *Adapter) Fetch(ctx context.Context) (domain.FetchResult, error) {
	fetchedAt := time.Now().UTC()
	fetchID := uuid.New().String()
	dateStr := fetchedAt.Format("2006-01-02")
	gcsPath := fmt.Sprintf("raw/azure/compute/%s/%s.json", dateStr, fetchID)

	body, err := a.client.FetchPriceList(ctx)
	if err != nil {
		return domain.FetchResult{}, fmt.Errorf("azure adapter: fetch price list: %w", err)
	}
	defer func() { _ = body.Close() }()

	pr, pw := io.Pipe()
	tee := io.TeeReader(body, pw)

	var observations []domain.PriceObservation
	g, gctx := errgroup.WithContext(ctx)

	// Goroutine 1: Write raw payload stream to storage
	g.Go(func() error {
		if err := a.storage.WriteStream(gctx, gcsPath, pr); err != nil {
			_ = pr.CloseWithError(err)
			return fmt.Errorf("azure adapter: storage write: %w", err)
		}
		return nil
	})

	// Goroutine 2: Parse and normalize streamed JSON
	g.Go(func() error {
		var normErr error
		observations, normErr = Normalize(tee, fetchedAt)
		if normErr != nil {
			_ = pw.CloseWithError(normErr)
			return fmt.Errorf("azure adapter: normalize: %w", normErr)
		}
		_ = pw.Close()
		return nil
	})

	if err := g.Wait(); err != nil {
		return domain.FetchResult{}, err
	}

	return domain.FetchResult{
		Observations: observations,
		RawGCSPath:   gcsPath,
	}, nil
}
