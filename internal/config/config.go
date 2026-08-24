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
	RateLimit     RateLimitConfig     `koanf:"ratelimit"`
	CORS          CORSConfig          `koanf:"cors"`
	Observability ObservabilityConfig `koanf:"observability"`
	GCP           GCPConfig           `koanf:"gcp"`
	IBM           IBMConfig           `koanf:"ibm"`
	Alibaba       AlibabaConfig       `koanf:"alibaba"`
	Freshness     FreshnessConfig     `koanf:"freshness"`
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
	JWTSecret        string `koanf:"jwt_secret" validate:"required,min=32"`
	AnonCookieSecret string `koanf:"anon_cookie_secret"`
}

type RateLimitConfig struct {
	StandardTierRate int64 `koanf:"standard_tier_rate"` // default: 120 req/min
	FreeTierRate     int64 `koanf:"free_tier_rate"`     // default: 20 req/min
	IPCeilingRate    int64 `koanf:"ip_ceiling_rate"`    // default: 60 req/min
	LoginRate        int64 `koanf:"login_rate"`         // default: 10 req/min
}

type CORSConfig struct {
	AllowedOrigins   []string `koanf:"allowed_origins"`
	AllowCredentials bool     `koanf:"allow_credentials"`
}

type ObservabilityConfig struct {
	OTLPEndpoint string `koanf:"otlp_endpoint"`
	OTLPHeaders  string `koanf:"otlp_headers"`
}

type GCPConfig struct {
	APIKey string `koanf:"api_key"`
}

type IBMConfig struct {
	APIKey string `koanf:"api_key"`
}

type AlibabaConfig struct {
	AccessKeyID     string `koanf:"access_key_id"`
	AccessKeySecret string `koanf:"access_key_secret"`
}

type FreshnessConfig struct {
	StalenessThresholdHours int64 `koanf:"staleness_threshold_hours"`
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

	if mainConfig.CORS.AllowCredentials {
		for _, origin := range mainConfig.CORS.AllowedOrigins {
			if origin == "*" {
				return nil, fmt.Errorf("config: CORS AllowCredentials cannot be true when AllowedOrigins contains wildcard '*'")
			}
		}
	}

	return mainConfig, nil
}

func resolveEnvInt64(primaryEnv, secondaryEnv string, fallback int64) int64 {
	if val := os.Getenv(primaryEnv); val != "" {
		if parsed, err := strconv.ParseInt(val, 10, 64); err == nil && parsed > 0 {
			return parsed
		}
	}
	if secondaryEnv != "" {
		if val := os.Getenv(secondaryEnv); val != "" {
			if parsed, err := strconv.ParseInt(val, 10, 64); err == nil && parsed > 0 {
				return parsed
			}
		}
	}
	return fallback
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

	// 8. Resolve IBM fallbacks
	if cfg.IBM.APIKey == "" {
		if val := os.Getenv("CLOUDVITTA_IBM_API_KEY"); val != "" {
			cfg.IBM.APIKey = val
		} else {
			cfg.IBM.APIKey = os.Getenv("IBM_API_KEY")
		}
	}

	// 9. Resolve Alibaba fallbacks
	if cfg.Alibaba.AccessKeyID == "" {
		if val := os.Getenv("CLOUDVITTA_ALIBABA_ACCESS_KEY_ID"); val != "" {
			cfg.Alibaba.AccessKeyID = val
		} else {
			cfg.Alibaba.AccessKeyID = os.Getenv("ALIBABA_ACCESS_KEY_ID")
		}
	}
	if cfg.Alibaba.AccessKeySecret == "" {
		if val := os.Getenv("CLOUDVITTA_ALIBABA_ACCESS_KEY_SECRET"); val != "" {
			cfg.Alibaba.AccessKeySecret = val
		} else {
			cfg.Alibaba.AccessKeySecret = os.Getenv("ALIBABA_ACCESS_KEY_SECRET")
		}
	}

	// 10. Resolve Auth JWT Secret fallback
	if cfg.Auth.JWTSecret == "" {
		if val := os.Getenv("CLOUDVITTA_AUTH_JWT_SECRET"); val != "" {
			cfg.Auth.JWTSecret = val
		} else if val := os.Getenv("CLOUDVITTA_JWT_SECRET"); val != "" {
			cfg.Auth.JWTSecret = val
		} else {
			cfg.Auth.JWTSecret = os.Getenv("JWT_SECRET")
		}
	}

	// 9. Resolve Auth Anon Cookie Secret fallback
	if cfg.Auth.AnonCookieSecret == "" {
		if val := os.Getenv("CLOUDVITTA_AUTH_ANON_COOKIE_SECRET"); val != "" {
			cfg.Auth.AnonCookieSecret = val
		} else if val := os.Getenv("ANON_COOKIE_SECRET"); val != "" {
			cfg.Auth.AnonCookieSecret = val
		} else {
			cfg.Auth.AnonCookieSecret = cfg.Auth.JWTSecret
		}
	}

	// 10. Resolve RateLimit fallbacks & defaults
	if cfg.RateLimit.StandardTierRate == 0 {
		cfg.RateLimit.StandardTierRate = resolveEnvInt64("CLOUDVITTA_RATELIMIT_STANDARD_TIER_RATE", "RATELIMIT_STANDARD_TIER_RATE", 120)
	}
	if cfg.RateLimit.FreeTierRate == 0 {
		cfg.RateLimit.FreeTierRate = resolveEnvInt64("CLOUDVITTA_RATELIMIT_FREE_TIER_RATE", "RATELIMIT_FREE_TIER_RATE", 20)
	}
	if cfg.RateLimit.IPCeilingRate == 0 {
		cfg.RateLimit.IPCeilingRate = resolveEnvInt64("CLOUDVITTA_RATELIMIT_IP_CEILING_RATE", "RATELIMIT_IP_CEILING_RATE", 60)
	}
	if cfg.RateLimit.LoginRate == 0 {
		cfg.RateLimit.LoginRate = resolveEnvInt64("CLOUDVITTA_RATELIMIT_LOGIN_RATE", "RATELIMIT_LOGIN_RATE", 10)
	}

	// 11. Resolve CORS fallbacks & defaults
	corsCredsVal := os.Getenv("CLOUDVITTA_CORS_ALLOW_CREDENTIALS")
	if corsCredsVal == "" {
		corsCredsVal = os.Getenv("CORS_ALLOW_CREDENTIALS")
	}
	if corsCredsVal != "" {
		if parsed, err := strconv.ParseBool(corsCredsVal); err == nil {
			cfg.CORS.AllowCredentials = parsed
		}
	} else if len(cfg.CORS.AllowedOrigins) == 0 {
		cfg.CORS.AllowCredentials = true
	}

	corsOriginsVal := os.Getenv("CLOUDVITTA_CORS_ALLOWED_ORIGINS")
	if corsOriginsVal == "" {
		corsOriginsVal = os.Getenv("CORS_ALLOWED_ORIGINS")
	}

	if corsOriginsVal != "" {
		parts := strings.Split(corsOriginsVal, ",")
		var origins []string
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				origins = append(origins, trimmed)
			}
		}
		if len(origins) > 0 {
			cfg.CORS.AllowedOrigins = origins
		}
	} else if len(cfg.CORS.AllowedOrigins) == 1 && strings.Contains(cfg.CORS.AllowedOrigins[0], ",") {
		parts := strings.Split(cfg.CORS.AllowedOrigins[0], ",")
		var origins []string
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				origins = append(origins, trimmed)
			}
		}
		if len(origins) > 0 {
			cfg.CORS.AllowedOrigins = origins
		}
	}

	if len(cfg.CORS.AllowedOrigins) == 0 {
		if cfg.CORS.AllowCredentials {
			cfg.CORS.AllowedOrigins = []string{
				"https://cloudvitta.dev",
				"http://localhost:3000",
				"http://localhost:5173",
			}
		} else {
			cfg.CORS.AllowedOrigins = []string{"*"}
		}
	}

	// 12. Resolve Freshness fallbacks & defaults
	if cfg.Freshness.StalenessThresholdHours == 0 {
		cfg.Freshness.StalenessThresholdHours = resolveEnvInt64("CLOUDVITTA_FRESHNESS_STALENESS_THRESHOLD_HOURS", "FRESHNESS_STALENESS_THRESHOLD_HOURS", 168)
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
