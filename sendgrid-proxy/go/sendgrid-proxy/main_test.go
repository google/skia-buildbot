// Reverse proxy that handles SendGrid API requests and adds in the SendGrid API Key.
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	goodAPIKey = "123456"
	badAPIKey  = "654321"
)

func setupForTest(t *testing.T, h http.HandlerFunc) *url.URL {
	unittest.MediumTest(t)
	// Create a stand-in for the SendGrid server.
	sendGridAPIServer := httptest.NewServer(h)
	t.Cleanup(func() {
		sendGridAPIServer.Close()
	})

	metrics2.GetCounter(requestsMetricName).Reset()
	metrics2.GetCounter(errorsMetricName).Reset()

	sendGridAPIServerURL, err := url.Parse(sendGridAPIServer.URL)
	require.NoError(t, err)
	return sendGridAPIServerURL
}

func TestNewProxy_HappyPath(t *testing.T) {
	sendGridAPIServerURL := setupForTest(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, fmt.Sprintf("Bearer: %s", goodAPIKey), r.Header.Get("Authorization"))
	})

	// Create our proxy and point to the server constructed above.
	p := newProxy(sendGridAPIServerURL, goodAPIKey)

	// Send it a request.
	r := httptest.NewRequest("POST", sendGridAPIServerURL.String(), nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, r)

	// A 200 OK response means the Bearer token was passed along correctly.
	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	require.Equal(t, int64(1), metrics2.GetCounter(requestsMetricName).Get())
	require.Equal(t, int64(0), metrics2.GetCounter(errorsMetricName).Get())
}

func TestNewProxy_BadAPIKey_ResponseIsStatusUnauthorized(t *testing.T) {
	sendGridAPIServerURL := setupForTest(t, func(w http.ResponseWriter, r *http.Request) {
		// Test the test.
		require.Equal(t, fmt.Sprintf("Bearer: %s", badAPIKey), r.Header.Get("Authorization"))
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})

	// Create our proxy and point to the server constructed above.
	p := newProxy(sendGridAPIServerURL, badAPIKey)

	// Send it a request.
	r := httptest.NewRequest("POST", sendGridAPIServerURL.String(), nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, r)

	// Confirm the StatusUnauthorized makes it back to the caller.
	require.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
	require.Equal(t, int64(1), metrics2.GetCounter(requestsMetricName).Get())
	require.Equal(t, int64(0), metrics2.GetCounter(errorsMetricName).Get())
}

func TestNewProxy_BadURLForTarget_ReturnsInternalServiceError(t *testing.T) {
	// Call setupForTest to reset the metrics.
	_ = setupForTest(t, nil)

	// Start with a bad target URL.
	target := &url.URL{
		Scheme: "http",
		Host:   "dummy.tld",
		Path:   "/",
	}

	// Point proxy to the bad URL.
	p := newProxy(target, goodAPIKey)

	// Send it a request.
	r := httptest.NewRequest("POST", target.String(), nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, r)

	// Confirm StatusInternalServerError and error metric gets incremented.
	require.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)
	require.Equal(t, int64(1), metrics2.GetCounter(requestsMetricName).Get())
	require.Equal(t, int64(1), metrics2.GetCounter(errorsMetricName).Get())
}
