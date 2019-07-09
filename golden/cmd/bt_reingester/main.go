package main

// bt_reingester will scan through all the files in a GCS bucket and ingest
// them into the bt_tracestore.
import (
	"context"
	"flag"

	"go.skia.org/infra/golden/go/tracestore/bt_tracestore"
)

func main() {
	var (
		btInstance = flag.String("bt_instance", "", "Bigtable instance to use in the project identified by 'project_id'")
		projectID  = flag.String("project_id", "skia-public", "GCP project ID.")
		btTableID  = flag.String("bt_table_id", "production", "BigTable table ID.")
	)
	flag.Parse()

	btc := bt_tracestore.BTConfig{
		ProjectID:  *projectID,
		InstanceID: *btInstance,
		TableID:    *btTableID,
		// TODO(kjlubick): VCS
	}

	_ = bt_tracestore.InitBT(context.Background(), btc)
}
