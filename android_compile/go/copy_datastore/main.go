// Copies Android compile server data from one cloud project to another.
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
	srcProject = flag.String("src_project", "google.com:skia-buildbots", "The source project.")
	dstProject = flag.String("dst_project", "google.com:skia-corp", "The destination project.")
	namespace  = flag.String("namespace", "android-compile-staging", "The Cloud Datastore namespace, such as 'android-compile'.")
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
	defer func(start time.Time) {
		elapsed := time.Since(start)
		sklog.Infof("Database migration took %s", elapsed)
	}(time.Now())

	// Delete the kind from the destination project first.
	removeCount, err := ds.DeleteAll(dstClient, ds.COMPILE_TASK, true /* wait */)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Removed %d entries of %s from %s", removeCount, ds.COMPILE_TASK, *dstProject)
	// Now migrate the kind.
	if err := ds.MigrateData(ctx, srcClient, dstClient, ds.COMPILE_TASK); err != nil {
		sklog.Fatal(err)
	}
}
