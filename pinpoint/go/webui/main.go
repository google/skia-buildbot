package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

var (
	port     = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

func serveHTTP(w http.ResponseWriter, r *http.Request) {
	client := httputils.NewTimeoutClient()
	resp, err := client.Get("https://pinpoint-dot-chromeperf.appspot.com/api/jobs")
	if err != nil {
		http.Error(w, "Failed to fetch jobs", http.StatusInternalServerError)
		sklog.Errorf("Failed fetching jobs: %s", err)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	if _, err := io.Copy(w, resp.Body); err != nil {
		http.Error(w, fmt.Sprintf("Failed writing response: %s", err), http.StatusInternalServerError)
	}
}

func main() {
	common.InitWithMust(
		"pinpoint-webui",
		common.PrometheusOpt(promPort),
	)

	http.Handle("/", http.HandlerFunc(serveHTTP))
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
