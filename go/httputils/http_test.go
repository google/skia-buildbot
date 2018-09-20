package httputils

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

func TestResponse2xxOnly(t *testing.T) {
	testutils.SmallTest(t)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code, err := strconv.Atoi(r.URL.Query().Get("code"))
		assert.NoError(t, err)
		w.WriteHeader(code)
	}))
	defer s.Close()
	test := func(c *http.Client, code int, expectError bool) {
		resp, err := c.Get(s.URL + "/get?code=" + strconv.Itoa(code))
		if expectError {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, code, resp.StatusCode)
			ReadAndClose(resp.Body)
		}
	}
	c := s.Client()
	test(c, http.StatusProcessing, false)
	test(c, http.StatusOK, false)
	test(c, http.StatusNotModified, false)
	test(c, http.StatusNotFound, false)
	test(c, http.StatusServiceUnavailable, false)
	c = Response2xxOnly(c)
	test(c, http.StatusProcessing, true)
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

func TestBackoffTransport2xxOnly(t *testing.T) {
	testutils.LargeTest(t) // BackoffTransport sleeps between requests.
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
	bt := NewConfiguredBackOffTransportAllResponses(config)
	wrapped := &MockRoundTripper{}
	bt.(*BackOffTransport).Transport = wrapped
	bt = Response2xxOnlyTransport{bt}

	// test takes a slice of response codes for the server to respond with (the last being repeated),
	// where 0 code means the wrapped RoundTripper returns an error, and whether we expect
	// BackOffTransport to return an error. If an error is not expected, verifies that the response
	// code from BackoffTransport is equal to the final value in codes.
	test := func(codes []int, expectError bool) {
		wrapped.responseCodes = codes
		r := httptest.NewRequest("GET", "http://example.com/foo", nil)
		now := time.Now()
		resp, err := bt.RoundTrip(r)
		dur := time.Now().Sub(now)
		if expectError {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, codes[len(codes)-1], resp.StatusCode)
			ReadAndClose(resp.Body)
		}
		if len(codes) > 1 {
			// There's not much we can assert other than there's a delay of at least
			// (INITIAL_INTERVAL * (1 - RANDOMIZATION_FACTOR)) after the first attempt.
			minDur := time.Duration(float64(INITIAL_INTERVAL) * (1 - RANDOMIZATION_FACTOR))
			assert.Truef(t, dur >= minDur, "For codes %v, expected duration to be at least %d, but was %d", codes, minDur, dur)
		}
	}
	// No retries.
	test([]int{http.StatusOK}, false)
	test([]int{http.StatusProcessing}, true)
	test([]int{http.StatusNotModified}, true)
	test([]int{http.StatusNotFound}, true)
	// Some retries before non-retriable status code.
	test([]int{http.StatusServiceUnavailable, http.StatusOK}, false)
	test([]int{http.StatusServiceUnavailable, http.StatusInternalServerError, http.StatusNotFound}, true)
	test([]int{http.StatusServiceUnavailable, http.StatusInternalServerError, http.StatusBadGateway, http.StatusNotModified}, true)
	// Retries exhausted for server error.
	test([]int{http.StatusInternalServerError}, true)
	// Retry transport error.
	test([]int{0, http.StatusOK}, false)
	test([]int{0, 0, http.StatusOK}, false)
	// Retries exhausted for transport error.
	test([]int{http.StatusInternalServerError, 0}, true)
}

func TestBackoffTransportAllResponses(t *testing.T) {
	testutils.LargeTest(t) // BackoffTransport sleeps between requests.
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
	bt := NewConfiguredBackOffTransportAllResponses(config)
	wrapped := &MockRoundTripper{}
	bt.(*BackOffTransport).Transport = wrapped

	// test takes a slice of response codes for the server to respond with (the last being repeated)
	// and verifies that the response code from BackoffTransport is equal to the final value in codes.
	// A 0 code means the RoundTripper returns an error.
	test := func(codes []int) {
		wrapped.responseCodes = codes
		r := httptest.NewRequest("GET", "http://example.com/foo", nil)
		now := time.Now()
		resp, err := bt.RoundTrip(r)
		dur := time.Now().Sub(now)
		expected := codes[len(codes)-1]
		if expected == 0 {
			assert.Equal(t, mockRoundTripErr, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, codes[len(codes)-1], resp.StatusCode)
			ReadAndClose(resp.Body)
		}
		if len(codes) > 1 {
			// There's not much we can assert other than there's a delay of at least
			// (INITIAL_INTERVAL * (1 - RANDOMIZATION_FACTOR)) after the first attempt.
			minDur := time.Duration(float64(INITIAL_INTERVAL) * (1 - RANDOMIZATION_FACTOR))
			assert.Truef(t, dur >= minDur, "For codes %v, expected duration to be at least %d, but was %d", codes, minDur, dur)
		}
	}
	// No retries.
	test([]int{http.StatusOK})
	test([]int{http.StatusProcessing})
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

func TestForceHTTPS(t *testing.T) {
	testutils.SmallTest(t)
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.WriteString(w, "Hello World!")
		assert.NoError(t, err)
	})
	// Test w/o ForceHTTPS in place.
	r := httptest.NewRequest("GET", "http://example.com/foo", nil)
	r.Header.Set(SCHEME_AT_LOAD_BALANCER_HEADER, "http")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, 200, w.Result().StatusCode)
	assert.Equal(t, "", w.Result().Header.Get("Location"))
	b, err := ioutil.ReadAll(w.Result().Body)
	assert.NoError(t, err)
	assert.Len(t, b, 12)

	// Add in ForceHTTPS behavior.
	h = HealthzAndHTTPS(h)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, 301, w.Result().StatusCode)
	assert.Equal(t, "https://example.com/foo", w.Result().Header.Get("Location"))

	// Test the healthcheck handling.
	r = httptest.NewRequest("GET", "http://example.com/", nil)
	r.Header.Set("User-Agent", "GoogleHC/1.0")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, 200, w.Result().StatusCode)
	assert.Equal(t, "", w.Result().Header.Get("Location"))
	b, err = ioutil.ReadAll(w.Result().Body)
	assert.NoError(t, err)
	assert.Len(t, b, 0)
}
