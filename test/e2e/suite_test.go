//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

const defaultBaseURL = "http://localhost:8080"

type E2ESuite struct {
	suite.Suite
	baseURL string
	client  *http.Client
}

func (s *E2ESuite) SetupSuite() {
	s.baseURL = os.Getenv("BASE_URL")
	if s.baseURL == "" {
		s.baseURL = defaultBaseURL
	}
	s.client = &http.Client{}
}

func TestE2E(t *testing.T) {
	suite.Run(t, new(E2ESuite))
}

func (s *E2ESuite) do(method, path string, body any) *http.Response {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		s.Require().NoError(err)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, s.baseURL+path, r)
	s.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	return resp
}

func (s *E2ESuite) decode(resp *http.Response, dst any) {
	defer resp.Body.Close()
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(dst))
}

func pathf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
