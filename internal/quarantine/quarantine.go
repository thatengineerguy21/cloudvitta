package quarantine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/storage"
)

// UnmappedItem captures details about a single API SKU or attribute that could
// not be mapped to CloudVitta's curated taxonomy.
type UnmappedItem struct {
	Provider   string    `json:"provider"`  // "aws" | "azure" | "gcp"
	Category   string    `json:"category"`  // "compute" | "storage" | "network"
	Kind       string    `json:"kind"`      // "region" | "product" | "storage_class" | "transfer_type"
	RawValue   string    `json:"raw_value"` // the exact unmapped string from the API
	SkuID      string    `json:"sku_id"`
	ObservedAt time.Time `json:"observed_at"`
}

// Sink defines the contract for recording unmapped items during normalization.
type Sink interface {
	Record(ctx context.Context, item UnmappedItem) error
}

// MemorySink is a concurrency-safe in-memory sink for testing and inspection.
type MemorySink struct {
	mu    sync.RWMutex
	items []UnmappedItem
}

// NewMemorySink constructs an empty MemorySink.
func NewMemorySink() *MemorySink {
	return &MemorySink{
		items: make([]UnmappedItem, 0),
	}
}

// Record appends an unmapped item to the in-memory list.
func (m *MemorySink) Record(_ context.Context, item UnmappedItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = append(m.items, item)
	return nil
}

// Items returns a copy of all recorded items.
func (m *MemorySink) Items() []UnmappedItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	copied := make([]UnmappedItem, len(m.items))
	copy(copied, m.items)
	return copied
}

// Count returns the number of items recorded so far.
func (m *MemorySink) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}

// StorageSink writes quarantined unmapped items as append-only JSONL to raw storage.
type StorageSink struct {
	storage   storage.RawStorage
	provider  string
	category  string
	fetchID   string
	fetchedAt time.Time

	mu    sync.Mutex
	items []UnmappedItem
}

// NewStorageSink constructs a new StorageSink targeting GCS / raw storage at
// quarantine/<provider>/<category>/<date>/<fetchID>.jsonl
func NewStorageSink(st storage.RawStorage, provider, category, fetchID string, fetchedAt time.Time) *StorageSink {
	return &StorageSink{
		storage:   st,
		provider:  provider,
		category:  category,
		fetchID:   fetchID,
		fetchedAt: fetchedAt,
		items:     make([]UnmappedItem, 0),
	}
}

// Record buffers an unmapped item to be written to storage.
func (s *StorageSink) Record(_ context.Context, item UnmappedItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
	return nil
}

// Count returns the number of buffered items.
func (s *StorageSink) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.items)
}

// Items returns a snapshot of buffered items.
func (s *StorageSink) Items() []UnmappedItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]UnmappedItem, len(s.items))
	copy(copied, s.items)
	return copied
}

// Flush serializes all buffered unmapped items to JSONL and writes them to storage.
func (s *StorageSink) Flush(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.items) == 0 || s.storage == nil {
		return nil
	}

	dateStr := s.fetchedAt.Format("2006-01-02")
	path := fmt.Sprintf("quarantine/%s/%s/%s/%s.jsonl", s.provider, s.category, dateStr, s.fetchID)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, item := range s.items {
		if err := enc.Encode(item); err != nil {
			return fmt.Errorf("quarantine: encode item: %w", err)
		}
	}

	if err := s.storage.WriteStream(ctx, path, &buf); err != nil {
		return fmt.Errorf("quarantine: write to storage: %w", err)
	}

	return nil
}
