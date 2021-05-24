/*
	Skia Commit Queue server
*/

package main

import (
	"context"
	"flag"
	"time"

	"cloud.google.com/go/datastore"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/poller"
)

var (
	// Flags
	host        = flag.String("host", "skcq.skia.org", "HTTP service host")
	workdir     = flag.String("workdir", ".", "Directory to use for scratch work.")
	fsNamespace = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'skcq'")
	fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")

	serviceAccountFile = flag.String("service_account_file", "/var/secrets/google/key.json", "Service account JSON file.")

	// Keep this really really fast.
	pollInterval = flag.Duration("poll_interval", 5*time.Second, "How often the server will poll Gerrit for CR+1 and CQ+1/CQ+2 changes.")
)

func main() {
	common.InitWithMust("skcq-be", common.PrometheusOpt(baseapp.PromPort), common.MetricsLoggingOpt())
	defer sklog.Flush()
	ctx := context.Background()

	// Instatiate authenticated http client.
	ts, err := auth.NewDefaultTokenSource(*baseapp.Local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_FULL_CONTROL, datastore.ScopeDatastore)
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Instantiate codereview.
	g, err := codereview.NewGerrit(httpClient)
	if err != nil {
		sklog.Fatalf("Could not init gerrit client: %s", err)
	}

	// Instantiate poller and turn it on.
	if err := poller.Start(ctx, *pollInterval, g); err != nil {
		sklog.Fatalf("Could not init SkCQ poller: %s", err)
	}

	select {}
}
