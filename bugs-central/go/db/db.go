package db

import (
	"context"
	"flag"

	"golang.org/x/oauth2"

	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
)

var (
	fsNamespace = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'bugs-central'")
	fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")

	fsClient *firestore.Client
)

func Init(ctx context.Context, ts oauth2.TokenSource) error {
	// Instantiate firestore.
	var err error
	fsClient, err = firestore.NewClient(ctx, *fsProjectID, "bugs-central", *fsNamespace, ts)
	if err != nil {
		return skerr.Wrapf(err, "could not init firestore")
	}
	return nil
}

// addToDB if value is different
func addToDB() error {
	return nil
}
