package config

import (
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	cfg := Load()

	if cfg.AppPort != "8080" {
		t.Errorf("AppPort default: got %q, want %q", cfg.AppPort, "8080")
	}
	if cfg.AppEnv != "development" {
		t.Errorf("AppEnv default: got %q, want %q", cfg.AppEnv, "development")
	}
	if cfg.DBMaxOpenConns != 25 {
		t.Errorf("DBMaxOpenConns default: got %d, want 25", cfg.DBMaxOpenConns)
	}
	if cfg.DBMaxIdleConns != 10 {
		t.Errorf("DBMaxIdleConns default: got %d, want 10", cfg.DBMaxIdleConns)
	}
	if cfg.DBConnMaxIdleTime != 5*time.Minute {
		t.Errorf("DBConnMaxIdleTime default: got %v, want 5m", cfg.DBConnMaxIdleTime)
	}
	if cfg.S3Bucket != "waste-proofs" {
		t.Errorf("S3Bucket default: got %q, want %q", cfg.S3Bucket, "waste-proofs")
	}
	if !cfg.S3UsePathStyle {
		t.Error("S3UsePathStyle default: got false, want true")
	}
	if cfg.RateLimitRPS != 5 {
		t.Errorf("RateLimitRPS default: got %v, want 5", cfg.RateLimitRPS)
	}
	if cfg.RateLimitBurst != 10 {
		t.Errorf("RateLimitBurst default: got %d, want 10", cfg.RateLimitBurst)
	}
	if cfg.WorkerOrganicCutoffDays != 3 {
		t.Errorf("WorkerOrganicCutoffDays default: got %d, want 3", cfg.WorkerOrganicCutoffDays)
	}
	if cfg.WorkerCancelInterval != time.Hour {
		t.Errorf("WorkerCancelInterval default: got %v, want 1h", cfg.WorkerCancelInterval)
	}
	if cfg.MaxUploadSizeMB != 10 {
		t.Errorf("MaxUploadSizeMB default: got %d, want 10", cfg.MaxUploadSizeMB)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("APP_PORT", "9090")
	t.Setenv("APP_ENV", "production")
	t.Setenv("DB_MAX_OPEN_CONNS", "50")
	t.Setenv("DB_MAX_IDLE_CONNS", "20")
	t.Setenv("DB_CONN_MAX_IDLE_TIME", "10m")
	t.Setenv("S3_BUCKET", "custom-bucket")
	t.Setenv("S3_USE_PATH_STYLE", "false")
	t.Setenv("RATE_LIMIT_RPS", "100")
	t.Setenv("RATE_LIMIT_BURST", "200")
	t.Setenv("WORKER_ORGANIC_CUTOFF_DAYS", "7")
	t.Setenv("WORKER_CANCEL_INTERVAL", "2h")
	t.Setenv("MAX_UPLOAD_SIZE_MB", "20")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "text")

	cfg := Load()

	if cfg.AppPort != "9090" {
		t.Errorf("AppPort: got %q, want %q", cfg.AppPort, "9090")
	}
	if cfg.AppEnv != "production" {
		t.Errorf("AppEnv: got %q, want %q", cfg.AppEnv, "production")
	}
	if cfg.DBMaxOpenConns != 50 {
		t.Errorf("DBMaxOpenConns: got %d, want 50", cfg.DBMaxOpenConns)
	}
	if cfg.DBMaxIdleConns != 20 {
		t.Errorf("DBMaxIdleConns: got %d, want 20", cfg.DBMaxIdleConns)
	}
	if cfg.DBConnMaxIdleTime != 10*time.Minute {
		t.Errorf("DBConnMaxIdleTime: got %v, want 10m", cfg.DBConnMaxIdleTime)
	}
	if cfg.S3Bucket != "custom-bucket" {
		t.Errorf("S3Bucket: got %q, want %q", cfg.S3Bucket, "custom-bucket")
	}
	if cfg.S3UsePathStyle {
		t.Error("S3UsePathStyle: got true, want false")
	}
	if cfg.RateLimitRPS != 100 {
		t.Errorf("RateLimitRPS: got %v, want 100", cfg.RateLimitRPS)
	}
	if cfg.RateLimitBurst != 200 {
		t.Errorf("RateLimitBurst: got %d, want 200", cfg.RateLimitBurst)
	}
	if cfg.WorkerOrganicCutoffDays != 7 {
		t.Errorf("WorkerOrganicCutoffDays: got %d, want 7", cfg.WorkerOrganicCutoffDays)
	}
	if cfg.WorkerCancelInterval != 2*time.Hour {
		t.Errorf("WorkerCancelInterval: got %v, want 2h", cfg.WorkerCancelInterval)
	}
	if cfg.MaxUploadSizeMB != 20 {
		t.Errorf("MaxUploadSizeMB: got %d, want 20", cfg.MaxUploadSizeMB)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel: got %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.LogFormat != "text" {
		t.Errorf("LogFormat: got %q, want %q", cfg.LogFormat, "text")
	}
}

func TestLoad_InvalidEnvFallsToDefault(t *testing.T) {
	t.Setenv("DB_MAX_OPEN_CONNS", "not-a-number")
	t.Setenv("RATE_LIMIT_RPS", "bad-float")
	t.Setenv("S3_USE_PATH_STYLE", "not-bool")
	t.Setenv("DB_CONN_MAX_IDLE_TIME", "not-duration")

	cfg := Load()

	if cfg.DBMaxOpenConns != 25 {
		t.Errorf("DBMaxOpenConns invalid env: got %d, want default 25", cfg.DBMaxOpenConns)
	}
	if cfg.RateLimitRPS != 5 {
		t.Errorf("RateLimitRPS invalid env: got %v, want default 5", cfg.RateLimitRPS)
	}
	if !cfg.S3UsePathStyle {
		t.Error("S3UsePathStyle invalid env: got false, want default true")
	}
	if cfg.DBConnMaxIdleTime != 5*time.Minute {
		t.Errorf("DBConnMaxIdleTime invalid env: got %v, want default 5m", cfg.DBConnMaxIdleTime)
	}
}
