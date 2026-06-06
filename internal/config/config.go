// Package config loads application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration values loaded from environment variables.
type Config struct {
	AppPort string
	AppEnv  string

	DebugPort string

	DatabaseURL       string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxIdleTime time.Duration
	DBConnMaxLifetime time.Duration
	DBApplicationName string

	S3Endpoint     string
	S3Bucket       string
	S3AccessKey    string
	S3SecretKey    string
	S3Region       string
	S3UsePathStyle bool

	MaxUploadSizeMB int

	RateLimitRPS   float64
	RateLimitBurst int

	LogLevel    string
	LogFormat   string
	MetricsPort string

	OTELEndpoint       string
	OTELServiceName    string
	OTELServiceVersion string

	WorkerCancelInterval    time.Duration
	WorkerOrganicCutoffDays int

	HTTPReadHeaderTimeout time.Duration
	HTTPReadTimeout       time.Duration
	HTTPWriteTimeout      time.Duration
	HTTPIdleTimeout       time.Duration

	HTTPShutdownTimeout   time.Duration
	WorkerShutdownTimeout time.Duration
}

// Load reads all configuration from environment variables, falling back to defaults.
func Load() *Config {
	return &Config{
		AppPort: getEnv("APP_PORT", "8080"),
		AppEnv:  getEnv("APP_ENV", "development"),

		DebugPort: getEnv("DEBUG_PORT", "6060"),

		DatabaseURL:       getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/waste_collection?sslmode=disable"),
		DBMaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
		DBConnMaxIdleTime: getEnvDuration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute),
		DBConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
		DBApplicationName: getEnv("DB_APPLICATION_NAME", "waste-collection-api"),

		S3Endpoint:     getEnv("S3_ENDPOINT", "http://localhost:9000"),
		S3Bucket:       getEnv("S3_BUCKET", "waste-proofs"),
		S3AccessKey:    getEnv("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey:    getEnv("S3_SECRET_KEY", "minioadmin"),
		S3Region:       getEnv("S3_REGION", "us-east-1"),
		S3UsePathStyle: getEnvBool("S3_USE_PATH_STYLE", true),

		MaxUploadSizeMB: getEnvInt("MAX_UPLOAD_SIZE_MB", 10),

		RateLimitRPS:   getEnvFloat("RATE_LIMIT_RPS", 5),
		RateLimitBurst: getEnvInt("RATE_LIMIT_BURST", 10),

		LogLevel:    getEnv("LOG_LEVEL", "info"),
		LogFormat:   getEnv("LOG_FORMAT", "json"),
		MetricsPort: getEnv("METRICS_PORT", "2112"),

		OTELEndpoint:       getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		OTELServiceName:    getEnv("OTEL_SERVICE_NAME", "community-waste-collection-api"),
		OTELServiceVersion: getEnv("OTEL_SERVICE_VERSION", "0.1.0"),

		WorkerCancelInterval:    getEnvDuration("WORKER_CANCEL_INTERVAL", time.Hour),
		WorkerOrganicCutoffDays: getEnvInt("WORKER_ORGANIC_CUTOFF_DAYS", 3),

		HTTPReadHeaderTimeout: getEnvDuration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
		HTTPReadTimeout:       getEnvDuration("HTTP_READ_TIMEOUT", 15*time.Second),
		HTTPWriteTimeout:      getEnvDuration("HTTP_WRITE_TIMEOUT", 15*time.Second),
		HTTPIdleTimeout:       getEnvDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),

		HTTPShutdownTimeout:   getEnvDuration("HTTP_SHUTDOWN_TIMEOUT", 15*time.Second),
		WorkerShutdownTimeout: getEnvDuration("WORKER_SHUTDOWN_TIMEOUT", 30*time.Second),
	}
}

// Validate enforces invariants that, if violated, would silently break runtime
// behaviour (BR-04 not firing, rate limiter rejecting all traffic, etc).
// Callers should fail-fast on error rather than papering over a misconfig.
func (c *Config) Validate() error {
	if c.WorkerOrganicCutoffDays < 1 {
		return fmt.Errorf("WORKER_ORGANIC_CUTOFF_DAYS must be >= 1 (got %d) — BR-04 would never trigger", c.WorkerOrganicCutoffDays)
	}
	if c.WorkerCancelInterval < time.Second {
		return fmt.Errorf("WORKER_CANCEL_INTERVAL must be >= 1s (got %s) — tight loop would saturate the DB", c.WorkerCancelInterval)
	}
	if c.RateLimitRPS < 1 {
		return fmt.Errorf("RATE_LIMIT_RPS must be >= 1 (got %g) — every request would be rejected", c.RateLimitRPS)
	}
	if c.RateLimitBurst < 1 {
		return fmt.Errorf("RATE_LIMIT_BURST must be >= 1 (got %d)", c.RateLimitBurst)
	}
	if c.MaxUploadSizeMB < 1 {
		return fmt.Errorf("MAX_UPLOAD_SIZE_MB must be >= 1 (got %d)", c.MaxUploadSizeMB)
	}
	if port, err := strconv.Atoi(c.AppPort); err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("APP_PORT must be a valid TCP port (got %q)", c.AppPort)
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL must not be empty")
	}
	if c.DBMaxOpenConns < 1 {
		return fmt.Errorf("DB_MAX_OPEN_CONNS must be >= 1 (got %d)", c.DBMaxOpenConns)
	}
	if c.DBMaxIdleConns < 0 {
		return fmt.Errorf("DB_MAX_IDLE_CONNS must be >= 0 (got %d)", c.DBMaxIdleConns)
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
