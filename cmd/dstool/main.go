package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/datastore"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/tryjobstore"
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
			params: []string{"kind"},
			fn:     deleteEntities,
		},

		"touch": &opsEntry{
			params: []string{"kind"},
			fn:     touchEntities,
		},
	}

	touchRegisteredEntities = map[ds.Kind]interface{}{
		ds.MASTER_EXP_CHANGE: &expstorage.ExpChange{},
		ds.TRYJOB_EXP_CHANGE: &expstorage.ExpChange{},
		ds.TRYJOB_RESULT:     &tryjobstore.TryjobResult{},
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
	if len(userParams) < len(op.params) {
		m := fmt.Sprintf("Command %s requires these parameters: %s", userCmd, strings.Join(op.params, " "))
		printUsage(m, 1)
	}

	// Execute the command.
	sklog.Infof("Executing '%s' in project/namespace: %s/%s", userCmd, *projectID, *dsNamespace)
	op.fn(dsClient, userParams...)
}

func printUsage(errMsg string, returnVal int) {
	flag.PrintDefaults()
	os.Exit(returnVal)
}

type procSliceFn func(client *datastore.Client, keys []*datastore.Key)

func processConcurrently(client *datastore.Client, procFn procSliceFn, kind ds.Kind, maxSliceSize int) {
	sklog.Infof("Processing all instance of %s", kind)

	// These variables are used in the processKeysForParent function and below.
	totalCount := int32(0)
	concurrentCh := make(chan bool, 1000)
	var wg sync.WaitGroup

	// This function calls procFn for all keys that have the same parent in
	// chunks of maxSliceSize.
	processKeysForOneParent := func(procSlice []*datastore.Key) {
		concurrentCh <- true
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-concurrentCh }()

			sklog.Infof("Received: %d keys", len(procSlice))
			for len(procSlice) > 0 {
				targetSlice := procSlice[0:util.MinInt(maxSliceSize, len(procSlice))]

				// Process the slice
				procFn(client, targetSlice)

				// Account for the processed slice and wait to avoid congestion.
				atomic.AddInt32(&totalCount, int32(len(targetSlice)))
				procSlice = procSlice[len(targetSlice):]
				if len(procSlice) > 0 {
					time.Sleep(1500 * time.Millisecond)
				}
			}
		}()
	}

	// pageSize defines how many keys we retrieve at once.
	pageSize := 10000
	iterCh, err := ds.IterKeys(client, kind, pageSize)
	if err != nil {
		sklog.Fatalf("Error getting key iterator: %s", err)
	}

	seen := map[string]bool{}
	byParent := map[int64][]*datastore.Key{}
	currParentID := int64(-1)

	go func() {
		for range time.Tick(5 * time.Second) {
			sklog.Infof("Processed %d", atomic.LoadInt32(&totalCount))
		}
	}()

	for keySlice := range iterCh {
		for _, key := range keySlice {
			var strKey string
			if key.Parent != nil {
				strKey = fmt.Sprintf("%d  :  %d", key.Parent.ID, key.ID)
			} else {
				strKey = fmt.Sprintf("%d", key.ID)
			}
			if seen[strKey] {
				sklog.Errorf("Seen this before: %d", key.ID)
				continue
			}
			seen[strKey] = true

			parentID := int64(0)
			if key.Parent != nil {
				parentID = key.Parent.ID
			}

			if currParentID == -1 {
				currParentID = parentID
			}

			byParent[parentID] = append(byParent[parentID], key)
			procSlice := []*datastore.Key(nil)
			if parentID != currParentID {
				sklog.Infof("Parent id: %d    Current parent id: %d", parentID, currParentID)
				procSlice = byParent[currParentID]
				delete(byParent, currParentID)
				currParentID = parentID
			}

			if len(byParent[0]) == maxSliceSize {
				procSlice = byParent[0]
				byParent[0] = nil
			}

			if len(procSlice) > 0 {
				sklog.Infof("Processing %d entries. Map: %d %d Concurrency: %d", len(procSlice), len(byParent), len(byParent[parentID]), len(concurrentCh))
				processKeysForOneParent(procSlice)
			}
		}
	}

	// Clean out any straggling keys.
	for ID, keySlice := range byParent {
		if len(keySlice) > 0 {
			sklog.Infof("Processing %d entries. Map: %d %d Concurrency: %d", len(keySlice), len(byParent), len(byParent[ID]), len(concurrentCh))
			processKeysForOneParent(keySlice)
		}
	}

	wg.Wait()
	sklog.Infof("Processed %d instances of %s", totalCount, kind)
}

func deleteEntities(client *datastore.Client, params ...string) {
	entityName := ds.Kind(params[0])
	processConcurrently(client, doDeleteSlice, entityName, 500)
}

func doDeleteSlice(client *datastore.Client, keys []*datastore.Key) {
	if err := client.DeleteMulti(context.TODO(), keys); err != nil {
		sklog.Fatalf("Error deleting slice: %s", err)
	}
}

func touchEntities(client *datastore.Client, params ...string) {
	kind := ds.Kind(params[0])

	instance, ok := touchRegisteredEntities[kind]
	if !ok {
		sklog.Fatalf("Kind %s is not registered with a datastructure  and can therefore not be used with the 'touch' command.")
	}

	sklog.Infof("Executing")

	// maxSliceSize is 10 because we can reliably read/write 10 entities in on
	// operation to no exceed the 10MB limit on single transactions.
	maxSliceSize := 10
	// Get a type that's an array of pointers to the instance we looked up.
	targetType := reflect.TypeOf(instance)
	if targetType.Kind() != reflect.Ptr {
		targetType = reflect.PtrTo(targetType)
	}
	targetType = reflect.SliceOf(targetType)

	// Use that array type in the function to process an input batch
	procFn := func(client *datastore.Client, keys []*datastore.Key) {
		ctx := context.TODO()
		sliceSize := util.MinInt(len(keys), maxSliceSize)
		targetArr := reflect.MakeSlice(targetType, sliceSize, sliceSize).Interface()
		if err := client.GetMulti(ctx, keys, targetArr); err != nil {
			sklog.Fatalf("Error touching slice: %s", err)
		}
		_, err := client.PutMulti(context.TODO(), keys, targetArr)
		if err != nil {
			sklog.Fatalf("Error touching slice: %s", err)
		}
	}

	processConcurrently(client, procFn, kind, maxSliceSize)
}
