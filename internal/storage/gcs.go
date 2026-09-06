package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
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
	mu           sync.RWMutex
	files        map[string][]byte
	tracer       trace.Tracer
	durationHist metric.Float64Histogram
}

// MemoryOption defines functional options for configuring MemoryRawStorage.
type MemoryOption func(*MemoryRawStorage)

// WithMemoryTracer configures a custom trace.Tracer for MemoryRawStorage.
func WithMemoryTracer(t trace.Tracer) MemoryOption {
	return func(m *MemoryRawStorage) {
		if t != nil {
			m.tracer = t
		}
	}
}

// WithMemoryMeter configures a custom metric.Meter for MemoryRawStorage.
func WithMemoryMeter(meter metric.Meter) MemoryOption {
	return func(m *MemoryRawStorage) {
		if meter != nil {
			hist, err := meter.Float64Histogram(
				"storage_operation_duration_seconds",
				metric.WithDescription("Duration of storage operations in seconds"),
				metric.WithUnit("s"),
			)
			if err == nil {
				m.durationHist = hist
			}
		}
	}
}

// NewMemoryRawStorage creates a new MemoryRawStorage instance.
func NewMemoryRawStorage(opts ...MemoryOption) *MemoryRawStorage {
	tracer := otel.Tracer("cloudvitta.storage.memory")
	meter := otel.Meter("cloudvitta.storage.memory")
	durationHist, _ := meter.Float64Histogram(
		"storage_operation_duration_seconds",
		metric.WithDescription("Duration of storage operations in seconds"),
		metric.WithUnit("s"),
	)

	m := &MemoryRawStorage{
		files:        make(map[string][]byte),
		tracer:       tracer,
		durationHist: durationHist,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// WriteStream reads all data from r and stores it in memory at path.
func (m *MemoryRawStorage) WriteStream(ctx context.Context, path string, r io.Reader) (err error) {
	start := time.Now()
	ctx, span := m.tracer.Start(ctx, "memory.write",
		trace.WithAttributes(attribute.String("storage.path", path)),
	)
	defer func() {
		dur := time.Since(start).Seconds()
		status := "ok"
		if err != nil {
			status = "error"
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		if m.durationHist != nil {
			m.durationHist.Record(ctx, dur, metric.WithAttributes(
				attribute.String("operation", "write"),
				attribute.String("backend", "memory"),
				attribute.String("status", status),
			))
		}
		span.End()
	}()

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
func (m *MemoryRawStorage) ReadStream(ctx context.Context, path string) (rc io.ReadCloser, err error) {
	start := time.Now()
	ctx, span := m.tracer.Start(ctx, "memory.read",
		trace.WithAttributes(attribute.String("storage.path", path)),
	)
	defer func() {
		dur := time.Since(start).Seconds()
		status := "ok"
		if err != nil {
			status = "error"
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		if m.durationHist != nil {
			m.durationHist.Record(ctx, dur, metric.WithAttributes(
				attribute.String("operation", "read"),
				attribute.String("backend", "memory"),
				attribute.String("status", status),
			))
		}
		span.End()
	}()

	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.files[path]
	if !ok {
		return nil, fmt.Errorf("storage: object not found: %s", path)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

// ListObjects returns all file paths matching the given prefix.
func (m *MemoryRawStorage) ListObjects(ctx context.Context, prefix string) (matches []string, err error) {
	start := time.Now()
	ctx, span := m.tracer.Start(ctx, "memory.list",
		trace.WithAttributes(attribute.String("storage.prefix", prefix)),
	)
	defer func() {
		dur := time.Since(start).Seconds()
		status := "ok"
		if err != nil {
			status = "error"
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		if m.durationHist != nil {
			m.durationHist.Record(ctx, dur, metric.WithAttributes(
				attribute.String("operation", "list"),
				attribute.String("backend", "memory"),
				attribute.String("status", status),
			))
		}
		span.End()
	}()

	m.mu.RLock()
	defer m.mu.RUnlock()
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

// GCSOption defines functional options for configuring GCSStorage.
type GCSOption func(*GCSStorage)

// WithGCSTracer configures a custom trace.Tracer for GCSStorage.
func WithGCSTracer(t trace.Tracer) GCSOption {
	return func(g *GCSStorage) {
		if t != nil {
			g.tracer = t
		}
	}
}

// WithGCSMeter configures a custom metric.Meter for GCSStorage.
func WithGCSMeter(m metric.Meter) GCSOption {
	return func(g *GCSStorage) {
		if m != nil {
			hist, err := m.Float64Histogram(
				"storage_operation_duration_seconds",
				metric.WithDescription("Duration of storage operations in seconds"),
				metric.WithUnit("s"),
			)
			if err == nil {
				g.durationHist = hist
			}
		}
	}
}

// GCSStorage is a Google Cloud Storage implementation of RawStorage.
type GCSStorage struct {
	client       *storage.Client
	bucketName   string
	tracer       trace.Tracer
	durationHist metric.Float64Histogram
}

// NewGCSStorage constructs a new GCSStorage instance with OpenTelemetry instrumentation.
func NewGCSStorage(client *storage.Client, bucketName string, opts ...GCSOption) *GCSStorage {
	tracer := otel.Tracer("cloudvitta.storage.gcs")
	meter := otel.Meter("cloudvitta.storage.gcs")
	durationHist, _ := meter.Float64Histogram(
		"storage_operation_duration_seconds",
		metric.WithDescription("Duration of storage operations in seconds"),
		metric.WithUnit("s"),
	)

	g := &GCSStorage{
		client:       client,
		bucketName:   bucketName,
		tracer:       tracer,
		durationHist: durationHist,
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// WriteStream streams data from r into the specified GCS object path within bucketName.
func (g *GCSStorage) WriteStream(ctx context.Context, path string, r io.Reader) (err error) {
	start := time.Now()
	ctx, span := g.tracer.Start(ctx, "gcs.write",
		trace.WithAttributes(
			attribute.String("gcs.bucket", g.bucketName),
			attribute.String("gcs.path", path),
		),
	)
	defer func() {
		dur := time.Since(start).Seconds()
		status := "ok"
		if err != nil {
			status = "error"
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			slog.ErrorContext(ctx, "gcs write failed",
				"bucket", g.bucketName,
				"path", path,
				"error", err,
			)
		} else {
			slog.DebugContext(ctx, "gcs write succeeded",
				"bucket", g.bucketName,
				"path", path,
			)
		}
		if g.durationHist != nil {
			g.durationHist.Record(ctx, dur, metric.WithAttributes(
				attribute.String("operation", "write"),
				attribute.String("backend", "gcs"),
				attribute.String("status", status),
			))
		}
		span.End()
	}()

	if g.bucketName == "" {
		return fmt.Errorf("gcs storage: bucket name is empty")
	}
	if g.client == nil {
		return fmt.Errorf("gcs storage: client is nil")
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
func (g *GCSStorage) ReadStream(ctx context.Context, path string) (rc io.ReadCloser, err error) {
	start := time.Now()
	ctx, span := g.tracer.Start(ctx, "gcs.read",
		trace.WithAttributes(
			attribute.String("gcs.bucket", g.bucketName),
			attribute.String("gcs.path", path),
		),
	)
	defer func() {
		dur := time.Since(start).Seconds()
		status := "ok"
		if err != nil {
			status = "error"
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			slog.ErrorContext(ctx, "gcs read failed",
				"bucket", g.bucketName,
				"path", path,
				"error", err,
			)
		}
		if g.durationHist != nil {
			g.durationHist.Record(ctx, dur, metric.WithAttributes(
				attribute.String("operation", "read"),
				attribute.String("backend", "gcs"),
				attribute.String("status", status),
			))
		}
		span.End()
	}()

	if g.bucketName == "" {
		return nil, fmt.Errorf("gcs storage: bucket name is empty")
	}
	if g.client == nil {
		return nil, fmt.Errorf("gcs storage: client is nil")
	}
	reader, err := g.client.Bucket(g.bucketName).Object(path).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcs storage: read object %s failed: %w", path, err)
	}
	return reader, nil
}

// ListObjects lists all object paths in the bucket matching the given prefix.
func (g *GCSStorage) ListObjects(ctx context.Context, prefix string) (paths []string, err error) {
	start := time.Now()
	ctx, span := g.tracer.Start(ctx, "gcs.list",
		trace.WithAttributes(
			attribute.String("gcs.bucket", g.bucketName),
			attribute.String("gcs.prefix", prefix),
		),
	)
	defer func() {
		dur := time.Since(start).Seconds()
		status := "ok"
		if err != nil {
			status = "error"
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			slog.ErrorContext(ctx, "gcs list failed",
				"bucket", g.bucketName,
				"prefix", prefix,
				"error", err,
			)
		}
		if g.durationHist != nil {
			g.durationHist.Record(ctx, dur, metric.WithAttributes(
				attribute.String("operation", "list"),
				attribute.String("backend", "gcs"),
				attribute.String("status", status),
			))
		}
		span.End()
	}()

	if g.bucketName == "" {
		return nil, fmt.Errorf("gcs storage: bucket name is empty")
	}
	if g.client == nil {
		return nil, fmt.Errorf("gcs storage: client is nil")
	}
	it := g.client.Bucket(g.bucketName).Objects(ctx, &storage.Query{Prefix: prefix})
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
