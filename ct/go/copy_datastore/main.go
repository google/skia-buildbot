// Copies CT data from one cloud project to another.
package main

import (
	"context"
	"flag"
	"time"

	"cloud.google.com/go/datastore"

	"go.skia.org/infra/ct/go/ctfe/task_types"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/option"
)

var (
	srcProject = flag.String("src_project", "google.com:skia-buildbots", "The source project.")
	dstProject = flag.String("dst_project", "skia-public", "The destination project.")
	namespace  = flag.String("namespace", "cluster-telemetry-testing", "The Cloud Datastore namespace, such as 'cluster-telemetry'.")
)

func main() {
	common.Init()

	// Construct clients.
	ts, err := auth.NewDefaultTokenSource(true, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := ds.InitWithOpt(*srcProject, *namespace, option.WithTokenSource(ts)); err != nil {
		sklog.Fatal(err)
	}
	srcClient := ds.DS
	if err := ds.InitWithOpt(*dstProject, *namespace, option.WithTokenSource(ts)); err != nil {
		sklog.Fatal(err)
	}
	dstClient := ds.DS

	ctx := context.Background()

	// Gather all the kinds we want to migrate.
	kinds := []ds.Kind{}
	for _, t := range task_types.Prototypes() {
		kinds = append(kinds, t.GetDatastoreKind())
	}
	kinds = append(kinds, ds.CLUSTER_TELEMETRY_IDS)

	// Start the migration.
	defer util.TimeTrack(time.Now(), "Database migration")
	for _, k := range kinds {
		// Delete the kind from the destination project first.
		removeCount, err := ds.DeleteAll(dstClient, k, true /* wait */)
		if err != nil {
			sklog.Fatal(err)
		}
		sklog.Infof("Removed %d entries of %s from %s", removeCount, k, *dstProject)

		// Now migrate the kind.
		if err := ds.MigrateData(ctx, srcClient, dstClient, k); err != nil {
			sklog.Fatal(err)
		}
	}
	sklog.Infoln("Database migration finished.")
}
