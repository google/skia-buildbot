package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2/google"
)

var (
	port     = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

const indexHTML = `
<!DOCTYPE html>
<html>
<body>
    <button id="test-job-btn">Test Job</button>
    <div id="jobs-container">Loading jobs...</div>
    <script>
        document.getElementById('test-job-btn').addEventListener('click', async () => {
            try {
                const response = await fetch('/api/testjob', { method: 'POST' });
                alert(await response.text());
            } catch (error) {
                alert('Error: ' + error);
            }
        });

        var jobsContainer = document.getElementById('jobs-container');
        fetch('/api/jobs')
            .then(response => response.text())
            .then(text => {
                jobsContainer.textContent = text;
            })
            .catch(err => {
                jobsContainer.textContent = 'Failed to load jobs: ' + err;
            });
    </script>
</body>
</html>
`

func main() {
	common.InitWithMust(
		"pinpoint-webui",
		common.PrometheusOpt(promPort),
	)

	ctx := context.Background()
	tokenSource, tokenSourceErr := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	var client *http.Client
	if tokenSourceErr != nil {
		sklog.Errorf("Failed to create token source: %s", tokenSourceErr)
	} else {
		client = httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()
	}

	http.HandleFunc("/healthz", httputils.ReadyHandleFunc)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte(indexHTML)); err != nil {
			http.Error(w, fmt.Sprintf("Failed to write response: %s", err), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/api/jobs", func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			http.Error(w, fmt.Sprintf("Service not fully initialized: %s", tokenSourceErr), http.StatusInternalServerError)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
		defer cancel()

		resp, err := httputils.GetWithContext(ctx, client, "https://pinpoint-dot-chromeperf.appspot.com/api/jobs")
		if err != nil {
			http.Error(w, "Failed to fetch jobs", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		if _, err := io.Copy(w, resp.Body); err != nil {
			http.Error(w, fmt.Sprintf("Failed to write response: %s", err), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/api/testjob", func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			http.Error(w, fmt.Sprintf("Service not fully initialized: %s", tokenSourceErr), http.StatusInternalServerError)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
		defer cancel()

		params := url.Values{}
		params.Set("comparison_mode", "try")
		params.Set("benchmark", "speedometer3.1.crossbench")
		params.Set("configuration", "win-11-perf")
		params.Set("story", "default")
		params.Set("base_git_hash", "-HEAD")
		params.Set("end_git_hash", "-HEAD")
		testJobUrl := fmt.Sprintf("%s?%s", "https://pinpoint-dot-chromeperf.appspot.com/api/new", params.Encode())

		resp, err := httputils.PostWithContext(ctx, client, testJobUrl, "application/json", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusMethodNotAllowed)
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			http.Error(w, fmt.Sprintf("Failed to write response: %s", err), http.StatusInternalServerError)
		}
	})

	sklog.Fatal(http.ListenAndServe(*port, nil))
}
