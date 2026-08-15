package aws

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

// Adapter handles fetching, persisting raw response to GCS, and normalizing AWS EC2 compute pricing.
type Adapter struct {
	client  *Client
	storage storage.RawStorage
}

// NewAdapter constructs a new AWS provider adapter.
func NewAdapter(client *Client, st storage.RawStorage) *Adapter {
	return &Adapter{
		client:  client,
		storage: st,
	}
}

// Fetch retrieves the AWS price list, concurrently streams raw JSON to storage and normalizes it.
// Returns a domain.FetchResult containing the observations and raw GCS path, and any error.
func (a *Adapter) Fetch(ctx context.Context, limiter *rate.Limiter) (domain.FetchResult, error) {
	if limiter != nil {
		if err := limiter.Wait(ctx); err != nil {
			return domain.FetchResult{}, fmt.Errorf("aws adapter: rate limit wait: %w", err)
		}
	}

	fetchedAt := time.Now().UTC()
	fetchID := uuid.New().String()
	dateStr := fetchedAt.Format("2006-01-02")
	gcsPath := fmt.Sprintf("raw/aws/compute/%s/%s.json", dateStr, fetchID)

	body, err := a.client.FetchPriceList(ctx)
	if err != nil {
		return domain.FetchResult{}, fmt.Errorf("aws adapter: fetch price list: %w", err)
	}
	defer func() { _ = body.Close() }()

	pr, pw := io.Pipe()
	tee := io.TeeReader(body, provider.IgnoreErrorWriter{W: pw})

	var observations []domain.PriceObservation
	g, _ := errgroup.WithContext(ctx)

	// Goroutine 1: Normalize reads from pr
	g.Go(func() error {
		var normErr error
		observations, normErr = Normalize(pr, fetchedAt)
		if normErr != nil {
			_ = pr.CloseWithError(normErr)
			return fmt.Errorf("aws adapter: normalize: %w", normErr)
		}
		_ = pr.Close()
		return nil
	})

	// Goroutine 2: GCS reads from tee
	g.Go(func() error {
		defer func() { _ = pw.Close() }()
		if err := a.storage.WriteStream(ctx, gcsPath, tee); err != nil {
			_ = pw.CloseWithError(err)
			return fmt.Errorf("aws adapter: storage write: %w", err)
		}
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
