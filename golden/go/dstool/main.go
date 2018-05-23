package main

import (
	"context"
	"flag"
	"math/rand"
	"time"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	gstorage "google.golang.org/api/storage/v1"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
)

// Command line flags.
var (
	dsNamespace        = flag.String("ds_namespace", "", "Cloud datastore namespace to be used by this instance.")
	projectID          = flag.String("project_id", common.PROJECT_ID, "GCP project ID.")
	serviceAccountFile = flag.String("service_account_file", "", "Credentials file for service account.")
)

func main() {
	common.Init()

	// Needed to use TimeSortableKey(...) which relies on an RNG. See docs there.
	rand.Seed(time.Now().UnixNano())
	setupDataStore(*projectID, *dsNamespace, *serviceAccountFile)

	entityName := flag.Args()[0]
	deleteEntities(entityName, *dsNamespace)

}

func deleteEntities(entityName, nameSpace string) {
	sklog.Infof("Deleting all instance of %s in namespace %s", entityName, nameSpace)

	client := ds.DS
	ctx := context.TODO()
	pageSize := 1000
	lastCursorStr := ""
	cursorStr := ""
	allKeys := make([]*datastore.Key, 0, pageSize)

	for {
		// Get the next page.
		query := ds.NewQuery(ds.Kind(entityName)).KeysOnly().Limit(pageSize)
		if cursorStr != "" {
			cursor, err := datastore.DecodeCursor(cursorStr)
			if err != nil {
				sklog.Fatalf("Bad cursor %q: %v", cursorStr, err)
			}
			query = query.Start(cursor)
		}

		it := client.Run(ctx, query)
		var err error
		var key *datastore.Key
		before := len(allKeys)

		for {
			if key, err = it.Next(nil); err != nil {
				break
			}
			allKeys = append(allKeys, key)
		}

		if err != iterator.Done {
			sklog.Fatalf("Error retrieving keys: %s", err)
		}
		newKeyCount := len(allKeys) - before
		sklog.Infof("LOOP: Retrieved %d keys.   Total: %d", newKeyCount, len(allKeys))

		cursor, err := it.Cursor()
		if err != nil {
			sklog.Fatalf("Error retrieving cursor: %s", err)
		}
		cursorStr = cursor.String()
		sklog.Infof("NEW Cursor string: %s", cursorStr)
		sklog.Infof("OLD Cursor string: %s", lastCursorStr)
		if (cursorStr == lastCursorStr) || (newKeyCount < pageSize) {
			break
		}
		lastCursorStr = cursorStr
	}
	sklog.Infof("Retrieved: %d keys", len(allKeys))
}

func setupDataStore(projectID, nameSpace, svcAccountFile string) {
	// Get the token source from the same service account. Needed to access cloud pubsub and datastore.
	tokenSource, err := auth.NewJWTServiceAccountTokenSource("", svcAccountFile, gstorage.CloudPlatformScope)
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account to get token source: %s", err)
	}

	if err := ds.InitWithOpt(projectID, nameSpace, option.WithTokenSource(tokenSource)); err != nil {
		sklog.Fatalf("Unable to configure cloud datastore: %s", err)
	}
}
