package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/machine/processor"
	"go.skia.org/infra/machine/go/machine/source"
	"go.skia.org/infra/machine/go/machine/store"
	"go.skia.org/infra/machine/go/machineserver/config"
)

const (
	machinesCollectionName = "machines"
)

// flags
var (
	configFlag = flag.String("config", "./configs/test.json", "The path to the configuration file.")
)

type server struct {
	processor processor.Processor
	source    source.Source
	store     store.Store
}

// See baseapp.Constructor.
func new() (baseapp.App, error) {
	var instanceConfig config.InstanceConfig
	err := util.WithReadFile(*configFlag, func(r io.Reader) error {
		return json.NewDecoder(r).Decode(&instanceConfig)
	})
	if err != nil {
		sklog.Fatal(err)
	}

	/*
		var allow allowed.Allow
		if !*baseapp.Local {
			ts, err := auth.NewJWTServiceAccountTokenSource("", *chromeInfraAuthJWT, auth.SCOPE_USERINFO_EMAIL)
			if err != nil {
				return nil, err
			}
			client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
			allow, err = allowed.NewAllowedFromChromeInfraAuth(client, *authGroup)
			if err != nil {
				return nil, err
			}
			allow = allowed.NewAllowedFromList([]string{"google.com"})
		} else {
			allow = allowed.NewAllowedFromList([]string{"fred@example.org", "barney@example.org", "wilma@example.org"})
		}

		login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil, nil, allow)

		ctx := context.Background()
		ts, err := auth.NewDefaultTokenSource(*baseapp.Local, pubsub.ScopePubSub, "https://www.googleapis.com/auth/datastore")
		if err != nil {
			return nil, err
		}

		pubsubClient, err := pubsub.NewClient(ctx, "skia-public", option.WithTokenSource(ts))
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to create PubSub client for project %s", "skia-public")
		}
		t := pubsubClient.Topic(*topic)
		exists, err := t.Exists(ctx)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to check existence of PubSub topic %q", t.ID())
		}
		if !exists {
			if _, err := pubsubClient.CreateTopic(ctx, t.ID()); err != nil {
				return nil, skerr.Wrapf(err, "failed to create PubSub topic %q", t.ID())
			}
		}

		firestoreClient, err := firestore.NewClient(ctx, firestore.FIRESTORE_PROJECT, "machineserver", *instance, ts)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to create firestore client for ")
		}

		return &server{
			pubsubClient:    pubsubClient,
			firestoreClient: firestoreClient,
			machines:        firestoreClient.Collection(machinesCollectionName),
		}, nil
	*/
	return nil, nil
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
