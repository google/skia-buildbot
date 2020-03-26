// Package process does the whole process of ingesting files into a trace store.
package process

import (
	"context"
	"time"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/config"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/ingest/parser"
	"go.skia.org/infra/perf/go/ingestevents"
	"google.golang.org/api/option"
)

// sendPubSubEvent sends the unencoded params and paramset found in a single
// ingested file to the PubSub topic specified in the selected Perf instances
// configuration data.
func sendPubSubEvent(pubSubClient *pubsub.Client, topicName string, params []paramtools.Params, paramset paramtools.ParamSet, filename string) error {
	if topicName == "" {
		return nil
	}
	traceIDs := make([]string, 0, len(params))
	for _, p := range params {
		key, err := query.MakeKey(p)
		if err != nil {
			continue
		}
		traceIDs = append(traceIDs, key)
	}
	ie := &ingestevents.IngestEvent{
		TraceIDs: traceIDs,
		ParamSet: paramset,
		Filename: filename,
	}
	body, err := ingestevents.CreatePubSubBody(ie)
	if err != nil {
		return skerr.Wrapf(err, "Failed to encode PubSub body for topic: %q", topicName)
	}
	msg := &pubsub.Message{
		Data: body,
	}
	ctx := context.Background()
	_, err = pubSubClient.Topic(topicName).Publish(ctx, msg).Get(ctx)

	return skerr.Wrap(err)
}

// Start a single go routine to process incoming ingestion files and write
// the data they contain to a trace store.
//
// Except for file.Sources of type "dir" this function should never return
// except on error.
func Start(ctx context.Context, local bool, instanceConfig *config.InstanceConfig) error {
	// Metrics.
	filesReceived := metrics2.GetCounter("perfserver_ingest_files_received")
	failedToParse := metrics2.GetCounter("perfserver_ingest_failed_to_parse")
	badGitHash := metrics2.GetCounter("perfserver_ingest_bad_githash")
	failedToWrite := metrics2.GetCounter("perfserver_ingest_failed_to_write")
	successfulWrite := metrics2.GetCounter("perfserver_ingest_successful_write")

	var pubSubClient *pubsub.Client
	if instanceConfig.IngestionConfig.FileIngestionTopicName != "" {
		ts, err := auth.NewDefaultTokenSource(false, pubsub.ScopePubSub)
		if err != nil {
			sklog.Fatalf("Failed to create TokenSource: %s", err)
		}

		pubSubClient, err = pubsub.NewClient(ctx, instanceConfig.IngestionConfig.SourceConfig.Project, option.WithTokenSource(ts))
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// New file.Source.
	source, err := builders.NewSourceFromConfig(ctx, instanceConfig, false)
	ch, err := source.Start(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}

	// New Parser.
	parser := parser.New(instanceConfig)

	// New TraceStore.
	store, err := builders.NewTraceStoreFromConfig(ctx, false, instanceConfig)
	if err != nil {
		return skerr.Wrap(err)
	}

	// New gitinfo.GitInfo.
	sklog.Infof("Cloning repo %q into %q", instanceConfig.GitRepoConfig.URL, instanceConfig.GitRepoConfig.Dir)
	g, err := perfgit.New(ctx, local, instanceConfig)
	if err != nil {
		return skerr.Wrap(err)
	}

	sklog.Info("Waiting on files to process.")
	for f := range ch {
		sklog.Infof("Ingest received: %v", f)
		filesReceived.Inc(1)

		// Parse the file.
		params, values, gitHash, err := parser.Parse(f)
		if err != nil {
			sklog.Errorf("Failed to parse %v: %s", f, err)
			failedToParse.Inc(1)
			continue
		}

		// Convert gitHash to commitNumber.
		commitNumber, err := g.CommitNumberFromGitHash(ctx, gitHash)
		if err != nil {
			badGitHash.Inc(1)
			sklog.Error("Failed to find gitHash %v: %s", f, err)
			continue
		}

		// Build paramset from params.
		ps := paramtools.NewParamSet()
		for _, p := range params {
			ps.AddParams(p)
		}

		// Write data to the trace store.
		if err := store.WriteTraces(commitNumber, params, values, ps, f.Name, time.Now()); err != nil {
			failedToWrite.Inc(1)
			sklog.Error("Failed to write %v: %s", f, err)
		}
		successfulWrite.Inc(1)

		if err := sendPubSubEvent(pubSubClient, instanceConfig.IngestionConfig.FileIngestionTopicName, params, ps, f.Name); err != nil {
			sklog.Errorf("Failed to send pubsub event: %s", err)
		} else {
			sklog.Info("FileIngestionTopicName pubsub message sent.")
		}
	}
	sklog.Infof("Exited while waiting on files. Should only happen on source_type=dir.")
	return nil
}
