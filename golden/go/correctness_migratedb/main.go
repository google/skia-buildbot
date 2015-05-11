package main

// Executes database migrations to the latest target version. In production this
// requires the root password for MySQL. The user will be prompted for that so
// it is not entered via the command line.

import (
	"flag"
	"reflect"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/golden/go/db"
	"go.skia.org/infra/golden/go/expstorage"
)

var (
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promptPassword = flag.Bool("password", false, "Prompt for root password.")
)

func main() {
	// Set up flags.
	dbConf := database.ConfigFromFlags(db.PROD_DB_HOST, db.PROD_DB_PORT, database.USER_ROOT, db.PROD_DB_NAME, db.MigrationSteps())

	// Global init to initialize glog and parse arguments.
	common.Init()

	v, err := skiaversion.GetVersion()
	if err != nil {
		glog.Fatalf("Unable to retrieve version: %s", err)
	}
	glog.Infof("Version %s, built at %s", v.Commit, v.Date)

	if *promptPassword {
		if err := dbConf.PromptForPassword(); err != nil {
			glog.Fatal(err)
		}
	}
	vdb, err := dbConf.NewVersionedDB()
	if err != nil {
		glog.Fatal(err)
	}

	// Get the current database version
	maxDBVersion := vdb.MaxDBVersion()
	glog.Infof("Latest database version: %d", maxDBVersion)

	dbVersion, err := vdb.DBVersion()
	if err != nil {
		glog.Fatalf("Unable to retrieve database version. Error: %s", err)
	}
	glog.Infof("Current database version: %d", dbVersion)

	if dbVersion < maxDBVersion {
		glog.Infof("Migrating to version: %d", maxDBVersion)
		err = vdb.Migrate(maxDBVersion)
		if err != nil {
			glog.Fatalf("Unable to retrieve database version. Error: %s", err)
		}
	}

	// Trigger migration scripts.
	migrate_expectation_storage(vdb)

	glog.Infoln("Database migration finished.")
}

// TODO(stephana): Remove migrate_expectation_storage when the original
// expectations table is removed from db.go.

// migrate_expectation_storage converts entries in the expectations table
// to the finer grained entries in the exp_change and exp_test_change tables.
// It works incrementally (continuing an interrupted migration) and
// is idempotent.
func migrate_expectation_storage(vdb *database.VersionedDB) {
	glog.Info("Expectations migration started.")

	const checkEmtpyStmt = `SELECT COUNT(*) FROM exp_change`
	const checkOldTableStmt = `SELECT COUNT(*) FROM expectations`
	const checkTimeStampStmt = `SELECT COUNT(*) FROM exp_change WHERE ts=?`

	v1ExpStore := expstorage.NewSQLExpectationStoreV1(vdb)
	sqlExpStore := expstorage.NewSQLExpectationStore(vdb).(*expstorage.SQLExpectationsStore)

	var count uint64
	var last *expstorage.Expectations = nil
	var veryLastCurrentExp *expstorage.Expectations

	// Tracks if we have skipped all records that have already been migrated.
	// We assume that we only can skip the first N records and we should not
	// skip anything after we encounter the first unskippable record.
	skippingFinished := true

	// Check if the exp_change is empty.
	row := vdb.DB.QueryRow(checkEmtpyStmt)
	err := row.Scan(&count)
	if err != nil {
		glog.Errorf("Unable to check if exp_change table is empty: %s", err)
		return
	}

	glog.Infof("Found %d records in exp_change table.", count)

	// If we have already expectations in the new table we want to load them.
	if count > 0 {
		last, err = sqlExpStore.Get()
		if err != nil {
			glog.Errorf("Unable to read expectations: %s", err)
			return
		}
		skippingFinished = false
	}

	counter := 0
	skipCounter := 0
	for curr := range v1ExpStore.IterExpectations() {
		iterationTimer := timer.New("Iteration")
		// Fail if any error occurs.
		if curr.Err != nil {
			glog.Errorf("Error iterating expectations: %s", curr.Err)
			return
		}

		// Check if the current timestamp is already in the exp_change table.
		row := vdb.DB.QueryRow(checkTimeStampStmt, curr.TS)
		err := row.Scan(&count)
		if err != nil {
			glog.Errorf("Unable to check if timestamp in exp_change table: %s", err)
			return
		}

		// If there is more than one we have a problem since these are VERY likely
		// to be unique.
		if count > 1 {
			glog.Errorf("Expected at most one row with timestamp %d", curr.TS)
			return
		}

		// Store the very last expectations of the old expectations store.
		// This is to cover the case where we skip all records.
		veryLastCurrentExp = curr.Exp

		// We have already migrated this entry.
		if count > 0 {
			// If this is true we have encountered at leas one unskipped record.
			if skippingFinished {
				glog.Errorf("Unexpected skipping of record %s %d %s", time.Unix(curr.TS/1000, 0), curr.TS, curr.UserID)
				return
			}
			glog.Infof("SKIPPING:  %s %d %s", time.Unix(curr.TS/1000, 0), curr.TS, curr.UserID)
			skipCounter++
			continue
		}

		skippingFinished = true

		var addExp *expstorage.Expectations
		var removeDigests map[string][]string
		if last == nil {
			addExp = curr.Exp
		} else {
			addExp, removeDigests = last.Delta(curr.Exp)
		}

		// Add the changes.
		if err = sqlExpStore.AddChangeWithTimeStamp(addExp.Tests, curr.UserID, curr.TS); err != nil {
			glog.Errorf("Unable to add expectations delta: %s", err)
			return
		}
		if err = sqlExpStore.RemoveChange(removeDigests); err != nil {
			glog.Errorf("Unable to remove expectations delta: %s", err)
			return
		}

		t := timer.New("Getting Expectations")
		// Make sure the new expectations match what's expected.
		last, err = sqlExpStore.Get()
		if err != nil {
			glog.Errorf("Unable to read expectations: %s", err)
			return
		}
		t.Stop()

		t = timer.New("Comparing Expectations")
		if !reflect.DeepEqual(last.Tests, curr.Exp.Tests) {
			glog.Errorf("Expected values do not match !")
			return
		}
		t.Stop()

		iterationTimer.Stop()
		glog.Infof("Migrated: %s  %s (%d/%d) - %d", time.Unix(curr.TS/1000, 0), curr.UserID, len(addExp.Tests), len(removeDigests), curr.TS)
		counter++
	}

	if !reflect.DeepEqual(last, veryLastCurrentExp) {
		glog.Errorf("The latest Expected values do not match !")
		return
	}

	// Get the size of the old table and new table and make sure they match
	var oldTableSize, newTableSize int
	row = vdb.DB.QueryRow(checkEmtpyStmt)
	err = row.Scan(&newTableSize)
	if err != nil {
		glog.Errorf("Unable to check if exp_change table is empty: %s", err)
		return
	}

	row = vdb.DB.QueryRow(checkOldTableStmt)
	err = row.Scan(&oldTableSize)
	if err != nil {
		glog.Errorf("Unable to check if exp_change table is empty: %s", err)
		return
	}
	if oldTableSize != newTableSize {
		glog.Errorf("Table sizes differ. Got %d but expected %d", newTableSize, oldTableSize)
		return
	}

	glog.Infof("Skipped %d changes.", skipCounter)
	glog.Infof("Migrated %d changes.", counter)
	glog.Info("Expectations migrated successfully.")
}
