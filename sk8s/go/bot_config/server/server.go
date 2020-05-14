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
	"go.skia.org/infra/sk8s/go/bot_config/machine"
)

const (
	serverReadTimeout  = 5 * time.Minute
	serverWriteTimeout = 5 * time.Minute
)

// Server is the core functionality of bot_config.
type Server struct {
	r       *mux.Router
	machine *machine.Machine

	getStateRequests             metrics2.Counter
	getStateRequestsSuccess      metrics2.Counter
	getSettingsRequests          metrics2.Counter
	getSettingsRequestsSuccess   metrics2.Counter
	getDimensionsRequests        metrics2.Counter
	getDimensionsRequestsSuccess metrics2.Counter
	onBeforeTaskSuccess          metrics2.Counter
	onAfterTaskSuccess           metrics2.Counter
}

// New returns a new instance of Server.
func New(m *machine.Machine) (*Server, error) {
	r := mux.NewRouter()
	ret := &Server{
		r:       r,
		machine: m,

		getStateRequests:             metrics2.GetCounter("bot_config_server_get_state_requests", map[string]string{"machine": m.MachineID}),
		getStateRequestsSuccess:      metrics2.GetCounter("bot_config_server_get_state_requests_success", map[string]string{"machine": m.MachineID}),
		getSettingsRequests:          metrics2.GetCounter("bot_config_server_get_settings_requests", map[string]string{"machine": m.MachineID}),
		getSettingsRequestsSuccess:   metrics2.GetCounter("bot_config_server_get_settings_requests_success", map[string]string{"machine": m.MachineID}),
		getDimensionsRequests:        metrics2.GetCounter("bot_config_server_get_dimensions_requests", map[string]string{"machine": m.MachineID}),
		getDimensionsRequestsSuccess: metrics2.GetCounter("bot_config_server_get_dimensions_requests_success", map[string]string{"machine": m.MachineID}),
		onBeforeTaskSuccess:          metrics2.GetCounter("bot_config_server_on_before_task_requests_success", map[string]string{"machine": m.MachineID}),
		onAfterTaskSuccess:           metrics2.GetCounter("bot_config_server_on_after_task_requests_success", map[string]string{"machine": m.MachineID}),
	}

	r.HandleFunc("/get_state", ret.getState).Methods("POST")
	r.HandleFunc("/get_settings", ret.getSettings).Methods("GET")
	r.HandleFunc("/get_dimensions", ret.getDimensions).Methods("POST")
	r.HandleFunc("/on_before_task", ret.onBeforeTask).Methods("GET")
	r.HandleFunc("/on_after_task", ret.onAfterTask).Methods("GET")
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
// https://chromium.googlesource.com/infra/luci/luci-py.git/+show/master/appengine/swarming/swarming_bot/// config/bot_config.py
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
// https://chromium.googlesource.com/infra/luci/luci-py.git/+show/master/appengine/swarming/swarming_bot/// config/bot_config.py
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
// https://chromium.googlesource.com/infra/luci/luci-py.git/+show/master/appengine/swarming/swarming_bot/config/bot_config.py
func (s *Server) getDimensions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.getDimensionsRequests.Inc(1)

	dim := map[string][]string{}
	if err := json.NewDecoder(r.Body).Decode(&dim); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON input.", http.StatusInternalServerError)
		return
	}
	for key, values := range s.machine.DimensionsForSwarming() {
		dim[key] = values
	}
	if err := json.NewEncoder(w).Encode(dim); err != nil {
		sklog.Errorf("Failed to encode JSON output: %s", err)
		return
	}
	s.getDimensionsRequestsSuccess.Inc(1)
}

// onBeforeTask is called when bot_config.py calls on_before_task.
//
// No other data is passed with this call.
//
// https://chromium.googlesource.com/infra/luci/luci-py.git/+show/master/appengine/swarming/swarming_bot/config/bot_config.py
func (s *Server) onBeforeTask(w http.ResponseWriter, r *http.Request) {
	s.machine.SetIsRunningSwarmingTask(true)
	s.onBeforeTaskSuccess.Inc(1)
}

// onAfterTask is called when bot_config.py calls on_after_task.
//
// No other data is passed with this call.
//
// https://chromium.googlesource.com/infra/luci/luci-py.git/+show/master/appengine/swarming/swarming_bot/config/bot_config.py
func (s *Server) onAfterTask(w http.ResponseWriter, r *http.Request) {
	s.machine.SetIsRunningSwarmingTask(false)
	s.onAfterTaskSuccess.Inc(1)
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
