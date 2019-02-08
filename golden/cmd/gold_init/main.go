package main

// Gold init initializes the cloud so it's ready to host a gold instances,
// e.g. create the Bigtable tables required.
import (
	"flag"

	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/litegit"
	"go.skia.org/infra/go/sklog"
)

// TODO(stephana): Look into whether this should be done by a script.

// Command line flags.
var (
	projectID     = flag.String("project_id", common.PROJECT_ID, "GCP project ID.")
	btInstance    = flag.String("instance", "", "Bigtable instance to use in the project identified by 'project_id'")
	btTable       = flag.String("table", "", "Bigtable table to use to store app data.")
	gitBtInstance = flag.String("git_instance", "", "Bigtable instance that stores git repo data.")
	gitTable      = flag.String("git_table", "", "Bigtable table that stores git repo data")
)

func main() {
	common.Init()

	if err := bt.InitBigtable(*projectID, *btInstance, *btTable, ingestion.ColumnFamilies); err != nil {
		sklog.Fatalf("Error initializing data table %s: %s", *btTable, err)
	}

	if *gitBtInstance != "" {
		if err := bt.InitBigtable(*projectID, *gitBtInstance, *gitTable, litegit.ColumnFamilies); err != nil {
			sklog.Fatalf("Error initializing git data table %s: ", *gitTable)
		}
	}
}
