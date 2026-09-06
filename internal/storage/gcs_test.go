package storage_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/storage"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestMemoryRawStorage_Tracing(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := tp.Tracer("test-memory-storage")
	st := storage.NewMemoryRawStorage(storage.WithMemoryTracer(tracer))

	ctx := context.Background()

	// 1. WriteStream
	payload := []byte("hello storage observability")
	err := st.WriteStream(ctx, "test/path.json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("WriteStream failed: %v", err)
	}

	// 2. ReadStream
	rc, err := st.ReadStream(ctx, "test/path.json")
	if err != nil {
		t.Fatalf("ReadStream failed: %v", err)
	}
	readBytes, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("io.ReadAll failed: %v", err)
	}
	if string(readBytes) != string(payload) {
		t.Errorf("expected %s, got %s", payload, readBytes)
	}

	// 3. ListObjects
	objs, err := st.ListObjects(ctx, "test/")
	if err != nil {
		t.Fatalf("ListObjects failed: %v", err)
	}
	if len(objs) != 1 || objs[0] != "test/path.json" {
		t.Errorf("expected ['test/path.json'], got %v", objs)
	}

	spans := exporter.GetSpans()
	spanNames := make(map[string]bool)
	for _, s := range spans {
		spanNames[s.Name] = true
	}

	if !spanNames["memory.write"] {
		t.Errorf("expected memory.write span, got: %v", spans)
	}
	if !spanNames["memory.read"] {
		t.Errorf("expected memory.read span, got: %v", spans)
	}
	if !spanNames["memory.list"] {
		t.Errorf("expected memory.list span, got: %v", spans)
	}
}

func TestGCSStorage_EmptyBucketValidation(t *testing.T) {
	st := storage.NewGCSStorage(nil, "")
	ctx := context.Background()

	err := st.WriteStream(ctx, "path", strings.NewReader("data"))
	if err == nil {
		t.Error("expected error for empty bucket on WriteStream")
	}

	_, err = st.ReadStream(ctx, "path")
	if err == nil {
		t.Error("expected error for empty bucket on ReadStream")
	}

	_, err = st.ListObjects(ctx, "prefix")
	if err == nil {
		t.Error("expected error for empty bucket on ListObjects")
	}
}

func TestGCSStorage_WriteStream_SpanExported(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := tp.Tracer("test-gcs-storage")
	st := storage.NewGCSStorage(nil, "test-bucket", storage.WithGCSTracer(tracer))

	ctx := context.Background()
	_ = st.WriteStream(ctx, "test/object.json", strings.NewReader("sample payload"))

	spans := exporter.GetSpans()
	found := false
	for _, s := range spans {
		if s.Name == "gcs.write" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected gcs.write span to be exported")
	}
}
