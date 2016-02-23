/*
	Central message passing app for Skia Infra.
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/geventbus"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/grandcentral/go/event"
)

// flags
var (
	host        = flag.String("host", "localhost", "HTTP service host")
	port        = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
	useMetadata = flag.Bool("use_metadata", true, "Load sensitive values from metadata not from flags.")
	testing     = flag.Bool("testing", false, "Set to true for locally testing rules. No email will be sent.")
	workdir     = flag.String("workdir", ".", "Directory to use for scratch work.")
	nsqdAddress = flag.String("nsqd", "", "Address and port of nsqd instance.")

	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
)

var eventBus *eventbus.EventBus

func mainHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write([]byte(`
<html>
<body>
Hello World
</body>
</html>
`)); err != nil {
		glog.Error(err)
	}
}

func googleStorageChangeHandler(w http.ResponseWriter, r *http.Request) {
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to read response body: %v", err))
		return
	}

	defer util.Close(r.Body)
	var data event.GoogleStorageEventData
	if err := json.Unmarshal(buf, &data); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to decode response body: %v", err))
		return
	}

	// Log the result and fire an event.
	glog.Infof("Google Storage notification from bucket \"%s\": %s", data.Bucket, data.Name)
	eventBus.Publish(event.GLOBAL_GOOGLE_STORAGE, data)
}

func buildbotEventHandler(w http.ResponseWriter, r *http.Request) {
	defer util.Close(r.Body)
	data := r.PostFormValue("packets")
	var events []*event.BuildbotEventData
	if err := json.Unmarshal([]byte(data), &events); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to decode request body: %s", err))
		return
	}

	// Log the event and fire it.
	for _, e := range events {
		eventBus.Publish(event.GLOBAL_BUILDBOT, e)
	}
}

func runServer(serverURL string) {
	r := mux.NewRouter()
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/googlestorage", googleStorageChangeHandler).Methods("POST")
	r.HandleFunc("/buildbot", buildbotEventHandler).Methods("POST")
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	glog.Infof("Ready to serve on %s", serverURL)
	glog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	defer common.LogPanic()
	common.InitWithMetrics2("grandcentral", influxHost, influxUser, influxPassword, influxDatabase, testing)
	v, err := skiaversion.GetVersion()
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Version %s, built at %s", v.Commit, v.Date)

	if *nsqdAddress == "" {
		glog.Fatal("Missing address of nsqd server.")
	}
	globalEventBus, err := geventbus.NewNSQEventBus(*nsqdAddress)
	if err != nil {
		glog.Fatalf("Unable to connect to NSQ server: %s", err)
	}
	eventBus = eventbus.New(globalEventBus)

	// Add a subscription for the each event type. This prevents the messages
	// to queue up if there are no other clients connected.
	eventBus.SubscribeAsync(event.GLOBAL_GOOGLE_STORAGE, func(evData interface{}) {})
	eventBus.SubscribeAsync(event.GLOBAL_BUILDBOT, func(evData interface{}) {})

	if *testing {
		*useMetadata = false
	}
	serverURL := "https://" + *host
	if *testing {
		serverURL = "http://" + *host + *port
	}
	runServer(serverURL)
}
