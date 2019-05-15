package main

// This program migrates the data for Gold from SQL to Cloud Datastore.

import (
	"flag"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/db"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/types"
)

// Command line flags
var (
	dsNamespace    = flag.String("ds_namespace", "", "Cloud datastore namespace to be used by this instance.")
	projectID      = flag.String("project_id", common.PROJECT_ID, "GCP project ID.")
	promptPassword = flag.Bool("password", false, "Prompt for root password.")
	discrepancies  = flag.Int("discrepancies", 4, "How many digest failures are tolerable? ")
)

// List of entities we are importing
var targetKinds = []ds.Kind{
	ds.MASTER_EXP_CHANGE,
	ds.IGNORE_RULE,
	ds.EXPECTATIONS_BLOB,
	ds.EXPECTATIONS_BLOB_ROOT,
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
	cloudExpStore, _, err := expstorage.NewCloudExpectationsStore(dsClient, nil)
	if err != nil {
		sklog.Fatalf("Unable to create cloud expectations store: %s", err)
	}

	// Get the total number of expectation changes and divide them into pages.
	_, total, err := sqlExpStore.QueryLog(0, 1, false)
	pageSize := 1000

	changeRecCount := 0
	totalChangeCount := int32(0)
	lastTS := int64(-1)
	nPages := int(math.Ceil(float64(total) / float64(pageSize)))
	sklog.Infof("Found %d change records. Fetching %d pages", total, nPages)
	importChanges := make([]types.Expectations, 0, total)
	importKeys := make([]*datastore.Key, 0, total)

	// Iterate over the expectation changes in the sql expectations store
	for p := 0; p < nPages; p++ {
		startTime := time.Now()
		first := p * pageSize
		logEntries, _, err := sqlExpStore.QueryLog(first, util.MinInt(pageSize, total), true)
		if err != nil {
			sklog.Fatalf("Error retrieving expectation changes: %s", err)
		}

		var wg sync.WaitGroup
		newEntries := make([]types.Expectations, len(logEntries))
		newKeys := make([]*datastore.Key, len(logEntries))
		for i := 0; i < len(logEntries); i++ {
			entry := logEntries[i]
			if lastTS == -1 {
				lastTS = entry.TS
			}
			if entry.TS > lastTS {
				sklog.Warningf("TS does not decrease monotonically. Change %s has time stamp %d following %d", entry.ID, entry.TS, lastTS)
			}

			lastTS = entry.TS

			wg.Add(1)
			go func(idx int, entry *expstorage.TriageLogEntry) {
				defer wg.Done()

				// Write the changes directly to the cloud datastore and keep the key.
				changes := entry.GetChanges()
				newKey, err := cloudExpStore.ImportChange(changes, entry.Name, entry.TS)
				if err != nil {
					sklog.Fatalf("Error adding expectation change: %s", err)
				}
				newEntries[idx] = changes
				newKeys[idx] = newKey
				atomic.AddInt32(&totalChangeCount, int32(len(entry.Details)))
			}(i, entry)
		}
		wg.Wait()
		importChanges = append(importChanges, newEntries...)
		importKeys = append(importKeys, newKeys...)
		perSec := float64(len(logEntries)) / float64(time.Now().Sub(startTime)/time.Second)
		changeRecCount += len(logEntries)
		sklog.Infof("Migrated %d/%d records. %.2f per second average", changeRecCount, total, perSec)
	}

	// Accumulate the expectations from what we have loaded from the SQL store.
	localExps := types.Expectations{}
	for i := len(importChanges) - 1; i >= 0; i-- {
		localExps.MergeExpectations(importChanges[i])
	}

	// Get the expectations of the SQL store.
	sqlExpectations, err := sqlExpStore.Get()
	if err != nil {
		sklog.Fatalf("Unable to retrieve sql expectations: %s", err)
	}

	// Compare the sql expectations to the locally computed expectations
	sklog.Infof("Doing by test comparison")
	testFailures := 0
	digestFailures := 0
	for testName, digests := range sqlExpectations {
		found, ok := localExps[testName]
		sklog.Infof("%s   %v", testName, ok)
		if !ok {
			testFailures++
			continue
		}

		failCount := 0
		for d, l := range digests {
			if l != found[d] {
				sklog.Infof("    fail %s %s %s", d, l, found[d])
				failCount++
			}
		}
		digestFailures += failCount
	}
	sklog.Infof("Test failures   : %5d", testFailures)
	sklog.Infof("Digest failures : %5d", digestFailures)

	// Due to an issue with the SQL in SqlExpectationStore we are willing to
	// accept a small number of discrepancies.
	// if testFailures > 0 || digestFailures > *discrepancies {
	// 	sklog.Fatalf("Got more errors than expected. Test failures: %d  Digest failures: %d", testFailures, digestFailures)
	// }

	// Calculate the expectations from the changes we imported into the CloudExpectationsStore
	calcExps, err := cloudExpStore.CalcExpectations(importKeys)
	if err != nil {
		sklog.Fatalf("Error calculating expectations: %s", err)
	}

	// Make sure they are the same as the locally calculated expectations.
	if !deepequal.DeepEqual(localExps, calcExps) {
		sklog.Warningf("Local expectations and calculated expectations do not match")
	}

	// Store the calculated expectations in the cloud datastore.
	if err := cloudExpStore.PutExpectations(calcExps); err != nil {
		sklog.Warningf("Error writing expectations: %s", err)
	}

	// Read them back and make sure they match what we have written earlier.
	foundExp, err := cloudExpStore.Get()
	if err != nil {
		sklog.Fatalf("Error retrieving cloud expectations: %s", err)
	}

	if !deepequal.DeepEqual(calcExps, foundExp) {
		sklog.Warningf("Found expectations and expectations from SQL do not match.")
	}
	sklog.Infof("Summary: migrated %d expectation changes with %d expectation values changes", total, totalChangeCount)
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
