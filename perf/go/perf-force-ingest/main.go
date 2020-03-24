// A command-line application that is used to trigger PubSub events for a given Perf config
// over a specific time range. One event per file will be generated for every file found
// in GCS in the given time range.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"google.golang.org/api/option"
)

// flags
var (
	configFilename = flag.String("config_filename", "./configs/nano.json", "Filename of the perf instance config to use.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	start          = flag.String("start", "", "Start the ingestion at this time, of the form: 2006-01-02. Default to one week ago.")
	end            = flag.String("end", "", "Ingest up to this time, of the form: 2006-01-02. Defaults to now.")
	dryrun         = flag.Bool("dry_run", false, "Just display the list of files to send.")
)

func main() {
	common.InitWithMust(
		"perf-force-ingest",
	)

	ctx := context.Background()
	if err := config.Init(*configFilename); err != nil {
		sklog.Fatal(err)
	}
	ts, err := auth.NewDefaultTokenSource(*local, storage.ScopeReadOnly)
	if err != nil {
		sklog.Fatalf("Failed to create TokenSource: %s", err)
	}

	pubSubClient, err := pubsub.NewClient(ctx, config.Config.IngestionConfig.SourceConfig.Project, option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatal(err)
	}
	topic := pubSubClient.Topic(config.Config.IngestionConfig.SourceConfig.Topic)

	now := time.Now()
	startTime := now.Add(-7 * 24 * time.Hour)
	if *start != "" {
		startTime, err = time.Parse("2006-01-02", *start)
		if err != nil {
			sklog.Fatal(err)
		}
	}
	endTime := now
	if *end != "" {
		endTime, err = time.Parse("2006-01-02", *end)
		if err != nil {
			sklog.Fatal(err)
		}
	}

	client := httputils.DefaultClientConfig().WithTokenSource(ts).WithoutRetries().Client()
	gcsClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		sklog.Fatalf("Failed to create GCS client: %s", err)
	}
	for _, prefix := range config.Config.IngestionConfig.SourceConfig.Sources {
		u, err := url.Parse(prefix)
		if err != nil {
			sklog.Fatalf("Failed to parse the prefix: %s", err)
		}

		dirs := fileutil.GetHourlyDirs(u.Path[1:], startTime.Unix(), endTime.Unix())
		for _, dir := range dirs {
			sklog.Infof("Directory: %q", dir)
			err := gcs.AllFilesInDir(gcsClient, u.Host, dir, func(item *storage.ObjectAttrs) {
				// The PubSub event data is a JSON serialized storage.ObjectAttrs object.
				// See https://cloud.google.com/storage/docs/pubsub-notifications#payload
				sklog.Infof("File: %q", item.Name)
				b, err := json.Marshal(storage.ObjectAttrs{
					Name:   item.Name,
					Bucket: u.Host,
				})
				if err != nil {
					sklog.Errorf("Failed to serialize event: %s", err)
				}
				if *dryrun {
					fmt.Println(item.Name, item.Bucket)
					return
				}
				topic.Publish(ctx, &pubsub.Message{
					Data: b,
				})
			})
			if err != nil {
				if err != nil {
					sklog.Errorf("Failed while walking GCS files: %s", err)
				}
			}
		}
	}
}
