package main

import (
	"flag"
	"math"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/datastore"

	"go.skia.org/infra/ct/go/db"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/types"
)

// Command line flags
var (
	dsNamespace    = flag.String("ds_namespace", "", "Cloud datastore namespace to be used by this instance.")
	projectID      = flag.String("project_id", common.PROJECT_ID, "GCP project ID.")
	promptPassword = flag.Bool("password", false, "Prompt for root password.")
)

// List of entities we are importing
var targetKinds = []ds.Kind{
	ds.MASTER_EXP_CHANGE,
	ds.MASTER_TEST_DIGEST_EXP,
	ds.IGNORE_RULE,
	ds.HELPER_RECENT_KEYS,
	ds.EXPECTATIONS_BLOB,
}

func main() {
	// Configure the MySQL database
	dbConf := database.ConfigFromFlags(db.PROD_DB_HOST, db.PROD_DB_PORT, database.USER_ROOT, db.PROD_DB_NAME, db.MigrationSteps())

	// Global init to initialize logging and parse arguments.
	common.Init()
	skiaversion.MustLogVersion()

	// Set up the SQL based expectations store
	vdb := setupMySQL(dbConf, *promptPassword)

	// Set up the cloud data store based expectations store
	// Needed to use TimeSortableKey(...) which relies on an RNG. See docs there.
	rand.Seed(time.Now().UnixNano())
	if err := ds.InitWithOpt(*projectID, *dsNamespace); err != nil {
		sklog.Fatalf("Unable to configure cloud datastore: %s", err)
	}
	dsClient := ds.DS

	// Remove entities from previous runs.
	removeExistingEntities(dsClient, targetKinds)

	// Migrate the expectation and ignore stores
	migrateExpectationStore(vdb, dsClient)
	migrateIgnoreStore(vdb, dsClient)

	sklog.Infoln("Database migration finished.")
}

func removeExistingEntities(dsClient *datastore.Client, targetKinds []ds.Kind) {
	// // Print out the current entity counts
	// for _, kind := range targetKinds {
	// 	printCount(dsClient, kind)
	// }

	// Remove all instances that might be there from a previous migration run.
	sklog.Infof("Removing old entries")
	var wg sync.WaitGroup
	for _, kind := range targetKinds {
		wg.Add(1)
		go func(kind ds.Kind) {
			defer wg.Done()
			removeCount, err := ds.DeleteAll(dsClient, kind, true)
			if err != nil {
				sklog.Fatalf("Error deleting entities of kind %s: %s", kind, err)
			}
			sklog.Infof("Removed %d %s entities", removeCount, kind)
		}(kind)
	}
	wg.Wait()
	sklog.Infof("Done removing old entries")
}

func migrateExpectationStore(vdb *database.VersionedDB, dsClient *datastore.Client) {
	sqlExpStore := expstorage.NewSQLExpectationStore(vdb)
	newExpStore, _, err := expstorage.NewCloudExpectationsStore(dsClient, nil)
	if err != nil {
		sklog.Fatalf("Unable to create cloud expectations store: %s", err)
	}

	// Get the cloud datastore directly to sideload the data via functions that
	// are not part of the ExpectationsStore interface.
	cloudExpStore := newExpStore.(*expstorage.CloudExpStore)

	// // TODO remove
	// func() {
	// 	sqlExpectations, err := sqlExpStore.Get()
	// 	if err != nil {
	// 		sklog.Fatalf("Unable to retrieve sql expectations: %s", err)
	// 	}

	// 	sklog.Infof("Writing expectations")
	// 	if err := cloudExpStore.PutExpectations(sqlExpectations.Tests); err != nil {
	// 		sklog.Fatalf("Error writting calculated expectations: %s", err)
	// 	}
	// 	sklog.Infof("Done writing expectations")
	// 	os.Exit(0)
	// }()

	// Get the total number of expectation changes and divide them into pages.
	_, total, err := sqlExpStore.QueryLog(0, 1, false)
	// total = 4
	pageSize := 1000

	changeRecCount := 0
	totalChangeCount := int32(0)
	lastTS := int64(0)
	nPages := int(math.Ceil(float64(total) / float64(pageSize)))
	sklog.Infof("Found %d change records. Fetching %d pages", total, nPages)
	importChanges := make([]map[string]types.TestClassification, total)
	localExps := expstorage.NewExpectations()

	for p := nPages - 1; p >= 0; p-- {
		first := p * pageSize
		logEntries, _, err := sqlExpStore.QueryLog(first, util.MinInt(pageSize, total), true)
		if err != nil {
			sklog.Fatalf("Error retrieving expectation changes: %s", err)
		}

		var wg sync.WaitGroup
		startTime := time.Now()
		for i := len(logEntries) - 1; i >= 0; i-- {
			entry := logEntries[i]
			if entry.TS < lastTS {
				sklog.Fatalf("TS not increasing monotonically. Change %d has time stamp %d following %d", entry.ID, entry.TS, lastTS)
			}
			lastTS = entry.TS

			wg.Add(1)
			go func(idx int, entry *expstorage.TriageLogEntry) {
				defer wg.Done()

				changes := entry.GetChanges()
				err := cloudExpStore.ImportChange(changes, entry.Name, entry.TS)
				if err != nil {
					sklog.Fatalf("Error adding expectation change: %s", err)
				}
				importChanges[idx] = changes
				atomic.AddInt32(&totalChangeCount, int32(len(entry.Details)))
			}(first+i, entry)
		}
		wg.Wait()

		for i := first + len(logEntries) - 1; i >= first; i-- {
			if importChanges[i] == nil {
				sklog.Fatalf("Error nil changes found")
			}
			localExps.AddDigests(importChanges[i])
		}

		avgTime := float64(time.Now().Sub(startTime)/time.Millisecond) / float64(len(logEntries))
		changeRecCount += len(logEntries)
		sklog.Infof("Migrated %d/%d records. %.2f ms average", changeRecCount, total, avgTime)
	}
	sklog.Infof("Done Migrating for now")
	foundExp := expstorage.NewExpectations()
	for i := len(importChanges) - 1; i >= 0; i-- {
		foundExp.AddDigests(importChanges[i])
	}

	sqlExpectations, err := sqlExpStore.Get()
	if err != nil {
		sklog.Fatalf("Unable to retrieve sql expectations: %s", err)
	}

	if !deepequal.DeepEqual(foundExp, localExps) {
		sklog.Fatalf("Found expectations and expectations from SQL do not match.")
	}

	// failure := false
	// sqlExp := expstorage.NewExpectations()
	// cloudExp := expstorage.NewExpectations()
	// allSQLEntriesIdx := 0
	// for p := nPages - 1; p >= 0; p-- {
	// 	first := p * pageSize
	// 	logEntries, _, err := sqlExpStore.QueryLog(first, pageSize, true)
	// 	if err != nil {
	// 		sklog.Fatalf("Error retrieving expectation changes: %s", err)
	// 	}

	// 	cloudLogEntries, _, err := newExpStore.QueryLog(first, pageSize, true)
	// 	if err != nil {
	// 		sklog.Fatalf("Error retrieving cloud expectation changes: %s", err)
	// 	}

	// 	sklog.Infof("Sizes: %d     %d", len(logEntries), len(cloudLogEntries))

	// 	for idx := len(logEntries) - 1; idx >= 0; idx-- {
	// 		sqlLogEntry := logEntries[idx]
	// 		if !deepequal.DeepEqual(sqlLogEntry, allSQLEntries[allSQLEntriesIdx]) {
	// 			fmt.Printf("\n\n\n%s\n\n\n%s\n\n\n", spew.Sdump(sqlLogEntry), spew.Sdump(allSQLEntries[allSQLEntriesIdx]))
	// 			sklog.Fatal("SQL entries in wrong order.")
	// 		}
	// 		allSQLEntriesIdx++

	// 		cloudLogEntry := cloudLogEntries[idx]

	// 		normalizeEntry(cloudLogEntry)
	// 		normalizeEntry(sqlLogEntry)
	// 		if !deepequal.DeepEqual(sqlLogEntry, cloudLogEntry) {
	// 			failure = true
	// 			fmt.Printf("Entries do not match:\n\n%s\n\n%s\n\n", spew.Sdump(sqlLogEntry), spew.Sdump(cloudLogEntry))
	// 		}
	// 		sqlExp.AddDigests(sqlLogEntry.GetChanges())
	// 		cloudExp.AddDigests(cloudLogEntry.GetChanges())
	// 	}
	// }

	// fmt.Printf("Done comparing entries.\n")

	// if failure {
	// 	return
	// }

	// if !deepequal.DeepEqual(sqlExp, cloudExp) {
	// 	sklog.Fatal("locally computed expectations do not match")
	// } else {
	// 	fmt.Printf("HURRAY !!!")
	// }

	// if !deepequal.DeepEqual(sqlExp, localChanges) {
	// 	sklog.Fatal("locally computed sql expectations do not match")
	// } else {
	// 	fmt.Printf("HURRAY 2 !!!")
	// }

	// Waiting for consistency.
	// for {
	// 	count, err := ds.Count(dsClient, ds.MASTER_EXP_CHANGE)
	// 	if err != nil {
	// 		sklog.Fatalf("ERROR counting: %s")
	// 	}
	// 	sklog.Infof("Count: %d\n", count)
	// 	if count >= total {
	// 		break
	// 	}
	// 	// Note: Testing showed that it takes about 2-3 minutes to reach consistency
	// 	// when inserting about 162k records rapidly.
	// 	time.Sleep(30 * time.Second)
	// }
	sklog.Infof("Database is consistent.")

	// Make sure the expectations are correct in the cloud datastore.
	if err := cloudExpStore.PutExpectations(foundExp.Tests); err != nil {
		sklog.Fatalf("Error writting calculated expectations: %s", err)
	}
	sklog.Infof("Done calculating expectations")

	cloudExpectations, err := newExpStore.Get()
	if err != nil {
		sklog.Fatalf("Unable to retrieve cloud expectations: %s", err)
	}
	sklog.Infof("Retrieved calculated cloud expectations")

	// for testName, digests := range cloudExpectations.Tests {
	// 	fmt.Printf("TEST: %s\n", testName)
	// 	for digest, label := range digests {
	// 		_, ok := cloudExpectations.Tests[testName][digest]
	// 		fmt.Printf("     %s: %s * %v\n", digest, label.String(), ok)
	// 	}
	// }
	// fmt.Printf("--------------------------\n")
	// fmt.Printf("--------------------------\n")
	// fmt.Printf("--------------------------\n")

	// for testName, digests := range sqlExpectations.Tests {
	// 	fmt.Printf("TEST: %s\n", testName)
	// 	for digest, label := range digests {
	// 		_, ok := cloudExpectations.Tests[testName][digest]
	// 		fmt.Printf("     %s: %s * %v\n", digest, label.String(), ok)
	// 	}
	// }

	// for testName, digests := range sqlExpectations.Tests {
	// 	foundTest, ok := cloudExpectations.Tests[testName]
	// 	if !ok {
	// 		fmt.Printf("Unable to find test %s\n", testName)
	// 	} else {
	// 		for digest, label := range digests {
	// 			foundLabel := foundTest[digest]
	// 			if foundLabel != label {
	// 				fmt.Printf("%s: %s : %s : %s do not match\n", testName, digest, label.String(), foundLabel.String())
	// 			}
	// 		}
	// 	}
	// }
	// fmt.Printf("ALL DONE CHECKING LABELS\n")

	if !deepequal.DeepEqual(sqlExpectations, cloudExpectations) {
		sklog.Fatalf("Expectations do not match.")
	}
	sklog.Infof("Summary: migrated %d expectation changes with %d expectation values changes", total, totalChangeCount)
}

func printCount(dsClient *datastore.Client, kind ds.Kind) {
	count, err := ds.Count(dsClient, kind)
	if err != nil {
		sklog.Fatalf("Error retrieving count: %s", err)
	}
	sklog.Infof("Found %d entities of kind %s", count, kind)
}

func normalizeEntry(e *expstorage.TriageLogEntry) {
	sort.Slice(e.Details, func(i, j int) bool {
		return (e.Details[i].TestName < e.Details[j].TestName) ||
			((e.Details[i].TestName == e.Details[j].TestName) && (e.Details[i].Digest < e.Details[j].Digest))
	})
	e.ID = 0
	e.TS = 0
	e.ChangeCount = 0
	e.UndoChangeID = 0
}

func migrateIgnoreStore(vdb *database.VersionedDB, dsClient *datastore.Client) {
	sqlIgnoreStore := ignore.NewSQLIgnoreStore(vdb, nil, nil)
	cloudIgnoreStore, err := ignore.NewCloudIgnoreStore(dsClient, nil, nil)
	if err != nil {
		sklog.Fatalf("Error creating CloudIgnoreStore: %s", err)
	}

	ignoreRules, err := sqlIgnoreStore.List(false)
	if err != nil {
		sklog.Fatalf("Error retrieving ignore rules: %s", err)
	}

	for _, rule := range ignoreRules {
		if err := cloudIgnoreStore.Create(rule); err != nil {
			sklog.Fatalf("Error creating new ignore rule: %s", err)
		}
	}

	cloudIgnoreRules, err := cloudIgnoreStore.List(false)
	if err != nil {
		sklog.Fatalf("Error retrieving ignore rules: %s", err)
	}
	if !deepequal.DeepEqual(ignoreRules, cloudIgnoreRules) {
		sklog.Fatalf("Ignore rules do not match !")
	}
	sklog.Infof("Migrated %d ignore rules", len(ignoreRules))
}

// Initialize the MySQL wrapper
func setupMySQL(dbConf *database.DatabaseConfig, promptPassword bool) *database.VersionedDB {
	if promptPassword {
		if err := dbConf.PromptForPassword(); err != nil {
			sklog.Fatal(err)
		}
	}
	vdb, err := dbConf.NewVersionedDB()
	if err != nil {
		sklog.Fatal(err)
	}
	return vdb
}
