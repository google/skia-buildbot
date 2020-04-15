package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/login"
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
	store machineStore.Store
}

// See baseapp.Constructor.
func new() (baseapp.App, error) {
	var instanceConfig config.InstanceConfig
	err := util.WithReadFile(*configFlag, func(r io.Reader) error {
		return json.NewDecoder(r).Decode(&instanceConfig)
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to open config file: %q", *configFlag)
	}

	ctx := context.Background()

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

	// Start our main loop.
	go func() {
		for event := range eventCh {
			store.Update(ctx, event.Host.Name, func(previous machine.Description) machine.Description {
				return processor.Process(ctx, previous, event)
			})
		}
	}()

	return &server{
		store: store,
	}, nil
}

// user returns the currently logged in user, or a placeholder if running locally.
func (srv *server) user(r *http.Request) string {
	user := "barney@example.org"
	if !*baseapp.Local {
		user = login.LoggedInAs(r)
	}
	return user
}

func (srv *server) mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	_, err := fmt.Fprintf(w, "Hello World!")
	if err != nil {
		sklog.Errorf("Failed to write response: %s", err)
	}
}

// See baseapp.App.
func (srv *server) AddHandlers(r *mux.Router) {
	r.HandleFunc("/", srv.mainHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler).Methods("GET")
}

// See baseapp.App.
func (srv *server) AddMiddleware() []mux.MiddlewareFunc {
	ret := []mux.MiddlewareFunc{}
	if !*baseapp.Local {
		ret = append(ret, login.ForceAuthMiddleware(login.DEFAULT_REDIRECT_URL), login.RestrictViewer)
	}
	return ret
}

func main() {
	baseapp.Serve(new, []string{"machines.skia.org"})
}
