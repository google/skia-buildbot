package main

// Executes database migrations to the latest target version. In production this
// requires the root password for MySQL. The user will be prompted for that so
// it is not entered via the command line.

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/skia-dev/glog"
	"skia.googlesource.com/buildbot.git/go/common"
	"skia.googlesource.com/buildbot.git/go/database"
	"skia.googlesource.com/buildbot.git/golden/go/db"
	"skia.googlesource.com/buildbot.git/golden/go/expstorage"
)

var (
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promptPassword = flag.Bool("password", false, "Prompt for root password.")
)

func main() {
	// Set up flags.
	database.SetupFlags(db.PROD_DB_HOST, db.PROD_DB_PORT, database.USER_ROOT, db.PROD_DB_NAME)

	// Global init to initialize glog and parse arguments.
	common.Init()

	var conf *database.DatabaseConfig
	var err error
	var password string

	// TODO(stephana): The code to prompt for the password should be
	// merged with similar code in the DB package. Same for a copy of this
	// in perf_migratedb.
	if *promptPassword {
		fmt.Printf("Enter root password: ")
		reader := bufio.NewReader(os.Stdin)
		password, err = reader.ReadString('\n')
		password = strings.Trim(password, "\n")
		conf, err = database.ConfigFromFlags(password, *local, db.MigrationSteps())
	} else {
		conf, err = database.ConfigFromFlagsAndMetadata(*local, db.MigrationSteps())
	}
	if err != nil {
		glog.Fatal(err)
	}
	vdb := database.NewVersionedDB(conf)

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
func migrate_expectation_storage(vdb *database.VersionedDB) {
	// Only trigger this migration if the new db tables are empty
	// to make sure they are idempotent.
	const emptyStmt = `SELECT count(*) FROM exp_change`
	var count uint64

	row := vdb.DB.QueryRow(emptyStmt)
	err := row.Scan(&count)
	if err != nil {
		glog.Errorf("Unable to test if exp_change table is empty: %s", err)
		return
	}

	if count != 0 {
		glog.Info("Skipping migrate_expectation_storage since exp_change is not empty.")
		return
	}
	glog.Info("Expectations migration started.")

	v1ExpStore := expstorage.NewSQLExpectationStoreV1(vdb)
	expStore := expstorage.NewSQLExpectationStore(vdb).(*expstorage.SQLExpectationsStore)

	// Iterate over the past expecations and add them to new table.
	var last *expstorage.Expectations = nil
	for curr := range v1ExpStore.IterExpectations() {
		// Fail if any error occurs.
		if curr.Err != nil {
			glog.Errorf("Error iterating expectations: %s", curr.Err)
			return
		}

		var addExp *expstorage.Expectations
		var removeDigests map[string][]string
		if last == nil {
			addExp = curr.Exp
		} else {
			addExp, removeDigests = last.Delta(curr.Exp)
		}

		// Add the changes.
		if err = expStore.AddChangeWithTimeStamp(addExp.Tests, curr.UserID, curr.TS); err != nil {
			glog.Errorf("Unable to add expectations delta: %s", err)
			return
		}
		if err = expStore.RemoveChange(removeDigests); err != nil {
			glog.Errorf("Unable to remove expectations delta: %s", err)
			return
		}

		// Make sure the new expectations match what's expected.
		last, err = expStore.Get()
		if err != nil {
			glog.Errorf("Unable to read expectations: %s", err)
		}

		if !reflect.DeepEqual(last.Tests, curr.Exp.Tests) {
			glog.Errorf("Expected values do not match !")
			return
		}

		glog.Infof("Migrated: %s  %s", time.Unix(curr.TS/1000, 0), curr.UserID)
	}

	glog.Info("Expectations migrated successfully.")
}
