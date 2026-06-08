package observability_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
)

// U3: InitTracer with an unreachable endpoint still returns a tracer and a
// non-nil shutdown function. The exporter error only surfaces at Shutdown time.
func TestInitTracer_UnreachableEndpoint(t *testing.T) {
	cfg := &config.Config{
		OTELEndpoint:       "http://127.0.0.1:9999", // nothing listening here
		OTELServiceName:    "test-service",
		OTELServiceVersion: "0.0.1",
	}
	tracer, shutdown, err := observability.InitTracer(context.Background(), cfg)
	require.NoError(t, err, "InitTracer must succeed even when the endpoint is unreachable")
	assert.NotNil(t, tracer)
	require.NotNil(t, shutdown, "shutdown function must not be nil")

	// Shutdown should complete without panicking (it may return an error if
	// the exporter times out, but that is acceptable in a unit test).
	ctx, cancel := context.WithTimeout(context.Background(), 3000*time.Millisecond)
	defer cancel()
	_ = shutdown(ctx)
}

// TestInitTracer_Shutdown_NilCtx exercises the shutdown path with a cancelled
// context to ensure no panic.
func TestInitTracer_Shutdown_CancelledCtx(t *testing.T) {
	cfg := &config.Config{
		OTELEndpoint:       "http://127.0.0.1:9999",
		OTELServiceName:    "test-service",
		OTELServiceVersion: "0.0.1",
	}
	_, shutdown, err := observability.InitTracer(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()          // already cancelled
	_ = shutdown(ctx) // must not panic
}
