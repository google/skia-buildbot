package main

// Gold init initializes the cloud so it's ready to host a gold instances,
// e.g. create the Bigtable tables required.
import (
	"flag"

	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sklog"
)

// TODO(stephana): Look into whether this should be done by a script.

// Command line flags.
var (
	btInstance = flag.String("bt_instance", "", "Bigtable instance to use in the project identified by 'project_id'")
	projectID  = flag.String("project_id", common.PROJECT_ID, "GCP project ID.")
)

func main() {
	common.Init()

	if err := bt.InitBigtable(*projectID, *btInstance, ingestion.BigTableConfig); err != nil {
		sklog.Fatalf("Error initializing bigtable")
	}
}
