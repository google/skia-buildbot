package main

import (
	"context"
	"flag"
	"net/http"

	"cloud.google.com/go/datastore"
	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2/google"

	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/alogin/proxylogin"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/job_creation/buildbucket_taskbackend"
)

const (
	// APP_NAME is the name of this app.
	APP_NAME = "task-scheduler-bbfe"
)

var (
	// Task Scheduler database.
	tsDb db.DBCloser

	// Flags.
	buildbucketTarget = flag.String("buildbucket_target", "", "Target name used by Buildbucket to address this Task Scheduler.")
	host              = flag.String("host", "localhost", "HTTP service host")
	port              = flag.String("port", ":8000", "HTTP service port for the web server (e.g., ':8000')")
	firestoreInstance = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"production\"")
	local             = flag.Bool("local", false, "Whether we're running on a dev machine vs in production.")
	promPort          = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

func runServer(serverURL string, bbHandler http.Handler, plogin alogin.Login) {
	r := chi.NewRouter()
	if bbHandler != nil {
		r.Handle("/prpc/*", alogin.ForceRole(bbHandler, plogin, roles.Buildbucket))
	}

	h := httputils.LoggingRequestResponse(r)
	h = httputils.XFrameOptionsDeny(h)
	h = alogin.StatusMiddleware(plogin)(h)
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
	}
	http.Handle("/", h)
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	// Global init.
	common.InitWithMust(
		APP_NAME,
		common.PrometheusOpt(promPort),
	)
	defer common.Defer()

	ctx, cancelFn := context.WithCancel(context.Background())
	cleanup.AtExit(cancelFn)

	// Set up token source and authenticated API clients.
	tokenSource, err := google.DefaultTokenSource(ctx, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatalf("Failed to create token source: %s", err)
	}

	// Initialize the database.
	tsDb, err = firestore.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, tokenSource)
	if err != nil {
		sklog.Fatalf("Failed to create Firestore DB client: %s", err)
	}
	cleanup.AtExit(func() {
		util.Close(tsDb)
	})

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	// Initialize Buildbucket TaskBackend.
	var bbHandler http.Handler
	if *buildbucketTarget != "" {
		httpClient := httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()
		bb2 := buildbucket.NewClient(httpClient)
		bbHandler = buildbucket_taskbackend.Handler(*buildbucketTarget, serverURL, common.PROJECT_REPO_MAPPING, tsDb, bb2)
	}

	plogin := proxylogin.NewWithDefaults()
	go runServer(serverURL, bbHandler, plogin)

	// Run indefinitely, responding to HTTP requests.
	select {}
}
