package main

import (
	"flag"
	"net/http"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	port               = flag.String("port", ":9210", "HTTP service address (e.g., ':8000')")
	local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promPort           = flag.String("prom_port", ":20004", "Metrics service address (e.g., ':10110')")
	serviceAccountPath = flag.String("service_account_path", "", "Path to the service account.  Can be empty string to use defaults or project metadata")
	powerServer        = flag.String("power_server", "https://power.skia.org", "")
)

var authedClient *http.Client

func powercycleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Only Post Supported", http.StatusMethodNotAllowed)
		return
	}
	if authedClient != nil {
		url := *powerServer + "/powercycled_bots"
		sklog.Infof("Posting to %s", url)
		defer util.Close(r.Body)
		resp, err := authedClient.Post(url, "application/json", r.Body)
		sklog.Infof("Error: %v", err)
		sklog.Infof("Response: %v", httputils.ReadAndClose(resp.Body))
		w.WriteHeader(http.StatusAccepted)
	} else {
		http.Error(w, "Does not have authenticated client", http.StatusServiceUnavailable)
	}
}

func main() {
	defer common.LogPanic()
	flag.Parse()
	if *local {
		common.InitWithMust(
			"powercycle-daemon",
			common.PrometheusOpt(promPort),
		)
	} else {
		common.InitWithMust(
			"powercycle-daemon",
			common.PrometheusOpt(promPort),
			common.CloudLoggingJWTOpt(serviceAccountPath),
		)
	}

	var err error
	authedClient, err = auth.NewJWTServiceAccountClient("", *serviceAccountPath, &http.Transport{Dial: httputils.DialTimeout}, auth.SCOPE_USERINFO_EMAIL)
	if err != nil {
		sklog.Fatalf("Failed to create authenticated HTTP client: %s", err)
	}
	sklog.Info("Got authenticated client.")

	r := mux.NewRouter()
	r.HandleFunc("/reportPowercycled", powercycleHandler).Methods("POST")

	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
