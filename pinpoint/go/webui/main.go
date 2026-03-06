package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
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
			http.Error(w, "Failed to write response", http.StatusInternalServerError)
		}
	})
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
