// Package gcssource implements files.Source on top of Google Cloud Storage.
package gcssource

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/files"
	"google.golang.org/api/option"
)

const (
	// maxParallelReceives is the number of Go routines we want to run. Determined experimentally.
	maxParallelReceives = 1
)

// pubSubEvent is used to deserialize the PubSub data.
//
// The PubSub event data is a JSON serialized storage.ObjectAttrs object.
// See https://cloud.google.com/storage/docs/pubsub-notifications#payload
type pubSubEvent struct {
	Bucket string `json:"bucket"`
	Name   string `json:"name"`
}

// GCSSource implements files.Source for Google Cloud Storage.
type GCSSource struct {
	// config if the InstanceConfig we are ingesting files for.
	config *config.InstanceConfig

	// local is true if running locally.
	local bool
}

// New returns a new *GCSSource
func New(config *config.InstanceConfig, local bool) *GCSSource {
	return &GCSSource{
		config: config,
		local:  local,
	}
}

// Start implements the files.Source interface.
func (s *GCSSource) Start(ctx context.Context) (<-chan files.File, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	ts, err := auth.NewDefaultTokenSource(s.local, storage.ScopeReadOnly, pubsub.ScopePubSub)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	client := httputils.DefaultClientConfig().WithTokenSource(ts).WithoutRetries().Client()
	gcsClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	pubSubClient, err := pubsub.NewClient(ctx, s.config.IngestionConfig.SourceConfig.Project, option.WithTokenSource(ts))
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// When running in production we have every instance use the same topic name so that
	// they load-balance pulling items from the topic.
	subName := fmt.Sprintf("%s-%s", s.config.IngestionConfig.SourceConfig.Topic, "prod")
	if s.local {
		// When running locally create a new topic for every host.
		subName = fmt.Sprintf("%s-%s", s.config.IngestionConfig.SourceConfig.Topic, hostname)
	}
	sub := pubSubClient.Subscription(subName)
	ok, err := sub.Exists(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create a reference to subscription: %q ", subName)
	}
	if !ok {
		sub, err = pubSubClient.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic: pubSubClient.Topic(s.config.IngestionConfig.SourceConfig.Topic),
		})
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to create subscription: %q", subName)
		}
	}

	// How many Go routines should be processing messages?
	sub.ReceiveSettings.MaxOutstandingMessages = maxParallelReceives
	sub.ReceiveSettings.NumGoroutines = maxParallelReceives

	nackCounter := metrics2.GetCounter("perf_files_gcssource_nack", nil)
	ackCounter := metrics2.GetCounter("perf_files_gcssource_ack", nil)

	ret := make(chan files.File)
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
				var event pubSubEvent
				if err := json.Unmarshal(msg.Data, &event); err != nil {
					sklog.Error(err)
					return
				}
				// Transaction logs for android_ingest are written to the same bucket,
				// which we should ignore.
				// TODO(jcgregorio) Write tx logs to a different dir.
				// OR restrict to SourceConfig.Sources?
				if strings.Contains(event.Name, "/tx_log/") {
					// Ack the file so we don't process it again.
					success = true
					return
				}
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
				ret <- files.File{
					Name:     fmt.Sprintf("gs://%s/%s", event.Bucket, event.Name),
					Contents: reader,
					Created:  attrs.Created,
				}
				success = true
			})
			if err != nil {
				sklog.Errorf("Failed receiving pubsub message: %s", err)
			}
		}
	}()

	return ret, nil
}
