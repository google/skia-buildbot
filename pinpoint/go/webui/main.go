package main

import (
	"flag"
	"fmt"
	"net/http"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
)

var (
	port     = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

func serveHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	if _, err := fmt.Fprintln(w, "Hello Pinpoint"); err != nil {
		sklog.Errorf("Failed writing response: %s", err)
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
