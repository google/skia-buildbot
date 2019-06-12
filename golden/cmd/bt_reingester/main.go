package main

// bt_reingester will scan through all the files in a GCS bucket and ingest
// them into the bt_tracestore
import (
	"flag"

	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sklog"
)

// TODO(stephana): Look into whether this should be done by a script.

// Command line flags.

func main() {
	var (
		btInstance = flag.String("bt_instance", "", "Bigtable instance to use in the project identified by 'project_id'")
		projectID  = flag.String("project_id", "skia-public", "GCP project ID.")
		btTableID  = flag.String("bt_table_id", "production", "BigTable table ID.")
	)
	flag.Parse()

	if err := bt.InitBigtable(*projectID, *btInstance, *btTableID, ingestion.ColumnFamilies); err != nil {
		sklog.Fatalf("Error initializing bigtable: %s", err)
	}
}
