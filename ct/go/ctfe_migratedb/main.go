package main

// Executes database migrations to the latest target version. In production this
// requires the root password for MySQL. The user will be prompted for that so
// it is not entered via the command line.

import (
	"flag"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/ct/go/db"
	"go.skia.org/infra/go/common"
)

var (
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promptPassword = flag.Bool("password", false, "Prompt for root password.")
	targetVersion  = flag.Int("target_version", -1, "Migration target version. Defaults to latest defined version.")
)

func main() {
	// Set up flags.
	dbConf := db.DBConfigFromFlags()

	// Global init to initialize glog and parse arguments.
	common.Init()

	if *promptPassword {
		if err := dbConf.PromptForPassword(); err != nil {
			glog.Fatal(err)
		}
	}
	if !*local {
		if err := dbConf.GetPasswordFromMetadata(); err != nil {
			glog.Fatal(err)
		}
	}
	vdb, err := dbConf.NewVersionedDB()
	if err != nil {
		glog.Fatal(err)
	}

	if *targetVersion < 0 {
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
	} else {
		glog.Infof("Migrating to version: %d", *targetVersion)
		err = vdb.Migrate(*targetVersion)
		if err != nil {
			glog.Fatalf("Unable to retrieve database version. Error: %s", err)
		}
	}
	glog.Infoln("Database migration finished.")
}
