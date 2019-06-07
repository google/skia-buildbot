// This executable will transfer a datastore-backed ExpectationsStore to a
// firestore-backed one.
// Example usage:
// ds_fs_migration -alsologtostderr -fs_namespace flutter -ds_namespace gold-flutter \
// -service_account_file /path/to/service-account.json

package main

import (
	"context"
	"flag"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/expstorage/ds_expstore"
	"go.skia.org/infra/golden/go/expstorage/fs_expstore"
	"go.skia.org/infra/golden/go/types"
	"google.golang.org/api/option"
	gstorage "google.golang.org/api/storage/v1"
)

var (
	dsNamespace        = flag.String("ds_namespace", "", "Cloud datastore namespace to be used by this instance.")
	dsProjectID        = flag.String("ds_project_id", common.PROJECT_ID, "GCP project ID.")
	serviceAccountFile = flag.String("service_account_file", "", "Credentials file for service account.")

	fsNamespace = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'flutter', 'skia', etc")
	fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")

	compareOnly = flag.Bool("compare_only", false, "Compare the two stores")
)

const (
	userID = "gold-migrator"
)

func main() {
	flag.Parse()

	if *fsNamespace == "" || *dsNamespace == "" {
		sklog.Fatal("Must provide --fs_namespace and --ds_namespace")
	}

	fes := initFirestore()
	des := initOldDatastore()

	sklog.Infof("Both stores loaded")

	exp, err := des.Get()
	if err != nil {
		sklog.Fatalf("Could not get datastore expectations: %s", err)
	}

	sklog.Infof("Datastore had expectations for %d tests", len(exp))

	fexp, err := fes.Get()
	if err != nil {
		sklog.Fatalf("Could not get firestore expectations: %s", err)
	}

	sklog.Infof("firestore had expectations for %d tests", len(fexp))

	toAdd := types.Expectations{}

	for test, digestMap := range exp {
		if _, ok := fexp[test]; !ok {
			toAdd[test] = digestMap
			continue
		}
		for digest, label := range digestMap {
			if fLabel, ok := fexp[test][digest]; !ok || label != fLabel {
				if _, ok := toAdd[test]; !ok {
					toAdd[test] = map[types.Digest]types.Label{}
				}
				toAdd[test][digest] = label
			}
		}
	}

	sklog.Infof("Found differences in %d tests", len(toAdd))

	if *compareOnly || len(toAdd) == 0 {
		return
	}

	sklog.Infof("Going to port expectations for %d tests", len(toAdd))

	if err := fes.AddChange(context.Background(), toAdd, userID); err != nil {
		sklog.Fatalf("Could not write the expectations to firestore: %s", err)
	}
	sklog.Infof("Done - they should be in firestore now.")

}

func initFirestore() expstorage.ExpectationsStore {
	firestore.EnsureNotEmulator()
	ts, err := auth.NewDefaultTokenSource( /*local=*/ true)
	if err != nil {
		sklog.Fatalf("Could not get token source: %s", err)
	}

	fsClient, err := firestore.NewClient(context.Background(), *fsProjectID, "gold", *fsNamespace, ts)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}

	f, err := fs_expstore.New(fsClient, nil, fs_expstore.ReadWrite)
	if err != nil {
		sklog.Fatalf("Unable to initialize fs_expstore: %s", err)
	}
	return f
}

func initOldDatastore() expstorage.ExpectationsStore {
	ds.EnsureNotEmulator()
	// Get the token source for the service account with access to GCS, the Monorail issue tracker,
	// cloud pubsub, and datastore.
	tokenSource, err := auth.NewJWTServiceAccountTokenSource("", *serviceAccountFile, gstorage.CloudPlatformScope, "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account: %s", err)
	}

	if err := ds.InitWithOpt(*dsProjectID, *dsNamespace, option.WithTokenSource(tokenSource)); err != nil {
		sklog.Fatalf("Unable to configure cloud datastore: %s", err)
	}

	// Set up the cloud expectations store
	expStore, err := ds_expstore.DeprecatedNew(ds.DS, nil)
	if err != nil {
		sklog.Fatalf("Unable to configure cloud expectations store: %s", err)
	}
	return expStore
}
