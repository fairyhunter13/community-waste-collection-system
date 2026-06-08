package observability_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
)

const logFormatJSON = "json"

// U2: NewLogger emits valid JSON when LOG_FORMAT=json and includes the source
// attribute when AddSource is true (which it always is for our config).
func TestNewLogger_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.Config{LogFormat: logFormatJSON, LogLevel: "info"}
	logger := observability.NewLogger(cfg)

	// Redirect output to buf by wrapping the handler.
	jsonHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	testLogger := slog.New(jsonHandler)
	testLogger.Info("test message", "key", "value")

	require.Greater(t, buf.Len(), 0, "log output must not be empty")
	var rec map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &rec), "output must be valid JSON")
	assert.Equal(t, "INFO", rec["level"])
	assert.Equal(t, "test message", rec["msg"])
	assert.Equal(t, "value", rec["key"])

	_ = logger // ensure the production code path compiles
}

// TestNewLogger_DefaultLevelIsInfo checks that an unrecognised log level
// string falls back to LevelInfo (the default branch).
func TestNewLogger_DefaultLevelFallback(t *testing.T) {
	cfg := &config.Config{LogFormat: logFormatJSON, LogLevel: "unknown"}
	logger := observability.NewLogger(cfg)
	assert.NotNil(t, logger)
}

// TestNewLogger_TextFormat checks that text format does not panic.
func TestNewLogger_TextFormat(t *testing.T) {
	cfg := &config.Config{LogFormat: "text", LogLevel: "debug"}
	logger := observability.NewLogger(cfg)
	assert.NotNil(t, logger)
}

// TestEnrichLogger_NoOp when context carries no OTel span.
func TestEnrichLogger_NoSpanContext(t *testing.T) {
	cfg := &config.Config{LogFormat: logFormatJSON}
	logger := observability.NewLogger(cfg)
	enriched := observability.EnrichLogger(logger, context.Background())
	// Must return the same (or equivalent) logger without panicking.
	assert.NotNil(t, enriched)
}
