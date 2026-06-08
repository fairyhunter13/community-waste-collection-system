//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"
	_ "github.com/lib/pq"
)

const defaultBaseURL = "http://localhost:8080"

type E2ESuite struct {
	suite.Suite
	baseURL string
	client  *http.Client
	db      *sqlx.DB
}

func (s *E2ESuite) SetupSuite() {
	s.baseURL = os.Getenv("BASE_URL")
	if s.baseURL == "" {
		s.baseURL = defaultBaseURL
	}
	s.client = &http.Client{}
	if url := os.Getenv("E2E_DB_URL"); url != "" {
		s.db, _ = sqlx.Connect("postgres", url)
	}
}

func (s *E2ESuite) TearDownSuite() {
	if s.db != nil {
		s.db.Close()
	}
}

func (s *E2ESuite) execDB(query string, args ...any) {
	if s.db == nil {
		s.T().Skip("E2E_DB_URL not set")
	}
	_, err := s.db.ExecContext(context.Background(), query, args...)
	s.Require().NoError(err)
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

// jpegProofBody builds a multipart body carrying a single "proof" part whose
// part-level Content-Type header is image/jpeg. The content starts with the
// JPEG SOI magic bytes (0xFF 0xD8 0xFF) so that the handler's magic-byte sniff
// accepts it; mime/multipart.CreateFormFile would emit application/octet-stream,
// which the handler's allowlist rejects.
func jpegProofBody() (body *bytes.Buffer, contentType string, err error) {
	body = &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", `form-data; name="proof"; filename="receipt.jpg"`)
	hdr.Set("Content-Type", "image/jpeg")
	part, err := mw.CreatePart(hdr)
	if err != nil {
		return nil, "", err
	}
	// Minimal JPEG header: SOI (0xFF 0xD8) + APP0 marker (0xFF 0xE0).
	// http.DetectContentType requires \xFF\xD8\xFF prefix to classify as image/jpeg.
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	if _, err = part.Write(jpegHeader); err != nil {
		return nil, "", err
	}
	if err = mw.Close(); err != nil {
		return nil, "", err
	}
	return body, mw.FormDataContentType(), nil
}

// confirmPayment confirms a payment by uploading a fake proof file.
func (s *E2ESuite) confirmPayment(paymentID string) {
	body, contentType, err := jpegProofBody()
	s.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPut, s.baseURL+pathf("/api/payments/%s/confirm", paymentID), body)
	s.Require().NoError(err)
	req.Header.Set("Content-Type", contentType)
	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	resp.Body.Close()
	s.Require().Equal(http.StatusOK, resp.StatusCode)
}
