package main

import (
	"context"
	"flag"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/ignore/ds_ignorestore"
	"go.skia.org/infra/golden/go/ignore/fs_ignorestore"
)

func main() {
	var (
		dsNamespace = flag.String("ds_namespace", "", "Cloud datastore namespace to be used by this instance.")
		dsProjectID = flag.String("ds_project_id", "", "Project id that houses the datastore instance.")
		fsNamespace = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'flutter', 'skia', etc")
		fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
	)
	flag.Parse()

	if *dsNamespace == "" || *dsProjectID == "" || *fsNamespace == "" {
		sklog.Fatalf("You must set --ds_namespace, --ds_project_id, and --fs_namespace")
	}
	ctx := context.Background()
	// Get the token source for the service account with access to the services
	// we need to operate
	tokenSource, err := auth.NewDefaultTokenSource(true, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account: %s", err)
	}

	if err := ds.InitWithOpt(*dsProjectID, *dsNamespace, option.WithTokenSource(tokenSource)); err != nil {
		sklog.Fatalf("Unable to configure cloud datastore: %s", err)
	}

	dsStore, err := ds_ignorestore.New(ds.DS)
	if err != nil {
		sklog.Fatalf("Unable to create DS ignorestore: %s", err)
	}

	// Auth note: the underlying firestore.NewClient looks at the
	// GOOGLE_APPLICATION_CREDENTIALS env variable, so we don't need to supply
	// a token source.
	fsClient, err := firestore.NewClient(ctx, *fsProjectID, "gold", *fsNamespace, nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}

	fsStore := fs_ignorestore.New(ctx, fsClient)

	oldOnes, err := dsStore.List(ctx)
	if err != nil {
		sklog.Fatalf("Could not get old ignores: %s", err)
	}

	sklog.Infof("Begin migration")
	for _, rule := range oldOnes {
		rule.ID = "" // The old ID has no meaning in the new system, so just remove it
		if err := fsStore.Create(ctx, rule); err != nil {
			sklog.Errorf("Could not create %#v in the new firestore system: %s", rule, err)
		}
	}
	sklog.Infof("Ported %d rules", len(oldOnes))
}
