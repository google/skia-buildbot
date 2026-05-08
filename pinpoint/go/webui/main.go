package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2/google"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/alogin/proxylogin"
	"go.skia.org/infra/kube/go/authproxy"
	"go.skia.org/infra/pinpoint/go/pinpoint"
)

var (
	port     = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

const (
	pinpointBaseURL = "https://pinpoint-dot-chromeperf.appspot.com"
	pinpointNewURL  = pinpointBaseURL + "/api/new"
)

const indexHTML = `
<!DOCTYPE html>
<html>
<body>
		<div>
				Welcome, {{.Email}}
		</div>
		<button id="test-job-btn">Test Job</button>
    <pre id="jobs-container">Loading jobs...</pre>
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
        fetch('/pinpoint/v1/jobs')
            .then(response => response.json())
            .then(json => {
                jobsContainer.textContent = JSON.stringify(json, null, 2);
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

	loginProvider, err := proxylogin.New(authproxy.WebAuthHeaderName, "")
	if err != nil {
		sklog.Fatalf("Failed to initialize login provider: %s", err)
	}

	tmpl, err := template.New("index").Parse(indexHTML)
	if err != nil {
		sklog.Fatalf("Failed to parse template: %s", err)
	}

	ctx := context.Background()
	tokenSource, tokenSourceErr := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	var client *http.Client
	if tokenSourceErr != nil {
		sklog.Errorf("Failed to create token source: %s", tokenSourceErr)
	} else {
		client = httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()
	}

	pinpointClient, err := pinpoint.New(ctx)
	if err != nil {
		sklog.Fatalf("Failed to create pinpoint client: %s", err)
	}
	handler, err := pinpoint.NewGatewayJSONHandler(ctx, pinpointClient)
	if err != nil {
		sklog.Fatalf("Failed to create JSON handler: %s", err)
	}
	http.Handle("/pinpoint/", handler)

	http.HandleFunc("/healthz", httputils.ReadyHandleFunc)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		email := loginProvider.LoggedInAs(r)
		if err := tmpl.Execute(w, struct{ Email string }{Email: string(email)}); err != nil {
			http.Error(
				w,
				fmt.Sprintf("Failed to expand template: %s", err),
				http.StatusInternalServerError,
			)
		}
	})

	http.HandleFunc("/api/testjob", func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			http.Error(
				w,
				fmt.Sprintf("Service not fully initialized: %s", tokenSourceErr),
				http.StatusInternalServerError,
			)
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

		email := loginProvider.LoggedInAs(r)
		if email != "" {
			params.Set("user", string(email))
		}
		testJobUrl := fmt.Sprintf("%s?%s", pinpointNewURL, params.Encode())

		resp, err := httputils.PostWithContext(ctx, client, testJobUrl, "application/json", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusMethodNotAllowed)
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			http.Error(
				w,
				fmt.Sprintf("Failed to write response: %s", err),
				http.StatusInternalServerError,
			)
		}
	})

	server := &http.Server{
		Addr:         *port,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	sklog.Fatal(server.ListenAndServe())
}
