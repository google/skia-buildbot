package main

// Executes database migrations to the latest target version. In production this
// requires the root password for MySQL. The user will be prompted for that so
// it is not entered via the command line.

import (
	"flag"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/database"
)

var (
	local = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
)

func main() {
	// Set up flags.
	database.SetupFlags(buildbot.PROD_DB_HOST, buildbot.PROD_DB_PORT, database.USER_ROOT, buildbot.PROD_DB_NAME)

	// Global init to initialize glog and parse arguments.
	common.Init()

	pw, err := database.PromptForPassword()
	if err != nil {
		glog.Fatal(err)
	}
	conf, err := database.ConfigFromFlags(pw, *local, buildbot.MigrationSteps())
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

	glog.Infoln("Database migration finished.")
}
