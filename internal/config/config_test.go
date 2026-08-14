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
	if cfg.Primary.Environment != "test" {
		t.Errorf("expected environment 'test', got %s", cfg.Primary.Environment)
	}
}

func TestLoad_FromDatabaseURLAndDefaults(t *testing.T) {
	os.Clearenv()
	t.Setenv("DATABASE_URL", "postgres://testuser:testpass@ep-test.neon.tech:5432/neondb?sslmode=require")
	t.Setenv("REDIS_URL", "redis://default:secret@redis.upstash.io:6379")
	t.Setenv("PORT", "9090")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error loading from DATABASE_URL and PORT, got %v", err)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Primary.Environment != "production" {
		t.Errorf("expected default environment 'production', got %s", cfg.Primary.Environment)
	}
	if cfg.Primary.LogLevel != "info" {
		t.Errorf("expected default log level 'info', got %s", cfg.Primary.LogLevel)
	}
	if cfg.Database.Host != "ep-test.neon.tech" {
		t.Errorf("expected db host 'ep-test.neon.tech', got %s", cfg.Database.Host)
	}
	if cfg.Database.User != "testuser" {
		t.Errorf("expected db user 'testuser', got %s", cfg.Database.User)
	}
	if cfg.Database.Password != "testpass" {
		t.Errorf("expected db password 'testpass', got %s", cfg.Database.Password)
	}
	if cfg.Database.Name != "neondb" {
		t.Errorf("expected db name 'neondb', got %s", cfg.Database.Name)
	}
	if cfg.Database.SSLMode != "require" {
		t.Errorf("expected db ssl mode 'require', got %s", cfg.Database.SSLMode)
	}
	if cfg.Redis.URL != "redis://default:secret@redis.upstash.io:6379" {
		t.Errorf("expected redis url, got %s", cfg.Redis.URL)
	}
}

func TestLoad_MissingDatabase(t *testing.T) {
	os.Clearenv()
	// No database host or DATABASE_URL provided
	t.Setenv("CLOUDVITTA_PRIMARY_ENVIRONMENT", "test")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing database configuration, got nil")
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
