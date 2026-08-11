package config_test

import (
	"os"
	"testing"
	"github.com/thatengineerguy21/CloudVitta/internal/config"
)

func TestLoad_Success(t *testing.T) {
	os.Setenv("CLOUDVITTA_SERVER_PORT", "8080")
	os.Setenv("CLOUDVITTA_PRIMARY_ENVIRONMENT", "test")
	os.Setenv("CLOUDVITTA_PRIMARY_LOG_LEVEL", "info")
	os.Setenv("CLOUDVITTA_DATABASE_URL", "postgres://test")
	os.Setenv("CLOUDVITTA_REDIS_URL", "redis://test")
	os.Setenv("CLOUDVITTA_STORAGE_GCS_BUCKET_NAME", "bucket")
	defer os.Clearenv()

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
	os.Setenv("CLOUDVITTA_PRIMARY_ENVIRONMENT", "test")
	os.Setenv("CLOUDVITTA_PRIMARY_LOG_LEVEL", "info")
	os.Setenv("CLOUDVITTA_DATABASE_URL", "postgres://test")
	os.Setenv("CLOUDVITTA_REDIS_URL", "redis://test")
	os.Setenv("CLOUDVITTA_STORAGE_GCS_BUCKET_NAME", "bucket")
	
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing PORT, got nil")
	}
}

func TestLoad_MalformedVar(t *testing.T) {
	os.Clearenv()
	os.Setenv("CLOUDVITTA_SERVER_PORT", "not-a-number")
	
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for malformed PORT, got nil")
	}
}
