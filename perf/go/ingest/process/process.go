// Package process does the whole process of ingesting files into a trace store.
package process

import (
	"context"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2/google"

	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/file"
	"go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/ingest/parser"
	"go.skia.org/infra/perf/go/ingestevents"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/tracing"
	"go.skia.org/infra/perf/go/types"
	"google.golang.org/api/option"
)

const writeRetries = 10

// defaultDatabaseTimeout is the context timeout used when making a request that
// involves the database. For more complex requests use config.QueryMaxRuntime.
const defaultDatabaseTimeout = 60 * time.Minute

// sendPubSubEvent sends the unencoded params and paramset found in a single
// ingested file to the PubSub topic specified in the selected Perf instances
// configuration data.
func sendPubSubEvent(ctx context.Context, pubSubClient *pubsub.Client, topicName string, params []paramtools.Params, paramset paramtools.ReadOnlyParamSet, filename string) error {
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

// workerInfo is all the information that a worker Go routine will need to
// process a single incoming file.
type workerInfo struct {
	filesReceived        metrics2.Counter
	failedToParse        metrics2.Counter
	skipped              metrics2.Counter
	badGitHash           metrics2.Counter
	failedToWrite        metrics2.Counter
	successfulWrite      metrics2.Counter
	successfulWriteCount metrics2.Counter
	dlEnabled            bool
	p                    *parser.Parser
	store                tracestore.TraceStore
	metadataStore        tracestore.MetadataStore
	g                    git.Git
	pubSubClient         *pubsub.Client
	instanceConfig       *config.InstanceConfig
}

// newWorker returns a new *workerInfo.
func newWorker(
	filesReceived metrics2.Counter,
	failedToParse metrics2.Counter,
	skipped metrics2.Counter,
	badGitHash metrics2.Counter,
	failedToWrite metrics2.Counter,
	successfulWrite metrics2.Counter,
	successfulWriteCount metrics2.Counter,
	dlEnabled bool,
	p *parser.Parser,
	store tracestore.TraceStore,
	metadataStore tracestore.MetadataStore,
	g git.Git,
	pubSubClient *pubsub.Client,
	instanceConfig *config.InstanceConfig,
) *workerInfo {
	return &workerInfo{
		filesReceived:        filesReceived,
		failedToParse:        failedToParse,
		skipped:              skipped,
		badGitHash:           badGitHash,
		failedToWrite:        failedToWrite,
		successfulWrite:      successfulWrite,
		successfulWriteCount: successfulWriteCount,
		dlEnabled:            dlEnabled,
		p:                    p,
		store:                store,
		metadataStore:        metadataStore,
		g:                    g,
		pubSubClient:         pubSubClient,
		instanceConfig:       instanceConfig,
	}
}

// processSingleFile parses a single incoming file and write the data to the
// datastore.
func (w *workerInfo) processSingleFile(f file.File) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultDatabaseTimeout)
	defer cancel()
	ctx, span := trace.StartSpan(ctx, "ingest.parser.processSingleFile")
	defer span.End()
	// This is also being tracked separate from trace.Span to gather the metrics
	// in Prom.
	processLatency := metrics2.NewTimer("ingest_processSingleFile_latency")
	processLatency.Start()
	defer processLatency.Stop()

	sklog.Infof("Ingest received: %v", f)
	w.filesReceived.Inc(1)

	// Parse the file.
	params, values, gitHash, fileLinks, err := w.p.Parse(ctx, f)
	if err != nil {
		if err == parser.ErrFileShouldBeSkipped {
			sklog.Debugf("File should be skipped %v: %s", f, err)
			if f.PubSubMsg != nil {
				f.PubSubMsg.Ack()
				sklog.Debugf("Message acked: %v", f.PubSubMsg)
			}
			w.skipped.Inc(1)
			return nil
		}
		sklog.Errorf("Failed to parse %v: %s", f, err)
		w.failedToParse.Inc(1)
		nackMessageIfNecessary(w.dlEnabled, f)
		return nil
	}

	// This occurs when we have split up a large file into smaller files
	// and written those files to a secondary GCS path which are processed
	// separately.
	if params == nil && values == nil {
		sklog.Infof("No param or values to process in file %s", f.Name)
		if f.PubSubMsg != nil {
			f.PubSubMsg.Ack()
			sklog.Debugf("Message acked: %v", f.PubSubMsg)
		}
		return nil
	}

	sklog.Info("Lookup CommitNumber")

	// if git_hash is missing from GCS file
	if len(gitHash) == 0 {
		sklog.Errorf("Unable to handle empty git hash.")
		nackMessageIfNecessary(w.dlEnabled, f)
		return nil
	}

	commitNumberFromFile := types.CommitNumber(0)
	if w.g.RepoSuppliedCommitNumber() {
		commitNumberFromFile, err = w.p.ParseCommitNumberFromGitHash(gitHash)
		if err != nil {
			sklog.Errorf("Unable to convert githash to integer commit number %q.", gitHash, err)
			nackMessageIfNecessary(w.dlEnabled, f)
			return nil
		}
	}

	// Convert gitHash or check the existance of a commitNumber.
	commitNumber, err := w.g.GetCommitNumber(ctx, gitHash, commitNumberFromFile)
	if err != nil {
		if err := w.g.Update(ctx); err != nil {
			sklog.Errorf("Failed to Update: ", err)
		}
		commitNumber, err = w.g.GetCommitNumber(ctx, gitHash, commitNumberFromFile)
		if err != nil {
			w.badGitHash.Inc(1)
			sklog.Error("Failed to find commit number %v: %s", f, err)

			// This means the commit number in the file is invalid. There is no point
			// in processing this file again since it will fail similarly,
			// so let's ack the pubsub message to prevent GCP Pubsub from retrying.
			if f.PubSubMsg != nil {
				f.PubSubMsg.Ack()
			}

			return nil
		}
	}

	sklog.Info("Build ParamSet")
	// Build paramset from params.
	ps := paramtools.NewParamSet()
	for _, p := range params {
		ps.AddParams(p)
	}
	ps.Normalize()

	sklog.Info("WriteTraces")
	const retries = writeRetries
	i := 0
	writeFailed := false
	for {
		// Write data to the trace store.
		var err error = nil
		err = w.store.WriteTraces(ctx, commitNumber, params, values, ps, f.Name, time.Now())
		if err == nil {
			break
		}
		i++
		if i > retries {
			writeFailed = true
			break
		}

		if err == context.DeadlineExceeded {
			// The timeout is already significantly high. If the write traces timed out,
			// it will likely timeout on a retry. Let's error out early in that case.
			sklog.Errorf("Timed out while writing traces from %q", f.Name)
			break
		}
	}
	if writeFailed {
		w.failedToWrite.Inc(1)
		sklog.Errorf("Failed to write after %d retries %q: %s", retries, f.Name, err)
		nackMessageIfNecessary(w.dlEnabled, f)
	} else {
		if f.PubSubMsg != nil {
			f.PubSubMsg.Ack()
			sklog.Debugf("Message acked: %v", f.PubSubMsg)
		}
		w.successfulWrite.Inc(1)
		w.successfulWriteCount.Inc(int64(len(params)))
	}

	if err := sendPubSubEvent(ctx, w.pubSubClient, w.instanceConfig.IngestionConfig.FileIngestionTopicName, params, ps.Freeze(), f.Name); err != nil {
		sklog.Errorf("Failed to send pubsub event: %s", err)
	} else {
		sklog.Info("FileIngestionTopicName pubsub message sent.")
	}

	if fileLinks != nil {
		err := w.metadataStore.InsertMetadata(ctx, f.Name, fileLinks)
		if err != nil {
			// log the error and continue.
			sklog.Errorf("Error inserting the links metadata into the database: %v", err)
		}
	}
	return nil
}

// worker ingests files that arrive on the given 'ch' channel.
func worker(ctx context.Context, wg *sync.WaitGroup, g git.Git, store tracestore.TraceStore, metadataStore tracestore.MetadataStore, ch <-chan file.File, pubSubClient *pubsub.Client, instanceConfig *config.InstanceConfig) {
	// Metrics.
	filesReceived := metrics2.GetCounter("perfserver_ingest_files_received")
	failedToParse := metrics2.GetCounter("perfserver_ingest_failed_to_parse")
	skipped := metrics2.GetCounter("perfserver_ingest_skipped")
	badGitHash := metrics2.GetCounter("perfserver_ingest_bad_githash")
	failedToWrite := metrics2.GetCounter("perfserver_ingest_failed_to_write")
	successfulWrite := metrics2.GetCounter("perfserver_ingest_successful_write")
	successfulWriteCount := metrics2.GetCounter("perfserver_ingest_num_points_written")
	dlEnabled := config.IsDeadLetterCollectionEnabled(instanceConfig)

	// New Parser.
	p, err := parser.New(ctx, instanceConfig)
	if err != nil {
		sklog.Errorf("Ingestion worker failed to create parser: %s", err)
		wg.Done()
		return
	}

	workerInfo := newWorker(filesReceived, failedToParse, skipped, badGitHash, failedToWrite, successfulWrite, successfulWriteCount, dlEnabled, p, store, metadataStore, g, pubSubClient, instanceConfig)

	for f := range ch {
		if err := ctx.Err(); err != nil {
			sklog.Error(err)
			break
		}
		if err := workerInfo.processSingleFile(f); err != nil {
			break
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
	if err := tracing.Init(local, instanceConfig); err != nil {
		sklog.Fatalf("Failed to start tracing: %s", err)
	}

	var pubSubClient *pubsub.Client
	if instanceConfig.IngestionConfig.FileIngestionTopicName != "" {
		ts, err := google.DefaultTokenSource(ctx, pubsub.ScopePubSub)
		if err != nil {
			sklog.Fatalf("Failed to create TokenSource: %s", err)
		}

		pubSubClient, err = pubsub.NewClient(ctx, instanceConfig.IngestionConfig.SourceConfig.Project, option.WithTokenSource(ts))
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// New file.Source.
	source, err := builders.NewSourceFromConfig(ctx, instanceConfig)
	if err != nil {
		return skerr.Wrap(err)
	}
	ch, err := source.Start(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}

	// New TraceStore.
	store, err := builders.NewTraceStoreFromConfig(ctx, instanceConfig)
	if err != nil {
		return skerr.Wrap(err)
	}

	metadataStore, err := builders.NewMetadataStoreFromConfig(ctx, instanceConfig)
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
		go worker(ctx, &wg, g, store, metadataStore, ch, pubSubClient, instanceConfig)
	}
	wg.Wait()

	sklog.Infof("Exited while waiting on files. Should only happen on source_type=dir.")
	return nil
}

func nackMessageIfNecessary(dlEnabled bool, f file.File) {
	if dlEnabled {
		// This message will be available to the ingestor immediately.
		f.PubSubMsg.Nack()
		sklog.Debugf("Message nacked during message process: %v", f.PubSubMsg)
	}
}
