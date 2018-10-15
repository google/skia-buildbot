package main

// This program allows to connect simple functions to command line arguments
// to perform maintenance tasks on Cloud Datastore namespaces.

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

// opsEntry defines the parameters and the function for an operation
type opsEntry struct {
	params []string
	fn     func(client *datastore.Client, params ...string)
}

var (
	// ops defines all the ops that are available on the command line
	ops = map[string]*opsEntry{
		// delete all entities of a kind
		"delete": &opsEntry{
			params: []string{"kind"},
			fn:     deleteEntities,
		},

		// touch (= load and save) all entities of a kind
		"touch": &opsEntry{
			params: []string{"kind"},
			fn:     touchEntities,
		},
	}

	// touchRegisteredEntities maps the entities to the types that should be
	// used to load and save the entities for the touch operation.
	touchRegisteredEntities = map[ds.Kind]interface{}{
		ds.MASTER_EXP_CHANGE: &expstorage.ExpChange{},
		ds.TRYJOB_EXP_CHANGE: &expstorage.ExpChange{},
		ds.TRYJOB_RESULT:     &tryjobstore.TryjobResult{},
	}
)

func main() {
	// Wire the printUsage function into the flags, so it is called when --help is passed.
	flag.Usage = func() { printUsage("", 0) }
	common.Init()

	if *dsNamespace == "" {
		printUsage("Cloud datastore namespace missing!", 1)
	}

	if err := ds.InitWithOpt(*projectID, *dsNamespace); err != nil {
		sklog.Fatalf("Error initializing cloud data store: %s", err)
	}
	dsClient := ds.DS

	userCmd := flag.Arg(0)
	op, ok := ops[userCmd]
	if !ok {
		printUsage(fmt.Sprintf("Unknown command: '%s'", userCmd), 1)
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

// deleteEntities implements delete command.
func deleteEntities(client *datastore.Client, params ...string) {
	entityName := ds.Kind(params[0])
	procFn := func(tx *datastore.Transaction, keys []*datastore.Key) {
		if err := tx.DeleteMulti(keys); err != nil {
			sklog.Errorf("Error deleting slice: %s", err)
		}
	}
	processConcurrently(client, procFn, entityName, 500)
}

// touchEntities implements the touch command.
func touchEntities(client *datastore.Client, params ...string) {
	kind := ds.Kind(params[0])

	instance, ok := touchRegisteredEntities[kind]
	if !ok {
		sklog.Fatalf("Kind %s is not registered with a datastructure  and can therefore not be used with the 'touch' command.", kind)
	}

	// maxSliceSize is 10 because we can reliably read/write 10 entities in an
	// operation to not exceed the 10MB limit on single transactions.
	maxSliceSize := 10
	// Get a type that's an array of pointers to the instance we looked up.
	targetType := reflect.TypeOf(instance)
	if targetType.Kind() != reflect.Ptr {
		targetType = reflect.PtrTo(targetType)
	}
	targetType = reflect.SliceOf(targetType)

	// Use that array type in the function to process an input batch
	procFn := func(tx *datastore.Transaction, keys []*datastore.Key) {
		sliceSize := util.MinInt(len(keys), maxSliceSize)
		targetArr := reflect.MakeSlice(targetType, sliceSize, sliceSize).Interface()
		if err := tx.GetMulti(keys, targetArr); err != nil {
			sklog.Fatalf("Error touching slice: %s", err)
		}
		_, err := tx.PutMulti(keys, targetArr)
		if err != nil {
			sklog.Errorf("Error touching slice: %s", err)
		}
	}
	processConcurrently(client, procFn, kind, maxSliceSize)
}

// procSliceFn processes a slice of keys of a parent in a transaction.
// It should assume that it can e.g. load or save all the keys at once. If
// an error occurs it's up to the function to abort (via sklog.Fatalf) or
// log an error.
type procSliceFn func(tx *datastore.Transaction, keys []*datastore.Key)

// processConcurrently calls the procFn function concurrently with keys grouped
// by parents. It will never pass more than maxSliceSize elements to the procFn
// function.
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

			for len(procSlice) > 0 {
				targetSlice := procSlice[0:util.MinInt(maxSliceSize, len(procSlice))]

				// Process the slice in a transaction. We rely on procFn to handle any
				// errors related to its task.
				_, err := client.RunInTransaction(context.TODO(), func(tx *datastore.Transaction) error {
					procFn(tx, targetSlice)
					return nil
				})

				// Errors caused by the transaction machinery cause us to abort the
				// program since our connection to the datastore has become unreliable.
				if err != nil {
					sklog.Fatalf("Error while running a transaction against the cloud datastore: %s", err)
				}

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
			sklog.Infof("Processed %d entities", atomic.LoadInt32(&totalCount))
		}
	}()

	for item := range iterCh {
		// If we encounter an error during iteration we consider our connection to
		// the datastore unreliable and abort the program.
		if item.Err != nil {
			sklog.Fatalf("Error iterating over keys: %s", item.Err)
		}
		keySlice := item.Keys
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
			} else if len(byParent[0]) >= maxSliceSize {
				// If we are processing entities without parents, we want to start
				// processing keys as soon as we have a large enough batch assembled to
				// avoid backlog.
				// Note: maxSliceSize is enforced in processKeysForOneParent so it's
				// fine if len(procSlice) is larger than maxSliceSize.
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

// printUsage prints the usage options and exits with the given return value.
func printUsage(errMsg string, returnVal int) {
	if errMsg != "" {
		fmt.Printf("\n    Error: %s\n\n", errMsg)
	}
	fmt.Printf("Usage: dstool [opts] cmd params\n\n")
	fmt.Printf("       Available commands:\n\n")
	for opName, op := range ops {
		fmt.Printf("            %s %s\n", opName, strings.Join(op.params, " "))
	}
	fmt.Printf("\nOptions:\n")
	flag.PrintDefaults()
	os.Exit(returnVal)
}
