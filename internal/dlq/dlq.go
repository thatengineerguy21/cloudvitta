package dlq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrEntryNotFound = errors.New("dlq: entry not found")
)

// Entry represents a single DLQ record for a failed ingestion job.
type Entry struct {
	Provider            string    `json:"provider"`
	Category            string    `json:"category"`
	Timestamp           time.Time `json:"timestamp"`
	LastError           string    `json:"last_error"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	Status              string    `json:"status"` // "failed" or "blocked"
}

// DLQ struct
type DLQ struct {
	client    redis.Cmdable
	keyPrefix string // default "ingestion:dlq"
}

// New constructor
func New(client redis.Cmdable) *DLQ {
	return &DLQ{
		client:    client,
		keyPrefix: "ingestion:dlq",
	}
}

func (d *DLQ) key(provider, category string) string {
	return fmt.Sprintf("%s:%s:%s", d.keyPrefix, provider, category)
}

func (d *DLQ) Record(ctx context.Context, provider, category string, jobErr error) error {
	k := d.key(provider, category)

	entry, err := d.Get(ctx, provider, category)
	if err != nil && !errors.Is(err, ErrEntryNotFound) {
		return err
	}

	if errors.Is(err, ErrEntryNotFound) {
		entry = Entry{
			Provider: provider,
			Category: category,
		}
	}

	entry.ConsecutiveFailures++
	entry.Timestamp = time.Now().UTC()
	entry.LastError = jobErr.Error()

	// Classify status
	msg := strings.ToLower(jobErr.Error())
	if strings.Contains(msg, "401") || strings.Contains(msg, "403") ||
		strings.Contains(msg, "authentication") || strings.Contains(msg, "authorization") ||
		strings.Contains(msg, "invalid credentials") || strings.Contains(msg, "config") {
		entry.Status = "blocked"
	} else {
		entry.Status = "failed"
	}

	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return d.client.Set(ctx, k, b, 0).Err()
}

func (d *DLQ) Clear(ctx context.Context, provider, category string) error {
	k := d.key(provider, category)
	return d.client.Del(ctx, k).Err()
}

func (d *DLQ) Get(ctx context.Context, provider, category string) (Entry, error) {
	k := d.key(provider, category)
	val, err := d.client.Get(ctx, k).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return Entry{}, ErrEntryNotFound
		}
		return Entry{}, err
	}

	var entry Entry
	if err := json.Unmarshal([]byte(val), &entry); err != nil {
		return Entry{}, err
	}

	return entry, nil
}
