//go:build e2e

// E3: TestGracefulShutdown_DrainsInFlightRequest verifies that the API server
// completes in-flight requests before shutting down on SIGTERM.
//
// The test requires a docker-compose stack where the app container is named
// "community-waste-collection-system-api-1" or the APP_CONTAINER env var is
// set. It also requires the E2E_DB_URL env var so it can seed a household
// used for the in-flight request.
//
// If the container name / docker socket is unavailable the test is skipped
// gracefully so local `make test-e2e` runs against a bare binary still work.
package e2e_test

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stretchr/testify/require"
)

// TestGracefulShutdown_DrainsInFlightRequest sends SIGTERM to the app
// container while a long-running request is in flight and asserts the request
// returns a valid HTTP response (not a connection reset).
func (s *E2ESuite) TestGracefulShutdown_DrainsInFlightRequest() {
	containerName := os.Getenv("APP_CONTAINER")
	if containerName == "" {
		containerName = "community-waste-collection-system-api-1"
	}

	// Confirm docker is reachable and the container exists.
	out, err := exec.Command("docker", "inspect", "--format", "{{.State.Running}}", containerName).Output()
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		s.T().Skipf("app container %q not running via docker; skipping graceful-shutdown test", containerName)
	}

	// Use the reports endpoint — it is read-only and therefore safe to hit
	// during a shutdown test even if the response is interrupted.
	url := s.baseURL + "/api/reports/payment-summary"

	var responseCode int64
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return
		}
		resp, err := s.client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		atomic.StoreInt64(&responseCode, int64(resp.StatusCode))
	}()

	// Give the goroutine a moment to send the request, then send SIGTERM.
	time.Sleep(50 * time.Millisecond)
	sigtermCmd := exec.Command("docker", "kill", "--signal", "SIGTERM", containerName)
	require.NoError(s.T(), sigtermCmd.Run(), "docker kill SIGTERM must succeed")

	// Wait for the request goroutine and the container to stop.
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		s.Fail("in-flight request did not complete within 20s after SIGTERM")
	}

	// The response code should be valid (2xx or 5xx) — not 0 (connection reset).
	code := atomic.LoadInt64(&responseCode)
	s.NotZero(code, "in-flight request must have received a complete HTTP response before shutdown")
}
