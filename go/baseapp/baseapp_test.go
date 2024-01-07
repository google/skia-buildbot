package baseapp

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

func TestSecurityMiddleware_NotLocalNoOptions(t *testing.T) {
	require.Equal(t, "base-uri 'none';  img-src 'self' ; object-src 'none' ; style-src 'self'  https://fonts.googleapis.com/ https://www.gstatic.com/ 'unsafe-inline' ; script-src 'strict-dynamic' $NONCE  'unsafe-inline' https: http: ; report-uri /cspreport ;", cspString([]string{"https://example.org"}, false, []Option{}))
}

func TestSecurityMiddleware_LocalNoOptions(t *testing.T) {
	require.Equal(t, "base-uri 'none';  img-src 'self' ; object-src 'none' ; style-src 'self'  https://fonts.googleapis.com/ https://www.gstatic.com/ 'unsafe-inline' ; script-src 'strict-dynamic' $NONCE 'unsafe-eval' 'unsafe-inline' https: http: ; report-uri /cspreport ;", cspString([]string{"https://example.org"}, true, []Option{}))
}

func TestSecurityMiddleware_NotLocalAllowWASM(t *testing.T) {
	require.Equal(t, "base-uri 'none';  img-src 'self' ; object-src 'none' ; style-src 'self'  https://fonts.googleapis.com/ https://www.gstatic.com/ 'unsafe-inline' ; script-src 'strict-dynamic' $NONCE 'unsafe-eval' 'unsafe-inline' https: http: ; report-uri /cspreport ;", cspString([]string{"https://example.org"}, false, []Option{AllowWASM{}}))
}

func TestSecurityMiddleware_NotLocalAllowAnyImages(t *testing.T) {
	require.Equal(t, "base-uri 'none';  img-src * 'unsafe-eval' blob: data: ; object-src 'none' ; style-src 'self'  https://fonts.googleapis.com/ https://www.gstatic.com/ 'unsafe-inline' ; script-src 'strict-dynamic' $NONCE  'unsafe-inline' https: http: ; report-uri /cspreport ;", cspString([]string{"https://example.org"}, false, []Option{AllowAnyImage{}}))
}

func testSetup(t *testing.T) string {
	// Create a resources directory with some static files.
	resourcesDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(resourcesDir, "a.txt"), []byte(`alpha`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(resourcesDir, "b.txt"), []byte(`beta`), 0644))

	osArgsBackup := os.Args
	os.Args = []string{"appname", "--local=true", fmt.Sprintf("--resources_dir=%s", resourcesDir)}
	t.Cleanup(func() {
		os.Args = osArgsBackup
		ctx := context.Background()

		// Gracefully shut down the HTTP server.
		require.NoError(t, server.Shutdown(ctx))
		assert.ErrorIs(t, listenAndServeErr, http.ErrServerClosed)
		require.NoError(t, metrics2.Shutdown(ctx))
	})

	return resourcesDir
}

func TestServe_EndToEnd_Success(t *testing.T) {
	resourcesDir := testSetup(t)

	// Start the server.
	isServeTest = true
	app := newE2ETestApp()
	go Serve(func() (App, error) {
		return app, nil
	}, []string{"localhost"}, Testing{})

	time.Sleep(2 * time.Second) // Give the HTTP server time to boot.

	endToEndImpl(t, resourcesDir, app)
}

func TestServe_EndToEndWithProvidedFlagSet_Success(t *testing.T) {
	resourcesDir := testSetup(t)

	myFlagSet := flag.NewFlagSet("for-testing", flag.PanicOnError)
	myFlagSet.Bool("some-option", false, "just used as existence test")
	os.Args = append(os.Args, "--some-option=true")

	// Start the server.
	isServeTest = true
	app := newE2ETestApp()
	go Serve(func() (App, error) {
		return app, nil
	}, []string{"localhost"}, Testing{}, NewFlagSet(myFlagSet))

	time.Sleep(2 * time.Second) // Give the HTTP server time to boot.

	endToEndImpl(t, resourcesDir, app)

	require.NotNil(t, myFlagSet.Lookup("some-option"))
}

func TestServe_EndToEndWithProvidedFlagSetAndLocalFlagDefinedTwice_Panics(t *testing.T) {
	_ = testSetup(t)

	myFlagSet := flag.NewFlagSet("for-testing", flag.PanicOnError)
	myFlagSet.Bool("local", false, "This flag duplicates --local defined by App and should cause a panic")

	// Start the server.
	isServeTest = true
	app := newE2ETestApp()
	require.Panics(t, func() {
		Serve(func() (App, error) {
			return app, nil
		}, []string{"localhost"}, Testing{}, NewFlagSet(myFlagSet))
	})
}

func TestServe_EndToEndWithProvidedFlagSetAndUnknownFlagProvidedOnCommandLine_Panics(t *testing.T) {
	_ = testSetup(t)

	myFlagSet := flag.NewFlagSet("for-testing", flag.PanicOnError)
	os.Args = append(os.Args, "--some-unknown-flag")

	// Start the server.
	isServeTest = true
	app := newE2ETestApp()
	require.Panics(t, func() {
		Serve(func() (App, error) {
			return app, nil
		}, []string{"localhost"}, Testing{}, NewFlagSet(myFlagSet))
	})
}

func endToEndImpl(t *testing.T, resourcesDir string, app *e2eTestApp) {
	// Base router.
	assertGet200OK(t, "http://localhost:8000", "Hello, world!")
	assertGet200OK(t, "http://localhost:8000/", "Hello, world!")
	assertGet200OK(t, "http://localhost:8000/about", "About us...")
	assertGet200OK(t, "http://localhost:8000/user/somegoogler/info", `Details for user "somegoogler"`)
	assertGet404NotFound(t, "http://localhost:8000/user/invalid-user-123/info") // Invalid username.
	assertGet404NotFound(t, "http://localhost:8000/about/")                     // The handler only recognizes /about.
	assertGet404NotFound(t, "http://localhost:8000/no-such-page")

	// API router.
	assertGet200OK(t, "http://localhost:8000/api/foo?city=chapel%20hill&city=durham&state=nc", `{"status": "ok"}`)
	assertPost200OK(t, "http://localhost:8000/api/bar", "category=salad&ingredient=tomato&ingredient=basil", `{"status": "ok"}`)
	assertGet404NotFound(t, "http://localhost:8000/api/no-such-endpoint")

	// Static assets.
	assertGet200OK(t, "http://localhost:8000/dist/a.txt", "alpha")
	assertGet200OK(t, "http://localhost:8000/dist/b.txt", "beta")
	assertGet200OK(t, "http://localhost:8000/static/a.txt", "alpha")
	assertGet200OK(t, "http://localhost:8000/static/b.txt", "beta")

	// CSP reporter. It prints a structured log entry in JSON format to stdout.
	assert.Contains(t, captureStdout(t, func() {
		assertPostJSON200OK(t, "http://localhost:8000/cspreport", `{"hello": "world"}`, "" /* =expectedResBody */)
	}), `{"type":"csp","body":{"hello":"world"}}`)

	// Other URLs.
	assertGet200OK(t, "http://localhost:8000/healthz", "" /* =expectedBody */)

	// Assert that the middleware added via App.AddMiddleware() works.
	assert.Equal(t, []string{
		"/",
		"/",
		"/about",
		"/user/somegoogler/info",
		"/user/invalid-user-123/info",
		"/about/",
		"/no-such-page",
		"/api/foo?city=chapel%20hill&city=durham&state=nc",
		"/api/bar",
		"/api/no-such-endpoint",
		"/dist/a.txt",
		"/dist/b.txt",
		"/static/a.txt",
		"/static/b.txt",
		"/cspreport",
		"/healthz",
	}, app.loggedURLs)

	// Assert that the API subrouter's middleware works, and that GET/POST params work as expected.
	assert.Equal(t, []apiRequestLogEntry{
		{
			url: "/api/foo?city=chapel%20hill&city=durham&state=nc",
			params: []paramAndValues{
				{name: "city", values: []string{"chapel hill", "durham"}},
				{name: "state", values: []string{"nc"}},
			},
		},
		{
			url: "/api/bar",
			params: []paramAndValues{
				{name: "category", values: []string{"salad"}},
				{name: "ingredient", values: []string{"tomato", "basil"}},
			},
		},
		{url: "/api/no-such-endpoint"},
	}, app.loggedAPIRequests)

}

type paramAndValues struct {
	name   string
	values []string
}

type apiRequestLogEntry struct {
	url    string
	params []paramAndValues
}

type e2eTestApp struct {
	loggedURLs        []string
	loggedAPIRequests []apiRequestLogEntry
}

func newE2ETestApp() *e2eTestApp {
	return &e2eTestApp{}
}

func (a *e2eTestApp) AddHandlers(r chi.Router) {
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte("<p>Hello, world!</p>")); err != nil {
			sklog.Errorf("writing HTTP response: %s", err)
		}
	})

	r.Get("/about", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte("<p>About us...</p>")); err != nil {
			sklog.Errorf("writing HTTP response: %s", err)
		}
	})

	r.Get("/user/{username:[a-zA-Z]+}/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte(fmt.Sprintf("<p>Details for user %q</p>", chi.URLParam(r, "username")))); err != nil {
			sklog.Errorf("writing HTTP response: %s", err)
		}
	})

	r.Route("/api", func(r chi.Router) {
		// Adds a logger for API request parameters.
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Parse GET and POST parameters.
				if err := r.ParseForm(); err != nil {
					sklog.Errorf("failed to parse form: %s", err)
				} else {
					a.logAPIRequest(r.URL.String(), r.Form)
				}
				next.ServeHTTP(w, r)
			})
		})

		r.Get("/foo", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write([]byte(`{"status": "ok"}`)); err != nil {
				sklog.Errorf("writing HTTP response: %s", err)
			}
		})

		r.Post("/bar", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write([]byte(`{"status": "ok"}`)); err != nil {
				sklog.Errorf("writing HTTP response: %s", err)
			}
		})
	})
}

func (a *e2eTestApp) AddMiddleware() []func(http.Handler) http.Handler {
	// Adds a simple URL logging middleware.
	return []func(http.Handler) http.Handler{
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				a.loggedURLs = append(a.loggedURLs, r.URL.String())
				next.ServeHTTP(w, r)
			})
		},
	}
}

func (a *e2eTestApp) logAPIRequest(url string, params url.Values) {
	// Sort for determinism.
	var names []string
	for name := range params {
		names = append(names, name)
	}
	sort.Strings(names)

	entry := apiRequestLogEntry{url: url}
	for _, name := range names {
		entry.params = append(entry.params, paramAndValues{
			name:   name,
			values: params[name],
		})
	}

	a.loggedAPIRequests = append(a.loggedAPIRequests, entry)
}

var _ App = (*e2eTestApp)(nil)

func makeHTTPRequest(t *testing.T, method, url, contentType string, body io.Reader) (int, string) {
	req, err := http.NewRequest(method, url, body)
	require.NoError(t, err)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, res.Body.Close()) }()
	resBody, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	return res.StatusCode, string(resBody)
}

func assertGet200OK(t *testing.T, url, expectedBody string) {
	statusCode, body := makeHTTPRequest(t, "GET", url, "" /* =contentType */, nil /* =body */)
	assert.Equal(t, http.StatusOK, statusCode)
	assert.Contains(t, body, expectedBody)
}

func assertGet404NotFound(t *testing.T, url string) {
	statusCode, _ := makeHTTPRequest(t, "GET", url, "" /* =contentType */, nil /* =body */)
	assert.Equal(t, http.StatusNotFound, statusCode)
}

func assertPost200OK(t *testing.T, url, reqBody, expectedResBody string) {
	statusCode, resBody := makeHTTPRequest(t, "POST", url, "application/x-www-form-urlencoded", bytes.NewBufferString(reqBody))
	assert.Equal(t, http.StatusOK, statusCode)
	assert.Contains(t, resBody, expectedResBody)
}

func assertPostJSON200OK(t *testing.T, url, reqBody, expectedResBody string) {
	statusCode, resBody := makeHTTPRequest(t, "POST", url, "application/json", bytes.NewBufferString(reqBody))
	assert.Equal(t, http.StatusOK, statusCode)
	assert.Contains(t, resBody, expectedResBody)
}

func captureStdout(t *testing.T, fn func()) string {
	stdout := os.Stdout
	defer func() { os.Stdout = stdout }()
	fakeStdout, err := os.CreateTemp("", "fake-stdout")
	require.NoError(t, err)
	defer func() { require.NoError(t, os.Remove(fakeStdout.Name())) }()
	os.Stdout = fakeStdout

	fn()

	require.NoError(t, fakeStdout.Close())
	bytes, err := os.ReadFile(fakeStdout.Name())
	require.NoError(t, err)
	return string(bytes)
}
