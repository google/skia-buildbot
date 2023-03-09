// This program serves a web application that displays screnshots captured by Puppeteer tests. It
// is meant to be used exclusively as a local development tool.
//
// Usage:
//
//	$ bazel run --config=mayberemote //:puppeteer_screenshot_server
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/puppeteer-tests/bazel/extract_puppeteer_screenshots/extract"
	"go.skia.org/infra/puppeteer-tests/bazel/puppeteer_screenshot_server/rpc_types"
)

func main() {
	var port = flag.Int("port", 8000, "TCP port to serve the web interface.")
	flag.Parse()

	// Get the path to the repository root (and ensure we are running under Bazel).
	workspaceDir := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if workspaceDir == "" {
		sklog.Fatal("The BUILD_WORKSPACE_DIRECTORY environment variable is not set. Are you running this program via Bazel?")
	}

	// Get working directory.
	workDir, err := os.Getwd()
	if err != nil {
		sklog.Fatalf("Could not get the working directory: %s", err)
	}

	// Create temporary directory where we will extract Puppeteer screenshots.
	screenshotsDir, err := os.MkdirTemp("/tmp", "puppeteer-screenshot-server-*")
	if err != nil {
		sklog.Fatalf("Could not create temporary directory: %s", err)
	}
	defer func() {
		if err := os.RemoveAll(screenshotsDir); err != nil {
			sklog.Fatalf("Cloud not delete temporary directory: %s", err)
		}
	}()
	fmt.Printf("Screenshots will be extracted at: %s\n", screenshotsDir)

	// Compute static assets dir (assumes the program is running under Bazel).
	staticAssetsDir := filepath.Join(workDir, "puppeteer-tests", "pages", "development")
	if _, err := os.Stat(staticAssetsDir); os.IsNotExist(err) {
		sklog.Fatalf("Stating directory with static assets: %s", err)
	}

	// Serve the UI.
	if err := serve(*port, workspaceDir, staticAssetsDir, screenshotsDir); err != nil {
		sklog.Fatal("HTTP server error: %s", err)
	}
}

func serve(port int, workspaceDir, staticAssetsDir, screenshotsDir string) error {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(staticAssetsDir, "index.html"))
	})
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticAssetsDir))))
	http.Handle("/screenshots/", http.StripPrefix("/screenshots/", http.FileServer(http.Dir(screenshotsDir))))

	http.HandleFunc("/rpc/get-screenshots", func(w http.ResponseWriter, r *http.Request) {
		handleGetScreenshotsRPC(w, r, workspaceDir, screenshotsDir)
	})

	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatalf("Could not resolve the hostname: %s", err)
	}
	fmt.Printf("Serving Puppeteer screenshots viewer at: http://%s:%d\n", hostname, port)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

func handleGetScreenshotsRPC(w http.ResponseWriter, r *http.Request, workspaceDir, screenshotsDir string) {
	// Extract screenshots.
	if err := extract.Extract(workspaceDir, screenshotsDir); err != nil {
		httputils.ReportError(w, err, "Could not extract screenshots.", http.StatusInternalServerError)
		return
	}

	// Scan screenshots directory and build RPC response.
	entries, err := os.ReadDir(screenshotsDir)
	if err != nil {
		httputils.ReportError(w, err, "Error scanning screenshots directory.", http.StatusInternalServerError)
		return
	}
	response := rpc_types.GetScreenshotsRPCResponse{
		ScreenshotsByApplication: map[string][]rpc_types.Screenshot{},
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".png") {
			sklog.Infof("Ignoring non-PNG file in screenshots directory: %s", entry.Name())
			continue
		}

		// Determine the application and test name for the current screenshot.
		//
		// If a screenshot does not follow the <app-name>_<test-name>.png naming pattern, we will give
		// it a special application name, and use the entire file name (minus the extension) as the
		// test name.
		appAndTestName := strings.SplitN(strings.TrimSuffix(entry.Name(), ".png"), "_", 2)
		app := "(unknown application)"
		testName := appAndTestName[0]
		if len(appAndTestName) == 2 {
			app = appAndTestName[0]
			testName = appAndTestName[1]
		}

		// Add screenshot to RPC response.
		response.ScreenshotsByApplication[app] = append(response.ScreenshotsByApplication[app], rpc_types.Screenshot{
			TestName: testName,
			URL:      fmt.Sprintf("/screenshots/%s", entry.Name()),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		httputils.ReportError(w, err, "Failed to encode JSON response.", http.StatusInternalServerError)
		return
	}
}
