package main

// Executes database migrations to the latest target version. In production this
// requires the root password for MySQL. The user will be prompted for that so
// it is not entered via the command line.

import (
	"flag"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/golden/go/db"
)

var (
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promptPassword = flag.Bool("password", false, "Prompt for root password.")
)

func main() {
	defer common.LogPanic()
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

	glog.Infoln("Database migration finished.")
}
