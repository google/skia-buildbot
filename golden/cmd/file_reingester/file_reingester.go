// Executable file_reingester will scan through all the files in a GCS bucket and create synthetic
// pubsub events to cause the files to be re-ingested.
package main

import (
	"context"
	"flag"
	"os"
	"strings"
	"time"

	"go.skia.org/infra/go/sklog/sklogimpl"
	"go.skia.org/infra/go/sklog/stdlogging"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/sklog"
)

func main() {
	var (
		ingesterTopic = flag.String("ingester_topic", "", "Pubsub topic on which to generate synthetic events.")
		projectID     = flag.String("project_id", "skia-public", "GCP project ID.")

		srcBucket  = flag.String("src_bucket", "", "Source bucket to ingest files from.")
		srcRootDir = flag.String("src_root_dir", "dm-json-v1", "Source root directory to ingest files in.")

		changelistIDs = common.NewMultiStringFlag("changelists", nil, "If provided, will only ingest data from the provided changelists")

		// In the early days, there was several invalid entries, because they did not specify
		// gitHash. Starting re-ingesting Skia on October 1, 2014 seems to be around when
		// the data is correct.
		startYear  = flag.Int("start_year", 2019, "year to start ingesting")
		startMonth = flag.Int("start_month", 1, "month to start ingesting")
		startDay   = flag.Int("start_day", 1, "day to start ingesting (at midnight UTC)")

		sleepBetweenDays = flag.Duration("sleep_between_days", 0, "If non-zero, the amount of time to wait after reingesting one day's worth of data.")
	)
	flag.Parse()
	sklogimpl.SetLogger(stdlogging.New(os.Stderr))

	ctx := context.Background()
	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		sklog.Fatalf("Failed to create GCS client: %s", err)
	}

	psc, err := pubsub.NewClient(ctx, *projectID)
	if err != nil {
		sklog.Fatalf("Could not make pubsub client for project %q: %s", *projectID, err)
	}

	// Check that the topic exists. Fail if it does not.
	topic := psc.Topic(*ingesterTopic)
	if exists, err := topic.Exists(ctx); err != nil {
		sklog.Fatalf("Error checking for existing topic %q: %s", *ingesterTopic, err)
	} else if !exists {
		sklog.Fatalf("topic %s does not exist in project %s", *ingesterTopic, *projectID)
	}

	sklog.Infof("starting scanning %q in project %s", *ingesterTopic, *projectID)

	beginning := time.Date(*startYear, time.Month(*startMonth), *startDay, 0, 0, 0, 0, time.UTC)

	root := *srcRootDir
	if *changelistIDs != nil && len(*changelistIDs) > 0 {
		sklog.Infof("Only processing cls: %+v", *changelistIDs)
		root = "trybot/dm-json-v1"
	}

	dirs := fileutil.GetHourlyDirs(root, beginning, time.Now())
	published := 0
	for _, dir := range dirs {
		sklog.Infof("Directory: %q", dir)
		var last *pubsub.PublishResult
		err := gcs.AllFilesInDir(gcsClient, *srcBucket, dir, func(item *storage.ObjectAttrs) {
			if matchesChangelist(changelistIDs, item.Name) {
				published++
				if published%1000 == 0 {
					sklog.Infof("%d reingeseted", published)
				}
				last = publishSyntheticStorageEvent(ctx, topic, item.Bucket, item.Name)
			}
		})
		if err != nil {
			sklog.Warningf("Error while processing dir %s: %s", dir, err)
		}
		if last != nil {
			_, err := last.Get(context.Background())
			if err != nil {
				sklog.Fatalf("Could not publish: %s", err)
			} else {
				sklog.Debugf("Published something for %s", dir)
			}
		}
		if strings.HasSuffix(dir, "/23") {
			if *sleepBetweenDays > time.Second {
				sklog.Infof("Waiting at the end of a day")
				time.Sleep(*sleepBetweenDays)
			}
		}
	}

	sklog.Infof("waiting for messages to publish")
	topic.Stop()
	sklog.Infof("done")
}

// matchesChangelist returns true if the given file name matches a changelist id. That is,
// there exists "/[clid]" somewhere in the name. For example:
//   trybot/dm-json-v1/2021/09/23/02/4140248__1/8835339621082367857/dm-1632364432749558598.json
// has a match for CL 4140248. It's implemented simply, meant for some adhoc re-ingestion.
func matchesChangelist(changelistIDs *[]string, name string) bool {
	if changelistIDs == nil || len(*changelistIDs) == 0 {
		return true
	}
	for _, id := range *changelistIDs {
		if strings.Contains(name, "/"+id) {
			return true
		}
	}
	return false
}

func publishSyntheticStorageEvent(ctx context.Context, topic *pubsub.Topic, bucket, fileName string) *pubsub.PublishResult {
	return topic.Publish(ctx, &pubsub.Message{
		// These are the important attributes read for ingestion.
		// https://cloud.google.com/storage/docs/pubsub-notifications#attributes
		Attributes: map[string]string{
			"bucketId": bucket,
			"objectId": fileName,
		},
		Data: nil, // We don't currently read anything from Data.
	})
}
