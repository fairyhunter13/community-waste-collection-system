// Package config loads application configuration from environment variables.
package config

import (
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
	}
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
