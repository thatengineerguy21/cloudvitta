package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

// RawStorage defines the interface for persisting and reading raw provider responses and quarantine records.
type RawStorage interface {
	WriteStream(ctx context.Context, path string, r io.Reader) error
	ReadStream(ctx context.Context, path string) (io.ReadCloser, error)
	ListObjects(ctx context.Context, prefix string) ([]string, error)
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

// ReadStream returns an io.ReadCloser for the data stored at path.
func (m *MemoryRawStorage) ReadStream(ctx context.Context, path string) (io.ReadCloser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.files[path]
	if !ok {
		return nil, fmt.Errorf("storage: object not found: %s", path)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

// ListObjects returns all file paths matching the given prefix.
func (m *MemoryRawStorage) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var matches []string
	for k := range m.files {
		if strings.HasPrefix(k, prefix) {
			matches = append(matches, k)
		}
	}
	return matches, nil
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

// ReadStream opens a streaming reader for the GCS object at path.
func (g *GCSStorage) ReadStream(ctx context.Context, path string) (io.ReadCloser, error) {
	if g.bucketName == "" {
		return nil, fmt.Errorf("gcs storage: bucket name is empty")
	}
	rc, err := g.client.Bucket(g.bucketName).Object(path).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcs storage: read object %s failed: %w", path, err)
	}
	return rc, nil
}

// ListObjects lists all object paths in the bucket matching the given prefix.
func (g *GCSStorage) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	if g.bucketName == "" {
		return nil, fmt.Errorf("gcs storage: bucket name is empty")
	}
	it := g.client.Bucket(g.bucketName).Objects(ctx, &storage.Query{Prefix: prefix})
	var paths []string
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("gcs storage: list objects with prefix %s failed: %w", prefix, err)
		}
		paths = append(paths, attrs.Name)
	}
	return paths, nil
}
