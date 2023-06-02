// Package gcssource implements files.Source on top of Google Cloud Storage.
package gcssource

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/pubsub/sub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/file"
	"go.skia.org/infra/perf/go/ingest/filter"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

const (
	// maxParallelReceives is the number of Go routines we want to run.
	// Determined experimentally.
	maxParallelReceives = 1

	// subscriptionSuffix is the name we append to a topic name to build a
	// subscription name.
	subscriptionSuffix = "-prod"
)

// pubSubEvent is used to deserialize the PubSub data.
//
// The PubSub event data is a JSON serialized storage.ObjectAttrs object.
// See https://cloud.google.com/storage/docs/pubsub-notifications#payload
type pubSubEvent struct {
	Bucket string `json:"bucket"`
	Name   string `json:"name"`
}

// GCSSource implements file.Source for Google Cloud Storage.
type GCSSource struct {
	// instanceConfig if the InstanceConfig we are ingesting files for.
	instanceConfig *config.InstanceConfig

	// storageClient is how we talk to Google Cloud Storage.
	storageClient *storage.Client

	// fileChannel is the output channel returned from Start.
	fileChannel chan<- file.File

	// subscription is the pubsub event subscription.
	subscription *pubsub.Subscription

	// started is true if Start has already been called.
	started bool

	// nackCounter is a metric of how many messages we've nacked.
	nackCounter metrics2.Counter

	// ackCounter is a metric of how many messages we've acked.
	ackCounter metrics2.Counter

	// filter to accept/reject files based on their filename.
	filter *filter.Filter

	// deadLetterEnabled is true if the dead letter topic is configured.
	deadLetterEnabled bool
}

// New returns a new *GCSSource
func New(ctx context.Context, instanceConfig *config.InstanceConfig, local bool) (*GCSSource, error) {
	ts, err := google.DefaultTokenSource(ctx, storage.ScopeReadOnly, pubsub.ScopePubSub)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	gcsClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	subName := instanceConfig.IngestionConfig.SourceConfig.Subscription
	if subName == "" {
		subName, err = sub.NewRoundRobinNameProvider(local, instanceConfig.IngestionConfig.SourceConfig.Topic).SubName()
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	sub, err := sub.NewWithSubName(ctx, local, instanceConfig.IngestionConfig.SourceConfig.Project, instanceConfig.IngestionConfig.SourceConfig.Topic, subName, maxParallelReceives)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// If we don't ack or nack a message, the pub/sub library will automatically
	// extend the ack deadline of all fetched Messages up to the duration specified,
	// which will stuck the ingestor and no new message will be consumed from pub/sub.
	// Disable automatic deadline extension by specifying a duration less than 0
	sub.ReceiveSettings.MaxExtension = -1

	f, err := filter.New(instanceConfig.IngestionConfig.SourceConfig.AcceptIfNameMatches, instanceConfig.IngestionConfig.SourceConfig.RejectIfNameMatches)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	dlEnabled := config.IsDeadLetterCollectionEnabled(instanceConfig)

	return &GCSSource{
		instanceConfig:    instanceConfig,
		storageClient:     gcsClient,
		nackCounter:       metrics2.GetCounter("perf_file_gcssource_nack", nil),
		ackCounter:        metrics2.GetCounter("perf_file_gcssource_ack", nil),
		subscription:      sub,
		filter:            f,
		deadLetterEnabled: dlEnabled,
	}, nil
}

// receiveSingleEventWrapper is the func we pass to Subscription.Receive.
func (s *GCSSource) receiveSingleEventWrapper(ctx context.Context, msg *pubsub.Message) {
	sklog.Debugf("Message received: %v", msg)
	ack := s.receiveSingleEvent(ctx, msg)
	if s.deadLetterEnabled {
		if !ack {
			s.nackCounter.Inc(1)
			msg.Nack()
			sklog.Debugf("Message nacked during receive check: %v", msg)
		}
		return
	}
	if ack {
		s.ackCounter.Inc(1)
		msg.Ack()
		sklog.Debugf("Message was acked: %v", msg)
	} else {
		s.nackCounter.Inc(1)
		msg.Nack()
		sklog.Debugf("Message was nacked: %v", msg)
	}
}

// receiveSingleEvent does all the work of receiving the event and returns true
// if the message should be ack'd, or false if it should be nack'd. Only nack
// responses that may be successful on a future try.
func (s *GCSSource) receiveSingleEvent(ctx context.Context, msg *pubsub.Message) bool {
	// Decode the event, which is a GCS event that a file was written.
	var event pubSubEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		sklog.Error(err)
		return true
	}

	filename := fmt.Sprintf("gs://%s/%s", event.Bucket, event.Name)

	// Apply filters to the filename.
	if s.filter.Reject(filename) {
		sklog.Errorf("File is rejected by the filename filter: %s", filename)
		return true
	}

	// Restrict files processed to those that appear in SourceConfig.Sources.
	found := false
	for _, prefix := range s.instanceConfig.IngestionConfig.SourceConfig.Sources {
		if strings.HasPrefix(filename, prefix) {
			found = true
			break
		}
	}
	if !found {
		sklog.Errorf("File %s is not in any config file listed buckets: %s", filename, s.instanceConfig.IngestionConfig.SourceConfig.Sources)
		return true
	}

	obj := s.storageClient.Bucket(event.Bucket).Object(event.Name)
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		sklog.Errorf("Failed to retrieve bucket %q object %q: %s", event.Bucket, event.Name, err)
		return false
	}
	reader, err := obj.NewReader(ctx)
	if err != nil {
		sklog.Error(err)
		return false
	}
	s.fileChannel <- file.File{
		Name:      filename,
		Contents:  reader,
		Created:   attrs.Created,
		PubSubMsg: msg,
	}
	return true
}

// Start implements the file.Source interface.
func (s *GCSSource) Start(ctx context.Context) (<-chan file.File, error) {
	if s.started {
		return nil, skerr.Fmt("Start can only be called once.")
	}
	s.started = true
	ret := make(chan file.File, maxParallelReceives)
	s.fileChannel = ret
	// Process all incoming PubSub requests.
	go func() {
		for {
			// Wait for PubSub events.
			err := s.subscription.Receive(ctx, s.receiveSingleEventWrapper)
			if err != nil {
				sklog.Errorf("Failed receiving pubsub message: %s", err)
			}
		}
	}()

	return ret, nil
}

// Confirm *GCSSource implements the file.Source interface.
var _ file.Source = (*GCSSource)(nil)
