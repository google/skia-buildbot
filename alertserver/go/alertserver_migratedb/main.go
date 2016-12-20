package main

// Executes database migrations to the latest target version. In production this
// requires the root password for MySQL. The user will be prompted for that so
// it is not entered via the command line.

import (
	"flag"

	"go.skia.org/infra/alertserver/go/alerting"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/sklog"
)

var (
	local = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
)

func main() {
	defer common.LogPanic()
	// Set up flags.
	dbConf := database.ConfigFromFlags(alerting.PROD_DB_HOST, alerting.PROD_DB_PORT, database.USER_ROOT, alerting.PROD_DB_NAME, alerting.MigrationSteps())

	// Global init to initialize glog and parse arguments.
	common.Init()

	if err := dbConf.PromptForPassword(); err != nil {
		sklog.Fatal(err)
	}
	vdb, err := dbConf.NewVersionedDB()
	if err != nil {
		sklog.Fatal(err)
	}

	// Get the current database version
	maxDBVersion := vdb.MaxDBVersion()
	sklog.Infof("Latest database version: %d", maxDBVersion)

	dbVersion, err := vdb.DBVersion()
	if err != nil {
		sklog.Fatalf("Unable to retrieve database version. Error: %s", err)
	}
	sklog.Infof("Current database version: %d", dbVersion)

	if dbVersion < maxDBVersion {
		sklog.Infof("Migrating to version: %d", maxDBVersion)
		err = vdb.Migrate(maxDBVersion)
		if err != nil {
			sklog.Fatalf("Unable to retrieve database version. Error: %s", err)
		}
	}

	sklog.Infoln("Database migration finished.")
}
