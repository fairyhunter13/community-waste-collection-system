// Package repository provides sqlx-backed implementations of domain repository interfaces.
package repository

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
)

// Connect opens a PostgreSQL connection pool using the provided configuration.
func Connect(cfg *config.Config) (*sqlx.DB, error) {
	dsn := injectApplicationName(cfg.DatabaseURL, cfg.DBApplicationName)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxIdleTime(cfg.DBConnMaxIdleTime)
	db.SetConnMaxLifetime(cfg.DBConnMaxLifetime)

	StartDBPoolStatsCollector(db)

	return db, nil
}

// MustConnect opens a PostgreSQL connection from a URL or panics. Use only in tests or program init.
func MustConnect(url string) *sqlx.DB {
	db, err := sqlx.Connect("postgres", url)
	if err != nil {
		panic(fmt.Sprintf("db connect: %v", err))
	}
	return db
}

// injectApplicationName appends application_name to the DSN if the user did
// not set one. Supports both URL (postgres://) and keyword DSN forms. On any
// parse failure the original DSN is returned unmodified — observability tag
// loss is acceptable, a startup failure here would not be.
func injectApplicationName(dsn, appName string) string {
	if appName == "" {
		return dsn
	}
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return dsn
		}
		q := u.Query()
		if q.Get("application_name") != "" {
			return dsn
		}
		q.Set("application_name", appName)
		u.RawQuery = q.Encode()
		return u.String()
	}
	// keyword DSN
	if strings.Contains(dsn, "application_name=") {
		return dsn
	}
	if dsn == "" {
		return "application_name=" + appName
	}
	return dsn + " application_name=" + appName
}

// dbStatsTickInterval is shadowed in tests; in production it runs the
// real time.Ticker.
var dbStatsTickInterval = 15 * time.Second

// StartDBPoolStatsCollector starts a background goroutine that scrapes
// db.Stats() and republishes it as Prometheus gauges. Safe to call once per
// DB instance; subsequent calls would double-count.
func StartDBPoolStatsCollector(db *sqlx.DB) {
	go func() {
		ticker := time.NewTicker(dbStatsTickInterval)
		defer ticker.Stop()
		ctx := context.Background()
		_ = ctx // reserved for future ctx-aware metrics
		for range ticker.C {
			s := db.Stats()
			observability.DBPoolOpenConnections.Set(float64(s.OpenConnections))
			observability.DBPoolInUse.Set(float64(s.InUse))
			observability.DBPoolIdle.Set(float64(s.Idle))
		}
	}()
}
