package frontend

import (
	"errors"
	"io"
	"net/http"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

// Proxy_GetHandler proxies a GET request to the given url.
//
// Takes the URL to fetch in the "url" query parameter.
//
// It is intended to be used to work around CORS issues, where a browser can't
// directly contact another server, e.g. googlesource.com.
func Proxy_GetHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	url := r.FormValue("url")
	if url == "" {
		httputils.ReportError(w, errors.New("Missing 'url' query parameter."), "Missing 'url' query parameter.", http.StatusBadRequest)
		return
	}

	// We don't use the httputils.NewTimeoutClient() because we don't want to
	// follow redirects, we want to proxy them.
	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		httputils.ReportError(w, err, "Failed to create request.", http.StatusInternalServerError)
		return
	}
	// Copy headers from the original request.
	req.Header = r.Header
	req.Header.Del("Referer")
	req.Header.Del("Origin")
	req.Header.Del("Accept-Encoding") // To avoid getting gzipped content we can't easily handle.

	resp, err := client.Do(req)
	if err != nil {
		httputils.ReportError(w, err, "Failed to make request.", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy headers from the proxied response.
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		// We can't call ReportError here because we've already written the header.
		sklog.Errorf("Failed to write proxied response: %s", err)
	}
}
