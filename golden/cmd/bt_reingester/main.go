// bt_reingester will scan through all the files in a GCS bucket and create synthetic
// pubsub events to cause the files to be re-ingested
package main

import (
	"context"
	"encoding/hex"
	"flag"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gevent"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/tracestore/bt_tracestore"
	"google.golang.org/api/option"
)

func main() {
	var (
		ingesterTopic = flag.String("ingester_topic", "", "Pubsub topic on which to generate synthetic events.")
		projectID     = flag.String("project_id", "skia-public", "GCP project ID.")

		srcBucket  = flag.String("src_bucket", "", "Source bucket to ingest files from.")
		srcRootDir = flag.String("src_root_dir", "dm-json-v1", "Source root directory to ingest files in.")

		btInstance = flag.String("bt_instance", "production", "BigTable instance to use in the project identified by 'project_id'")
		btTableID  = flag.String("bt_table_id", "", "BigTable table ID for the traces.")
	)
	flag.Parse()

	bt.EnsureNotEmulator()

	btc := bt_tracestore.BTConfig{
		ProjectID:  *projectID,
		InstanceID: *btInstance,
		TableID:    *btTableID,
	}
	// Create the table if set
	if *btInstance != "" && *btTableID != "" {
		err := bt_tracestore.InitBT(btc)
		sklog.Fatalf("created table: %#v", err)
	}

	tokenSrc, err := auth.NewDefaultTokenSource(true, storage.ScopeReadOnly)
	if err != nil {
		sklog.Fatalf("Failed to auth: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(tokenSrc).With2xxOnly().WithDialTimeout(time.Second * 10).Client()

	gb, err := gevent.New(*projectID, *ingesterTopic, "re-ingester")
	if err != nil {
		sklog.Fatalf("Unable to create global event bus: %s", err)
	}

	gcsClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		sklog.Fatalf("Failed to create GCS client: %s", err)
	}

	dirs := fileutil.GetHourlyDirs(*srcRootDir, beginning.Unix(), time.Now().Unix())
	for _, dir := range dirs {

		sklog.Infof("Directory: %q", dir)
		err := gcs.AllFilesInDir(gcsClient, *srcBucket, dir, func(item *storage.ObjectAttrs) {
			e := eventbus.NewStorageEvent(item.Bucket, item.Name, item.Updated.Unix(), hex.EncodeToString(item.MD5))
			gb.PublishStorageEvent(e)
		})
		if err != nil {
			sklog.Warningf("Error while processing dir %s: %s", dir, err)
		}
	}

	sklog.Infof("done")
	// Let's be extra paranoid because gevent is working asynchronously, we don't want to
	// terminate before it is done.
	time.Sleep(1 * time.Minute)
	sklog.Infof("done with wait time for any missed pubsub events")
}

// In the early days, there was several invalid entries, because they did not specify
// gitHash. Starting re-ingesting Skia on October 1, 2014 seems to be around when
// the data is correct.
var beginning = time.Date(2014, time.October, 1, 0, 0, 0, 0, time.UTC)
