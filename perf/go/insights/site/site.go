// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package site serves the perfinsights landing page, which tells people how to connect an agent
// and lets them try the API, and /llms.txt, which tells the same to LLMs.
package site

import (
	"bytes"
	_ "embed"
	"encoding/json"
	htmltemplate "html/template"
	"net/http"
	"sort"
	"strings"
	texttemplate "text/template"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/insights/mcptools"
	"go.skia.org/infra/perf/go/insights/rest"
)

var (
	//go:embed index.html
	indexTemplate string

	//go:embed trace.html
	traceTemplate string

	//go:embed llms.txt
	llmsTemplate string

	//go:embed site.css
	siteCSS []byte

	//go:embed site.js
	siteJS []byte
)

// ContentSecurityPolicy only allows same-origin scripts, styles and fetches, and no inline code.
const ContentSecurityPolicy = "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// restEquivalents maps tools to the matching REST endpoints.
var restEquivalents = map[string]string{
	mcptools.ToolListSheriffs:             "GET /api/v1/sheriffs",
	mcptools.ToolSummarizeAnomalies:       "GET /api/v1/insights:summarize?sheriff=...&days=7",
	mcptools.ToolSuggestTriage:            "GET /api/v1/insights:suggestTriage?sheriff=...&days=1",
	mcptools.ToolListAnomalies:            "GET /api/v1/anomalies?sheriff=...",
	mcptools.ToolGetAnomalies:             "GET /api/v1/anomalies/{id}, POST /api/v1/anomalies:batchGet",
	mcptools.ToolGetAnomalyTrace:          "GET /api/v1/anomalies/{id}/trace",
	mcptools.ToolGetAnomalyGroup:          "GET /api/v1/anomalyGroups/{id}",
	mcptools.ToolFindAnomaliesForRevision: "GET /api/v1/revisions/{rev}/anomalies",
}

// examplePrompts are questions an agent connected to perfinsights can answer.
var examplePrompts = []string{
	"Which performance regressions are untriaged for sheriff <name> this week? Group the ones that may share a root cause.",
	"Suggest how to triage today's untriaged anomalies for sheriff <name>, with links to apply it in Perf and the bug drafts.",
	"Explain anomaly <id>: what changed, why was it flagged, and which commits are suspects?",
	"Show me the trace of anomaly <id> over the last 300 commits: did it recover?",
	"Did commit <revision> regress anything?",
	"Summarize the last 30 days of anomalies for sheriff <name> by benchmark and bot.",
}

// Endpoint is an HTTP endpoint listed on the pages.
type Endpoint struct {
	Method  string
	Path    string
	Summary string

	// Parameters lists the query and body parameters, required ones marked with "*".
	Parameters []string
}

// serviceEndpoints are the endpoints outside the REST API.
var serviceEndpoints = []Endpoint{
	{Method: "POST", Path: "/mcp", Summary: "MCP (Streamable HTTP, stateless): the tools above, for agents."},
	{Method: "GET", Path: "/trace", Summary: "Trace viewer page: the chart and details of one anomaly.", Parameters: []string{"id", "before", "after"}},
	{Method: "GET", Path: "/openapi.json", Summary: "The OpenAPI 3 contract of the REST API."},
	{Method: "GET", Path: "/llms.txt", Summary: "How to use this server, for LLMs."},
	{Method: "GET", Path: "/healthz", Summary: "Health check: whether the server can read the database."},
}

// orderedKeys returns the keys of a JSON object in document order.
func orderedKeys(obj json.RawMessage) ([]string, error) {
	dec := json.NewDecoder(bytes.NewReader(obj))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, skerr.Fmt("not a JSON object")
	}
	keys := []string{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		key, ok := t.(string)
		if !ok {
			return nil, skerr.Fmt("unexpected token %v", t)
		}
		keys = append(keys, key)
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	return keys, nil
}

// openAPIOperation is the part of an OpenAPI operation shown on the pages.
type openAPIOperation struct {
	Summary    string `json:"summary"`
	Parameters []struct {
		Name     string `json:"name"`
		In       string `json:"in"`
		Required bool   `json:"required"`
	} `json:"parameters"`
	RequestBody *struct {
		Content map[string]struct {
			Schema struct {
				Required   []string                   `json:"required"`
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"schema"`
		} `json:"content"`
	} `json:"requestBody"`
}

// endpointsFromOpenAPI returns the operations of an OpenAPI document, in document order, and its
// version.
func endpointsFromOpenAPI(spec []byte) ([]Endpoint, string, error) {
	var doc struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
		Paths json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(spec, &doc); err != nil {
		return nil, "", skerr.Wrapf(err, "parsing the OpenAPI document")
	}
	paths, err := orderedKeys(doc.Paths)
	if err != nil {
		return nil, "", skerr.Wrapf(err, "reading the OpenAPI paths")
	}
	var ops map[string]map[string]openAPIOperation
	if err := json.Unmarshal(doc.Paths, &ops); err != nil {
		return nil, "", skerr.Wrapf(err, "parsing the OpenAPI paths")
	}
	ret := []Endpoint{}
	for _, path := range paths {
		for _, method := range []string{"get", "post", "put", "patch", "delete"} {
			op, ok := ops[path][method]
			if !ok {
				continue
			}
			params := []string{}
			for _, p := range op.Parameters {
				if p.In == "path" {
					continue // Already visible in the path.
				}
				params = append(params, requiredMark(p.Name, p.Required))
			}
			if op.RequestBody != nil {
				if body, ok := op.RequestBody.Content["application/json"]; ok {
					required := map[string]bool{}
					for _, r := range body.Schema.Required {
						required[r] = true
					}
					names := make([]string, 0, len(body.Schema.Properties))
					for name := range body.Schema.Properties {
						names = append(names, name)
					}
					sort.Strings(names)
					for _, name := range names {
						params = append(params, "body: "+requiredMark(name, required[name]))
					}
				}
			}
			ret = append(ret, Endpoint{Method: strings.ToUpper(method), Path: path, Summary: op.Summary, Parameters: params})
		}
	}
	return ret, doc.Info.Version, nil
}

func requiredMark(name string, required bool) string {
	if required {
		return name + "*"
	}
	return name
}

// Options configure the site.
type Options struct {
	// InstanceName is the name of the Perf instance.
	InstanceName string

	// InstanceURL is the URL of the Perf UI. Optional.
	InstanceURL string

	// Visibility is "internal" or "public".
	Visibility string

	// PublicURL is the base URL clients use to reach this server, e.g. http://127.0.0.1:8090.
	PublicURL string

	// BasePath is the path the site is mounted under, e.g. /insights, used for the links and
	// assets of the pages. Empty when it is served at the root. Only plain path segments are
	// allowed (see rest.IsValidBasePath), so the pages can't load scripts or send requests
	// anywhere else.
	BasePath string
}

// Tool describes a tool on the pages.
type Tool struct {
	mcptools.ToolInfo
	REST string
}

type pageData struct {
	Options
	Version          string
	MCPURL           string
	Tools            []Tool
	Endpoints        []Endpoint
	ServiceEndpoints []Endpoint
	Prompts          []string
}

// Site serves the pages. They are rendered once, by New. It is safe for concurrent use.
type Site struct {
	index []byte
	trace []byte
	llms  []byte
}

// renderHTML renders an html/template, which escapes every value for its context.
func renderHTML(name, text string, data pageData) ([]byte, error) {
	t, err := htmltemplate.New(name).Funcs(htmltemplate.FuncMap{"join": strings.Join}).Parse(text)
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing %s", name)
	}
	var b bytes.Buffer
	if err := t.Execute(&b, data); err != nil {
		return nil, skerr.Wrapf(err, "rendering %s", name)
	}
	return b.Bytes(), nil
}

// New renders the pages. The REST endpoints come from the OpenAPI document served at
// /openapi.json, and the tools from the MCP server, so the pages always match the API.
func New(opts Options) (*Site, error) {
	opts.PublicURL = strings.TrimRight(opts.PublicURL, "/")
	opts.BasePath = strings.TrimRight(opts.BasePath, "/")
	if !rest.IsValidBasePath(opts.BasePath) {
		return nil, skerr.Fmt("invalid base path %q: want a path like /insights", opts.BasePath)
	}
	endpoints, version, err := endpointsFromOpenAPI(rest.OpenAPISpec())
	if err != nil {
		return nil, err
	}
	d := pageData{
		Options:          opts,
		Version:          version,
		MCPURL:           opts.PublicURL + "/mcp",
		Endpoints:        endpoints,
		ServiceEndpoints: serviceEndpoints,
		Prompts:          examplePrompts,
	}
	for _, ti := range mcptools.Catalog() {
		d.Tools = append(d.Tools, Tool{ToolInfo: ti, REST: restEquivalents[ti.Name]})
	}

	index, err := renderHTML("index.html", indexTemplate, d)
	if err != nil {
		return nil, err
	}
	trace, err := renderHTML("trace.html", traceTemplate, d)
	if err != nil {
		return nil, err
	}

	// llms.txt is served as text/plain with nosniff, so text/template is enough.
	tt, err := texttemplate.New("llms.txt").Funcs(texttemplate.FuncMap{"join": strings.Join}).Parse(llmsTemplate)
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing llms.txt")
	}
	var llms bytes.Buffer
	if err := tt.Execute(&llms, d); err != nil {
		return nil, skerr.Wrapf(err, "rendering llms.txt")
	}
	return &Site{index: index, trace: trace, llms: llms.Bytes()}, nil
}

// RegisterHandlers registers the pages on r.
func (s *Site) RegisterHandlers(r chi.Router) {
	r.Get("/", serve("text/html; charset=utf-8", s.index))
	// The trace viewer is static too: site.js reads the anomaly id from the query string and
	// loads the data from the REST API.
	r.Get("/trace", serve("text/html; charset=utf-8", s.trace))
	r.Get("/llms.txt", serve("text/plain; charset=utf-8", s.llms))
	r.Get("/static/site.css", serve("text/css; charset=utf-8", siteCSS))
	r.Get("/static/site.js", serve("text/javascript; charset=utf-8", siteJS))
}

func serve(contentType string, body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		SetSecurityHeaders(h)
		h.Set("Content-Type", contentType)
		h.Set("Cache-Control", "no-cache")
		if _, err := w.Write(body); err != nil {
			sklog.Warningf("Failed to write %s: %s", r.URL.Path, err)
		}
	}
}

// SetSecurityHeaders sets the headers that protect the pages in browsers.
func SetSecurityHeaders(h http.Header) {
	h.Set("Content-Security-Policy", ContentSecurityPolicy)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
}
