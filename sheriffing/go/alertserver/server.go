/*
	Provides roll-up statuses and alerting for Skia build/test/perf.
*/

package main

import (
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

import (
	"github.com/golang/glog"
	"github.com/influxdb/influxdb/client"
)

import (
	"skia.googlesource.com/buildbot.git/go/metadata"
	"skia.googlesource.com/buildbot.git/sheriffing/go/alerting"
)

const (
	INFLUXDB_NAME_METADATA_KEY     = "influxdb_name"
	INFLUXDB_PASSWORD_METADATA_KEY = "influxdb_password"
)

var (
	alertManager   *alerting.AlertManager = nil
	alertsTemplate *template.Template     = nil
	indexTemplate  *template.Template     = nil
)

// flags
var (
	port              = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	useMetadata       = flag.Bool("use_metadata", true, "Load sensitive values from metadata not from flags.")
	influxDbHost      = flag.String("influxdb_host", "localhost:8086", "The InfluxDB hostname.")
	influxDbName      = flag.String("influxdb_name", "root", "The InfluxDB username.")
	influxDbPassword  = flag.String("influxdb_password", "root", "The InfluxDB password.")
	influxDbDatabase  = flag.String("influxdb_database", "", "The InfluxDB database.")
	alertPollInterval = flag.String("alert_poll_interval", "1s", "How often to check for new alerts.")
)

func indexHandler(w http.ResponseWriter, r *http.Request) {
	indexPage := struct {
		Alerts []*alerting.Alert
	}{
		Alerts: alertManager.Alerts(),
	}
	w.Header().Set("Content-Type", "text/html")
	if err := indexTemplate.Execute(w, indexPage); err != nil {
		glog.Error(err)
	}
}

func alertHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	alertsPage := struct {
		Alerts []*alerting.Alert
	}{
		Alerts: alertManager.Alerts(),
	}
	if err := alertsTemplate.Execute(w, alertsPage); err != nil {
		glog.Error(err)
	}
}

func runServer() {
	_, filename, _, _ := runtime.Caller(0)
	cwd := filepath.Join(filepath.Dir(filename), "../..")
	if err := os.Chdir(cwd); err != nil {
		glog.Fatal(err)
	}
	indexTemplate = template.Must(template.ParseFiles(filepath.Join(cwd, "templates/index.html")))
	alertsTemplate = template.Must(template.ParseFiles(filepath.Join(cwd, "templates/alerts.html")))

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/alerts", alertHandler)
	glog.Info("Ready to serve.")
	glog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	flag.Parse()
	defer glog.Flush()
	parsedPollInterval, err := time.ParseDuration(*alertPollInterval)
	if err != nil {
		glog.Fatal(fmt.Sprintf("Failed to parse -alertPollInterval: %s", *alertPollInterval))
	}
	if *useMetadata {
		*influxDbName = metadata.MustGet(INFLUXDB_NAME_METADATA_KEY)
		*influxDbPassword = metadata.MustGet(INFLUXDB_PASSWORD_METADATA_KEY)
	}
	dbClient, err := client.New(&client.ClientConfig{*influxDbHost, *influxDbName, *influxDbPassword, *influxDbDatabase, nil, false, false})
	if err != nil {
		glog.Fatal(fmt.Sprintf("Failed to initialize InfluxDB client: %s", err))
	}
	alertManager, err = alerting.NewAlertManager(dbClient, "alerts.cfg", parsedPollInterval)
	if err != nil {
		glog.Fatal(fmt.Sprintf("Failed to create AlertManager: %v", err))
	}
	runServer()
}
