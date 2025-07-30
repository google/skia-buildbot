package types

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"

	"go.skia.org/infra/go/cache"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/time/rate"
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

// VerifyFunc is a function which finds and validates the attestation for the
// given Docker image ID. It returns true if any attestation exists with a valid
// signature and false if no such attestation exists, or an error if any of the
// required API calls failed.
//
// VerifyFunc is an adapter which allows the use of ordinary functions as
// Client implementations.
type VerifyFunc func(ctx context.Context, imageID string) (bool, error)

// Verify implements Client.
func (f VerifyFunc) Verify(ctx context.Context, imageID string) (bool, error) {
	return f(ctx, imageID)
}

var _ Client = VerifyFunc(nil)

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

// cache.Cache uses strings for keys and values.
const (
	cachedValueTrue  = "true"
	cachedValueFalse = "false"
)

// WithCache returns a Client which uses the given cache.
func WithCache(wrapped Client, cache cache.Cache) VerifyFunc {
	return func(ctx context.Context, imageID string) (bool, error) {
		cachedValue, err := cache.GetValue(ctx, imageID)
		if err != nil {
			return false, skerr.Wrapf(err, "failed to retrieve cached value for %s", imageID)
		}
		switch cachedValue {
		case cachedValueTrue:
			return true, nil
		case cachedValueFalse:
			return false, nil
		default:
			verified, err := wrapped.Verify(ctx, imageID)
			if err != nil {
				return false, skerr.Wrap(err)
			}
			cachedValue = cachedValueFalse
			if verified {
				cachedValue = cachedValueTrue
			}
			if err := cache.SetValue(ctx, imageID, cachedValue); err != nil {
				return false, skerr.Wrapf(err, "failed to set cached value for %s", imageID)
			}
			return verified, nil
		}
	}
}

// WithRateLimiter returns a Client which uses the given rate.Limiter.
func WithRateLimiter(wrapped Client, lim *rate.Limiter) VerifyFunc {
	return func(ctx context.Context, imageID string) (bool, error) {
		if err := lim.Wait(ctx); err != nil {
			return false, skerr.Wrap(err)
		}
		return wrapped.Verify(ctx, imageID)
	}
}

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
