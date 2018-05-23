package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/davecgh/go-spew/spew"

	"go.skia.org/infra/golden/go/expstorage"

	"cloud.google.com/go/datastore"
	gstorage "google.golang.org/api/storage/v1"

	"go.skia.org/infra/ct/go/db"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"google.golang.org/api/option"
)

var (
	dsNamespace        = flag.String("ds_namespace", "", "Cloud datastore namespace to be used by this instance.")
	projectID          = flag.String("project_id", common.PROJECT_ID, "GCP project ID.")
	promptPassword     = flag.Bool("password", false, "Prompt for root password.")
	serviceAccountFile = flag.String("service_account_file", "", "Credentials file for service account.")
)

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
	dsClient := setupDataStore(*projectID, *dsNamespace, *serviceAccountFile)

	// Migrate the expectation and ignore stores
	migrateExpectationStore(vdb, dsClient)
	migrateIgnoreStore(vdb, dsClient)

	sklog.Infoln("Database migration finished.")
}

func migrateExpectationStore(vdb *database.VersionedDB, dsClient *datastore.Client) {
	sqlExpStore := expstorage.NewSQLExpectationStore(vdb)
	newExpStore, _, err := expstorage.NewCloudExpectationsStore(dsClient, nil)
	if err != nil {
		sklog.Fatalf("Unable to create cloud expectations store: %s", err)
	}

	// Disable updating of expecations for each commit, this makes the import
	// much faster and we calculate the expecations in the database at the end.
	cloudExpStore := newExpStore.(*expstorage.CloudExpStore)
	cloudExpStore.DisableExpUpdate()

	// Remove all instances that might be there from a previous migration run.
	sklog.Infof("Removing old entries")
	if err := ds.DeleteAll(dsClient, ds.MASTER_EXP_CHANGE, true); err != nil {
		sklog.Fatalf("Error deleting entities: %s", err)
	}
	sklog.Infof("Done removing old entries")

	// Get the total number of expectation changes and divide them into pages.
	_, total, err := sqlExpStore.QueryLog(0, 1, false)
	pageSize := 100

	// TODO: REMOVE FOR TESTING ONLY.
	// total = pageSize * 10
	localChanges := expstorage.NewExpectations()

	totalChangeRecs := 0
	lastTS := int64(0)
	nPages := int(math.Ceil(float64(total) / float64(pageSize)))
	sklog.Infof("Found %d change records. Fetching %d pages", total, nPages)
	allSQLEntries := make([]*expstorage.TriageLogEntry, 0, total)

	for p := nPages - 1; p >= 0; p-- {
		first := p * pageSize
		logEntries, _, err := sqlExpStore.QueryLog(first, pageSize, true)
		if err != nil {
			sklog.Fatalf("Error retrieving expectation changes: %s", err)
		}

		changeCount := 0
		avgTime := 0.0
		for i := len(logEntries) - 1; i >= 0; i-- {
			entry := logEntries[i]
			if entry.TS < lastTS {
				sklog.Fatalf("TS not increasing monotonically. Change %d has time stamp %d following %d", entry.ID, entry.TS, lastTS)
			}
			allSQLEntries = append(allSQLEntries, entry)

			lastTS = entry.TS
			changeCount += len(entry.Details)
			changes := entry.GetChanges()
			start := time.Now()
			if err := newExpStore.AddChange(changes, entry.Name); err != nil {
				sklog.Fatalf("Error adding expectation change: %s", err)
			}
			avgTime += float64(time.Now().Sub(start) / time.Millisecond)

			localChanges.AddDigests(changes)
		}
		totalChangeRecs += len(logEntries)
		sklog.Infof("Migrated %d/%d records with %d changes. %.2f ms average", totalChangeRecs, total, changeCount, avgTime/float64(len(logEntries)))
	}

	failure := false
	sqlExp := expstorage.NewExpectations()
	cloudExp := expstorage.NewExpectations()
	allSQLEntriesIdx := 0
	for p := nPages - 1; p >= 0; p-- {
		first := p * pageSize
		logEntries, _, err := sqlExpStore.QueryLog(first, pageSize, true)
		if err != nil {
			sklog.Fatalf("Error retrieving expectation changes: %s", err)
		}

		cloudLogEntries, _, err := newExpStore.QueryLog(first, pageSize, true)
		if err != nil {
			sklog.Fatalf("Error retrieving cloud expectation changes: %s", err)
		}

		sklog.Infof("Sizes: %d     %d", len(logEntries), len(cloudLogEntries))

		for idx := len(logEntries) - 1; idx >= 0; idx-- {
			sqlLogEntry := logEntries[idx]
			if !testutils.DeepEqual(sqlLogEntry, allSQLEntries[allSQLEntriesIdx]) {
				fmt.Printf("\n\n\n%s\n\n\n%s\n\n\n", spew.Sdump(sqlLogEntry), spew.Sdump(allSQLEntries[allSQLEntriesIdx]))
				sklog.Fatal("SQL entries in wrong order.")
			}
			allSQLEntriesIdx++

			cloudLogEntry := cloudLogEntries[idx]

			normalizeEntry(cloudLogEntry)
			normalizeEntry(sqlLogEntry)
			if !testutils.DeepEqual(sqlLogEntry, cloudLogEntry) {
				failure = true
				fmt.Printf("Entries do not match:\n\n%s\n\n%s\n\n", spew.Sdump(sqlLogEntry), spew.Sdump(cloudLogEntry))
			}
			sqlExp.AddDigests(sqlLogEntry.GetChanges())
			cloudExp.AddDigests(cloudLogEntry.GetChanges())
		}
	}

	fmt.Printf("Done comparing entries.\n")

	if failure {
		return
	}

	if !testutils.DeepEqual(sqlExp, cloudExp) {
		sklog.Fatal("locally computed expecations do not match")
	} else {
		fmt.Printf("HURRAY !!!")
	}

	if !testutils.DeepEqual(sqlExp, localChanges) {
		sklog.Fatal("locally computed sql expecations do not match")
	} else {
		fmt.Printf("HURRAY 2 !!!")
	}

	count, err := ds.Count(dsClient, ds.MASTER_EXP_CHANGE)
	if err != nil {
		sklog.Fatalf("ERROR counting: %s")
	}
	sklog.Infof("Count: %d\n", count)

	// Make sure the expectations are correct in the cloud datastore.
	if err := cloudExpStore.CalcExpectations(); err != nil {
		sklog.Fatalf("Error calculating expectations: %s", err)
	}

	// sqlExpectations, err := sqlExpStore.Get()
	// if err != nil {
	// 	sklog.Fatalf("Unable to retrieve sql expectations: %s", err)
	// }
	sqlExpectations := localChanges

	cloudExpectations, err := newExpStore.Get()
	if err != nil {
		sklog.Fatalf("Unable to retrieve cloud expectations: %s", err)
	}

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

	for testName, digests := range sqlExpectations.Tests {
		foundTest, ok := cloudExpectations.Tests[testName]
		if !ok {
			fmt.Printf("Unable to find test %s\n", testName)
		} else {
			for digest, label := range digests {
				foundLabel := foundTest[digest]
				if foundLabel != label {
					fmt.Printf("%s: %s : %s : %s do not match\n", testName, digest, label.String(), foundLabel.String())
				}
			}
		}
	}
	fmt.Printf("ALL DONE CHECKING LABELS\n")

	if testutils.DeepEqual(sqlExpectations, cloudExpectations) {
		sklog.Infof("Expectations match.")
	} else {
		sklog.Fatalf("Expectations do not match.")
	}
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

// Initialize the Cloud Datastore client
func setupDataStore(projectID, nameSpace, svcAccountFile string) *datastore.Client {
	opts := []option.ClientOption{}

	if svcAccountFile != "" {
		// Get the token source from the same service account.
		tokenSource, err := auth.NewJWTServiceAccountTokenSource("", svcAccountFile, gstorage.CloudPlatformScope)
		if err != nil {
			sklog.Fatalf("Failed to authenticate service account to get token source: %s", err)
		}
		opts = append(opts, option.WithTokenSource(tokenSource))
	}

	if err := ds.InitWithOpt(projectID, nameSpace, opts...); err != nil {
		sklog.Fatalf("Unable to configure cloud datastore: %s", err)
	}
	return ds.DS
}
