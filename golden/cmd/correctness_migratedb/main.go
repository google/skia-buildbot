package main

// Executes database migrations to the latest target version. In production this
// requires the root password for MySQL. The user will be prompted for that so
// it is not entered via the command line.

import (
	"flag"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/db"
)

var (
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promptPassword = flag.Bool("password", false, "Prompt for root password.")
)

func main() {
	// Set up flags.
	dbConf := database.ConfigFromFlags(db.PROD_DB_HOST, db.PROD_DB_PORT, database.USER_ROOT, db.PROD_DB_NAME, db.MigrationSteps())

	// Global init to initialize logging and parse arguments.
	common.Init()
	skiaversion.MustLogVersion()

	if *promptPassword {
		if err := dbConf.PromptForPassword(); err != nil {
			sklog.Fatal(err)
		}
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
