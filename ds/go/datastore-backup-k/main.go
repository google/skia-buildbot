// Trigger backups of Cloud Datastore entities to Cloud Storage using the
// datastore v1beta1 API.
//
// See http://go/datastore-backup-example for an example in the APIs Explorer.
package main

import (
	"context"
	"flag"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/ds/go/backup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2/google"
)

// flags
var (
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	project  = flag.String("project", "skia-public", "Name of the project we are running in.")
	bucket   = flag.String("bucket", "skia-backups-skia-public", "Name of a bucket in 'project' to store the backups.")
	local    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
)

func main() {
	common.InitWithMust(
		"datastore_backup_k",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	ts, err := google.DefaultTokenSource(context.Background(), datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatalf("Failed to auth: %s", err)
	}

	// backup package handles retries and specifically handles "resource exhausted" HTTP status code.
	client := httputils.DefaultClientConfig().WithTokenSource(ts).WithoutRetries().Client()
	if err := backup.Step(client, *project, *bucket); err != nil {
		sklog.Errorf("Failed to do first backup step: %s", err)
	}
	for range time.Tick(24 * time.Hour) {
		if err := backup.Step(client, *project, *bucket); err != nil {
			sklog.Errorf("Failed to backup: %s", err)
		}
	}
}
