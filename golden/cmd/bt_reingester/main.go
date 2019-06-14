// bt_reingester will scan through all the files in a GCS bucket and create synthetic
// pubsub events to cause the files to be re-ingested
package main

import (
	"context"
	"encoding/hex"
	"flag"
	"sync"
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
	"google.golang.org/api/option"
)

func main() {
	var (
		ingesterTopic = flag.String("ingester_topic", "", "Pubsub topic on which to generate synthetic events.")
		projectID     = flag.String("project_id", "skia-public", "GCP project ID.")

		srcBucket  = flag.String("src_bucket", "", "Source bucket to ingest files from.")
		srcRootDir = flag.String("src_root_dir", "dm-json-v1", "Source root directory to ingest files in.")
	)
	flag.Parse()

	bt.EnsureNotEmulator()

	tokenSrc, err := auth.NewDefaultTokenSource(true, storage.ScopeReadOnly)
	if err != nil {
		sklog.Fatalf("Failed to auth: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(tokenSrc).With2xxOnly().WithDialTimeout(time.Second * 10).Client()

	gb, err := gevent.New(*projectID, *ingesterTopic, "re-ingester")
	if err != nil {
		sklog.Fatalf("no gevents: %s", err)
	}
	gb.PublishStorageEvent(eventbus.NewStorageEvent("skia-gold-chrome-gpu", "dm-json-v1/test.json", time.Now().Unix(), "foo"))

	gcsClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		sklog.Fatalf("Failed to create GCS client: %s", err)
	}

	concurrentIngesters := make(chan bool, 10)
	var wg sync.WaitGroup

	dirs := fileutil.GetHourlyDirs(*srcRootDir, beginning.Unix(), time.Now().Unix())
	for _, dir := range dirs {
		concurrentIngesters <- true
		wg.Add(1)

		go func(dir string) {
			defer func() {
				<-concurrentIngesters
				wg.Done()
			}()
			sklog.Infof("Directory: %q", dir)
			err := gcs.AllFilesInDir(gcsClient, *srcBucket, dir, func(item *storage.ObjectAttrs) {
				e := eventbus.NewStorageEvent(item.Bucket, item.Name, item.Updated.Unix(), hex.EncodeToString(item.MD5))
				gb.PublishStorageEvent(e)
			})
			if err != nil {
				sklog.Warningf("error for dir %s: %s", dir, err)
			}
		}(dir)
	}

	wg.Wait()
	sklog.Infof("done")
	time.Sleep(1 * time.Minute)
	sklog.Infof("done with wait time for any missed pubsub events")
}

// In the early days, there was several invalid entries, because they did not specify
// gitHash. Starting re-ingesting Skia on October 1, 2014 seems to be around when
// the data is correct.
var beginning = time.Date(2019, time.January, 15, 0, 0, 0, 0, time.UTC)
