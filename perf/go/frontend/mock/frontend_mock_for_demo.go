// Copyright 2025 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/unrolled/secure"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/frontend"
)

const (
	envPortFileBaseName = "port"
	pagesDir            = "perf/pages/production"
	imagesDir           = "perf/images"
)

func renderPage(f *frontend.Frontend, state *State, templateName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		cfg := ConfigForPersona(state.GetPersona())
		cfgBytes, err := json.Marshal(cfg)
		if err != nil {
			http.Error(w, fmt.Sprintf("Marshal error: %v", err), http.StatusInternalServerError)
			return
		}
		if f == nil || f.GetTemplates() == nil {
			_, _ = w.Write(cfgBytes)
			return
		}
		err = f.GetTemplates().ExecuteTemplate(w, templateName, map[string]interface{}{
			"context":                      template.JS(cfgBytes),
			"Nonce":                        secure.CSPNonce(r.Context()),
			"InstanceName":                 cfg.InstanceName,
			"GoogleAnalyticsMeasurementID": "G-MOCK-ID",
		})
		if err != nil {
			sklog.Errorf("Render error: %v", err)
			http.Error(w, fmt.Sprintf("Render error: %v", err), http.StatusInternalServerError)
		}
	}
}

// RegisterHandlers wires all page routes, mock control endpoints, and Perf API
// handlers onto the provided chi router. The mock server primarily serves the
// Playwright E2E test suites, while retaining lightweight stub responses for
// legacy manual demo routes (/admin/alerts, /c, /d).
func RegisterHandlers(r *chi.Mux, mockApi *Server, f *frontend.Frontend) {
	r.Use(middleware.StripSlashes)

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if strings.HasSuffix(req.URL.Path, ".js.map") {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
			next.ServeHTTP(w, req)
		})
	})

	r.Handle("/dist/images/*", http.StripPrefix("/dist/images", http.FileServer(http.Dir(imagesDir))))
	r.Handle("/dist/*", http.StripPrefix("/dist", http.FileServer(http.Dir(pagesDir))))

	state := mockApi.State()
	routes := map[string]string{
		"/e":            "newindex.html",
		"/m":            "multiexplore.html",
		"/e2":           "multiexplore-v2.html",
		"/c":            "clusters2.html",
		"/pg":           "playground.html",
		"/t":            "triage.html",
		"/d":            "dryrunalert.html",
		"/f":            "favorites.html",
		"/v":            "revisions.html",
		"/u":            "report.html",
		"/help":         "help.html",
		"/a":            "regressions.html",
		"/admin/alerts": "alerts.html",
		"/l":            "extralinks.html",
	}
	for route, tmpl := range routes {
		r.HandleFunc(route, renderPage(f, state, tmpl))
	}
	r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/e", http.StatusFound)
	})

	// Mock control plane endpoints.
	r.Post("/_/mock/reset", mockApi.ResetHandler)
	r.Post("/_/mock/set_persona", mockApi.SetPersonaHandler)
	r.Get("/_/mock/mutations", mockApi.MutationsHandler)
	r.Post("/_/test/reset", mockApi.ResetHandler)
	r.Get("/_/test/recorded", mockApi.TestRecordedHandler)

	// Perf JSON API endpoints.
	r.Get("/_/login/status", mockApi.LoginStatusHandler)
	r.Get("/_/initpage", mockApi.InitPageHandler)
	r.Get("/_/initpage/{id}", mockApi.InitPageHandler)
	r.Get("/_/defaults", mockApi.DefaultsHandler)
	r.Post("/_/nextParamList", mockApi.NextParamListHandler)
	r.Post("/_/count", mockApi.CountHandler)
	r.Post("/_/frame/start", mockApi.FrameStartHandler)
	r.Post("/_/cidRange", mockApi.CidRangeHandler)
	r.Post("/_/cid", mockApi.CidHandler)
	r.Post("/_/details", mockApi.DetailsHandler)
	r.Post("/_/links", mockApi.DetailsHandler)
	r.Post("/_/links_batch", mockApi.SubrepoLinksBatchHandler)
	r.Post("/_/shift", mockApi.ShiftHandler)
	r.Get("/_/alerts", mockApi.AlertsHandler)
	r.Get("/_/alert/list/{show}", mockApi.AlertListHandler)
	r.Get("/_/alert/new", mockApi.AlertNewHandler)
	r.Post("/_/alert/update", mockApi.AlertUpdateHandler)
	r.Post("/_/alert/delete/{id:[0-9]+}", mockApi.AlertDeleteHandler)
	r.Post("/_/alert/bug/try", mockApi.AlertBugTryHandler)
	r.Post("/_/alert/notify/try", mockApi.AlertNotifyTryHandler)
	r.Get("/_/subscriptions", mockApi.SubscriptionsHandler)
	r.Post("/_/dryrun/start", mockApi.DryrunStartHandler)
	r.Post("/_/cluster/start", mockApi.ClusterStartHandler)
	r.Post("/_/keys", mockApi.ShortcutUpdateHandler)
	r.Post("/_/shortcut/update", mockApi.ShortcutUpdateHandler)
	r.Post("/_/shortcut/get", mockApi.GetShortcutHandler)
	r.Get("/_/shortcut/get", mockApi.GetShortcutHandler)
	r.Get("/_/anomalies/sheriff_list", mockApi.GetSheriffListHandler)
	r.Get("/_/anomalies/anomaly_list", mockApi.GetAnomaliesHandler)
	r.Post("/_/anomalies/group_report", mockApi.GroupReportHandler)
	r.Post("/_/anomalies/calculate_regr_shortcut", mockApi.CalculateRegrShortcutHandler)
	r.Post("/_/triage/edit_anomalies", mockApi.TriageEditAnomaliesHandler)
	r.Post("/_/triage/associate_alerts", mockApi.TriageAssociateAlertsHandler)
	r.Post("/_/triage/file_bug", mockApi.TriageFileBugHandler)
	r.Post("/_/triage/list_issues", mockApi.TriageListIssuesHandler)
	r.Post("/_/user_issues", mockApi.UserIssuesHandler)
	r.Get("/_/revision", mockApi.RevisionHandler)
	r.Get("/_/favorites", mockApi.FavoritesHandler)
	r.Post("/_/reg", mockApi.RegressionsHandler)
	r.Post("/_/bisect/create", mockApi.BisectCreateHandler)
	r.Post("/_/try", mockApi.TryJobCreateHandler)
	r.Post("/_/fe_telemetry", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func main() {
	fs := flag.NewFlagSet("mock", flag.ExitOnError)
	portFlag := fs.String("port", ":8080", "HTTP port")
	_ = fs.Parse(os.Args[1:])

	envDir := os.Getenv("ENV_DIR")
	envReadyFile := os.Getenv("ENV_READY_FILE")
	testOnEnv := envDir != "" && envReadyFile != ""

	targetPort := *portFlag
	if testOnEnv {
		targetPort = ":0"
	}

	f := &frontend.Frontend{}
	f.InitForMock(http.Dir(pagesDir))

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	mockApi := NewServer()
	RegisterHandlers(r, mockApi, f)

	listener, err := net.Listen("tcp", targetPort)
	if err != nil {
		sklog.Fatalf("Failed to create listener: %s", err)
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port

	if testOnEnv {
		envPortFile := filepath.Join(envDir, envPortFileBaseName)
		if err := os.WriteFile(envPortFile, []byte(strconv.Itoa(actualPort)), 0644); err != nil {
			sklog.Fatalf("Failed to write port file: %s", err)
		}
		if err := os.WriteFile(envReadyFile, []byte{}, 0644); err != nil {
			sklog.Fatalf("Failed to write ready file: %s", err)
		}
	}

	sklog.Fatal(http.Serve(listener, r))
}
