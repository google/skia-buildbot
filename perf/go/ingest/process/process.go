// Package process does the whole process of ingesting files into a trace store.
package process

import (
	"context"
	"sync"
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
	"go.skia.org/infra/perf/go/file"
	"go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/ingest/parser"
	"go.skia.org/infra/perf/go/ingestevents"
	"go.skia.org/infra/perf/go/tracestore"
	"google.golang.org/api/option"
)

const writeRetries = 10

// sendPubSubEvent sends the unencoded params and paramset found in a single
// ingested file to the PubSub topic specified in the selected Perf instances
// configuration data.
func sendPubSubEvent(ctx context.Context, pubSubClient *pubsub.Client, topicName string, params []paramtools.Params, paramset paramtools.ParamSet, filename string) error {
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
	_, err = pubSubClient.Topic(topicName).Publish(ctx, msg).Get(ctx)

	return skerr.Wrap(err)
}

// worker ingests files that arrive on the given 'ch' channel.
func worker(ctx context.Context, wg *sync.WaitGroup, g *git.Git, store tracestore.TraceStore, ch <-chan file.File, pubSubClient *pubsub.Client, instanceConfig *config.InstanceConfig) {
	// Metrics.
	filesReceived := metrics2.GetCounter("perfserver_ingest_files_received")
	failedToParse := metrics2.GetCounter("perfserver_ingest_failed_to_parse")
	skipped := metrics2.GetCounter("perfserver_ingest_skipped")
	badGitHash := metrics2.GetCounter("perfserver_ingest_bad_githash")
	failedToWrite := metrics2.GetCounter("perfserver_ingest_failed_to_write")
	successfulWrite := metrics2.GetCounter("perfserver_ingest_successful_write")
	successfulWriteCount := metrics2.GetCounter("perfserver_ingest_num_points_written")

	// New Parser.
	p := parser.New(instanceConfig)

	for f := range ch {
		if err := ctx.Err(); err != nil {
			sklog.Error(err)
			break
		}
		sklog.Infof("Ingest received: %v", f)
		filesReceived.Inc(1)

		// Parse the file.
		params, values, gitHash, err := p.Parse(f)
		sklog.Infof("Parse error: %s", err)
		if err != nil {
			if err == parser.ErrFileShouldBeSkipped {
				skipped.Inc(1)
			} else {
				sklog.Errorf("Failed to parse %v: %s", f, err)
				failedToParse.Inc(1)
			}
			continue
		}

		sklog.Info("Lookup CommitNumber")
		// Convert gitHash to commitNumber.
		commitNumber, err := g.CommitNumberFromGitHash(ctx, gitHash)
		if err != nil {
			if err := g.Update(ctx); err != nil {
				sklog.Errorf("Failed to Update: ", err)

			}
			commitNumber, err = g.CommitNumberFromGitHash(ctx, gitHash)
			if err != nil {
				badGitHash.Inc(1)
				sklog.Error("Failed to find gitHash %v: %s", f, err)
				continue
			}
		}

		sklog.Info("Build ParamSet")
		// Build paramset from params.
		ps := paramtools.NewParamSet()
		for _, p := range params {
			ps.AddParams(p)
		}

		sklog.Info("WriteTraces")
		const retries = writeRetries
		i := 0
		for {
			// Write data to the trace store.
			err := store.WriteTraces(ctx, commitNumber, params, values, ps, f.Name, time.Now())
			if err == nil {
				break
			}
			i++
			if i > retries {
				failedToWrite.Inc(1)
				sklog.Errorf("Failed to write after %d retries %q: %s", retries, f.Name, err)
				continue
			}
		}
		successfulWrite.Inc(1)
		successfulWriteCount.Inc(int64(len(params)))

		if err := sendPubSubEvent(ctx, pubSubClient, instanceConfig.IngestionConfig.FileIngestionTopicName, params, ps, f.Name); err != nil {
			sklog.Errorf("Failed to send pubsub event: %s", err)
		} else {
			sklog.Info("FileIngestionTopicName pubsub message sent.")
		}
	}
	wg.Done()
}

// Start a single go routine to process incoming ingestion files and write
// the data they contain to a trace store.
//
// Except for file.Sources of type "dir" this function should never return
// except on error.
func Start(ctx context.Context, local bool, numParallelIngesters int, instanceConfig *config.InstanceConfig) error {

	var pubSubClient *pubsub.Client
	if instanceConfig.IngestionConfig.FileIngestionTopicName != "" {
		ts, err := auth.NewDefaultTokenSource(local, pubsub.ScopePubSub)
		if err != nil {
			sklog.Fatalf("Failed to create TokenSource: %s", err)
		}

		pubSubClient, err = pubsub.NewClient(ctx, instanceConfig.IngestionConfig.SourceConfig.Project, option.WithTokenSource(ts))
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// New file.Source.
	source, err := builders.NewSourceFromConfig(ctx, instanceConfig, local)
	ch, err := source.Start(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}

	// New TraceStore.
	store, err := builders.NewTraceStoreFromConfig(ctx, local, instanceConfig)
	if err != nil {
		return skerr.Wrap(err)
	}

	// New perfgit.Git.
	sklog.Infof("Cloning repo %q into %q", instanceConfig.GitRepoConfig.URL, instanceConfig.GitRepoConfig.Dir)
	g, err := builders.NewPerfGitFromConfig(ctx, local, instanceConfig)
	if err != nil {
		return skerr.Wrap(err)
	}
	// Polling isn't needed because we call update on the repo if we find a git hash we don't recognize.
	// g.StartBackgroundPolling(ctx, gitRefreshDuration)

	sklog.Info("Waiting on files to process.")

	var wg sync.WaitGroup

	for i := 0; i < numParallelIngesters; i++ {
		wg.Add(1)
		go worker(ctx, &wg, g, store, ch, pubSubClient, instanceConfig)
	}
	wg.Wait()

	sklog.Infof("Exited while waiting on files. Should only happen on source_type=dir.")
	return nil
}
