package config

import (
	"strings"

	"github.com/go-playground/validator/v10"
	_ "github.com/joho/godotenv/autoload"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	Primary  PrimaryConfig  `koanf:"primary" validate:"required"`
	Server   ServerConfig   `koanf:"server" validate:"required"`
	Database DatabaseConfig `koanf:"database" validate:"required"`
	Redis    RedisConfig    `koanf:"redis" validate:"required"`
	Storage  StorageConfig  `koanf:"storage" validate:"required"`
}

type PrimaryConfig struct {
	Environment string `koanf:"environment" validate:"required"`
	LogLevel    string `koanf:"log_level" validate:"required"`
}

type ServerConfig struct {
	Port int `koanf:"port" validate:"required"`
}

type DatabaseConfig struct {
	URL string `koanf:"url" validate:"required"`
}

type RedisConfig struct {
	URL string `koanf:"url" validate:"required"`
}

type StorageConfig struct {
	GCSBucketName string `koanf:"gcs_bucket_name" validate:"required"`
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

	validate := validator.New()
	err = validate.Struct(mainConfig)
	if err != nil {
		return nil, err
	}

	return mainConfig, nil
}
