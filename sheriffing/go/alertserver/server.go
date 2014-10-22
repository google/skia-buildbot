/*
	Provides roll-up statuses and alerting for Skia build/test/perf.
*/

package main

import (
	"encoding/json"
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
	host              = flag.String("host", "localhost", "HTTP service host")
	port              = flag.String("port", "8000", "HTTP service port (e.g., '8000')")
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

func alertJsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	type displayAlert struct {
		Name         string `json:"name"`
		Query        string `json:"query"`
		Condition    string `json:"condition"`
		Active       bool   `json:"active"`
		Snoozed      bool   `json:"snoozed"`
		Triggered    int64  `json:"triggered"`
		SnoozedUntil int64  `json:"snoozed_until"`
	}
	alerts := struct {
		Alerts []displayAlert `json:"alerts"`
	}{
		Alerts: []displayAlert{},
	}
	for _, a := range alertManager.Alerts() {
		alerts.Alerts = append(alerts.Alerts, displayAlert{
			Name:         a.Name,
			Query:        a.Query,
			Condition:    a.Condition,
			Active:       a.Active(),
			Snoozed:      a.Snoozed(),
			Triggered:    a.Triggered().Unix(),
			SnoozedUntil: a.SnoozedUntil().Unix(),
		})
	}
	bytes, err := json.Marshal(&alerts)
	if err != nil {
		glog.Error(err)
	}
	w.Write(bytes)
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
	http.HandleFunc("/json/alerts", alertJsonHandler)
	serverUrl := *host + ":" + *port
	glog.Infof("Ready to serve on http://%s", serverUrl)
	glog.Fatal(http.ListenAndServe(serverUrl, nil))
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
