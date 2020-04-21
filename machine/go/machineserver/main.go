package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"text/template"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/machine"
	machineProcessor "go.skia.org/infra/machine/go/machine/processor"
	"go.skia.org/infra/machine/go/machine/source/pubsubsource"
	machineStore "go.skia.org/infra/machine/go/machine/store"
	"go.skia.org/infra/machine/go/machineserver/config"
)

// flags
var (
	configFlag = flag.String("config", "./configs/test.json", "The path to the configuration file.")
)

type server struct {
	store     machineStore.Store
	templates *template.Template
}

// See baseapp.Constructor.
func new() (baseapp.App, error) {
	ctx := context.Background()

	var allow allowed.Allow
	if !*baseapp.Local {
		allowed.NewAllowedFromList([]string{"google.com"})
	} else {
		allow = allowed.NewAllowedFromList([]string{"barney@example.org"})
	}

	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil, nil, allow)

	var instanceConfig config.InstanceConfig
	err := util.WithReadFile(*configFlag, func(r io.Reader) error {
		return json.NewDecoder(r).Decode(&instanceConfig)
	})
	if err != nil {
		sklog.Fatalf("Failed to open config file: %q: %s", *configFlag, err)
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
	eventCh, err := source.Start(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to start pubsubsource.")
	}
	storeUpdateFail := metrics2.GetCounter("machineserver_store_update_fail")

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
		store: store,
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

func (s *server) loadTemplates() {
	s.templates = template.Must(template.New("").ParseFiles(
		filepath.Join(*baseapp.ResourcesDir, "index.html"),
	))
}

func (s *server) mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *baseapp.Local {
		s.loadTemplates()
	}
	descriptions, err := s.store.List(r.Context())
	sklog.Infof("%#v", descriptions)
	if err != nil {
		httputils.ReportError(w, err, "Failed to get list of machines.", http.StatusInternalServerError)
		return
	}

	if err := s.templates.ExecuteTemplate(w, "index.html", descriptions); err != nil {
		sklog.Error("Failed to expand template:", err)
	}

	if err != nil {
		sklog.Errorf("Failed to write response: %s", err)
	}
}

// See baseapp.App.
func (s *server) AddHandlers(r *mux.Router) {
	r.HandleFunc("/", s.mainHandler)
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
