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
	pubsubUtils "go.skia.org/infra/go/pubsub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/configs"
	"go.skia.org/infra/machine/go/machine"
	machineProcessor "go.skia.org/infra/machine/go/machine/processor"
	"go.skia.org/infra/machine/go/machine/source/pubsubsource"
	machineStore "go.skia.org/infra/machine/go/machine/store"
	"go.skia.org/infra/machine/go/machineserver/config"
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
	store, err := machineStore.New(ctx, *baseapp.Local, instanceConfig)
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
	s.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*baseapp.ResourcesDir, "index.html"),
	))
}

func (s *server) loadTemplates() {
	if *baseapp.Local {
		s.loadTemplatesImpl()
	}
	s.loadTemplatesOnce.Do(s.loadTemplatesImpl)
}

func (s *server) mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	s.loadTemplates()
	if err := s.templates.ExecuteTemplate(w, "index.html", map[string]string{
		// Look in webpack.config.js for where the nonce templates are injected.
		"Nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

func (s *server) machinesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	descriptions, err := s.store.List(r.Context())
	if err != nil {
		httputils.ReportError(w, err, "Failed to read from datastore", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(descriptions); err != nil {
		sklog.Errorf("Failed to write response: %s", err)
	}
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

func (s *server) machineToggleUpdateHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := strings.TrimSpace(vars["id"])
	if id == "" {
		http.Error(w, "ID must be supplied.", http.StatusBadRequest)
		return
	}

	var ret machine.Description
	err := s.store.Update(r.Context(), id, func(in machine.Description) machine.Description {
		ret = in.Copy()
		if ret.ScheduledForDeletion == ret.PodName {
			ret.ScheduledForDeletion = ""
		} else {
			ret.ScheduledForDeletion = ret.PodName
		}
		ret.Annotation = machine.Annotation{
			User:      user(r),
			Message:   fmt.Sprintf("Requested update for %q", ret.PodName),
			Timestamp: time.Now(),
		}
		return ret
	})
	auditlog.Log(r, "toggle-update", struct {
		MachineID            string
		PodName              string
		ScheduledForDeletion string
	}{
		MachineID:            id,
		PodName:              ret.PodName,
		ScheduledForDeletion: ret.ScheduledForDeletion,
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
			Message:   fmt.Sprintf("Requested powercycle for %q", ret.PodName),
			Timestamp: time.Now(),
		}
		return ret
	})
	auditlog.Log(r, "toggle-powercycle", struct {
		MachineID  string
		PodName    string
		PowerCycle bool
	}{
		MachineID:  id,
		PodName:    ret.PodName,
		PowerCycle: ret.PowerCycle,
	})
	if err != nil {
		httputils.ReportError(w, err, "Failed to update machine.", http.StatusInternalServerError)
		return
	}
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
	err := s.store.Update(r.Context(), id, func(in machine.Description) machine.Description {
		ret = in.Copy()

		newDescription := machine.NewDescription()
		ret.Dimensions = newDescription.Dimensions

		ret.Annotation = machine.Annotation{
			User:      user(r),
			Message:   fmt.Sprintf("Requested device removal"),
			Timestamp: time.Now(),
		}
		return ret
	})
	auditlog.Log(r, "remove-device", struct {
		MachineID string
		PodName   string
	}{
		MachineID: id,
		PodName:   ret.PodName,
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
	var note machine.Annotation
	if err := json.NewDecoder(r.Body).Decode(&note); err != nil {
		httputils.ReportError(w, err, "Failed to parse incoming note.", http.StatusBadRequest)
		return
	}
	note.User = user(r)
	note.Timestamp = time.Now()

	var ret machine.Description
	err := s.store.Update(r.Context(), id, func(in machine.Description) machine.Description {
		ret = in.Copy()
		ret.Note = note
		return ret
	})
	auditlog.Log(r, "set-note", note)
	if err != nil {
		httputils.ReportError(w, err, "Failed to update machine.", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *server) podsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	pods, err := s.switchboard.ListPods(r.Context())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get list of pods.", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(pods); err != nil {
		sklog.Errorf("Failed to write response: %s", err)
	}
}

// See baseapp.App.
func (s *server) AddHandlers(r *mux.Router) {
	r.HandleFunc("/", s.mainHandler).Methods("GET")
	r.HandleFunc("/_/machines", s.machinesHandler).Methods("GET")
	r.HandleFunc("/_/machine/toggle_mode/{id:.+}", s.machineToggleModeHandler).Methods("GET")
	r.HandleFunc("/_/machine/toggle_update/{id:.+}", s.machineToggleUpdateHandler).Methods("GET")
	r.HandleFunc("/_/machine/toggle_powercycle/{id:.+}", s.machineTogglePowerCycleHandler).Methods("GET")
	r.HandleFunc("/_/machine/remove_device/{id:.+}", s.machineRemoveDeviceHandler).Methods("GET")
	r.HandleFunc("/_/machine/delete_machine/{id:.+}", s.machineDeleteMachineHandler).Methods("GET")
	r.HandleFunc("/_/machine/set_note/{id:.+}", s.machineSetNoteHandler).Methods("POST")
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
