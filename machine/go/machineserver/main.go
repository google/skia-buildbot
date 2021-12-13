package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"github.com/unrolled/secure"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auditlog"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	pubsubUtils "go.skia.org/infra/go/pubsub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/configs"
	"go.skia.org/infra/machine/go/machine"
	machineProcessor "go.skia.org/infra/machine/go/machine/processor"
	"go.skia.org/infra/machine/go/machine/source/pubsubsource"
	machineStore "go.skia.org/infra/machine/go/machine/store"
	"go.skia.org/infra/machine/go/machineserver/config"
	"go.skia.org/infra/machine/go/machineserver/rpc"
	firestoreSwitchboard "go.skia.org/infra/machine/go/switchboard"
	"go.skia.org/infra/machine/go/switchboard/cleanup"
)

// flags
var (
	configFlag = flag.String("config", "test.json", "The name to the configuration file, such as prod.json or test.json, as found in machine/go/configs.")
)

type server struct {
	store             machineStore.Store
	templates         *template.Template
	loadTemplatesOnce sync.Once
	switchboard       firestoreSwitchboard.Switchboard
}

// See baseapp.Constructor.
func new() (baseapp.App, error) {
	pubsubUtils.EnsureNotEmulator()

	ctx := context.Background()

	var allow allowed.Allow
	if !*baseapp.Local {
		allow = allowed.NewAllowedFromList([]string{"google.com"})
	} else {
		allow = allowed.NewAllowedFromList([]string{"barney@example.org"})
	}

	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil, nil, allow)

	var instanceConfig config.InstanceConfig
	b, err := fs.ReadFile(configs.Configs, *configFlag)
	if err != nil {
		sklog.Fatalf("Failed to read config file %q: %s", *configFlag, err)
	}
	err = json.Unmarshal(b, &instanceConfig)
	if err != nil {
		sklog.Fatal(err)
	}

	processor := machineProcessor.New(ctx)
	source, err := pubsubsource.New(ctx, *baseapp.Local, instanceConfig)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	store, err := machineStore.NewFirestoreImpl(ctx, *baseapp.Local, instanceConfig)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	switchboard, err := firestoreSwitchboard.New(ctx, *baseapp.Local, instanceConfig)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	eventCh, err := source.Start(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to start pubsubsource.")
	}
	storeUpdateFail := metrics2.GetCounter("machineserver_store_update_fail")
	switchboardImpl, err := firestoreSwitchboard.New(ctx, *baseapp.Local, instanceConfig)
	if err != nil {
		sklog.Fatal(err)
	}
	cleaner := cleanup.New(switchboardImpl)

	// Start our main loop.
	go func() {
		for event := range eventCh {
			err := store.Update(ctx, event.Host.Name, func(previous machine.Description) machine.Description {
				return processor.Process(ctx, previous, event)
			})
			if err != nil {
				storeUpdateFail.Inc(1)
				sklog.Errorf("Failed to update: %s", err)
			}
		}
	}()

	// Start the process to clean up stale MeetingPoints.
	go cleaner.Start(ctx)

	s := &server{
		store:       store,
		switchboard: switchboard,
	}
	s.loadTemplates()
	return s, nil
}

// user returns the currently logged in user, or a placeholder if running locally.
func user(r *http.Request) string {
	user := "barney@example.org"
	if !*baseapp.Local {
		user = login.LoggedInAs(r)
	}
	return user
}

func (s *server) loadTemplatesImpl() {
	s.templates = template.Must(template.New("").Delims("{%", "%}").ParseGlob(
		filepath.Join(*baseapp.ResourcesDir, "*.html"),
	))
}

func (s *server) loadTemplates() {
	if *baseapp.Local {
		s.loadTemplatesImpl()
	}
	s.loadTemplatesOnce.Do(s.loadTemplatesImpl)
}

// sendJSONResponse sends a JSON representation of any data structure as an
// HTTP response. If the conversion to JSON has an error, the error is logged.
func sendJSONResponse(data interface{}, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		sklog.Errorf("Failed to write response: %s", err)
	}
}

// sendHTMLResponse renders the given template, passing it the current
// context's CSP nonce. If template rendering fails, it logs an error.
func (s *server) sendHTMLResponse(templateName string, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	s.loadTemplates() // just to support template changes during dev
	if err := s.templates.ExecuteTemplate(w, templateName, map[string]string{
		// Look in webpack.config.js for where the nonce templates are injected.
		"Nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

func (s *server) machinesPageHandler(w http.ResponseWriter, r *http.Request) {
	s.sendHTMLResponse("index.html", w, r)
}

func (s *server) machinesHandler(w http.ResponseWriter, r *http.Request) {
	descriptions, err := s.store.List(r.Context())
	if err != nil {
		httputils.ReportError(w, err, "Failed to read from datastore", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(rpc.ToListMachinesResponse(descriptions), w)
}

func (s *server) machineToggleModeHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := strings.TrimSpace(vars["id"])
	if id == "" {
		http.Error(w, "ID must be supplied.", http.StatusBadRequest)
		return
	}

	resultMode := machine.ModeAvailable
	err := s.store.Update(r.Context(), id, func(in machine.Description) machine.Description {
		ret := in.Copy()
		if ret.Mode == machine.ModeAvailable {
			ret.Mode = machine.ModeMaintenance
		} else {
			ret.Mode = machine.ModeAvailable
		}
		ret.Annotation = machine.Annotation{
			User:      user(r),
			Message:   fmt.Sprintf("Changed mode to %q", ret.Mode),
			Timestamp: time.Now(),
		}
		resultMode = ret.Mode
		return ret
	})
	auditlog.Log(r, "toggle-mode", struct {
		MachineID string
		Mode      machine.Mode
	}{
		MachineID: id,
		Mode:      resultMode,
	})
	if err != nil {
		httputils.ReportError(w, err, "Failed to update machine.", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *server) machineTogglePowerCycleHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := strings.TrimSpace(vars["id"])
	if id == "" {
		http.Error(w, "ID must be supplied.", http.StatusBadRequest)
		return
	}

	var ret machine.Description
	err := s.store.Update(r.Context(), id, func(in machine.Description) machine.Description {
		ret = in.Copy()
		ret.PowerCycle = !ret.PowerCycle
		ret.Annotation = machine.Annotation{
			User:      user(r),
			Message:   fmt.Sprintf("Requested powercycle for %q", id),
			Timestamp: time.Now(),
		}
		return ret
	})
	auditlog.Log(r, "toggle-powercycle", struct {
		MachineID  string
		PowerCycle bool
	}{
		MachineID:  id,
		PowerCycle: ret.PowerCycle,
	})
	if err != nil {
		httputils.ReportError(w, err, "Failed to update machine.", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *server) machineSetAttachedDeviceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := strings.TrimSpace(vars["id"])
	if id == "" {
		http.Error(w, "ID must be supplied.", http.StatusBadRequest)
		return
	}

	var attachedDeviceRequest rpc.SetAttachedDevice
	if err := json.NewDecoder(r.Body).Decode(&attachedDeviceRequest); err != nil {
		httputils.ReportError(w, err, "Failed to parse incoming note.", http.StatusBadRequest)
		return
	}

	var ret machine.Description
	err := s.store.Update(r.Context(), id, func(in machine.Description) machine.Description {
		ret = in.Copy()
		ret.AttachedDevice = attachedDeviceRequest.AttachedDevice
		return ret
	})
	if err != nil {
		httputils.ReportError(w, err, "Failed to update machine.", http.StatusInternalServerError)
		return
	}
	auditlog.Log(r, "set-attached-device", struct {
		MachineID      string
		AttachedDevice machine.AttachedDevice
	}{
		MachineID:      id,
		AttachedDevice: attachedDeviceRequest.AttachedDevice,
	})
	w.WriteHeader(http.StatusOK)
}

func (s *server) machineRemoveDeviceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := strings.TrimSpace(vars["id"])
	if id == "" {
		http.Error(w, "ID must be supplied.", http.StatusBadRequest)
		return
	}

	var ret machine.Description
	ctx := r.Context()
	err := s.store.Update(ctx, id, func(in machine.Description) machine.Description {
		ret = in.Copy()

		ret.Dimensions = machine.SwarmingDimensions{}
		ret.Annotation = machine.Annotation{
			User:      user(r),
			Message:   fmt.Sprintf("Requested device removal of %s", id),
			Timestamp: now.Now(ctx),
		}
		ret.Temperature = nil
		ret.SuppliedDimensions = nil
		ret.SSHUserIP = ""
		return ret
	})
	auditlog.Log(r, "remove-device", struct {
		MachineID string
	}{
		MachineID: id,
	})
	if err != nil {
		httputils.ReportError(w, err, "Failed to update machine.", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *server) machineDeleteMachineHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := strings.TrimSpace(vars["id"])
	if id == "" {
		http.Error(w, "ID must be supplied.", http.StatusBadRequest)
		return
	}

	auditlog.Log(r, "delete-machine", struct {
		MachineID string
	}{
		MachineID: id,
	})

	if err := s.store.Delete(r.Context(), id); err != nil {
		httputils.ReportError(w, err, "Failed to delete machine.", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *server) machineSetNoteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := strings.TrimSpace(vars["id"])
	if id == "" {
		http.Error(w, "ID must be supplied.", http.StatusBadRequest)
		return
	}
	var note rpc.SetNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&note); err != nil {
		httputils.ReportError(w, err, "Failed to parse incoming note.", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	newNote := machine.Annotation{
		Message:   note.Message,
		User:      user(r),
		Timestamp: now.Now(ctx),
	}

	var ret machine.Description
	err := s.store.Update(ctx, id, func(in machine.Description) machine.Description {
		ret = in.Copy()
		ret.Note = newNote
		return ret
	})
	auditlog.Log(r, "set-note", note)
	if err != nil {
		httputils.ReportError(w, err, "Failed to update machine.", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// machineSupplyChromeOSInfoHandler takes in the information needed to connect a given machine with
// a ChromeOS device (via SSH).
func (s *server) machineSupplyChromeOSInfoHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := strings.TrimSpace(vars["id"])
	if id == "" {
		http.Error(w, "Machine ID must be supplied.", http.StatusBadRequest)
		return
	}
	var req rpc.SupplyChromeOSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to parse request.", http.StatusBadRequest)
		return
	}
	if req.SSHUserIP == "" || len(req.SuppliedDimensions) == 0 {
		http.Error(w, "Missing fields.", http.StatusBadRequest)
		return
	}
	var ret machine.Description
	ctx := r.Context()
	err := s.store.Update(ctx, id, func(in machine.Description) machine.Description {
		ret = in.Copy()
		ret.SSHUserIP = req.SSHUserIP
		ret.SuppliedDimensions = req.SuppliedDimensions
		ret.LastUpdated = now.Now(ctx)
		return ret
	})
	auditlog.Log(r, "supply-dimensions", req)
	if err != nil {
		httputils.ReportError(w, err, "Failed to process dimensions.", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *server) meetingPointsHandler(w http.ResponseWriter, r *http.Request) {
	meetingPoints, err := s.switchboard.ListMeetingPoints(r.Context())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get list of meeting points", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(meetingPoints, w)
}

func (s *server) podsHandler(w http.ResponseWriter, r *http.Request) {
	pods, err := s.switchboard.ListPods(r.Context())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get list of pods.", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(pods, w)
}

// See baseapp.App.
func (s *server) AddHandlers(r *mux.Router) {
	r.HandleFunc("/", s.machinesPageHandler).Methods("GET")
	r.HandleFunc("/_/machines", s.machinesHandler).Methods("GET")
	r.HandleFunc("/_/machine/toggle_mode/{id:.+}", s.machineToggleModeHandler).Methods("POST")
	r.HandleFunc("/_/machine/toggle_powercycle/{id:.+}", s.machineTogglePowerCycleHandler).Methods("POST")
	r.HandleFunc("/_/machine/set_attached_device/{id:.+}", s.machineSetAttachedDeviceHandler).Methods("POST")
	r.HandleFunc("/_/machine/remove_device/{id:.+}", s.machineRemoveDeviceHandler).Methods("POST")
	r.HandleFunc("/_/machine/delete_machine/{id:.+}", s.machineDeleteMachineHandler).Methods("POST")
	r.HandleFunc("/_/machine/set_note/{id:.+}", s.machineSetNoteHandler).Methods("POST")
	r.HandleFunc("/_/machine/supply_chromeos/{id:.+}", s.machineSupplyChromeOSInfoHandler).Methods("POST")
	r.HandleFunc("/_/meeting_points", s.meetingPointsHandler).Methods("GET")
	r.HandleFunc("/_/pods", s.podsHandler).Methods("GET")
	r.HandleFunc("/loginstatus/", login.StatusHandler).Methods("GET")
}

// See baseapp.App.
func (s *server) AddMiddleware() []mux.MiddlewareFunc {
	ret := []mux.MiddlewareFunc{}
	if !*baseapp.Local {
		ret = append(ret, login.ForceAuthMiddleware(login.DEFAULT_REDIRECT_URL), login.RestrictViewer)
	}
	return ret
}

func main() {
	// TODO(jcgregorio) We should feed instanceConfig.Web.AllowedHosts to baseapp.Serve.
	baseapp.Serve(new, []string{"machines.skia.org"})
}
