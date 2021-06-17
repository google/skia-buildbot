/*
	Skia Commit Queue server
*/

package main

import (
	"context"
	"flag"
	"time"

	"cloud.google.com/go/datastore"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/db"
	"go.skia.org/infra/skcq/go/poller"
)

var (
	// Flags
	host        = flag.String("host", "skcq.skia.org", "HTTP service host")
	fsNamespace = flag.String("fs_namespace", "", "The namespace this instance should operate in. e.g. staging or prod")
	fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")

	serviceAccountFile = flag.String("service_account_file", "/var/secrets/google/key.json", "Service account JSON file.")

	// canModifyCfgsOnTheFly = flag.String("can_modify_cfgs_on_the_fly", "project-skia-tryjob-access", "Which go/cria group is allowed to modify skcq.json and tasks.json on the fly.")
	// This should be more restrcitive...
	canModifyCfgsOnTheFly = flag.String("can_modify_cfgs_on_the_fly", "project-skia-committers", "Which go/cria group is allowed to modify skcq.json and tasks.json on the fly.")

	// TODO(rmistry): When all Skia repositories use SkCQ remove this flag. DO NOT NEED THIS IF WE ONLY GO BY THE CONFIG!
	// supportedReposAllowlist = common.NewMultiStringFlag("repos_allowlist", nil, "All Skia repos supported by SkCQ")
	// Keep this really really fast.
	pollInterval = flag.Duration("poll_interval", 10*time.Second, "How often the server will poll Gerrit for CR+1 and CQ+1/CQ+2 changes.")

	publicFEInstanceURL = flag.String("public_fe_url", "localhost", "The public FE instance URL.")
	corpFEInstanceURL   = flag.String("corp_fe_url", "localhost", "The corp FE instance URL.")

	reposAllowList = common.NewMultiStringFlag("allowed_repo", nil, "Which repos should be processed by SkCQ. If not specified then all repos will be processed.")
	reposBlockList = common.NewMultiStringFlag("blocked_repo", nil, "Which repos should not be processed by SkCQ. If not specified then no repos will be skipped.")
)

func main() {
	common.InitWithMust("skcq-be", common.PrometheusOpt(baseapp.PromPort), common.MetricsLoggingOpt())
	defer sklog.Flush()
	ctx := context.Background()

	ts, err := auth.NewDefaultTokenSource(*baseapp.Local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_FULL_CONTROL, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatal("Could not create token source: %s", err)
	}

	// Instatiate authenticated http client.
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Instantiate DB client.
	dbClient, err := db.New(ctx, ts, *fsNamespace, *fsProjectID)
	if err != nil {
		sklog.Fatalf("Could not init DB: %s", err)
	}

	// Instantiate codereview.
	g, err := codereview.NewGerrit(httpClient, gerrit.ConfigSkia, gerrit.GerritSkiaURL)
	if err != nil {
		sklog.Fatalf("Could not init gerrit client: %s", err)
	}

	// Instantiate poller and turn it on.
	cfgModifyAllowed, err := allowed.NewAllowedFromChromeInfraAuth(httpClient, *canModifyCfgsOnTheFly)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := poller.Start(ctx, *pollInterval, g, httpClient, dbClient, cfgModifyAllowed, *publicFEInstanceURL, *corpFEInstanceURL, *reposAllowList, *reposBlockList); err != nil {
		sklog.Fatalf("Could not init SkCQ poller: %s", err)
	}

	select {}
}
