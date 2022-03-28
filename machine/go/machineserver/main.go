package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

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
	changeSink "go.skia.org/infra/machine/go/machine/change/sink"
	"go.skia.org/infra/machine/go/machine/event/source/pubsubsource"
	machineProcessor "go.skia.org/infra/machine/go/machine/processor"
	machineStore "go.skia.org/infra/machine/go/machine/store"
	"go.skia.org/infra/machine/go/machineserver/config"
	"go.skia.org/infra/machine/go/machineserver/rpc"
)

// flags
var (
	configFlag             = flag.String("config", "test.json", "The name to the configuration file, such as prod.json or test.json, as found in machine/go/configs.")
	allowedServiceAccounts = flag.String("service_accounts", "skolo-jumphost@skia-public.iam.gserviceaccount.com", "A comma separated list of service accounts that can access the JSON API.")
)

var errFailedToGetID = errors.New("failed to get id from URL")

type server struct {
	store             machineStore.Store
	templates         *template.Template
	loadTemplatesOnce sync.Once
	changeSink        changeSink.Sink
}

// See baseapp.Constructor.
func new() (baseapp.App, error) {
	pubsubUtils.EnsureNotEmulator()

	ctx := context.Background()

	var allowList []string
	if !*baseapp.Local {
		allowList = []string{"google.com"}
	} else {
		allowList = []string{"barney@example.org"}
	}

	// Add in service accounts.
	for _, s := range strings.Split(*allowedServiceAccounts, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			allowList = append(allowList, s)
		}
	}

	sklog.Infof("AllowList: %s", allowList)

	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil, nil, allowed.NewAllowedFromList(allowList))

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

	store, err := machineStore.NewFirestoreImpl(ctx, *baseapp.Local, instanceConfig)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	eventSource, err := pubsubsource.New(ctx, *baseapp.Local, instanceConfig)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	eventCh, err := eventSource.Start(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to start pubsubsource.")
	}
	storeUpdateFail := metrics2.GetCounter("machineserver_store_update_fail")

	changeSink, err := changeSink.New(ctx, *baseapp.Local, instanceConfig.DescriptionChangeSource)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to start change sink.")
	}

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

	s := &server{
		store:      store,
		changeSink: changeSink,
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

// getID retrieves the value of {id:.+} from URLs. It reports an error on the
// ResponseWrite if none is found.
func getID(w http.ResponseWriter, r *http.Request) (string, error) {
	vars := mux.Vars(r)
	id := strings.TrimSpace(vars["id"])
	if id == "" {
		http.Error(w, "Machine ID must be supplied.", http.StatusBadRequest)
		return "", errFailedToGetID
	}
	return id, nil
}

// sendHTMLResponse renders the given template, passing it the current
// context's CSP nonce. If template rendering fails, it logs an error.
func (s *server) sendHTMLResponse(templateName string, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	s.loadTemplates() // just to support template changes during dev
	if err := s.templates.ExecuteTemplate(w, templateName, map[string]string{
		// Look in //machine/pages/BUILD.bazel for where the nonce templates are injected.
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

func (s *server) triggerDescriptionUpdateEvent(ctx context.Context, id string) {
	if err := s.changeSink.Send(ctx, id); err != nil {
		sklog.Errorf("Failed to trigger change event: %s", err)
	}
}

// toggleMode is used in machineToggleModeHandler and passed to s.store.Update
// to toggle the Description mode between Available and Maintenance.
func toggleMode(ctx context.Context, user string, in machine.Description) machine.Description {
	ret := in.Copy()
	if ret.Mode == machine.ModeAvailable {
		ret.Mode = machine.ModeMaintenance
	} else {
		ret.Mode = machine.ModeAvailable
	}
	ret.Annotation = machine.Annotation{
		User:      user,
		Message:   fmt.Sprintf("Changed mode to %q", ret.Mode),
		Timestamp: now.Now(ctx),
	}
	return ret
}

func (s *server) machineToggleModeHandler(w http.ResponseWriter, r *http.Request) {
	id, err := getID(w, r)
	if err != nil {
		return
	}
	ctx := r.Context()

	resultMode := machine.ModeAvailable
	err = s.store.Update(ctx, id, func(in machine.Description) machine.Description {
		ret := toggleMode(ctx, user(r), in)
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
	s.triggerDescriptionUpdateEvent(r.Context(), id)

	w.WriteHeader(http.StatusOK)
}

// togglePowerCycle is used in machineTogglePowerCycleHandler and passed to
// s.store.Update to toggle the Description PowerCycle boolean.
func togglePowerCycle(ctx context.Context, id, user string, in machine.Description) machine.Description {
	ret := in.Copy()
	ret.PowerCycle = !ret.PowerCycle
	ret.Annotation = machine.Annotation{
		User:      user,
		Message:   fmt.Sprintf("Requested powercycle for %q", id),
		Timestamp: now.Now(ctx),
	}
	return ret
}

func (s *server) machineTogglePowerCycleHandler(w http.ResponseWriter, r *http.Request) {
	id, err := getID(w, r)
	if err != nil {
		return
	}

	ctx := r.Context()
	var ret machine.Description
	err = s.store.Update(r.Context(), id, func(in machine.Description) machine.Description {
		return togglePowerCycle(ctx, id, user(r), in)
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

// setAttachedDevice is used in machineSetAttachedDeviceHandler and passed to
// s.store.Update to set the value of Description.AttachedDevice.
func setAttachedDevice(ad machine.AttachedDevice, in machine.Description) machine.Description {
	ret := in.Copy()
	ret.AttachedDevice = ad
	return ret
}

func (s *server) machineSetAttachedDeviceHandler(w http.ResponseWriter, r *http.Request) {
	id, err := getID(w, r)
	if err != nil {
		return
	}

	var attachedDeviceRequest rpc.SetAttachedDevice
	if err := json.NewDecoder(r.Body).Decode(&attachedDeviceRequest); err != nil {
		httputils.ReportError(w, err, "Failed to parse incoming note.", http.StatusBadRequest)
		return
	}

	err = s.store.Update(r.Context(), id, func(in machine.Description) machine.Description {
		return setAttachedDevice(attachedDeviceRequest.AttachedDevice, in)
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
	s.triggerDescriptionUpdateEvent(r.Context(), id)
	w.WriteHeader(http.StatusOK)
}

// removeDevice is used in machineRemoveDeviceHandler and passed to
// s.store.Update to clear values in the Description that come from attached
// devices.
func removeDevice(ctx context.Context, id, user string, in machine.Description) machine.Description {
	ret := in.Copy()

	ret.Dimensions = machine.SwarmingDimensions{}
	ret.Annotation = machine.Annotation{
		User:      user,
		Message:   fmt.Sprintf("Requested device removal of %q", id),
		Timestamp: now.Now(ctx),
	}
	ret.Temperature = nil
	ret.SuppliedDimensions = nil
	ret.SSHUserIP = ""
	return ret
}

func (s *server) machineRemoveDeviceHandler(w http.ResponseWriter, r *http.Request) {
	id, err := getID(w, r)
	if err != nil {
		return
	}

	ctx := r.Context()
	err = s.store.Update(ctx, id, func(in machine.Description) machine.Description {
		return removeDevice(ctx, id, user(r), in)
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
	s.triggerDescriptionUpdateEvent(r.Context(), id)
	w.WriteHeader(http.StatusOK)
}

func (s *server) machineDeleteMachineHandler(w http.ResponseWriter, r *http.Request) {
	id, err := getID(w, r)
	if err != nil {
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
	s.triggerDescriptionUpdateEvent(r.Context(), id)
	w.WriteHeader(http.StatusOK)
}

// setNote is used in machineSetNoteHandler and passed to s.store.Update to set
// the Description.Note value.
func setNote(ctx context.Context, user string, note rpc.SetNoteRequest, in machine.Description) machine.Description {
	ret := in.Copy()

	ret.Note = machine.Annotation{
		Message:   note.Message,
		User:      user,
		Timestamp: now.Now(ctx),
	}
	return ret
}

func (s *server) machineSetNoteHandler(w http.ResponseWriter, r *http.Request) {
	id, err := getID(w, r)
	if err != nil {
		return
	}

	var note rpc.SetNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&note); err != nil {
		httputils.ReportError(w, err, "Failed to parse incoming note.", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	err = s.store.Update(ctx, id, func(in machine.Description) machine.Description {
		return setNote(ctx, user(r), note, in)
	})
	auditlog.Log(r, "set-note", note)
	if err != nil {
		httputils.ReportError(w, err, "Failed to update machine.", http.StatusInternalServerError)
		return
	}
	s.triggerDescriptionUpdateEvent(r.Context(), id)
	w.WriteHeader(http.StatusOK)
}

// setChromeOSInfo is used in machineSetChromeOSInfoHandler and passed to
// s.store.Update to set Description values used by ChromeOS devices.
func setChromeOSInfo(ctx context.Context, req rpc.SupplyChromeOSRequest, in machine.Description) machine.Description {
	ret := in.Copy()
	ret.SSHUserIP = req.SSHUserIP
	ret.SuppliedDimensions = req.SuppliedDimensions
	ret.LastUpdated = now.Now(ctx)
	return ret
}

// machineSupplyChromeOSInfoHandler takes in the information needed to connect a given machine with
// a ChromeOS device (via SSH).
func (s *server) machineSupplyChromeOSInfoHandler(w http.ResponseWriter, r *http.Request) {
	id, err := getID(w, r)
	if err != nil {
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
	ctx := r.Context()
	err = s.store.Update(ctx, id, func(in machine.Description) machine.Description {
		return setChromeOSInfo(ctx, req, in)
	})
	auditlog.Log(r, "supply-dimensions", req)
	if err != nil {
		httputils.ReportError(w, err, "Failed to process dimensions.", http.StatusInternalServerError)
		return
	}
	s.triggerDescriptionUpdateEvent(r.Context(), id)
	w.WriteHeader(http.StatusOK)
}

func (s *server) apiMachineDescriptionHandler(w http.ResponseWriter, r *http.Request) {
	id, err := getID(w, r)
	if err != nil {
		return
	}

	desc, err := s.store.Get(r.Context(), id)
	if err != nil {
		httputils.ReportError(w, err, "Failed to read from datastore", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(rpc.ToFrontendDescription(desc), w)
}

func (s *server) apiPowerCycleListHandler(w http.ResponseWriter, r *http.Request) {
	toPowerCycle, err := s.store.ListPowerCycle(r.Context())
	if err != nil {
		httputils.ReportError(w, err, "Failed to read from datastore", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(toPowerCycle, w)
}

func setPowerCycleFalse(in machine.Description) machine.Description {
	ret := in.Copy()
	ret.PowerCycle = false
	return ret
}

func (s *server) apiPowerCycleCompleteHandler(w http.ResponseWriter, r *http.Request) {
	id, err := getID(w, r)
	if err != nil {
		return
	}

	err = s.store.Update(r.Context(), id, setPowerCycleFalse)
	auditlog.Log(r, "powercycle-complete", id)
	if err != nil {
		httputils.ReportError(w, err, "Failed to update machine.", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func setPowerCycleState(newState machine.PowerCycleState, in machine.Description) machine.Description {
	ret := in.Copy()
	ret.PowerCycleState = newState
	return ret
}

func (s *server) apiPowerCycleStateUpdateHandler(w http.ResponseWriter, r *http.Request) {
	var req rpc.UpdatePowerCycleStateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to parse request.", http.StatusBadRequest)
		return
	}

	for _, updateRequest := range req.Machines {
		err := s.store.Update(r.Context(), updateRequest.MachineID, func(in machine.Description) machine.Description {
			return setPowerCycleState(updateRequest.PowerCycleState, in)
		})
		if err != nil {
			httputils.ReportError(w, err, "Failed to update machine.PowerCycleState", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

// See baseapp.App.
func (s *server) AddHandlers(r *mux.Router) {
	// Pages
	r.HandleFunc("/", s.machinesPageHandler).Methods("GET")

	// UI API
	r.HandleFunc("/_/machines", s.machinesHandler).Methods("GET")
	r.HandleFunc("/_/machine/toggle_mode/{id:.+}", s.machineToggleModeHandler).Methods("POST")
	r.HandleFunc("/_/machine/toggle_powercycle/{id:.+}", s.machineTogglePowerCycleHandler).Methods("POST")
	r.HandleFunc("/_/machine/set_attached_device/{id:.+}", s.machineSetAttachedDeviceHandler).Methods("POST")
	r.HandleFunc("/_/machine/remove_device/{id:.+}", s.machineRemoveDeviceHandler).Methods("POST")
	r.HandleFunc("/_/machine/delete_machine/{id:.+}", s.machineDeleteMachineHandler).Methods("POST")
	r.HandleFunc("/_/machine/set_note/{id:.+}", s.machineSetNoteHandler).Methods("POST")
	r.HandleFunc("/_/machine/supply_chromeos/{id:.+}", s.machineSupplyChromeOSInfoHandler).Methods("POST")
	r.HandleFunc("/loginstatus/", login.StatusHandler).Methods("GET")

	// Public API
	r.HandleFunc(rpc.MachineDescriptionURL, s.apiMachineDescriptionHandler).Methods("GET")
	r.HandleFunc(rpc.PowerCycleListURL, s.apiPowerCycleListHandler).Methods("GET")
	r.HandleFunc(rpc.PowerCycleCompleteURL, s.apiPowerCycleCompleteHandler).Methods("POST")
	r.HandleFunc(rpc.PowerCycleStateUpdateURL, s.apiPowerCycleStateUpdateHandler).Methods("POST")
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
