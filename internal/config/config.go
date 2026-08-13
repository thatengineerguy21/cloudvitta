package config

import (
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
	URL string `koanf:"url" validate:"required"`
}

type StorageConfig struct {
	GCSBucketName string `koanf:"gcs_bucket_name" validate:"required"`
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

	mainConfig.Database.applyDefaults()

	validate := validator.New()
	err = validate.Struct(mainConfig)
	if err != nil {
		return nil, err
	}

	return mainConfig, nil
}
