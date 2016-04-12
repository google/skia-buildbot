// fiddle is the web server for fiddle.
package main

import (
	"flag"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/influxdb"
)

// flags
var (
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")

	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
)

// mainHandler handles the GET of the main page.
func mainHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte("Hello world!")); err != nil {
			glog.Errorf("Failed to write: %s", err)
		}
	}
}

func main() {
	defer common.LogPanic()
	common.InitWithMetrics2("push", influxHost, influxUser, influxPassword, influxDatabase, local)
	r := mux.NewRouter()
	r.HandleFunc("/", mainHandler)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	glog.Infoln("Ready to serve.")
	glog.Fatal(http.ListenAndServe(*port, nil))
}
