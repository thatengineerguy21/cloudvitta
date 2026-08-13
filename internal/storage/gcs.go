package storage

import (
	"context"
	"fmt"
	"io"
	"sync"

	"cloud.google.com/go/storage"
)

// RawStorage defines the interface for persisting raw provider responses to storage.
type RawStorage interface {
	WriteStream(ctx context.Context, path string, r io.Reader) error
}

// MemoryRawStorage is an in-memory implementation of RawStorage for testing and local dev.
type MemoryRawStorage struct {
	mu    sync.RWMutex
	files map[string][]byte
}

// NewMemoryRawStorage creates a new MemoryRawStorage instance.
func NewMemoryRawStorage() *MemoryRawStorage {
	return &MemoryRawStorage{
		files: make(map[string][]byte),
	}
}

// WriteStream reads all data from r and stores it in memory at path.
func (m *MemoryRawStorage) WriteStream(ctx context.Context, path string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("storage: read stream failed: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[path] = data
	return nil
}

// Get retrieves the stored bytes for a path, returning false if not found.
func (m *MemoryRawStorage) Get(path string) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.files[path]
	return data, ok
}

// GetFiles returns a snapshot map of all stored file paths to their byte content.
func (m *MemoryRawStorage) GetFiles() map[string][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	copyMap := make(map[string][]byte, len(m.files))
	for k, v := range m.files {
		copyMap[k] = v
	}
	return copyMap
}

// GCSStorage is a Google Cloud Storage implementation of RawStorage.
type GCSStorage struct {
	client     *storage.Client
	bucketName string
}

// NewGCSStorage constructs a new GCSStorage instance.
func NewGCSStorage(client *storage.Client, bucketName string) *GCSStorage {
	return &GCSStorage{
		client:     client,
		bucketName: bucketName,
	}
}

// WriteStream streams data from r into the specified GCS object path within bucketName.
func (g *GCSStorage) WriteStream(ctx context.Context, path string, r io.Reader) error {
	if g.bucketName == "" {
		return fmt.Errorf("gcs storage: bucket name is empty")
	}
	wc := g.client.Bucket(g.bucketName).Object(path).NewWriter(ctx)
	if _, err := io.Copy(wc, r); err != nil {
		_ = wc.Close()
		return fmt.Errorf("gcs storage: copy to object %s failed: %w", path, err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("gcs storage: close writer for object %s failed: %w", path, err)
	}
	return nil
}
