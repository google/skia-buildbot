package main

import (
	"context"
	"flag"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/sklog"
)

var (
	local      = flag.Bool("local", true, "True if running locally.")
	fsInstance = flag.String("firestore_instance", "", "Firestore instance to use.")
	path       = flag.String("path", "", "Document path to delete; relative to the parent doc of this instance; if not specified, delete the entire instance.")
)

func main() {
	common.Init()

	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*local, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatal(err)
	}
	c, err := firestore.NewClient(ctx, firestore.FIRESTORE_PROJECT, firestore.APP_TASK_SCHEDULER, *fsInstance, ts)
	if err != nil {
		sklog.Fatal(err)
	}
	ref := c.ParentDoc
	if *path != "" {
		ref = c.Doc(*path)
	}
	sklog.Infof("Recursively deleting %s", ref.Path)
	if err := c.RecursiveDelete(ref, 3, 60*time.Minute); err != nil {
		sklog.Fatal(err)
	}
}
