// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package site

import (
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/insights/mcptools"
	"go.skia.org/infra/perf/go/insights/rest"
)

func newTestServer(t *testing.T, opts Options) *httptest.Server {
	s, err := New(opts)
	require.NoError(t, err)
	r := chi.NewRouter()
	s.RegisterHandlers(r)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts
}

func get(t *testing.T, ts *httptest.Server, path string) (*http.Response, string) {
	resp, err := ts.Client().Get(ts.URL + path)
	require.NoError(t, err)
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, string(b)
}

func TestIndex_HasSecurityHeadersAndDocumentsAllTools(t *testing.T) {
	ts := newTestServer(t, Options{InstanceName: "demo", Visibility: "internal", PublicURL: "http://127.0.0.1:8090/"})
	resp, body := get(t, ts, "/")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.Equal(t, ContentSecurityPolicy, resp.Header.Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "no-referrer", resp.Header.Get("Referrer-Policy"))
	assert.NotEmpty(t, resp.Header.Get("Permissions-Policy"))

	assert.Contains(t, body, "http://127.0.0.1:8090/mcp")
	for _, tool := range mcptools.Catalog() {
		assert.Contains(t, body, tool.Name)
	}
}

func TestIndex_HasNoInlineScriptsOrStyles(t *testing.T) {
	ts := newTestServer(t, Options{InstanceName: "demo", Visibility: "internal", PublicURL: "http://127.0.0.1:8090"})
	_, body := get(t, ts, "/")
	assertNoInlineCode(t, body, "/static/site.js")
}

// assertNoInlineCode checks that a page only loads script from scriptSrc: the CSP blocks inline
// code, so the pages must not depend on it.
func assertNoInlineCode(t *testing.T, body, scriptSrc string) {
	tags := regexp.MustCompile(`<script[^>]*>`).FindAllString(body, -1)
	require.NotEmpty(t, tags)
	for _, tag := range tags {
		assert.Contains(t, tag, `src="`+scriptSrc+`"`)
	}
	assert.NotContains(t, body, "<style")
	assert.NotRegexp(t, `\sstyle=`, body)
	assert.NotRegexp(t, `\son[a-z]+=`, body)
}

func TestTracePage_HasSecurityHeadersAndNoInlineCode(t *testing.T) {
	ts := newTestServer(t, Options{InstanceName: "demo", Visibility: "internal", PublicURL: "http://127.0.0.1:8090"})
	resp, body := get(t, ts, "/trace?id=68bf6dad-916e-43f4-b59e-4d72d19f5ee3&before=250")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.Equal(t, ContentSecurityPolicy, resp.Header.Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assertNoInlineCode(t, body, "/static/site.js")

	// site.js finds the page and its elements by these.
	assert.Contains(t, body, `data-page="trace"`)
	for _, id := range []string{"trace-title", "trace-id", "trace-before", "trace-after", "trace-run", "trace-status", "trace-result", "trace-details"} {
		assert.Contains(t, body, `id="`+id+`"`)
	}
	assert.Contains(t, body, `href="/"`, "links back to the main page")
}

func TestIndex_LinksToTheTraceViewerInsteadOfEmbeddingIt(t *testing.T) {
	ts := newTestServer(t, Options{InstanceName: "demo", Visibility: "internal", PublicURL: "http://127.0.0.1:8090"})
	_, body := get(t, ts, "/")
	assert.Contains(t, body, `data-page="home"`)
	assert.Contains(t, body, `href="/trace"`)
	assert.NotContains(t, body, `id="trace-run"`)
	assert.NotContains(t, body, `id="trace-result"`)
}

func TestBasePath_PrefixesTheLinksAndAssetsOfThePages(t *testing.T) {
	ts := newTestServer(t, Options{
		InstanceName: "demo",
		Visibility:   "internal",
		PublicURL:    "https://perf.example.com/insights",
		BasePath:     "/insights/",
	})
	for _, path := range []string{"/", "/trace"} {
		_, body := get(t, ts, path)
		assert.Contains(t, body, `href="/insights/static/site.css"`, path)
		assert.Contains(t, body, `data-base="/insights"`, path)
		assertNoInlineCode(t, body, "/insights/static/site.js")
	}
	_, body := get(t, ts, "/trace")
	assert.Contains(t, body, `href="/insights/"`, "links back to the main page")
	_, body = get(t, ts, "/")
	assert.Contains(t, body, `href="/insights/openapi.json"`)
	assert.Contains(t, body, `href="/insights/llms.txt"`)
	assert.Contains(t, body, `href="/insights/trace"`)
	assert.Contains(t, body, "https://perf.example.com/insights/mcp")
}

func TestNew_RejectsUnsafeBasePaths(t *testing.T) {
	for _, bp := range []string{"insights", "//evil.example.com", "/a b", "/x?y=1", "/x#y", `/"><script>`, "/../x", "/a/./b", "javascript:alert(1)"} {
		_, err := New(Options{InstanceName: "demo", BasePath: bp})
		assert.Error(t, err, bp)
	}
	for _, bp := range []string{"", "/", "/insights", "/a/b-c_d"} {
		_, err := New(Options{InstanceName: "demo", BasePath: bp})
		assert.NoError(t, err, bp)
	}
}

func TestIndex_EscapesConfigValues(t *testing.T) {
	ts := newTestServer(t, Options{
		InstanceName: `<script>alert(1)</script>`,
		InstanceURL:  "javascript:alert(1)",
		Visibility:   "internal",
		PublicURL:    "http://127.0.0.1:8090",
	})
	_, body := get(t, ts, "/")
	assert.NotContains(t, body, "<script>alert(1)</script>")
	assert.Contains(t, body, "&lt;script&gt;alert(1)&lt;/script&gt;")
	assert.NotContains(t, body, `href="javascript:`)
}

func TestLLMsTxt_IsPlainTextAndListsTools(t *testing.T) {
	ts := newTestServer(t, Options{InstanceName: "demo", Visibility: "internal", PublicURL: "http://127.0.0.1:8090"})
	resp, body := get(t, ts, "/llms.txt")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.True(t, strings.HasPrefix(body, "# perfinsights"))
	assert.Contains(t, body, "MCP (Streamable HTTP, stateless, POST only): http://127.0.0.1:8090/mcp")
	assert.Contains(t, body, "- list_anomalies(sheriff*, ")
	for _, tool := range mcptools.Catalog() {
		assert.Contains(t, body, "- "+tool.Name+"(")
	}
}

func TestStaticAssets_HaveContentTypesAndSafeDOMAPIs(t *testing.T) {
	ts := newTestServer(t, Options{InstanceName: "demo", PublicURL: "http://127.0.0.1:8090"})
	resp, _ := get(t, ts, "/static/site.css")
	assert.Equal(t, "text/css; charset=utf-8", resp.Header.Get("Content-Type"))
	resp, js := get(t, ts, "/static/site.js")
	assert.Equal(t, "text/javascript; charset=utf-8", resp.Header.Get("Content-Type"))
	for _, unsafe := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "eval("} {
		assert.NotContains(t, js, unsafe)
	}

	resp, _ = get(t, ts, "/static/other.js")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestIndex_ListsEveryEndpointAndTheVersion(t *testing.T) {
	ts := newTestServer(t, Options{InstanceName: "demo", Visibility: "internal", PublicURL: "http://127.0.0.1:8090"})
	_, body := get(t, ts, "/")
	endpoints, version, err := endpointsFromOpenAPI(rest.OpenAPISpec())
	require.NoError(t, err)
	assert.Equal(t, mcptools.ServerVersion, version, "the REST API and the MCP server have one version")
	assert.Contains(t, body, "v"+version)
	require.GreaterOrEqual(t, len(endpoints), 10)
	for _, e := range append(endpoints, serviceEndpoints...) {
		assert.Contains(t, body, `<span class="method">`+e.Method+`</span> <code class="rest">`+template.HTMLEscapeString(e.Path)+`</code>`)
	}
}

func TestEndpointsFromOpenAPI_KeepsDocumentOrderAndListsParameters(t *testing.T) {
	endpoints, _, err := endpointsFromOpenAPI(rest.OpenAPISpec())
	require.NoError(t, err)
	assert.Equal(t, "/api/v1/instance", endpoints[0].Path)
	byKey := map[string]Endpoint{}
	for _, e := range endpoints {
		byKey[e.Method+" "+e.Path] = e
	}
	assert.Equal(t, []string{"commits_before", "commits_after"}, byKey["GET /api/v1/anomalies/{id}/trace"].Parameters)
	assert.Equal(t, []string{"body: ids*", "body: view"}, byKey["POST /api/v1/anomalies:batchGet"].Parameters)
	assert.Equal(t, []string{"sheriff*", "days"}, byKey["GET /api/v1/insights:suggestTriage"].Parameters)
	assert.NotEmpty(t, byKey["GET /api/v1/insights:suggestTriage"].Summary)

	_, _, err = endpointsFromOpenAPI([]byte("not json"))
	assert.Error(t, err)
}

func TestLLMsTxt_ListsTheRESTEndpoints(t *testing.T) {
	ts := newTestServer(t, Options{InstanceName: "demo", Visibility: "internal", PublicURL: "http://127.0.0.1:8090"})
	_, body := get(t, ts, "/llms.txt")
	assert.Contains(t, body, "- Version: "+mcptools.ServerVersion)
	assert.Contains(t, body, "## REST endpoints")
	assert.Contains(t, body, "- GET /api/v1/insights:suggestTriage: ")
	assert.Contains(t, body, "- POST /mcp: ")
	assert.Contains(t, body, "- GET /trace: Trace viewer page: the chart and details of one anomaly. Parameters: id, before, after.")
	assert.Contains(t, body, "http://127.0.0.1:8090/trace?id=<anomaly id>")
}
