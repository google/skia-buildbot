package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/pinpoint/go/pinpoint"
)

var (
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir = flag.String("resources_dir", "pinpoint/webui/dist/browser", "Directory containing static resources (defaults to relative runfiles path)")
)

func webAppHandler(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, r.URL.Path)
		fi, err := os.Stat(path)
		if os.IsNotExist(err) || fi.IsDir() {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.ServeFile(w, r, path)
	}
}

func main() {
	common.InitWithMust(
		"pinpoint-webui",
		common.PrometheusOpt(promPort),
	)

	ctx := context.Background()
	pinpointClient, err := pinpoint.New(ctx)
	if err != nil {
		sklog.Fatalf("Failed to create pinpoint client: %s", err)
	}
	handler, err := pinpoint.NewGatewayJSONHandler(ctx, pinpointClient)
	if err != nil {
		sklog.Fatalf("Failed to create JSON handler: %s", err)
	}
	http.Handle("/pinpoint/", handler)

	http.HandleFunc("/", webAppHandler(*resourcesDir))

	http.HandleFunc("/healthz", httputils.ReadyHandleFunc)
	server := &http.Server{
		Addr:         *port,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	sklog.Fatal(server.ListenAndServe())
}
