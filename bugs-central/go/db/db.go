package db

import (
	"context"
	"flag"
	"fmt"
	"sync"
	"time"

	firestore_api "cloud.google.com/go/firestore"
	"golang.org/x/oauth2"

	"go.skia.org/infra/bugs-central/go/bugs"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
)

var (
	fsNamespace = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'bugs-central'")
	fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")

	fsClient *firestore.Client
	// mtx to control access to firestore
	mtx sync.RWMutex
)

const (
	// For accessing Firestore.
	DEFAULT_ATTEMPTS      = 3
	PUT_SINGLE_TIMEOUT    = 10 * time.Second
	DELETE_SINGLE_TIMEOUT = 10 * time.Second
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

// Use bugs.RecognizedClient and bugs.IssueSource instead?
func GetFromDB(client bugs.RecognizedClient, source bugs.IssueSource, subComponent string) error {
	mtx.RLock()
	defer mtx.RUnlock()

	// Query firestore for this client+source+subComponent combination.
	col := fsClient.Collection(string(client))
	doc := col.Doc(string(source))
	var subCol *firestore_api.CollectionRef
	if subComponent != "" {
		subCol = doc.Collection(subComponent)
	}
	fmt.Println("IN GET FROM DB")
	fmt.Println(col)
	fmt.Println(doc)
	fmt.Println(subCol)

	// col.Doc(id)
	if _, createErr := fsClient.Create(ctx, col.Doc(id), buildInfo, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT); createErr != nil {
		return false, skerr.Wrap(createErr)
	}

	return nil
}

// putInDB if value is different
func PutInDB() error {
	mtx.Lock()
	defer mtx.Unlock()

	return nil
}
