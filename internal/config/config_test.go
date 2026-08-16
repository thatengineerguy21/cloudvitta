package config_test

import (
	"os"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/config"
)

const testJWTSecret = "super-secret-jwt-key-with-at-least-32-bytes-length!"

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
	t.Setenv("CLOUDVITTA_AUTH_JWT_SECRET", testJWTSecret)

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
	if cfg.Auth.JWTSecret != testJWTSecret {
		t.Errorf("expected JWTSecret %s, got %s", testJWTSecret, cfg.Auth.JWTSecret)
	}
}

func TestLoad_FromDatabaseURLAndDefaults(t *testing.T) {
	os.Clearenv()
	t.Setenv("DATABASE_URL", "postgres://testuser:testpass@ep-test.neon.tech:5432/neondb?sslmode=require")
	t.Setenv("REDIS_URL", "redis://default:secret@redis.upstash.io:6379")
	t.Setenv("GCS_BUCKET_NAME", "cloudvitta-raw-fixtures")
	t.Setenv("PORT", "9090")
	t.Setenv("JWT_SECRET", testJWTSecret)

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
	if cfg.Storage.GCSBucketName != "cloudvitta-raw-fixtures" {
		t.Errorf("expected gcs bucket name, got %s", cfg.Storage.GCSBucketName)
	}
	if cfg.Auth.JWTSecret != testJWTSecret {
		t.Errorf("expected auth jwt secret, got %s", cfg.Auth.JWTSecret)
	}
}

func TestLoad_MissingDatabase(t *testing.T) {
	os.Clearenv()
	t.Setenv("CLOUDVITTA_PRIMARY_ENVIRONMENT", "test")
	t.Setenv("CLOUDVITTA_PRIMARY_LOG_LEVEL", "info")
	t.Setenv("CLOUDVITTA_SERVER_PORT", "8080")
	t.Setenv("CLOUDVITTA_REDIS_URL", "redis://localhost:6379")
	t.Setenv("CLOUDVITTA_STORAGE_GCS_BUCKET_NAME", "test-bucket")
	t.Setenv("CLOUDVITTA_AUTH_JWT_SECRET", testJWTSecret)

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing database configuration, got nil")
	}
}

func TestLoad_MissingRedisURL(t *testing.T) {
	os.Clearenv()
	t.Setenv("CLOUDVITTA_PRIMARY_ENVIRONMENT", "test")
	t.Setenv("CLOUDVITTA_PRIMARY_LOG_LEVEL", "info")
	t.Setenv("CLOUDVITTA_SERVER_PORT", "8080")
	t.Setenv("CLOUDVITTA_DATABASE_HOST", "localhost")
	t.Setenv("CLOUDVITTA_DATABASE_PORT", "5432")
	t.Setenv("CLOUDVITTA_DATABASE_USER", "postgres")
	t.Setenv("CLOUDVITTA_DATABASE_NAME", "cloudvitta")
	t.Setenv("CLOUDVITTA_DATABASE_SSL_MODE", "disable")
	t.Setenv("CLOUDVITTA_STORAGE_GCS_BUCKET_NAME", "test-bucket")
	t.Setenv("CLOUDVITTA_AUTH_JWT_SECRET", testJWTSecret)

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing Redis URL, got nil")
	}
}

func TestLoad_MissingStorageGCSBucketName(t *testing.T) {
	os.Clearenv()
	t.Setenv("CLOUDVITTA_PRIMARY_ENVIRONMENT", "test")
	t.Setenv("CLOUDVITTA_PRIMARY_LOG_LEVEL", "info")
	t.Setenv("CLOUDVITTA_SERVER_PORT", "8080")
	t.Setenv("CLOUDVITTA_DATABASE_HOST", "localhost")
	t.Setenv("CLOUDVITTA_DATABASE_PORT", "5432")
	t.Setenv("CLOUDVITTA_DATABASE_USER", "postgres")
	t.Setenv("CLOUDVITTA_DATABASE_NAME", "cloudvitta")
	t.Setenv("CLOUDVITTA_DATABASE_SSL_MODE", "disable")
	t.Setenv("CLOUDVITTA_REDIS_URL", "redis://localhost:6379")
	t.Setenv("CLOUDVITTA_AUTH_JWT_SECRET", testJWTSecret)

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing Storage GCS Bucket Name, got nil")
	}
}

func TestAuthConfig_JWTSecretValidation(t *testing.T) {
	os.Clearenv()
	t.Setenv("CLOUDVITTA_PRIMARY_ENVIRONMENT", "test")
	t.Setenv("CLOUDVITTA_PRIMARY_LOG_LEVEL", "info")
	t.Setenv("CLOUDVITTA_SERVER_PORT", "8080")
	t.Setenv("CLOUDVITTA_DATABASE_HOST", "localhost")
	t.Setenv("CLOUDVITTA_DATABASE_PORT", "5432")
	t.Setenv("CLOUDVITTA_DATABASE_USER", "postgres")
	t.Setenv("CLOUDVITTA_DATABASE_NAME", "cloudvitta")
	t.Setenv("CLOUDVITTA_DATABASE_SSL_MODE", "disable")
	t.Setenv("CLOUDVITTA_REDIS_URL", "redis://localhost:6379")
	t.Setenv("CLOUDVITTA_STORAGE_GCS_BUCKET_NAME", "test-bucket")

	// Missing JWT secret
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing JWT secret, got nil")
	}

	// Short JWT secret (<32 bytes)
	t.Setenv("CLOUDVITTA_AUTH_JWT_SECRET", "too-short-secret")
	_, err = config.Load()
	if err == nil {
		t.Fatal("expected error for short JWT secret (<32 bytes), got nil")
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

func TestLoad_RateLimitAndCORSDefaults(t *testing.T) {
	os.Clearenv()
	t.Setenv("DATABASE_URL", "postgres://testuser:testpass@localhost:5432/neondb?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("GCS_BUCKET_NAME", "test-bucket")
	t.Setenv("JWT_SECRET", testJWTSecret)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error loading config, got %v", err)
	}

	// RateLimit defaults
	if cfg.RateLimit.StandardTierRate != 120 {
		t.Errorf("expected default StandardTierRate 120, got %d", cfg.RateLimit.StandardTierRate)
	}
	if cfg.RateLimit.FreeTierRate != 20 {
		t.Errorf("expected default FreeTierRate 20, got %d", cfg.RateLimit.FreeTierRate)
	}
	if cfg.RateLimit.IPCeilingRate != 60 {
		t.Errorf("expected default IPCeilingRate 60, got %d", cfg.RateLimit.IPCeilingRate)
	}
	if cfg.RateLimit.LoginRate != 10 {
		t.Errorf("expected default LoginRate 10, got %d", cfg.RateLimit.LoginRate)
	}

	// Auth AnonCookieSecret fallback to JWTSecret
	if cfg.Auth.AnonCookieSecret != testJWTSecret {
		t.Errorf("expected default AnonCookieSecret fallback %s, got %s", testJWTSecret, cfg.Auth.AnonCookieSecret)
	}

	// CORS defaults with credentials require explicit origins
	if len(cfg.CORS.AllowedOrigins) != 3 || cfg.CORS.AllowedOrigins[0] != "https://cloudvitta.dev" {
		t.Errorf("expected default AllowedOrigins [https://cloudvitta.dev, ...], got %v", cfg.CORS.AllowedOrigins)
	}
	if !cfg.CORS.AllowCredentials {
		t.Errorf("expected default AllowCredentials true, got %v", cfg.CORS.AllowCredentials)
	}
}

func TestLoad_RateLimitAndCORSOverrides(t *testing.T) {
	os.Clearenv()
	t.Setenv("DATABASE_URL", "postgres://testuser:testpass@localhost:5432/neondb?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("GCS_BUCKET_NAME", "test-bucket")
	t.Setenv("JWT_SECRET", testJWTSecret)
	t.Setenv("CLOUDVITTA_AUTH_ANON_COOKIE_SECRET", "custom-anon-cookie-secret-32-chars-long!")
	t.Setenv("CLOUDVITTA_RATELIMIT_STANDARD_TIER_RATE", "300")
	t.Setenv("CLOUDVITTA_RATELIMIT_FREE_TIER_RATE", "50")
	t.Setenv("CLOUDVITTA_RATELIMIT_IP_CEILING_RATE", "100")
	t.Setenv("CLOUDVITTA_RATELIMIT_LOGIN_RATE", "5")
	t.Setenv("CLOUDVITTA_CORS_ALLOWED_ORIGINS", "https://cloudvitta.dev,https://app.cloudvitta.dev")
	t.Setenv("CLOUDVITTA_CORS_ALLOW_CREDENTIALS", "false")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error loading config, got %v", err)
	}

	if cfg.RateLimit.StandardTierRate != 300 {
		t.Errorf("expected StandardTierRate 300, got %d", cfg.RateLimit.StandardTierRate)
	}
	if cfg.RateLimit.FreeTierRate != 50 {
		t.Errorf("expected FreeTierRate 50, got %d", cfg.RateLimit.FreeTierRate)
	}
	if cfg.RateLimit.IPCeilingRate != 100 {
		t.Errorf("expected IPCeilingRate 100, got %d", cfg.RateLimit.IPCeilingRate)
	}
	if cfg.RateLimit.LoginRate != 5 {
		t.Errorf("expected LoginRate 5, got %d", cfg.RateLimit.LoginRate)
	}
	if cfg.Auth.AnonCookieSecret != "custom-anon-cookie-secret-32-chars-long!" {
		t.Errorf("expected custom AnonCookieSecret, got %s", cfg.Auth.AnonCookieSecret)
	}
	if len(cfg.CORS.AllowedOrigins) != 2 || cfg.CORS.AllowedOrigins[0] != "https://cloudvitta.dev" || cfg.CORS.AllowedOrigins[1] != "https://app.cloudvitta.dev" {
		t.Errorf("expected custom AllowedOrigins, got %v", cfg.CORS.AllowedOrigins)
	}
	if cfg.CORS.AllowCredentials {
		t.Errorf("expected AllowCredentials false, got %v", cfg.CORS.AllowCredentials)
	}
}

func TestLoad_CORS_WildcardWithCredentials_Validation(t *testing.T) {
	os.Clearenv()
	t.Setenv("DATABASE_URL", "postgres://testuser:testpass@localhost:5432/neondb?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("GCS_BUCKET_NAME", "test-bucket")
	t.Setenv("JWT_SECRET", testJWTSecret)
	t.Setenv("CLOUDVITTA_CORS_ALLOWED_ORIGINS", "*")
	t.Setenv("CLOUDVITTA_CORS_ALLOW_CREDENTIALS", "true")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when AllowCredentials=true with AllowedOrigins=['*'], got nil")
	}

	// Wildcard without credentials should succeed
	t.Setenv("CLOUDVITTA_CORS_ALLOW_CREDENTIALS", "false")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error when AllowCredentials=false with AllowedOrigins=['*'], got %v", err)
	}
	if len(cfg.CORS.AllowedOrigins) != 1 || cfg.CORS.AllowedOrigins[0] != "*" {
		t.Errorf("expected AllowedOrigins ['*'], got %v", cfg.CORS.AllowedOrigins)
	}
}
