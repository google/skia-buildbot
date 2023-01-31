package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/unrolled/secure"

	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/alogin/proxylogin"
	"go.skia.org/infra/go/auditlog"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/common"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/configs"
	"go.skia.org/infra/machine/go/machine"
	changeSink "go.skia.org/infra/machine/go/machine/change/sink"
	sseChangeSink "go.skia.org/infra/machine/go/machine/change/sink/sse"
	httpEventSource "go.skia.org/infra/machine/go/machine/event/source/httpsource"
	"go.skia.org/infra/machine/go/machine/pools"
	machineProcessor "go.skia.org/infra/machine/go/machine/processor"
	machineStore "go.skia.org/infra/machine/go/machine/store"
	"go.skia.org/infra/machine/go/machine/store/cdb"
	"go.skia.org/infra/machine/go/machineserver/config"
	"go.skia.org/infra/machine/go/machineserver/rpc"
)

var errFailedToGetID = errors.New("failed to get id from URL")

type flags struct {
	configFlag              string
	changeEventSSERPeerPort int
	namespace               string
	labelSelector           string
	local                   bool
	port                    string
	promPort                string
	resourcesDir            string
}

func (f *flags) Register(fs *flag.FlagSet) {
	fs.StringVar(&f.configFlag, "config", "test.json", "The name to the configuration file, such as prod.json or test.json, as found in machine/go/configs.")
	fs.IntVar(&f.changeEventSSERPeerPort, "change_event_sser_peer_port", 4000, "The port used to communicate among peers messages that need to be sent over SSE.")
	fs.StringVar(&f.namespace, "namespace", "default", "The namespace this application is running under in k8s.")
	fs.StringVar(&f.labelSelector, "label_selector", "app=machineserver", "A label selector that finds all peer pods of this application in k8s.")
	fs.BoolVar(&f.local, "local", false, "Running locally if true. As opposed to in production.")
	fs.StringVar(&f.port, "port", ":8000", "HTTP service address (e.g., ':8000')")
	fs.StringVar(&f.promPort, "prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	fs.StringVar(&f.resourcesDir, "resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
}

type server struct {
	flags *flags

	store             machineStore.Store
	templates         *template.Template
	loadTemplatesOnce sync.Once
	httpEventSource   *httpEventSource.HTTPSource

	// Change Sinks.
	sserChangeSink changeSink.Sink

	// Event Sources.
	httpSourceCh <-chan machine.Event

	sserServer sseChangeSink.SSE

	processor machineProcessor.Processor

	login alogin.Login
}

func new(args []string) (*server, error) {
	ctx := context.Background()

	// Register and parse flags.
	flags := &flags{}
	flagSet := flag.NewFlagSet("machineserver", flag.ExitOnError)
	flags.Register(flagSet)

	common.InitWithMust(
		"machineserver",
		common.PrometheusOpt(&flags.promPort),
		common.MetricsLoggingOpt(),
		common.FlagSetOpt(flagSet),
	)

	var instanceConfig config.InstanceConfig
	b, err := fs.ReadFile(configs.Configs, flags.configFlag)
	if err != nil {
		sklog.Fatalf("read config file %q: %s", flags.configFlag, err)
	}
	err = json.Unmarshal(b, &instanceConfig)
	if err != nil {
		sklog.Fatal(err)
	}

	processor := machineProcessor.New(ctx)

	if instanceConfig.ConnectionString == "" {
		sklog.Fatal("ConnectionString must be supplied in the instance config")
	}

	pools, err := pools.New(instanceConfig)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	db, err := pgxpool.Connect(ctx, instanceConfig.ConnectionString)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	store := cdb.New(db, pools)

	httpSource, err := httpEventSource.New()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	httpSourceCh, err := httpSource.Start(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	sserChangeSink, err := sseChangeSink.New(ctx, flags.local, flags.namespace, flags.labelSelector, flags.changeEventSSERPeerPort)
	if err != nil {
		return nil, skerr.Wrapf(err, "create sser Server")
	}

	s := &server{
		flags:           flags,
		store:           store,
		sserChangeSink:  sserChangeSink,
		login:           proxylogin.NewWithDefaults(),
		httpEventSource: httpSource,
		sserServer:      *sserChangeSink,
		processor:       processor,
		httpSourceCh:    httpSourceCh,
	}
	s.loadTemplates()
	go s.listenMachineEvents(ctx)
	return s, nil
}

// Starts listening for the arrival of machine.Events. This function doesn't
// return unless the context is cancelled.
func (s *server) listenMachineEvents(ctx context.Context) {
	storeUpdateFail := metrics2.GetCounter("machineserver_store_update_fail")

	sklog.Infof("Start machine.Event listening loop")
	for {
		select {
		case event := <-s.httpSourceCh:
			processEventArrival(ctx, s.store, storeUpdateFail, s.processor, event)
		case <-ctx.Done():
			return
		}
	}
}

func processEventArrival(ctx context.Context, store machineStore.Store, storeUpdateFail metrics2.Counter, processor machineProcessor.Processor, event machine.Event) {
	err := store.Update(ctx, event.Host.Name, func(previous machine.Description) machine.Description {
		return processor.Process(ctx, previous, event)
	})
	if err != nil {
		storeUpdateFail.Inc(1)
		sklog.Errorf("Failed to update: %s", err)
	}
}

func (s *server) audit(w http.ResponseWriter, r *http.Request, action string, body interface{}) {
	auditlog.LogWithUser(r, s.login.LoggedInAs(r).String(), action, body)
}

func (s *server) loadTemplatesImpl() {
	s.templates = template.Must(template.New("").Delims("{%", "%}").ParseGlob(
		filepath.Join(s.flags.resourcesDir, "*.html"),
	))
}

func (s *server) loadTemplates() {
	if s.flags.local {
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
	sendJSONResponse(descriptions, w)
}

func (s *server) triggerDescriptionUpdateEvent(ctx context.Context, id string) {
	if err := s.sserChangeSink.Send(ctx, id); err != nil {
		sklog.Errorf("Failed to trigger SSE change event: %s", err)
	}
}

// toggleMode is used in machineToggleModeHandler and passed to s.store.Update
// to toggle the Description mode between Available and Maintenance.
func toggleMode(ctx context.Context, user string, in machine.Description) machine.Description {
	ret := in.Copy()
	ts := now.Now(ctx)
	var annotation string
	if !ret.InMaintenanceMode() {
		ret.MaintenanceMode = fmt.Sprintf("%s %s", user, ts.Format(time.RFC3339))
		annotation = "Enabled Maintenance Mode"
	} else {
		ret.MaintenanceMode = ""
		annotation = "Cleared Maintenance Mode."
	}
	ret.Annotation = machine.Annotation{
		User:      user,
		Message:   annotation,
		Timestamp: now.Now(ctx),
	}
	machine.SetSwarmingQuarantinedMessage(&ret)
	return ret
}

func (s *server) machineToggleModeHandler(w http.ResponseWriter, r *http.Request) {
	id, err := getID(w, r)
	if err != nil {
		return
	}
	s.audit(w, r, "toggle-mode", id)
	ctx := r.Context()

	err = s.store.Update(ctx, id, func(in machine.Description) machine.Description {
		ret := toggleMode(ctx, string(s.login.LoggedInAs(r)), in)
		return ret
	})
	if err != nil {
		httputils.ReportError(w, err, "Failed to update machine.", http.StatusInternalServerError)
		return
	}
	s.triggerDescriptionUpdateEvent(r.Context(), id)

	w.WriteHeader(http.StatusOK)
}

func clearQuarantined(in machine.Description) machine.Description {
	ret := in.Copy()
	ret.IsQuarantined = false
	machine.SetSwarmingQuarantinedMessage(&ret)
	return ret
}

func (s *server) machineClearQuarantinedHandler(w http.ResponseWriter, r *http.Request) {
	id, err := getID(w, r)
	if err != nil {
		return
	}
	s.audit(w, r, "clear-quarantine", id)
	ctx := r.Context()

	if err := s.store.Update(ctx, id, clearQuarantined); err != nil {
		httputils.ReportError(w, err, "Failed to update machine.", http.StatusInternalServerError)
		return
	}
	s.triggerDescriptionUpdateEvent(ctx, id)

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
	s.audit(w, r, "toggle-powercycle", id)

	ctx := r.Context()
	err = s.store.Update(r.Context(), id, func(in machine.Description) machine.Description {
		return togglePowerCycle(ctx, id, string(s.login.LoggedInAs(r)), in)
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

	s.audit(w, r, "set-attached-device", attachedDeviceRequest)

	err = s.store.Update(r.Context(), id, func(in machine.Description) machine.Description {
		return setAttachedDevice(attachedDeviceRequest.AttachedDevice, in)
	})
	if err != nil {
		httputils.ReportError(w, err, "Failed to update machine.", http.StatusInternalServerError)
		return
	}
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

	s.audit(w, r, "remove-device", id)

	ctx := r.Context()
	err = s.store.Update(ctx, id, func(in machine.Description) machine.Description {
		return removeDevice(ctx, id, string(s.login.LoggedInAs(r)), in)
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

	s.audit(w, r, "delete-machine", id)

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

	s.audit(w, r, "set-note", note)

	ctx := r.Context()
	err = s.store.Update(ctx, id, func(in machine.Description) machine.Description {
		return setNote(ctx, string(s.login.LoggedInAs(r)), note, in)
	})
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

	s.audit(w, r, "supply-dimensions", req)

	ctx := r.Context()
	err = s.store.Update(ctx, id, func(in machine.Description) machine.Description {
		return setChromeOSInfo(ctx, req, in)
	})
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
	sendJSONResponse(desc, w)
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

	s.audit(w, r, "powercycle-complete", id)

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

	s.audit(w, r, "powercycle-update", req)

	for _, updateRequest := range req.Machines {
		if _, err := s.store.Get(r.Context(), updateRequest.MachineID); err != nil {
			sklog.Infof("Got powercycle info for a non-existent machine %q: %s", updateRequest.MachineID, err)
			continue
		}
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

func (s *server) loginStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	st := s.login.Status(r)
	if s.flags.local {
		st.EMail = "barney@example.org"
	}
	if err := json.NewEncoder(w).Encode(st); err != nil {
		httputils.ReportError(w, err, "Failed to encode login status", http.StatusInternalServerError)
	}
}

// Wrapper functions for http.Handlers based on the combinations we need of
// GZipping, CSP, and Role enforcement.

func gzip(h http.Handler) http.Handler {
	return httputils.GzipRequestResponse(h)
}

func (s *server) editor(h http.Handler) http.Handler {
	if !s.flags.local {
		return proxylogin.ForceRoleMiddleware(s.login, roles.Editor)(h)
	}
	return h
}

func (s *server) secure(h http.Handler) http.Handler {
	return baseapp.SecurityMiddleware([]string{"machines.skia.org"}, s.flags.local, nil)(h)
}

func (s *server) secureGzip(h http.Handler) http.Handler {
	return s.secure(gzip(h))
}

func (s *server) editorSecureGzip(h http.Handler) http.Handler {
	return s.editor(s.secureGzip(h))
}

func (s *server) AddHandlers(r *mux.Router) {
	r.HandleFunc("/healthz", httputils.ReadyHandleFunc)

	// Pages
	r.Handle("/", s.secureGzip(http.HandlerFunc(s.machinesPageHandler))).Methods("GET")

	// Resources
	if s.flags.resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(1)
		s.flags.resourcesDir = filepath.Join(filepath.Dir(filename), "../../dist")
	}
	r.PathPrefix("/dist/").Handler(http.StripPrefix("/dist/", s.secureGzip(http.HandlerFunc(httputils.MakeResourceHandler(s.flags.resourcesDir))))).Methods("GET")

	// UI API
	r.Handle("/_/machine/toggle_mode/{id:.+}", s.editorSecureGzip(http.HandlerFunc(s.machineToggleModeHandler))).Methods("POST")
	r.Handle("/_/machine/toggle_powercycle/{id:.+}", s.editorSecureGzip(http.HandlerFunc(s.machineTogglePowerCycleHandler))).Methods("POST")
	r.Handle("/_/machine/set_attached_device/{id:.+}", s.editorSecureGzip(http.HandlerFunc(s.machineSetAttachedDeviceHandler))).Methods("POST")
	r.Handle("/_/machine/remove_device/{id:.+}", s.editorSecureGzip(http.HandlerFunc(s.machineRemoveDeviceHandler))).Methods("POST")
	r.Handle("/_/machine/delete_machine/{id:.+}", s.editorSecureGzip(http.HandlerFunc(s.machineDeleteMachineHandler))).Methods("POST")
	r.Handle("/_/machine/set_note/{id:.+}", s.editorSecureGzip(http.HandlerFunc(s.machineSetNoteHandler))).Methods("POST")
	r.Handle("/_/machine/supply_chromeos/{id:.+}", s.editorSecureGzip(http.HandlerFunc(s.machineSupplyChromeOSInfoHandler))).Methods("POST")
	r.Handle("/_/machine/clear_quarantined/{id:.+}", s.editorSecureGzip(http.HandlerFunc(s.machineClearQuarantinedHandler))).Methods("POST")

	// External APIs
	r.Handle(rpc.PowerCycleCompleteURL, s.editorSecureGzip(http.HandlerFunc(s.apiPowerCycleCompleteHandler))).Methods("POST")
	r.Handle(rpc.PowerCycleStateUpdateURL, s.editorSecureGzip(http.HandlerFunc(s.apiPowerCycleStateUpdateHandler))).Methods("POST")
	r.Handle(rpc.MachineEventURL, s.editorSecureGzip(s.httpEventSource)).Methods("POST")
	r.Handle(rpc.SSEMachineDescriptionUpdatedURL, s.editor(s.sserServer.GetHandler(context.Background()))) // GZip interferes with SSE.

	// Public APIs
	r.Handle("/_/machines", gzip(http.HandlerFunc(s.machinesHandler))).Methods("GET")
	r.Handle(rpc.MachineDescriptionURL, gzip(http.HandlerFunc(s.apiMachineDescriptionHandler))).Methods("GET")
	r.Handle(rpc.PowerCycleListURL, gzip(http.HandlerFunc(s.apiPowerCycleListHandler))).Methods("GET")
	r.Handle("/loginstatus/", gzip(http.HandlerFunc(s.loginStatus))).Methods("GET")
}

func main() {
	s, err := new(os.Args[1:])
	if err != nil {
		sklog.Fatal(err)
	}
	r := mux.NewRouter()
	s.AddHandlers(r)

	sklog.Infof("Ready to serve at: %q", s.flags.port)
	server := &http.Server{
		Addr:           s.flags.port,
		Handler:        r,
		MaxHeaderBytes: 1 << 20,
	}
	sklog.Fatal(server.ListenAndServe())
}
