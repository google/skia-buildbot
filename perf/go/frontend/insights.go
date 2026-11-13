// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/insights"
	"go.skia.org/infra/perf/go/insights/mcptools"
	"go.skia.org/infra/perf/go/insights/rest"
	"go.skia.org/infra/perf/go/insights/site"
	"go.skia.org/infra/perf/go/insights/tracereader"
)

const (
	// insightsPath is where perfinsights is served. It is not under /api/v1, which is reserved
	// for the perf.v1 API.
	insightsPath = "/insights"

	// insightsRequestTimeout bounds each perfinsights request, including its database queries.
	// The database pool requires a deadline on every query.
	insightsRequestTimeout = 15 * time.Second

	// insightsMaxConcurrentRequests bounds the perfinsights requests served at once, so agents
	// can't take the database connections the Perf UI needs.
	insightsMaxConcurrentRequests = 8
)

// insightsRoles are the roles that can read perfinsights.
var insightsRoles = roles.Roles{roles.Viewer, roles.Editor, roles.Admin}

// mountInsights serves perfinsights under /insights: a page, a REST API and an MCP endpoint that
// let LLM agents read the anomalies of this instance. It only reads Perf data, through read-only
// interfaces of the stores Perf already has; see perf/go/insights. Enabled by the
// enable_insights instance config.
func (f *Frontend) mountInsights(router chi.Router) error {
	uiURL := strings.TrimRight(config.Config.URL, "/")
	visibility := "internal"
	if vc := config.Config.VisibilityConfig; vc != nil && vc.ShowOnlyPublicTraces {
		visibility = "public"
	}
	stores := insights.Stores{
		Regressions:   f.regStore,
		Subscriptions: f.subStore,
		AnomalyGroups: f.anomalygroupStore,
		Culprits:      f.culpritStore,
		Commits:       f.perfGit,
		Alerts:        f.configProvider,
	}
	if f.db != nil {
		// One small query per trace request, instead of the trace store's index of every trace.
		stores.Traces = tracereader.New(f.db)
	}
	opts := insights.Options{
		InstanceName: config.Config.InstanceName,
		InstanceURL:  uiURL,
		Visibility:   visibility,
	}
	if f.urlProvider != nil && uiURL != "" {
		urls := f.urlProvider
		opts.ExploreURL = func(ctx context.Context, start, end int, params map[string][]string) string {
			return uiURL + urls.Explore(ctx, start, end, params, false, nil)
		}
	}
	svc, err := insights.New(stores, opts)
	if err != nil {
		return skerr.Wrapf(err, "creating the perfinsights service")
	}
	return mountInsightsService(router, svc, f.loginProvider, uiURL+insightsPath)
}

// mountInsightsService serves svc under insightsPath on router. publicURL is the URL clients use
// to reach insightsPath, shown on the pages.
func mountInsightsService(router chi.Router, svc *insights.Service, login alogin.Login, publicURL string) error {
	api, err := rest.NewWithBasePath(svc, insightsPath)
	if err != nil {
		return skerr.Wrap(err)
	}
	instance := svc.Instance()
	pages, err := site.New(site.Options{
		InstanceName: instance.Name,
		InstanceURL:  instance.URL,
		Visibility:   instance.Visibility,
		PublicURL:    publicURL,
		BasePath:     insightsPath,
	})
	if err != nil {
		return skerr.Wrap(err)
	}
	mcpHandler := mcptools.NewHTTPHandler(mcptools.NewServer(svc))

	router.Route(insightsPath, func(r chi.Router) {
		r.Use(requireInsightsRole(login), limitConcurrency(insightsMaxConcurrentRequests), withDeadline(insightsRequestTimeout))
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			rest.WriteError(w, insights.NotFound("see "+insightsPath+"/llms.txt or "+insightsPath+"/openapi.json for the available endpoints", "no such endpoint"))
		})
		r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
			rest.WriteJSON(w, http.StatusMethodNotAllowed, insights.ErrorEnvelope{Error: &insights.Error{
				Code:    insights.CodeInvalidArgument,
				Message: "method not allowed",
				Hint:    "see " + insightsPath + "/openapi.json for the method of each endpoint",
			}})
		})
		api.RegisterHandlers(r)
		r.Handle("/mcp", mcpHandler)
		pages.RegisterHandlers(r)
		r.Get("/healthz", insightsHealthz(svc))
	})
	return nil
}

// requireInsightsRole only serves signed-in users with a Perf role. The auth proxy in front of
// Perf already requires both, except for requests it lets through unchecked (POSTs, when it is
// configured to allow them), so they are checked again here.
func requireInsightsRole(login alogin.Login) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if login == nil || login.LoggedInAs(r) == "" {
				rest.WriteError(w, &insights.Error{
					Code:    insights.CodeUnauthenticated,
					Message: "authentication required",
					Hint:    "sign in to this Perf instance",
				})
				return
			}
			if !login.Roles(r).IsAuthorized(insightsRoles) {
				rest.WriteError(w, &insights.Error{Code: insights.CodePermissionDenied, Message: "the viewer role is required"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// limitConcurrency serves at most n requests at once and rejects the others with 429.
func limitConcurrency(n int) func(http.Handler) http.Handler {
	sem := make(chan struct{}, n)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				next.ServeHTTP(w, r)
			default:
				w.Header().Set("Retry-After", "2")
				rest.WriteError(w, &insights.Error{
					Code:    insights.CodeResourceExhausted,
					Message: "too many perfinsights requests at once",
					Hint:    "retry in a few seconds, and make fewer calls in parallel",
				})
			}
		})
	}
}

// withDeadline bounds the time spent on each request.
func withDeadline(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// insightsHealthz reports whether perfinsights can read the database.
func insightsHealthz(svc *insights.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := svc.Ping(r.Context()); err != nil {
			sklog.Errorf("perfinsights health check failed: %s", err)
			rest.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		rest.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
