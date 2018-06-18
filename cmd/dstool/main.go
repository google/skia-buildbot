package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
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

		"list": &opsEntry{
			params: nil,
			fn:     listKinds,
		},
	}

	registeredEntities = map[ds.Kind]interface{}{
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

func deleteEntities(client *datastore.Client, params ...string) {
	entityName := ds.Kind(params[0])
	sklog.Infof("Deleting all instance of %s", entityName)

	pageSize := 10000
	iterCh, err := ds.IterKeys(client, entityName, pageSize)
	if err != nil {
		sklog.Fatalf("Error getting key iterator: %s", err)
	}

	seen := map[string]bool{}
	totalCount := int32(0)

	concurrentDel := make(chan bool, 100)
	var wg sync.WaitGroup

	byParent := map[int64][]*datastore.Key{}
	currParentID := int64(-1)

	go func() {
		for range time.Tick(5 * time.Second) {
			sklog.Infof("Deleted %d", atomic.LoadInt32(&totalCount))
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
			deleteSlice := []*datastore.Key(nil)
			if parentID != currParentID {
				deleteSlice = byParent[currParentID]
				delete(byParent, currParentID)
				currParentID = parentID
			}

			if len(byParent[0]) == 500 {
				deleteSlice = byParent[0]
				byParent[0] = nil
			}

			if len(deleteSlice) > 0 {
				sklog.Infof("Deleting %d entries. Map: %d %d Concurrency: %d", len(deleteSlice), len(byParent), len(byParent[parentID]), len(concurrentDel))
				doDelete(client, &wg, concurrentDel, deleteSlice, &totalCount)
			}
		}
	}

	// Clean out any straggling keys.
	for ID, keySlice := range byParent {
		if len(keySlice) > 0 {
			sklog.Infof("Deleting %d entries. Map: %d %d Concurrency: %d", len(keySlice), len(byParent), len(byParent[ID]), len(concurrentDel))
			doDelete(client, &wg, concurrentDel, keySlice, &totalCount)
		}
	}

	wg.Wait()
	sklog.Infof("Deleted %d instances of %s", totalCount, entityName)
}

func doDelete(client *datastore.Client, wg *sync.WaitGroup, concurrentDel chan bool, deleteSlice []*datastore.Key, totalCount *int32) {
	concurrentDel <- true
	wg.Add(1)
	go func(deleteSlice []*datastore.Key) {
		defer wg.Done()
		defer func() { <-concurrentDel }()

		for len(deleteSlice) > 0 {
			targetSlice := deleteSlice[0:util.MinInt(500, len(deleteSlice))]
			if err := client.DeleteMulti(context.TODO(), targetSlice); err != nil {
				sklog.Fatalf("Error deleting slice: %s", err)
			}

			atomic.AddInt32(totalCount, int32(len(targetSlice)))
			deleteSlice = deleteSlice[len(targetSlice):]
			if len(deleteSlice) > 0 {
				time.Sleep(1500 * time.Millisecond)
			}
		}
	}(deleteSlice)
}

func touchEntities(client *datastore.Client, params ...string) {
	kind := ds.Kind(params[0])
	instance, ok := registeredEntities[kind]
	if !ok {
		sklog.Fatalf("Kind %s is not registered with a datastructure  and can therefore not be used with the 'touch' command.")
	}

	iterCh, err := ds.IterKind(client, kind, instance)
	if err != nil {
		sklog.Fatalf("Unable to retrieve iterator: %s", err)
	}

	ctx := context.TODO()
	total := int32(0)
	success := int32(0)
	var wg sync.WaitGroup
	for item := range iterCh {
		wg.Add(1)
		go func(item *ds.Item) {
			defer wg.Done()
			_, err := client.Put(ctx, item.Key, item.Instance)
			if err != nil {
				sklog.Errorf("Error writing record: %s", err)
			} else {
				atomic.AddInt32(&success, 1)
			}
			currTotal := atomic.AddInt32(&total, 1)
			if currTotal%1000 == 0 {
				sklog.Infof("%d / %d records processed successfully", atomic.LoadInt32(&success), currTotal)
			}
		}(item)
	}
	wg.Wait()
	sklog.Info("%d entities touched", total)
}

func listKinds(client *datastore.Client, params ...string) {
	query := datastore.NewQuery("__kind__").KeysOnly()
	keys, err := client.GetAll(context.TODO(), query, nil)
	if err != nil {
		sklog.Fatalf("Error retrieving kinds: %s", err)
	}

	sklog.Infof("List of Kinds in the project:")
	for _, k := range keys {
		if !strings.HasPrefix(k.Name, "_") {
			sklog.Infof("   %s", k.Name)
		}
	}
}
