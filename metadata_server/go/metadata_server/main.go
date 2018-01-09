package main

import (
	"context"
	"flag"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
)

var (
	// APP_NAME is the name of this app.
	APP_NAME = "metadata_server"

	// Flags.
	port      = flag.String("port", ":8000", "HTTP service port for the web server (e.g., ':8000')")
	promPort  = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	tokenFile = flag.String("token_file", "", "Path to a file containing a valid OAuth2 token for the service account.")
)

func main() {
	common.InitWithMust(
		APP_NAME,
		common.PrometheusOpt(promPort),
		//common.CloudLoggingOpt(),
	)

	v, err := skiaversion.GetVersion()
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Version %s, built at %s", v.Commit, v.Date)

	// TODO(borenet): Load these from a file?
	projectMetadata := map[string]string{
		"mykey": "myvalue",
	}
	instanceMetadata := map[string]map[string]string{}

	tok, err := metadata.NewServiceAccountToken(*tokenFile)
	if err != nil {
		sklog.Fatal(err)
	}
	go tok.UpdateLoop(time.Hour, context.Background())

	r := mux.NewRouter()
	metadata.SetupServer(r, projectMetadata, instanceMetadata, tok)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infof("Ready to serve on http://localhost%s", *port)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
