package frontend

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

const (
	// maxProxiedResponseBytes limits the response size to 10MB to prevent DoS.
	maxProxiedResponseBytes = 10 * 1024 * 1024

	// proxyRequestTimeout is the timeout for outgoing proxy requests.
	proxyRequestTimeout = 30 * time.Second
)

// isAllowedProxyURL validates whether the given URL is safe and allowed to be proxied.
// Only HTTPS requests to googlesource.com and its subdomains (*.googlesource.com) are permitted.
func isAllowedProxyURL(u *url.URL) bool {
	if u == nil {
		return false
	}
	if u.Scheme != "https" {
		return false
	}
	// Disallow user credentials in URL (e.g., https://user:pass@host)
	if u.User != nil {
		return false
	}
	// Only allow standard HTTPS port (empty or 443)
	if u.Port() != "" && u.Port() != "443" {
		return false
	}
	hostname := strings.ToLower(u.Hostname())
	if hostname == "" {
		return false
	}
	if hostname == "googlesource.com" || strings.HasSuffix(hostname, ".googlesource.com") {
		return true
	}
	return false
}

// proxyTransport allows injecting a custom RoundTripper for unit testing.
var proxyTransport http.RoundTripper

// Proxy_GetHandler proxies a GET request to the given url.
//
// Takes the URL to fetch in the "url" query parameter.
//
// It is intended to be used to work around CORS issues, where a browser can't
// directly contact another server, e.g. googlesource.com.
func Proxy_GetHandler(w http.ResponseWriter, r *http.Request) {
	urlParam := r.FormValue("url")
	if urlParam == "" {
		httputils.ReportError(w, errors.New("Missing 'url' query parameter."), "Missing 'url' query parameter.", http.StatusBadRequest)
		return
	}

	parsedURL, err := url.Parse(urlParam)
	if err != nil {
		httputils.ReportError(w, err, "Invalid 'url' query parameter.", http.StatusBadRequest)
		return
	}

	if !isAllowedProxyURL(parsedURL) {
		httputils.ReportError(w, errors.New("URL target is not allowed."), "URL target is not allowed.", http.StatusBadRequest)
		return
	}

	client := &http.Client{
		Timeout:   proxyRequestTimeout,
		Transport: proxyTransport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Do not follow redirects automatically to avoid redirect-based SSRF.
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		httputils.ReportError(w, err, "Failed to create request.", http.StatusInternalServerError)
		return
	}

	// Do NOT copy client request headers (especially Cookie, Authorization, etc.) to prevent credential leakage.
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Skia-Perf-Proxy")

	resp, err := client.Do(req)
	if err != nil {
		httputils.ReportError(w, err, "Failed to make request.", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, io.LimitReader(resp.Body, maxProxiedResponseBytes)); err != nil {
		// We can't call ReportError here because we've already written the header.
		sklog.Errorf("Failed to write proxied response: %s", err)
	}
}
