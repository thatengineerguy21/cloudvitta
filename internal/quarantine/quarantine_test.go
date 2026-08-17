package quarantine_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

type mockStorage struct {
	mu    sync.Mutex
	files map[string][]byte
}

func newMockStorage() *mockStorage {
	return &mockStorage{files: make(map[string][]byte)}
}

func (m *mockStorage) WriteStream(_ context.Context, key string, r io.Reader) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.files[key] = data
	return nil
}

func (m *mockStorage) ReadStream(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[key]
	if !ok {
		return nil, io.EOF
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *mockStorage) ListObjects(_ context.Context, prefix string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []string
	for k := range m.files {
		if len(prefix) == 0 || (len(k) >= len(prefix) && k[:len(prefix)] == prefix) {
			res = append(res, k)
		}
	}
	return res, nil
}

func (m *mockStorage) Exists(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.files[key]
	return ok, nil
}

func TestMemorySink(t *testing.T) {
	ctx := context.Background()
	sink := quarantine.NewMemorySink()

	item := quarantine.UnmappedItem{
		Provider:   "aws",
		Category:   "compute",
		Kind:       "region",
		RawValue:   "Mars (Olympus Mons)",
		SkuID:      "SKU-MARS-001",
		ObservedAt: time.Now().UTC(),
	}

	if err := sink.Record(ctx, item); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sink.Count() != 1 {
		t.Errorf("expected count 1, got %d", sink.Count())
	}

	items := sink.Items()
	if len(items) != 1 || items[0].RawValue != "Mars (Olympus Mons)" {
		t.Errorf("unexpected items: %+v", items)
	}
}

func TestStorageSink(t *testing.T) {
	ctx := context.Background()
	st := newMockStorage()
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	sink := quarantine.NewStorageSink(st, "azure", "storage", "fetch-123", now)

	item1 := quarantine.UnmappedItem{
		Provider:   "azure",
		Category:   "storage",
		Kind:       "storage_class",
		RawValue:   "Quantum Storage Tier",
		SkuID:      "SKU-AZ-001",
		ObservedAt: now,
	}
	item2 := quarantine.UnmappedItem{
		Provider:   "azure",
		Category:   "storage",
		Kind:       "region",
		RawValue:   "Jupiter East",
		SkuID:      "SKU-AZ-002",
		ObservedAt: now,
	}

	_ = sink.Record(ctx, item1)
	_ = sink.Record(ctx, item2)

	if err := sink.Flush(ctx); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	expectedPath := "quarantine/azure/storage/2026-08-17/fetch-123.jsonl"
	data, ok := st.files[expectedPath]
	if !ok {
		t.Fatalf("file %s was not written to storage", expectedPath)
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	var decoded []quarantine.UnmappedItem
	for dec.More() {
		var it quarantine.UnmappedItem
		if err := dec.Decode(&it); err != nil {
			t.Fatalf("decode jsonl error: %v", err)
		}
		decoded = append(decoded, it)
	}

	if len(decoded) != 2 {
		t.Errorf("expected 2 decoded items, got %d", len(decoded))
	}
	if decoded[0].RawValue != "Quantum Storage Tier" || decoded[1].RawValue != "Jupiter East" {
		t.Errorf("unexpected decoded items: %+v", decoded)
	}
}
