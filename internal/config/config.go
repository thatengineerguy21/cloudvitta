package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port        string
	Env         string
	DatabaseURL string
	RedisURL    string
	JWTSecret   string
}

func Load() (*Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	env := os.Getenv("ENV")
	if env == "" {
		env = "development"
	}

	dbURL := os.Getenv("DATABASE_URL")
	redisURL := os.Getenv("REDIS_URL")
	jwtSecret := os.Getenv("JWT_SECRET")

	cfg := &Config{
		Port:        port,
		Env:         env,
		DatabaseURL: dbURL,
		RedisURL:    redisURL,
		JWTSecret:   jwtSecret,
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Env == "production" {
		if c.DatabaseURL == "" {
			return fmt.Errorf("DATABASE_URL is required in production")
		}
		if c.RedisURL == "" {
			return fmt.Errorf("REDIS_URL is required in production")
		}
		if c.JWTSecret == "" {
			return fmt.Errorf("JWT_SECRET is required in production")
		}
	}
	return nil
}
