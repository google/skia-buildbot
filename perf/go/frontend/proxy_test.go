package frontend

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func TestIsAllowedProxyURL(t *testing.T) {
	testCases := []struct {
		urlStr  string
		allowed bool
	}{
		// Allowed cases
		{"https://chromium.googlesource.com/v8/v8/+log/master?format=JSON", true},
		{"https://skia.googlesource.com/buildbot/+log/main?format=JSON", true},
		{"https://googlesource.com/repo/+log/main", true},
		{"https://a.b.googlesource.com/repo", true},
		{"https://chromium.googlesource.com:443/v8/v8", true},

		// Disallowed schemes
		{"http://chromium.googlesource.com/v8/v8", false},
		{"ftp://chromium.googlesource.com/v8/v8", false},
		{"file:///etc/passwd", false},
		{"javascript:alert(1)", false},
		{"gopher://chromium.googlesource.com/", false},

		// Disallowed domains / hosts
		{"https://attacker.com/v8/v8", false},
		{"https://evilgooglesource.com/v8/v8", false},
		{"https://googlesource.com.attacker.com/v8/v8", false},
		{"https://169.254.169.254/computeMetadata/v1/", false},
		{"https://127.0.0.1:8080/", false},
		{"https://localhost/", false},

		// Disallowed user credentials or non-standard ports
		{"https://user:pass@chromium.googlesource.com/", false},
		{"https://chromium.googlesource.com:8443/v8/v8", false},
		{"https://chromium.googlesource.com:80/v8/v8", false},
	}

	for _, tc := range testCases {
		u, err := url.Parse(tc.urlStr)
		if err != nil {
			require.False(t, tc.allowed, "Malformed URL should not be allowed: %s", tc.urlStr)
			continue
		}
		require.Equal(t, tc.allowed, isAllowedProxyURL(u), "URL: %s", tc.urlStr)
	}

	require.False(t, isAllowedProxyURL(nil))
}

func TestProxy_GetHandler_MissingURL_ReturnsBadRequest(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/_/json/", nil)
	w := httptest.NewRecorder()

	Proxy_GetHandler(w, r)

	require.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
	require.Contains(t, w.Body.String(), "Missing 'url' query parameter.")
}

func TestProxy_GetHandler_DisallowedURL_ReturnsBadRequest(t *testing.T) {
	disallowedURLs := []string{
		"http://chromium.googlesource.com/v8/v8",
		"https://attacker.com/steal-cookie",
		"https://169.254.169.254/computeMetadata/v1/",
		"https://evilgooglesource.com/v8",
		"https://user:pass@chromium.googlesource.com/",
		"https://chromium.googlesource.com:8080/",
	}

	for _, u := range disallowedURLs {
		r := httptest.NewRequest(http.MethodGet, "/_/json/?url="+url.QueryEscape(u), nil)
		w := httptest.NewRecorder()

		Proxy_GetHandler(w, r)

		require.Equal(t, http.StatusBadRequest, w.Result().StatusCode, "Expected Bad Request for URL: %s", u)
	}
}

func TestProxy_GetHandler_AllowedURL_ProxiesWithoutLeakingCredentials(t *testing.T) {
	targetURL := "https://chromium.googlesource.com/v8/v8/+log/1234..5678?format=JSON"
	expectedBody := ")]}'\n{\"log\": [{\"commit\": \"1234\"}]}"

	var capturedReq *http.Request
	origTransport := proxyTransport
	defer func() { proxyTransport = origTransport }()

	proxyTransport = roundTripFunc(func(req *http.Request) *http.Response {
		capturedReq = req
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: io.NopCloser(bytes.NewBufferString(expectedBody)),
		}
	})

	r := httptest.NewRequest(http.MethodGet, "/_/json/?url="+url.QueryEscape(targetURL), nil)
	// Inject sensitive client credentials/cookies into incoming request
	r.Header.Set("Cookie", "session_id=secret-session-token; skia_auth=secret-auth")
	r.Header.Set("Authorization", "Bearer sensitive-oauth-token")
	r.Header.Set("X-UberProxy-Signed-UpTick", "sensitive-internal-data")
	r.Header.Set("X-Goog-Authenticated-User-Email", "victim@google.com")

	w := httptest.NewRecorder()
	Proxy_GetHandler(w, r)

	// Verify the outgoing request received by the upstream proxy
	require.NotNil(t, capturedReq)
	require.Equal(t, targetURL, capturedReq.URL.String())
	require.Empty(t, capturedReq.Header.Get("Cookie"), "Session cookies must NOT be forwarded")
	require.Empty(t, capturedReq.Header.Get("Authorization"), "Authorization header must NOT be forwarded")
	require.Empty(t, capturedReq.Header.Get("X-UberProxy-Signed-UpTick"), "Internal proxy headers must NOT be forwarded")
	require.Empty(t, capturedReq.Header.Get("X-Goog-Authenticated-User-Email"), "Internal user identity headers must NOT be forwarded")
	require.Equal(t, "application/json", capturedReq.Header.Get("Accept"))
	require.Equal(t, "Skia-Perf-Proxy", capturedReq.Header.Get("User-Agent"))

	// Verify the proxied response returned to the caller
	res := w.Result()
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "application/json", res.Header.Get("Content-Type"))
	require.Equal(t, "nosniff", res.Header.Get("X-Content-Type-Options"))
	require.Equal(t, expectedBody, w.Body.String())
}
