package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	_ "github.com/joho/godotenv/autoload"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	Primary       PrimaryConfig       `koanf:"primary" validate:"required"`
	Server        ServerConfig        `koanf:"server" validate:"required"`
	Database      DatabaseConfig      `koanf:"database" validate:"required"`
	Redis         RedisConfig         `koanf:"redis" validate:"required"`
	Storage       StorageConfig       `koanf:"storage" validate:"required"`
	Auth          AuthConfig          `koanf:"auth" validate:"required"`
	Observability ObservabilityConfig `koanf:"observability"`
	GCP           GCPConfig           `koanf:"gcp"`
}

type PrimaryConfig struct {
	Environment string `koanf:"environment" validate:"required"`
	LogLevel    string `koanf:"log_level" validate:"required"`
}

type ServerConfig struct {
	Port int `koanf:"port" validate:"required"`
}

type DatabaseConfig struct {
	// Host specifies the database host address.
	Host string `koanf:"host" validate:"required"`
	// Port specifies the database network port.
	Port int `koanf:"port" validate:"required"`
	// User specifies the database user name.
	User string `koanf:"user" validate:"required"`
	// Password specifies the database password.
	Password string `koanf:"password"`
	// Name specifies the database name.
	Name string `koanf:"name" validate:"required"`
	// SSLMode specifies the SSL connection mode.
	SSLMode string `koanf:"ssl_mode" validate:"required"`
	// MaxOpenConns specifies maximum open database connections.
	MaxOpenConns int `koanf:"max_open_conns"`
	// MaxIdleConns specifies maximum idle database connections.
	MaxIdleConns int `koanf:"max_idle_conns"`
	// ConnMaxLifetime specifies maximum connection lifetime in seconds.
	ConnMaxLifetime int `koanf:"conn_max_lifetime"`
	// ConnMaxIdleTime specifies maximum idle time in seconds.
	ConnMaxIdleTime int `koanf:"conn_max_idle_time"`
}

// applyDefaults sets conservative pool and connection defaults when no override is provided.
// Cloud Run scales horizontally, so each instance keeps a small pool.
func (d *DatabaseConfig) applyDefaults() {
	if d.Port == 0 {
		d.Port = 5432
	}
	if d.SSLMode == "" {
		d.SSLMode = "require"
	}
	if d.MaxOpenConns == 0 {
		d.MaxOpenConns = 5
	}
	if d.MaxIdleConns == 0 {
		d.MaxIdleConns = 2
	}
	if d.ConnMaxLifetime == 0 {
		d.ConnMaxLifetime = 1800 // 30 minutes
	}
	if d.ConnMaxIdleTime == 0 {
		d.ConnMaxIdleTime = 300 // 5 minutes
	}
}

type RedisConfig struct {
	URL string `koanf:"url" validate:"required"`
}

type StorageConfig struct {
	GCSBucketName string `koanf:"gcs_bucket_name" validate:"required"`
}

type AuthConfig struct {
	JWTSecret string `koanf:"jwt_secret" validate:"required,min=32"`
}

type ObservabilityConfig struct {
	OTLPEndpoint string `koanf:"otlp_endpoint"`
	OTLPHeaders  string `koanf:"otlp_headers"`
}

type GCPConfig struct {
	APIKey string `koanf:"api_key"`
}

func Load() (*Config, error) {
	k := koanf.New(".")

	// Prefix environment variables with the application name to avoid conflicts.
	err := k.Load(env.Provider("CLOUDVITTA_", ".", func(s string) string {
		s = strings.TrimPrefix(s, "CLOUDVITTA_")
		parts := strings.SplitN(s, "_", 2)
		if len(parts) == 2 {
			return strings.ToLower(parts[0] + "." + parts[1])
		}
		return strings.ToLower(s)
	}), nil)
	if err != nil {
		return nil, err
	}

	mainConfig := &Config{}
	err = k.Unmarshal("", mainConfig)
	if err != nil {
		return nil, err
	}

	if err := resolveEnvFallbacks(mainConfig); err != nil {
		return nil, err
	}

	mainConfig.Database.applyDefaults()

	validate := validator.New()
	err = validate.Struct(mainConfig)
	if err != nil {
		return nil, err
	}

	return mainConfig, nil
}

// resolveEnvFallbacks populates configuration fields with standard environment variables
// (e.g. PORT on Cloud Run, DATABASE_URL on Neon, REDIS_URL on Upstash).
func resolveEnvFallbacks(cfg *Config) error {
	// 1. Resolve Primary defaults / env fallbacks
	if cfg.Primary.Environment == "" {
		if envVal := os.Getenv("ENVIRONMENT"); envVal != "" {
			cfg.Primary.Environment = envVal
		} else {
			cfg.Primary.Environment = "production"
		}
	}
	if cfg.Primary.LogLevel == "" {
		if logVal := os.Getenv("LOG_LEVEL"); logVal != "" {
			cfg.Primary.LogLevel = logVal
		} else {
			cfg.Primary.LogLevel = "info"
		}
	}

	// 2. Resolve Server Port (Cloud Run injects PORT)
	if cfg.Server.Port == 0 {
		if portStr := os.Getenv("PORT"); portStr != "" {
			if portVal, pErr := strconv.Atoi(portStr); pErr == nil && portVal > 0 {
				cfg.Server.Port = portVal
			}
		}
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}

	// 3. Resolve Database URL overrides
	if err := resolveDatabaseURLOverrides(&cfg.Database); err != nil {
		return err
	}

	// 4. Resolve Redis URL fallback
	if cfg.Redis.URL == "" {
		cfg.Redis.URL = os.Getenv("REDIS_URL")
	}

	// 5. Resolve Storage GCS Bucket Name fallback
	if cfg.Storage.GCSBucketName == "" {
		cfg.Storage.GCSBucketName = os.Getenv("GCS_BUCKET_NAME")
	}

	// 6. Resolve Observability fallbacks
	if cfg.Observability.OTLPEndpoint == "" {
		cfg.Observability.OTLPEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if cfg.Observability.OTLPHeaders == "" {
		cfg.Observability.OTLPHeaders = os.Getenv("OTEL_EXPORTER_OTLP_HEADERS")
	}

	// 7. Resolve GCP fallbacks
	if cfg.GCP.APIKey == "" {
		if val := os.Getenv("CLOUDVITTA_GCP_API_KEY"); val != "" {
			cfg.GCP.APIKey = val
		} else {
			cfg.GCP.APIKey = os.Getenv("GCP_API_KEY")
		}
	}

	// 8. Resolve Auth JWT Secret fallback
	if cfg.Auth.JWTSecret == "" {
		if val := os.Getenv("CLOUDVITTA_AUTH_JWT_SECRET"); val != "" {
			cfg.Auth.JWTSecret = val
		} else if val := os.Getenv("CLOUDVITTA_JWT_SECRET"); val != "" {
			cfg.Auth.JWTSecret = val
		} else {
			cfg.Auth.JWTSecret = os.Getenv("JWT_SECRET")
		}
	}

	return nil
}

// resolveDatabaseURLOverrides parses DATABASE_URL or CLOUDVITTA_DATABASE_URL into target DatabaseConfig.
func resolveDatabaseURLOverrides(target *DatabaseConfig) error {
	dbURL := os.Getenv("CLOUDVITTA_DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL != "" && (target.Host == "" || target.Name == "") {
		if err := parseDatabaseURL(dbURL, target); err != nil {
			return fmt.Errorf("config: invalid database URL: %w", err)
		}
	}
	return nil
}

func parseDatabaseURL(rawURL string, target *DatabaseConfig) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	if u.Hostname() != "" {
		target.Host = u.Hostname()
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err == nil {
			target.Port = port
		}
	}

	if u.User != nil {
		target.User = u.User.Username()
		if pass, ok := u.User.Password(); ok {
			target.Password = pass
		}
	}

	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName != "" {
		target.Name = dbName
	}

	sslMode := u.Query().Get("sslmode")
	if sslMode != "" {
		target.SSLMode = sslMode
	}

	return nil
}
