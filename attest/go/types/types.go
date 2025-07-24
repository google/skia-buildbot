package types

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// ErrBadImageFormat is the error returned by Client.Verify when the format
	// of the given image is not correct.
	ErrBadImageFormat = "expected image of the form gcr.io/project/repository@sha256:digest"
)

// IsErrBadImageFormat returns true if the given error is ErrBadImageFormat.
func IsErrBadImageFormat(err error) bool {
	return err != nil && skerr.Unwrap(err).Error() == ErrBadImageFormat
}

var validAttestorRegex = regexp.MustCompile(`^projects\/[0-9A-Za-z_-]+\/attestors\/[0-9A-Za-z_-]+$`)

// ValidateAttestor returns an error if the given attestor does not appear to be
// a fully-qualified resource name.
func ValidateAttestor(attestor string) error {
	if !validAttestorRegex.MatchString(attestor) {
		return skerr.Fmt("expected a fully-qualified resource name for attestor")
	}
	return nil
}

var validImageRegex = regexp.MustCompile(`^[0-9A-Za-z_.]+\/[0-9A-Za-z_-]+\/[0-9A-Za-z_-]+@sha256:[0-9a-f]{64}$`)

// ValidateImageID returns ErrBadImageFormat if the given image ID does not have
// the expected format.
func ValidateImageID(imageID string) error {
	if !validImageRegex.MatchString(imageID) {
		return skerr.Fmt(ErrBadImageFormat)
	}
	return nil
}

// Client performs validation of Docker images.
type Client interface {
	// Verify finds and validates the attestation for the given Docker image ID.
	// It returns true if any attestation exists with a valid signature and
	// false if no such attestation exists, or an error if any of the required
	// API calls failed.
	Verify(ctx context.Context, imageID string) (bool, error)
}

// HttpClient implements Client by communicating with the attest service.
type HttpClient struct {
	host string
	c    *http.Client
}

// NewHttpClient returns an HttpClient instance.
func NewHttpClient(host string, c *http.Client) *HttpClient {
	return &HttpClient{
		host: host,
		c:    c,
	}
}

// VerifyRequest is the body of an HTTP request to Client.Verify.
type VerifyRequest struct {
	ImageID string `json:"imageID"`
}

// VerifyRequest is the body of an HTTP response from Client.Verify.
type VerifyResponse struct {
	Verified bool `json:"verifiedAttestation"`
}

// Verify implements Client.
func (c *HttpClient) Verify(ctx context.Context, imageID string) (bool, error) {
	// Validate the imageID before sending any requests.
	if err := ValidateImageID(imageID); err != nil {
		return false, skerr.Wrap(err)
	}

	// Create the request.
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(VerifyRequest{
		ImageID: imageID,
	}); err != nil {
		return false, skerr.Wrapf(err, "failed to encode request body")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host, &buf)
	if err != nil {
		return false, skerr.Wrapf(err, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")

	// Execute the request.
	resp, err := c.c.Do(req)
	if err != nil {
		return false, skerr.Wrapf(err, "failed to execute request")
	}
	defer util.Close(resp.Body)

	// Decode the response and return.
	b, err := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return false, skerr.Fmt("request failed with status %s: %s", resp.Status, string(b))
	}
	var result VerifyResponse
	if err := json.NewDecoder(bytes.NewReader(b)).Decode(&result); err != nil {
		return false, skerr.Wrapf(err, "failed to decode response body")
	}
	return result.Verified, nil
}

var _ Client = &HttpClient{}

// Server wraps a Client and serves HTTP requests.
type Server struct {
	wrappedClient Client
}

func NewServer(wrapped Client) *Server {
	return &Server{
		wrappedClient: wrapped,
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Decode the request.
	var req VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "failed to decode request body", http.StatusBadRequest)
		return
	}

	// Verify the image.
	verified, err := s.wrappedClient.Verify(r.Context(), req.ImageID)
	if IsErrBadImageFormat(err) {
		http.Error(w, skerr.Unwrap(err).Error(), http.StatusBadRequest)
		return
	} else if err != nil {
		sklog.Errorf("Failed checking attestation of %s: %s", req.ImageID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Encode the response.
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(VerifyResponse{
		Verified: verified,
	}); err != nil {
		http.Error(w, "failed to encode response body", http.StatusInternalServerError)
		return
	}
}
