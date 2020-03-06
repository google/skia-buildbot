// Copies CT data from one cloud project to another.
package main

import (
	"context"
	"flag"
	"time"

	"cloud.google.com/go/datastore"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/option"
)

var (
	srcProject   = flag.String("src_project", "skia-tree-status-staging", "The source project.")
	dstProject   = flag.String("dst_project", "skia-public", "The destination project.")
	srcNamespace = flag.String("src_namespace", "", "The source project's datastore namespace.")
	dstNamespace = flag.String("dst_namespace", "tree-status-staging", "The destination project's datastore namespace.")

	kindsToMigrate = []ds.Kind{
		"Status",
		"Sheriffs", "SheriffSchedules",
		"Robocops", "RobocopSchedules",
		"Troopers", "TrooperSchedules",
		"GpuSheriffs", "GpuSheriffSchedules",
	}
)

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	sklog.Infof("===== %s took %s =====", name, elapsed)
}

func main() {
	common.Init()

	// Construct clients.
	ts, err := auth.NewDefaultTokenSource(true, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := ds.InitWithOpt(*srcProject, *srcNamespace, option.WithTokenSource(ts)); err != nil {
		sklog.Fatal(err)
	}
	srcClient := ds.DS
	if err := ds.InitWithOpt(*dstProject, *dstNamespace, option.WithTokenSource(ts)); err != nil {
		sklog.Fatal(err)
	}
	dstClient := ds.DS

	ctx := context.Background()

	// Start the migration.
	defer timeTrack(time.Now(), "Database migration")
	for _, k := range kindsToMigrate {
		// Delete the kind from the destination project first.
		removeCount, err := ds.DeleteAll(dstClient, k, true /* wait */)
		if err != nil {
			sklog.Fatal(err)
		}
		sklog.Infof("Removed %d entries of %s from %s", removeCount, k, *dstProject)

		if err := ds.MigrateData(ctx, srcClient, dstClient, k, true /* createNewKey */); err != nil {
			sklog.Fatal(err)
		}
	}
	sklog.Info("Database migration finished.")
}
