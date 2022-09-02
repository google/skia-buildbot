package httputils

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/mockhttpclient"
)

func TestResponse2xxOnly(t *testing.T) {

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code, err := strconv.Atoi(r.URL.Query().Get("code"))
		require.NoError(t, err)
		w.WriteHeader(code)
	}))
	defer s.Close()
	test := func(c *http.Client, code int, expectError bool) {
		resp, err := c.Get(s.URL + "/get?code=" + strconv.Itoa(code))
		if expectError {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, code, resp.StatusCode)
			ReadAndClose(resp.Body)
		}
	}
	c := s.Client()
	test(c, http.StatusSwitchingProtocols, false)
	test(c, http.StatusOK, false)
	test(c, http.StatusNotModified, false)
	test(c, http.StatusNotFound, false)
	test(c, http.StatusServiceUnavailable, false)
	c = Response2xxOnly(c)
	test(c, http.StatusSwitchingProtocols, true)
	test(c, http.StatusOK, false)
	test(c, http.StatusNotModified, true)
	test(c, http.StatusNotFound, true)
	test(c, http.StatusServiceUnavailable, true)
}

var (
	mockRoundTripErr = errors.New("Can not round trip on a one-way street.")
)

type MockRoundTripper struct {
	// responseCodes gives the expected response for subsequent requests. The last response code is
	// repeated for subsequent requests. 0 means return mockRoundTripErr. You must set this field to a
	// non-empty slice before RoundTrip is called.
	responseCodes []int
}

func (t *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	code := t.responseCodes[0]
	if len(t.responseCodes) > 1 {
		t.responseCodes = t.responseCodes[1:]
	}
	if code == 0 {
		return nil, mockRoundTripErr
	}
	w := httptest.NewRecorder()
	w.WriteHeader(code)
	return w.Result(), nil
}

func TestBackoffTransport(t *testing.T) {
	// Use a fail-faster config so the test doesn't take so long.
	maxInterval := 600 * time.Millisecond
	config := &BackOffConfig{
		initialInterval: INITIAL_INTERVAL,
		maxInterval:     maxInterval,
		// Tests below expect at least three retries.
		maxElapsedTime:      3 * maxInterval,
		randomizationFactor: RANDOMIZATION_FACTOR,
		backOffMultiplier:   BACKOFF_MULTIPLIER,
	}
	wrapped := &MockRoundTripper{}
	bt := NewConfiguredBackOffTransport(config, wrapped)

	// test takes a slice of response codes for the server to respond with (the last being repeated)
	// and verifies that the response code from BackoffTransport is equal to the final value in codes.
	// A 0 code means the RoundTripper returns an error.
	test := func(codes []int) {
		wrapped.responseCodes = codes
		r, err := http.NewRequest("GET", "http://example.com/foo", nil)
		require.NoError(t, err)
		now := time.Now()
		resp, err := bt.RoundTrip(r)
		dur := time.Since(now)
		expected := codes[len(codes)-1]
		if expected == 0 {
			require.Equal(t, mockRoundTripErr, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, codes[len(codes)-1], resp.StatusCode)
			ReadAndClose(resp.Body)
		}
		if len(codes) > 1 {
			// There's not much we can assert other than there's a delay of at least
			// (INITIAL_INTERVAL * (1 - RANDOMIZATION_FACTOR)) after the first attempt.
			minDur := time.Duration(float64(INITIAL_INTERVAL) * (1 - RANDOMIZATION_FACTOR))
			require.Truef(t, dur >= minDur, "For codes %v, expected duration to be at least %d, but was %d", codes, minDur, dur)
		}
	}
	// No retries.
	test([]int{http.StatusOK})
	test([]int{http.StatusSwitchingProtocols})
	test([]int{http.StatusNotModified})
	test([]int{http.StatusNotFound})
	// Some retries before non-retriable status code.
	test([]int{http.StatusServiceUnavailable, http.StatusOK})
	test([]int{http.StatusServiceUnavailable, http.StatusInternalServerError, http.StatusNotFound})
	test([]int{http.StatusServiceUnavailable, http.StatusInternalServerError, http.StatusBadGateway, http.StatusNotModified})
	// Retries exhausted for server error.
	test([]int{http.StatusInternalServerError})
	// Retry transport error.
	test([]int{0, http.StatusOK})
	test([]int{0, 0, http.StatusOK})
	// Retries exhausted for transport error.
	test([]int{http.StatusInternalServerError, 0})
}

// RoundTripperFunc transforms a function into a RoundTripper
type RoundTripperFunc func(req *http.Request) (*http.Response, error)

func (f RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestBackoffTransportWithContext(t *testing.T) {
	// Use a fail-faster config so the test doesn't take so long.
	maxInterval := 600 * time.Millisecond
	config := &BackOffConfig{
		initialInterval: INITIAL_INTERVAL,
		maxInterval:     maxInterval,
		// We should never reach this deadline.
		maxElapsedTime:      10 * maxInterval,
		randomizationFactor: RANDOMIZATION_FACTOR,
		backOffMultiplier:   BACKOFF_MULTIPLIER,
	}

	// Test canceling the context after the nth request. See MockRoundTripper docs for codes;
	// len(codes) > cancelAfter. Request context will be canceled during the request with index
	// cancelAfter. Asserts that the number of retries agrees with cancelAfter.
	test := func(codes []int, cancelAfter int) {
		mock := MockRoundTripper{
			responseCodes: codes,
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		callCount := 0
		wrapped := func(req *http.Request) (*http.Response, error) {
			if cancelAfter == callCount {
				cancel()
			}
			callCount++
			return mock.RoundTrip(req)
		}
		bt := NewConfiguredBackOffTransport(config, RoundTripperFunc(wrapped))
		req, err := http.NewRequestWithContext(ctx, "GET", "http://example.com/foo", nil)
		require.NoError(t, err)
		resp, err := bt.RoundTrip(req)
		// We expect no calls after the context is canceled.
		require.Equal(t, cancelAfter, callCount-1)
		// We expect the result to be the result of the call when the context is canceled.
		expected := codes[cancelAfter]
		if expected == 0 {
			require.Equal(t, mockRoundTripErr, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, expected, resp.StatusCode)
			ReadAndClose(resp.Body)
		}
	}
	// No retries needed.
	test([]int{http.StatusOK}, 0)
	// Context is canceled, so no retry.
	test([]int{http.StatusServiceUnavailable}, 0)
	// Second request should never happen.
	test([]int{http.StatusServiceUnavailable, http.StatusInternalServerError}, 0)
	// Some retries before context canceled.
	test([]int{http.StatusServiceUnavailable, http.StatusOK}, 1)
	test([]int{http.StatusServiceUnavailable, http.StatusInternalServerError}, 1)
	test([]int{http.StatusServiceUnavailable, http.StatusInternalServerError, http.StatusBadGateway}, 2)

	// Transport error; context is canceled, so no retry.
	test([]int{0}, 0)
	// Transport error; some retries before context is canceled.
	test([]int{0, 0}, 1)
	test([]int{0, http.StatusOK}, 1)
	test([]int{0, http.StatusInternalServerError}, 1)
	test([]int{http.StatusInternalServerError, 0}, 1)
}

func TestForceHTTPS(t *testing.T) {
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.WriteString(w, "Hello World!")
		require.NoError(t, err)
	})
	// Test w/o ForceHTTPS in place.
	r := httptest.NewRequest("GET", "http://example.com/foo", nil)
	r.Header.Set(SCHEME_AT_LOAD_BALANCER_HEADER, "http")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, 200, w.Result().StatusCode)
	require.Equal(t, "", w.Result().Header.Get("Location"))
	b, err := ioutil.ReadAll(w.Result().Body)
	require.NoError(t, err)
	require.Len(t, b, 12)

	// Add in ForceHTTPS behavior.
	h = HealthzAndHTTPS(h)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, 301, w.Result().StatusCode)
	require.Equal(t, "https://example.com/foo", w.Result().Header.Get("Location"))

	// Test the healthcheck handling.
	r = httptest.NewRequest("GET", "http://example.com/", nil)
	r.Header.Set("User-Agent", "GoogleHC/1.0")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, 200, w.Result().StatusCode)
	require.Equal(t, "", w.Result().Header.Get("Location"))
	b, err = ioutil.ReadAll(w.Result().Body)
	require.NoError(t, err)
	require.Len(t, b, 0)
}

func TestGetWithContextSunnyDay(t *testing.T) {

	content := []byte("something")
	m := mockhttpclient.NewURLMock()
	resp := mockhttpclient.MockGetDialogue(content)
	m.Mock("https://example.com/foo", resp)

	r, err := GetWithContext(context.Background(), m.Client(), "https://example.com/foo")
	require.NoError(t, err)
	msg, err := ioutil.ReadAll(r.Body)
	require.NoError(t, err)
	assert.Equal(t, content, msg)
	require.NoError(t, r.Body.Close())
}

func TestGetWithContextCanceled(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := GetWithContext(ctx, http.DefaultClient, "https://example.com/")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canceled")
}

func TestPostWithContextSunnyDay(t *testing.T) {

	const mimeType = "text/plain"
	const input = "something"
	output := []byte("different")
	m := mockhttpclient.NewURLMock()
	resp := mockhttpclient.MockPostDialogue(mimeType, []byte(input), output)
	m.Mock("https://example.com/foo", resp)

	r, err := PostWithContext(context.Background(), m.Client(), "https://example.com/foo", mimeType, strings.NewReader(input))
	require.NoError(t, err)
	msg, err := ioutil.ReadAll(r.Body)
	require.NoError(t, err)
	assert.Equal(t, output, msg)
	require.NoError(t, r.Body.Close())
}

func TestPostWithContextCancelled(t *testing.T) {

	const mimeType = "text/plain"
	const input = "something"

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := PostWithContext(ctx, http.DefaultClient, "https://example.com", mimeType, strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canceled")
}

func TestCrossOriginResourcePolicy_Success(t *testing.T) {

	w := httptest.NewRecorder()
	var h http.Handler
	h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	h = CrossOriginResourcePolicy(h)
	r := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "cross-origin", w.Header().Get("Cross-Origin-Resource-Policy"))
}
