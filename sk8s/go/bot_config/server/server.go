// Package server is the core functionality of bot_config.
package server

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/sk8s/go/bot_config/adb"
)

const (
	serverReadTimeout  = 5 * time.Minute
	serverWriteTimeout = 5 * time.Minute
)

// Server is the core functionality of bot_config.
type Server struct {
	r *mux.Router
	a adb.Adb

	getStateRequests             metrics2.Counter
	getStateRequestsSuccess      metrics2.Counter
	getSettingsRequests          metrics2.Counter
	getSettingsRequestsSuccess   metrics2.Counter
	getDimensionsRequests        metrics2.Counter
	getDimensionsRequestsSuccess metrics2.Counter
}

// New returns a new instance of Server.
func New() (*Server, error) {
	r := mux.NewRouter()
	ret := &Server{
		r: r,
		a: adb.New(),

		getStateRequests:             metrics2.GetCounter("bot_config_server_get_state_requests"),
		getStateRequestsSuccess:      metrics2.GetCounter("bot_config_server_get_state_requests_success"),
		getSettingsRequests:          metrics2.GetCounter("bot_config_server_get_settings_requests"),
		getSettingsRequestsSuccess:   metrics2.GetCounter("bot_config_server_get_settings_requests_success"),
		getDimensionsRequests:        metrics2.GetCounter("bot_config_server_get_dimensions_requests"),
		getDimensionsRequestsSuccess: metrics2.GetCounter("bot_config_server_get_dimensions_requests_success"),
	}

	r.HandleFunc("/get_state", ret.getState).Methods("POST")
	r.HandleFunc("/get_settings", ret.getSettings).Methods("GET")
	r.HandleFunc("/get_dimensions", ret.getDimensions).Methods("POST")
	r.Use(
		httputils.HealthzAndHTTPS,
		httputils.LoggingGzipRequestResponse,
	)

	return ret, nil
}

// getState implements get_state in bot_config.py.
//
// The input is a JSON dictionary via POST that is returned from os_utilities.get_state(), and will
// emit an updated JSON dictionary on return.
//
// https://chromium.googlesource.com/infra/luci/luci-py.git/+/master/appengine/swarming/swarming_bot/// config/bot_config.py
func (s *Server) getState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.getStateRequests.Inc(1)

	dict := map[string]interface{}{}
	if err := json.NewDecoder(r.Body).Decode(&dict); err != nil {
		httputils.ReportError(w, err, "Failed to decode settings", http.StatusInternalServerError)
		return
	}

	// TODO(jcgregorio) Hook this up to Machine State server.
	delete(dict, "quarantined")

	// TODO(jcgregorio) Also gather/report device temp to Machine State.

	dict["sk_rack"] = os.Getenv("MY_RACK_NAME")
	if err := json.NewEncoder(w).Encode(dict); err != nil {
		sklog.Errorf("Failed to encode state: %s", err)
		return
	}
	s.getStateRequestsSuccess.Inc(1)
}

type isolated struct {
	Size int64 `json:"size"`
}

type caches struct {
	Isolated isolated `json:"isolated"`
}

type settings struct {
	Caches caches `json:"caches"`
}

// getSettings implements get_settings for bot_config.py
//
// Will emit a JSON dictionary on GET with the settings.
//
// https://chromium.googlesource.com/infra/luci/luci-py.git/+/master/appengine/swarming/swarming_bot/// config/bot_config.py
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.getSettingsRequests.Inc(1)
	dict := settings{
		caches{
			isolated{
				Size: 8 * 1024 * 1024 * 1024,
			},
		},
	}
	if err := json.NewEncoder(w).Encode(dict); err != nil {
		sklog.Errorf("Failed to encode settings: %s", err)
		return
	}
	s.getSettingsRequestsSuccess.Inc(1)
}

// getDimensions implements get_dimensions in bot_config.py.
//
// The input is a JSON dictionary via POST  that is returned from
// os_utilities.get_dimensions(). This command will emit an updated JSON
// dictionary in the response.
//
// https://chromium.googlesource.com/infra/luci/luci-py.git/+/master/appengine/swarming/swarming_bot/config/bot_config.py
func (s *Server) getDimensions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.getDimensionsRequests.Inc(1)

	dim := map[string][]string{}
	if err := json.NewDecoder(r.Body).Decode(&dim); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON input.", http.StatusInternalServerError)
		return
	}
	dim["zone"] = []string{"us", "us-skolo", "us-skolo-1"} // TODO(jcgregorio) Add rack number in here?
	dim["inside_docker"] = []string{"1", "containerd"}

	var err error
	dim, err = s.a.DimensionsFromProperties(r.Context(), dim)
	if err != nil {
		// Output isn't going into a browser so send the full err text across.
		httputils.ReportError(w, err, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(dim); err != nil {
		sklog.Errorf("Failed to encode JSON output: %s", err)
		return
	}
	s.getDimensionsRequestsSuccess.Inc(1)
}

// Start the http server. This function never returns.
func (s *Server) Start(port string) error {
	// Start serving.
	sklog.Info("Ready to serve.")
	server := &http.Server{
		Addr:           port,
		Handler:        s.r,
		ReadTimeout:    serverReadTimeout,
		WriteTimeout:   serverWriteTimeout,
		MaxHeaderBytes: 1 << 20,
	}
	return server.ListenAndServe()
}
