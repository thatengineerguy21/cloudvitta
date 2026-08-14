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
	Redis         RedisConfig         `koanf:"redis"`
	Storage       StorageConfig       `koanf:"storage"`
	Observability ObservabilityConfig `koanf:"observability"`
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

// applyDefaults sets conservative pool defaults when no override is provided.
// Cloud Run scales horizontally, so each instance keeps a small pool.
func (d *DatabaseConfig) applyDefaults() {
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
	URL string `koanf:"url"`
}

type StorageConfig struct {
	GCSBucketName string `koanf:"gcs_bucket_name"`
}

type ObservabilityConfig struct {
	OTLPEndpoint string `koanf:"otlp_endpoint"`
	OTLPHeaders  string `koanf:"otlp_headers"`
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

	// 1. Resolve Primary defaults / env fallbacks
	if mainConfig.Primary.Environment == "" {
		if envVal := os.Getenv("ENVIRONMENT"); envVal != "" {
			mainConfig.Primary.Environment = envVal
		} else {
			mainConfig.Primary.Environment = "production"
		}
	}
	if mainConfig.Primary.LogLevel == "" {
		if logVal := os.Getenv("LOG_LEVEL"); logVal != "" {
			mainConfig.Primary.LogLevel = logVal
		} else {
			mainConfig.Primary.LogLevel = "info"
		}
	}

	// 2. Resolve Server Port (Cloud Run injects PORT)
	if mainConfig.Server.Port == 0 {
		if portStr := os.Getenv("PORT"); portStr != "" {
			if portVal, pErr := strconv.Atoi(portStr); pErr == nil && portVal > 0 {
				mainConfig.Server.Port = portVal
			}
		}
	}
	if mainConfig.Server.Port == 0 {
		mainConfig.Server.Port = 8080
	}

	// 3. Resolve Database URL fallback (e.g. DATABASE_URL or CLOUDVITTA_DATABASE_URL)
	dbURL := os.Getenv("CLOUDVITTA_DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL != "" && (mainConfig.Database.Host == "" || mainConfig.Database.Name == "") {
		if err := parseDatabaseURL(dbURL, &mainConfig.Database); err != nil {
			return nil, fmt.Errorf("config: invalid database URL: %w", err)
		}
	}

	// 4. Resolve Redis URL fallback
	if mainConfig.Redis.URL == "" {
		mainConfig.Redis.URL = os.Getenv("REDIS_URL")
	}

	// 5. Resolve Storage GCS Bucket Name fallback
	if mainConfig.Storage.GCSBucketName == "" {
		mainConfig.Storage.GCSBucketName = os.Getenv("GCS_BUCKET_NAME")
	}

	// 6. Resolve Observability fallbacks
	if mainConfig.Observability.OTLPEndpoint == "" {
		mainConfig.Observability.OTLPEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if mainConfig.Observability.OTLPHeaders == "" {
		mainConfig.Observability.OTLPHeaders = os.Getenv("OTEL_EXPORTER_OTLP_HEADERS")
	}

	mainConfig.Database.applyDefaults()

	validate := validator.New()
	err = validate.Struct(mainConfig)
	if err != nil {
		return nil, err
	}

	return mainConfig, nil
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
	} else if target.Port == 0 {
		target.Port = 5432
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
	} else if target.SSLMode == "" {
		target.SSLMode = "require"
	}

	return nil
}
