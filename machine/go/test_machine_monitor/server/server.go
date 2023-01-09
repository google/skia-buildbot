// Package server is the core functionality of test_machine_monitor.
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
	"go.skia.org/infra/machine/go/test_machine_monitor/machine"
)

const (
	serverReadTimeout  = 5 * time.Minute
	serverWriteTimeout = 5 * time.Minute
	OnBeforeTaskPath   = "/on_before_task"
	OnAfterTaskPath    = "/on_after_task"
)

// Server is the core functionality of test_machine_monitor.
type Server struct {
	r                      *mux.Router
	machine                *machine.Machine
	triggerInterrogationCh chan<- bool

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
func New(m *machine.Machine, triggerInterrogationCh chan<- bool) (*Server, error) {
	r := mux.NewRouter()
	ret := &Server{
		r:                      r,
		machine:                m,
		triggerInterrogationCh: triggerInterrogationCh,

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
	r.HandleFunc(OnBeforeTaskPath, ret.onBeforeTask).Methods("GET")
	r.HandleFunc(OnAfterTaskPath, ret.onAfterTask).Methods("GET")
	r.Use(
		httputils.HealthzAndHTTPS,
		httputils.LoggingGzipRequestResponse,
	)

	return ret, nil
}

// getState implements get_state in botconfig.py.
//
// The input is a JSON dictionary via POST that is returned from os_utilities.get_state(), and will
// emit an updated JSON dictionary on return.
//
// https://chromium.googlesource.com/infra/luci/luci-py.git/+/refs/heads/main/appengine/swarming/swarming_bot/config/bot_config.py
func (s *Server) getState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.getStateRequests.Inc(1)

	dict := map[string]interface{}{}
	if err := json.NewDecoder(r.Body).Decode(&dict); err != nil {
		httputils.ReportError(w, err, "Failed to decode settings", http.StatusInternalServerError)
		return
	}

	// Swarming doesn't get to decide a machine's maintenance or quarantined
	// state. Always remove them since the presence of the key is the only thing
	// that matters, the value is ignored.
	delete(dict, "quarantined")
	delete(dict, "maintenance")

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

// getSettings implements get_settings for botconfig.py
//
// Will emit a JSON dictionary on GET with the settings.
//
// https://chromium.googlesource.com/infra/luci/luci-py.git/+/refs/heads/main/appengine/swarming/swarming_bot/config/bot_config.py
func (s *Server) getSettings(w http.ResponseWriter, _ *http.Request) {
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
// The input is a JSON dictionary via POST that is returned from
// os_utilities.get_dimensions(). This command will emit an updated JSON
// dictionary in the response.
//
// https://chromium.googlesource.com/infra/luci/luci-py.git/+show/master/appengine/swarming/swarming_bot/config/botconfig.py
func (s *Server) getDimensions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.getDimensionsRequests.Inc(1)

	dim := map[string][]string{}
	if err := json.NewDecoder(r.Body).Decode(&dim); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON input.", http.StatusInternalServerError)
		return
	}
	for key, values := range s.machine.DimensionsForSwarming() {
		if values != nil {
			dim[key] = values
		} else {
			sklog.Errorf("Found bad value: %v for key: %q", values, key)
		}
	}
	if err := json.NewEncoder(w).Encode(dim); err != nil {
		sklog.Errorf("Failed to encode JSON output: %s", err)
		return
	}
	s.getDimensionsRequestsSuccess.Inc(1)
}

// onBeforeTask implements the on_before_task hook in bot_config.py.
//
// No other data is passed with this call.
//
// https://chromium.googlesource.com/infra/luci/luci-py.git/+/refs/heads/main/appengine/swarming/swarming_bot/config/bot_config.py
func (s *Server) onBeforeTask(http.ResponseWriter, *http.Request) {
	s.machine.SetIsRunningSwarmingTask(true)
	s.onBeforeTaskSuccess.Inc(1)
}

// onAfterTask implements the on_after_task hook in bot_config.py.
//
// No other data is passed with this call.
//
// https://chromium.googlesource.com/infra/luci/luci-py.git/+/refs/heads/main/appengine/swarming/swarming_bot/config/bot_config.py
func (s *Server) onAfterTask(_ http.ResponseWriter, r *http.Request) {
	s.machine.SetIsRunningSwarmingTask(false)
	s.onAfterTaskSuccess.Inc(1)
	// Don't use r.Context() here as that seems to get cancelled by Swarming
	// pretty quickly.

	// Do this in a Go routine so as we can return from this HTTP handler
	// quickly.
	go func() {
		defer metrics2.FuncTimer().Stop()
		// Do the reboot first so that the device will be ready
		// when doing the interrogation.
		if err := s.machine.RebootDevice(r.Context()); err != nil {
			sklog.Warningf("Failed to reboot device: %s", err)
		}
		s.triggerInterrogationCh <- true
	}()

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
