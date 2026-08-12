package config_test

import (
	"os"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/config"
)

func TestLoad_Success(t *testing.T) {
	t.Setenv("CLOUDVITTA_SERVER_PORT", "8080")
	t.Setenv("CLOUDVITTA_PRIMARY_ENVIRONMENT", "test")
	t.Setenv("CLOUDVITTA_PRIMARY_LOG_LEVEL", "info")
	t.Setenv("CLOUDVITTA_DATABASE_HOST", "localhost")
	t.Setenv("CLOUDVITTA_DATABASE_PORT", "5432")
	t.Setenv("CLOUDVITTA_DATABASE_USER", "postgres")
	t.Setenv("CLOUDVITTA_DATABASE_NAME", "cloudvitta")
	t.Setenv("CLOUDVITTA_DATABASE_SSL_MODE", "disable")
	t.Setenv("CLOUDVITTA_REDIS_URL", "redis://test")
	t.Setenv("CLOUDVITTA_STORAGE_GCS_BUCKET_NAME", "bucket")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
}

func TestLoad_MissingVar(t *testing.T) {
	os.Clearenv()
	// Set everything except PORT
	t.Setenv("CLOUDVITTA_PRIMARY_ENVIRONMENT", "test")
	t.Setenv("CLOUDVITTA_PRIMARY_LOG_LEVEL", "info")
	t.Setenv("CLOUDVITTA_DATABASE_HOST", "localhost")
	t.Setenv("CLOUDVITTA_DATABASE_PORT", "5432")
	t.Setenv("CLOUDVITTA_DATABASE_USER", "postgres")
	t.Setenv("CLOUDVITTA_DATABASE_NAME", "cloudvitta")
	t.Setenv("CLOUDVITTA_DATABASE_SSL_MODE", "disable")
	t.Setenv("CLOUDVITTA_REDIS_URL", "redis://test")
	t.Setenv("CLOUDVITTA_STORAGE_GCS_BUCKET_NAME", "bucket")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing PORT, got nil")
	}
}

func TestLoad_MalformedVar(t *testing.T) {
	os.Clearenv()
	t.Setenv("CLOUDVITTA_SERVER_PORT", "not-a-number")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for malformed PORT, got nil")
	}
}
