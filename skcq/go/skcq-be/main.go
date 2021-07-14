/*
	SkCQ backend server
*/

package main

import (
	"context"
	"flag"
	"net/http"
	"net/http/pprof"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/caches"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/db"
	"go.skia.org/infra/skcq/go/poller"
)

var (
	// Flags
	host                  = flag.String("host", "skcq.skia.org", "HTTP service host")
	fsNamespace           = flag.String("fs_namespace", "", "The namespace this instance should operate in. e.g. staging or prod")
	fsProjectID           = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
	chromeInfraAuthJWT    = flag.String("chrome_infra_auth_jwt", "/var/secrets/skia-public-auth/key.json", "The JWT key for the service account that has access to chrome infra auth.")
	canModifyCfgsOnTheFly = flag.String("can_modify_cfgs_on_the_fly", "project-skia-committers", "Which go/cria group is allowed to modify skcq.json and tasks.json on the fly.")
	pollInterval          = flag.Duration("poll_interval", 3*time.Second, "How often the server will poll Gerrit for CR+1 and CQ+1/CQ+2 changes.")

	publicFEInstanceURL = flag.String("public_fe_url", "localhost", "The public FE instance URL.")
	corpFEInstanceURL   = flag.String("corp_fe_url", "localhost", "The corp FE instance URL.")

	reposAllowList = common.NewMultiStringFlag("allowed_repo", nil, "Which repos should be processed by SkCQ. If not specified then all repos will be processed.")
	reposBlockList = common.NewMultiStringFlag("blocked_repo", nil, "Which repos should not be processed by SkCQ. If not specified then no repos will be skipped.")

	debugPort = flag.String("debug_port", "", "Port for debugging pprof (e.g., ':10110')")
)

// maybeStartDebugServer starts an internal HTTP server for debugging purposes
// if requested.
func maybeStartDebugServer() {
	// Start the internal server on the internal port if requested.
	if *debugPort != "" {
		// Add the profiling endpoints to the internal router.
		internalRouter := mux.NewRouter()

		// Set up the health check endpoint.
		internalRouter.HandleFunc("/healthz", httputils.ReadyHandleFunc)

		// Register pprof handlers
		internalRouter.HandleFunc("/debug/pprof/", pprof.Index)
		internalRouter.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		internalRouter.HandleFunc("/debug/pprof/profile", pprof.Profile)
		internalRouter.HandleFunc("/debug/pprof/{profile}", pprof.Index)

		go func() {
			sklog.Infof("Internal server on http://127.0.0.1" + *debugPort)
			sklog.Fatal(http.ListenAndServe(*debugPort, internalRouter))
		}()
	}
}

func main() {
	common.InitWithMust("skcq-be", common.PrometheusOpt(baseapp.PromPort), common.MetricsLoggingOpt())
	defer sklog.Flush()
	ctx := context.Background()

	maybeStartDebugServer()

	// Create the token source to use for DB client and HTTP client.
	ts, err := auth.NewDefaultTokenSource(*baseapp.Local, datastore.ScopeDatastore, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT)
	if err != nil {
		sklog.Fatal("Could not create token source: %s", err)
	}

	// Instantiate DB client.
	dbClient, err := db.New(ctx, ts, *fsNamespace, *fsProjectID)
	if err != nil {
		sklog.Fatalf("Could not init DB: %s", err)
	}

	// Instantiate authenticated HTTP client.
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Instantiate codereview.
	g, err := codereview.NewGerrit(httpClient, gerrit.ConfigChromium, gerrit.GerritSkiaURL)
	if err != nil {
		sklog.Fatalf("Could not init gerrit client: %s", err)
	}

	// Instantiate the cache.
	currentChangesCache, err := caches.GetCurrentChangesCache(ctx, dbClient)
	if err != nil {
		sklog.Fatalf("Could not get current changes cache: %s", err)
	}
	sklog.Infof("CurrentChangesCache: %+v", currentChangesCache.Get())

	// Instantiate client for go/cria.
	criaTs, err := auth.NewJWTServiceAccountTokenSource("", *chromeInfraAuthJWT, auth.SCOPE_USERINFO_EMAIL)
	if err != nil {
		sklog.Fatal(err)
	}
	criaClient := httputils.DefaultClientConfig().WithTokenSource(criaTs).With2xxOnly().Client()
	cfgModifyAllowed, err := allowed.NewAllowedFromChromeInfraAuth(criaClient, *canModifyCfgsOnTheFly)
	if err != nil {
		sklog.Fatalf("Could not create allowed for go/cria: %s", err)
	}

	// Start the poller.
	if err := poller.Start(ctx, *pollInterval, g, currentChangesCache, httpClient, criaClient, dbClient, cfgModifyAllowed, *publicFEInstanceURL, *corpFEInstanceURL, *reposAllowList, *reposBlockList); err != nil {
		sklog.Fatalf("Could not init SkCQ poller: %s", err)
	}

	// Wait forever.
	select {}
}
