package main

import (
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

// Constants for test_on_env compatibility
const (
	envPortFileBaseName = "port"
)

func (m *MockFrontend) render(templateName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		// NOTE: This configuration is critical for frontend logic (e.g. Pinpoint button visibility).
		// Do not modify the git_repo_url or other core settings.
		mockContext := template.JS(`{
      "instance_url": "https://chrome-perf.corp.goog",
      "instance_name": "chrome-perf-demo",
      "header_image_url": "",
      "commit_range_url": "https://chromium.googlesource.com/chromium/src/+log/{begin}..{end}",
      "key_order": ["arch", "os"],
      "demo": true,
      "radius": 7,
      "num_shift": 10,
      "interesting": 25,
      "step_up_only": false,
      "display_group_by": false,
      "hide_list_of_commits_on_explore": true,
      "notifications": "none",
      "fetch_chrome_perf_anomalies": false,
      "fetch_anomalies_from_sql": false,
      "feedback_url": "",
      "chat_url": "",
      "help_url_override": "",
      "trace_format": "chrome",
      "need_alert_action": false,
      "bug_host_url": "b",
      "git_repo_url": "https://chromium.googlesource.com/chromium/src",
      "keys_for_commit_range": [],
      "keys_for_useful_links": [],
      "skip_commit_detail_display": false,
      "image_tag": "fake-tag",
      "remove_default_stat_value": false,
      "enable_skia_bridge_aggregation": false,
      "show_json_file_display": false,
      "always_show_commit_info": false,
      "show_triage_link": false,
      "show_bisect_btn": true,
      "app_version": "test-version",
      "enable_v2_ui": false,
      "dev_mode": false
    }`)

		err := m.f.GetTemplates().ExecuteTemplate(w, templateName, map[string]interface{}{
			"context":                      mockContext,
			"Nonce":                        secure.CSPNonce(r.Context()),
			"InstanceName":                 "Chrome Perf Demo",
			"GoogleAnalyticsMeasurementID": "G-MOCK-ID",
		})
		if err != nil {
			sklog.Errorf("Render error: %v", err)
			http.Error(w, fmt.Sprintf("Render error: %v", err), 500)
		}
	}
}

/* A primitive server that renders the frontend with mock data.
Unlike per-component demo servers, this server actually renders the real pages
(luckily, our server-side rendering is limited to substitution of several
variables into the templates).
*/

func main() {
	// 1. Parse Flags
	fs := flag.NewFlagSet("mock", flag.ExitOnError)
	portFlag := fs.String("port", ":8080", "HTTP port")
	_ = fs.Parse(os.Args[1:])

	// 2. Detect test_on_env environment variables
	// These are set by the test_on_env Bazel rule or runner script.
	envDir := os.Getenv("ENV_DIR")
	envReadyFile := os.Getenv("ENV_READY_FILE")
	testOnEnv := envDir != "" && envReadyFile != ""

	// Determine the port to listen on.
	// If running inside test_on_env, we use ":0" to let the OS choose a free port
	// to avoid collisions during parallel tests.
	targetPort := *portFlag
	if testOnEnv {
		targetPort = ":0"
	}

	// 3. Directory Config
	const pagesDir = "perf/pages/production"
	const imagesDir = "perf/images"

	// 4. Init Frontend in mock mode
	f := &frontend.Frontend{}
	f.InitForMock(http.Dir(pagesDir))

	m := &MockFrontend{f: f}
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.StripSlashes)

	// --- Browser/SourceMap Support ---
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, ".js.map") {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
			next.ServeHTTP(w, r)
		})
	})

	// --- Static Assets ---
	r.Handle("/dist/images/*", http.StripPrefix("/dist/images", http.FileServer(http.Dir(imagesDir))))
	r.Handle("/dist/*", http.StripPrefix("/dist", http.FileServer(http.Dir(pagesDir))))

	// --- Page Routes ---
	routes := map[string]string{
		"/e": "newindex.html", "/m": "multiexplore.html", "/c": "clusters2.html",
		"/pg": "playground.html", "/t": "triage.html", "/d": "dryrunalert.html",
		"/r": "trybot.html", "/f": "favorites.html", "/v": "revisions.html",
		"/u": "report.html", "/help": "help.html", "/a": "regressions.html",
		"/admin/alerts": "alerts.html", "/l": "extralinks.html",
	}
	for route, tmpl := range routes {
		r.HandleFunc(route, m.render(tmpl))
	}

	// --- API Mocks ---
	r.Route("/_/", func(r chi.Router) {
		// Core
		r.Get("/login/status", m.loginStatusHandler)
		r.Get("/defaults", m.defaultsHandler)
		r.Get("/initpage/{id}", m.initPageHandler)
		r.Get("/status/{id}", m.statusHandler)

		// Query Builder (Stateless)
		r.Post("/count", m.countHandler)
		r.Post("/nextParamList", m.nextParamListHandler)

		// Graph & Frame
		r.Post("/frame/start", m.frameStartHandler)
		r.Post("/cid", m.cidHandler)
		r.Post("/details", m.detailsHandler)
		r.Post("/cidRange", m.cidRangeHandler)
		r.Post("/shift", m.shiftHandler)
		r.Post("/links", m.linksHandler)

		// Alerts
		r.Route("/alert", func(r chi.Router) {
			r.Get("/list/{show}", m.alertListHandler)
			r.Get("/new", m.alertNewHandler)
			r.Post("/update", m.alertUpdateHandler)
			r.Post("/delete/{id:[0-9]+}", m.alertDeleteHandler)
			r.Post("/bug/try", m.alertBugTryHandler)
			r.Post("/notify/try", m.alertNotifyTryHandler)
		})
		r.Get("/subscriptions", m.subscriptionsHandler)
		r.Post("/dryrun/start", m.dryrunStartHandler)

		// Anomalies
		r.Route("/anomalies", func(r chi.Router) {
			r.Get("/sheriff_list", m.sheriffListHandler)
			r.Get("/anomaly_list", m.anomalyListHandler)
			r.Post("/group_report", m.anomaliesGroupReportHandler)
		})

		// Regressions
		r.Get("/alerts", m.regressionsAlertsHandler)
		r.Post("/reg", m.regressionsListHandler)
		r.Get("/regressions", m.regressionsListHandler)
		r.Post("/cluster/start", m.clusterStartHandler)

		// Triage
		r.Route("/triage", func(r chi.Router) {
			r.Post("/file_bug", m.triageFileBugHandler)
			r.Post("/edit_anomalies", m.triageEditAnomaliesHandler)
			r.Post("/associate_alerts", m.triageAssociateAlertsHandler)
			r.Post("/list_issues", m.triageListIssuesHandler)
		})

		// User Issues
		r.Post("/user_issues", m.userIssuesHandler)
		r.Post("/user_issue/save", m.userIssueSaveHandler)
		r.Post("/user_issue/delete", m.userIssueDeleteHandler)

		// Favorites
		r.Get("/favorites", m.favoritesListHandler)
		r.Post("/favorites/new", m.favoritesNewHandler)

		// Shortcuts
		r.Post("/keys", m.shortcutKeysHandler)
		r.Post("/shortcut/get", m.shortcutHandler)
		r.Post("/shortcut/update", m.shortcutUpdateHandler)

		// Pinpoint
		r.Post("/bisect/create", m.bisectCreateHandler)

		// Telemetry
		r.Post("/fe_telemetry", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	})

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/e", http.StatusFound)
	})

	// 5. Start Listening
	// We use net.Listen to get the actual port before starting the HTTP server
	// (Crucial for test_on_env/Puppeteer support)
	listener, err := net.Listen("tcp", targetPort)
	if err != nil {
		sklog.Fatalf("Failed to create listener: %s", err)
	}

	// Retrieve the actual port (useful if :0 was used)
	actualPort := listener.Addr().(*net.TCPAddr).Port

	// 6. Signal Readiness (for test_on_env)
	if testOnEnv {
		// First, write the TCP port number so the test target knows where to connect.
		envPortFile := filepath.Join(envDir, envPortFileBaseName)
		err = os.WriteFile(envPortFile, []byte(strconv.Itoa(actualPort)), 0644)
		if err != nil {
			sklog.Fatalf("Failed to write port file: %s", err)
		}

		// Then, write the ready file to signal that the server is up.
		err = os.WriteFile(envReadyFile, []byte{}, 0644)
		if err != nil {
			sklog.Fatalf("Failed to write ready file: %s", err)
		}
		fmt.Printf("\n🚀 Test environment signals written. Port: %d", actualPort)
	}

	fmt.Printf("\n✅ Perf Mock Server Running")
	fmt.Printf("\n🔗 http://localhost:%d/e\n\n", actualPort)

	// 7. Serve HTTP
	sklog.Fatal(http.Serve(listener, r))
}
