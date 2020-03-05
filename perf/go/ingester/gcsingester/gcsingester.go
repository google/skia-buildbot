// Package gcsingester implements ingester.Source from Google Cloud Storage.
package gcsingester

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/tracestore/btts"
	"google.golang.org/api/option"
)

// GCSIngesterSource implements ingester.Source.
type GCSIngesterSource struct {
	// nackCounter is the number files we weren't able to ingest.
	nackCounter metrics2.Counter
	// ackCounter is the number files we were able to ingest.
	ackCounter metrics2.Counter
}

// New returns a instance of GCSIngesterSource.
func New(cfg *config.InstanceConfig, local bool) (*GCSIngesterSource, error) {

	common.InitWithMust(
		"perf-ingest",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	// nackCounter is the number files we weren't able to ingest.
	nackCounter := metrics2.GetCounter("nack", nil)
	// ackCounter is the number files we were able to ingest.
	ackCounter := metrics2.GetCounter("ack", nil)

	ctx := context.Background()
	var ok bool
	cfg, ok = config.PERF_BIGTABLE_CONFIGS[*configName]
	if !ok {
		sklog.Fatalf("Invalid --config value: %q", *configName)
	}
	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatalf("Failed to get hostname: %s", err)
	}
	ts, err := auth.NewDefaultTokenSource(local, storage.ScopeReadOnly, pubsub.ScopePubSub, auth.SCOPE_GERRIT)
	if err != nil {
		sklog.Fatalf("Failed to create TokenSource: %s", err)
	}

	client := httputils.DefaultClientConfig().WithTokenSource(ts).WithoutRetries().Client()
	gcsClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		sklog.Fatalf("Failed to create GCS client: %s", err)
	}
	pubSubClient, err = pubsub.NewClient(ctx, cfg.DataStoreConfig.Project, option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatal(err)
	}

	if !*local {
		if _, err := gitauth.New(ts, "/tmp/git-cookie", true, ""); err != nil {
			sklog.Fatal(err)
		}
	}

	// When running in production we have every instance use the same topic name so that
	// they load-balance pulling items from the topic.
	subName := fmt.Sprintf("%s-%s", cfg.IngestionConfig.Topic, "prod")
	if *local {
		// When running locally create a new topic for every host.
		subName = fmt.Sprintf("%s-%s", cfg.IngestionConfig.Topic, hostname)
	}
	sub := pubSubClient.Subscription(subName)
	ok, err = sub.Exists(ctx)
	if err != nil {
		sklog.Fatalf("Failed checking subscription existence: %s", err)
	}
	if !ok {
		sub, err = pubSubClient.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic: pubSubClient.Topic(cfg.IngestionConfig.Topic),
		})
		if err != nil {
			sklog.Fatalf("Failed creating subscription: %s", err)
		}
	}

	// How many Go routines should be processing messages?
	sub.ReceiveSettings.MaxOutstandingMessages = MAX_PARALLEL_RECEIVES
	sub.ReceiveSettings.NumGoroutines = MAX_PARALLEL_RECEIVES

	vcs, err := gitinfo.CloneOrUpdate(ctx, cfg.GitRepoConfig.URL, "/tmp/skia_ingest_checkout", true)
	if err != nil {
		sklog.Fatal(err)
	}

	store, err := btts.NewBigTableTraceStoreFromConfig(ctx, cfg, ts, true)
	if err != nil {
		sklog.Fatal(err)
	}

	// Process all incoming PubSub requests.
	go func() {
		for {
			// Wait for PubSub events.
			err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
				// Set success to true if we should Ack the PubSub message, otherwise
				// the message will be Nack'd, and PubSub will try to send the message
				// again.
				success := false
				defer func() {
					if success {
						ackCounter.Inc(1)
						msg.Ack()
					} else {
						nackCounter.Inc(1)
						msg.Nack()
					}
				}()
				// Decode the event, which is a GCS event that a file was written.
				var event Event
				if err := json.Unmarshal(msg.Data, &event); err != nil {
					sklog.Error(err)
					return
				}
				// Transaction logs for android_ingest are written to the same bucket,
				// which we should ignore.
				if strings.Contains(event.Name, "/tx_log/") {
					// Ack the file so we don't process it again.
					success = true
					return
				}
				// Load the file.
				obj := gcsClient.Bucket(event.Bucket).Object(event.Name)
				attrs, err := obj.Attrs(ctx)
				if err != nil {
					sklog.Errorf("Failed to retrieve bucket %q object %q: %s", event.Bucket, event.Name, err)
					return
				}
				reader, err := obj.NewReader(ctx)
				if err != nil {
					sklog.Error(err)
					return
				}
				defer util.Close(reader)
				sklog.Infof("Filename: %q", attrs.Name)
				// Pull data out of file and write it into BigTable.
				fullName := fmt.Sprintf("gs://%s/%s", event.Bucket, event.Name)
				err = processSingleFile(ctx, store, vcs, fullName, reader, attrs.Created, cfg.IngestionConfig.Branches)
				if err := reader.Close(); err != nil {
					sklog.Errorf("Failed to close: %s", err)
				}
				if err == NonRecoverableError {
					success = true
				} else if err != nil {
					sklog.Errorf("Failed to write results: %s", err)
					return
				}
				success = true
			})
			if err != nil {
				sklog.Errorf("Failed receiving pubsub message: %s", err)
			}
		}
	}()
	return nil
}
