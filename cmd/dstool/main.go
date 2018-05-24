package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"go.skia.org/infra/golden/go/expstorage"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/iterator"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
)

// Command line flags.
var (
	dsNamespace = flag.String("ds_namespace", "", "Cloud datastore namespace to be used by this instance.")
	projectID   = flag.String("project_id", common.PROJECT_ID, "GCP project ID.")
)

type opsEntry struct {
	params []string
	fn     func(client *datastore.Client, params ...string)
}

var (
	ops = map[string]*opsEntry{
		"delete": &opsEntry{
			params: []string{"kind | *"},
			fn:     deleteEntities,
		},

		"touch": &opsEntry{
			params: []string{"kind"},
			fn:     touchEntities,
		},
	}

	registeredEntities = map[ds.Kind]interface{}{
		ds.MASTER_EXP_CHANGE: &expstorage.ExpChange{},
		ds.TRYJOB_EXP_CHANGE: &expstorage.ExpChange{},
		ds.TEST_DIGEST_EXP:   &expstorage.TestDigestExp{},
	}
)

// ops:
//
//   clear out a namespace (or delete results of a query)
//   touch to re-index all entities of a given kind (get -> put)
//     --namespace
//

func main() {
	common.Init()

	// Needed to use TimeSortableKey(...) which relies on an RNG. See docs there.
	rand.Seed(time.Now().UnixNano())
	if err := ds.InitWithOpt(*projectID, *dsNamespace); err != nil {
		sklog.Fatalf("Error initializing cloud data store: %s", err)
	}
	dsClient := ds.DS

	userCmd := flag.Arg(0)
	if userCmd == "" {
		printUsage("Invalid command: "+userCmd, 1)
	}

	op, ok := ops[userCmd]
	if !ok {
		printUsage("Unknown command: "+userCmd, 1)
	}

	userParams := flag.Args()[1:]
	if len(userParams) != len(op.params) {
		m := fmt.Sprintf("Command %s requires these parameters: %s", userCmd, strings.Join(op.params, " "))
		printUsage(m, 1)
	}

	// Execute the command.
	sklog.Infof("Operating in project/namespace: %s/%s", *projectID, *dsNamespace)
	op.fn(dsClient, userParams...)
}

func printUsage(errMsg string, returnVal int) {
	flag.PrintDefaults()
	os.Exit(returnVal)
}

func deleteEntities(client *datastore.Client, params ...string) {
	entityName := params[0]

	sklog.Infof("Deleting all instance of %s in namespace %s", entityName)

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

func touchEntities(client *datastore.Client, params ...string) {
	entityName := params[0]
	instance, ok := registeredEntities[entityName]
	if !ok {
		sklog.Fatalf("Kind %s is not registered with a datastructure  and can therefore not be used with the 'touch' command.")
	}

	// Iterate over the keys in the namespace.

	// Load the entity

}
