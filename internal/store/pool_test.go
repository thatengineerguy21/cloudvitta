package store_test

import (
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/config"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

func TestNewPoolConfig(t *testing.T) {
	cfg := config.DatabaseConfig{
		Host:            "localhost",
		Port:            5432,
		User:            "cloudvitta",
		Password:        "secret",
		Name:            "cloudvitta_test",
		SSLMode:         "disable",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 300,
		ConnMaxIdleTime: 60,
	}

	poolCfg, err := store.NewPoolConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if poolCfg.MaxConns != 25 {
		t.Errorf("expected MaxConns=25, got %d", poolCfg.MaxConns)
	}
	if poolCfg.MinConns != 5 {
		t.Errorf("expected MinConns=5, got %d", poolCfg.MinConns)
	}
	if poolCfg.MaxConnLifetime != 300*time.Second {
		t.Errorf("expected MaxConnLifetime=300s, got %v", poolCfg.MaxConnLifetime)
	}
	if poolCfg.MaxConnIdleTime != 60*time.Second {
		t.Errorf("expected MaxConnIdleTime=60s, got %v", poolCfg.MaxConnIdleTime)
	}

	// Verify otelpgx tracer is attached to ConnConfig
	if poolCfg.ConnConfig.Tracer == nil {
		t.Fatal("expected ConnConfig.Tracer to be non-nil (otelpgx tracer attached)")
	}
}

func TestNewPoolConfig_WithoutTracer(t *testing.T) {
	cfg := config.DatabaseConfig{
		Host: "localhost",
		Port: 5432,
		User: "cloudvitta",
		Name: "cloudvitta_test",
	}

	poolCfg, err := store.NewPoolConfig(cfg, store.WithoutTracer())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if poolCfg.ConnConfig.Tracer != nil {
		t.Fatal("expected ConnConfig.Tracer to be nil when WithoutTracer() option is provided")
	}
}
