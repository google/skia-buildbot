// gold_ingestion is the server process that runs an arbitrary number of
// ingesters and stores them to the appropriate backends.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/ingestion"
	"go.skia.org/infra/golden/go/ingestion/sqlingestionstore"
	"go.skia.org/infra/golden/go/ingestion_processors"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/tracing"
)

const (
	// Arbitrarily picked.
	maxSQLConnections = 20
)

type ingestionServerConfig struct {
	config.Common

	// As of 2019, the primary way to ingest data is event-driven. That is, when
	// new files are put into a GCS bucket, PubSub fires an event and that is the
	// primary way for an ingester to be notified about a file.
	// The 2 parameters below configure the manual polling of the source, which
	// is a backup way to ingest data in the unlikely case that a PubSub event is
	// dropped (PubSub will try and re-try to send events for up to seven days by default).
	// BackupPollInterval is how often to do a scan.
	BackupPollInterval config.Duration `json:"backup_poll_interval"`
	// BackupPollScope is how far back in time to scan. It should be longer than BackupPollInterval.
	BackupPollScope config.Duration `json:"backup_poll_scope"`

	// IngestionFilesTopic is the PubSub topic on which messages will be placed that correspond
	// to files to ingest.
	IngestionFilesTopic string `json:"ingestion_files_topic"`

	// IngestionSubscription is the subscription ID used by all replicas. By setting the
	// subscriber ID to be the same on all replicas, only one of the replicas will get each
	// event (usually). We like our subscription names to be unique and keyed to the instance,
	// for easier following up on "Why are there so many backed up messages?"
	IngestionSubscription string `json:"ingestion_subscription"`

	// FilesProcessedInParallel controls how many goroutines are used to process PubSub messages.
	// The default is 4, but if instances are handling lots of small files, this can be increased.
	FilesProcessedInParallel int `json:"files_processed_in_parallel" optional:"true"`

	// PrimaryBranchConfig describes how the primary branch ingestion should be configured.
	PrimaryBranchConfig ingesterConfig `json:"primary_branch_config"`

	// PubSubFetchSize is how many worker messages to ask PubSub for. This defaults to 10, but for
	// instances that have many small files ingested, this can be higher for better utilization
	// and throughput.
	PubSubFetchSize int `json:"pubsub_fetch_size" optional:"true"`

	// SecondaryBranchConfig is the optional config for ingestion on secondary branches (e.g. Tryjobs).
	SecondaryBranchConfig *ingesterConfig `json:"secondary_branch_config" optional:"true"`

	// TODO(kjlubick) Restore this functionality. Without it, we cannot ingest from internal jobs.
	// URL of the secondary repo that has GitRepoURL as a dependency.
	SecondaryRepoURL string `json:"secondary_repo_url" optional:"true"`
	// Regular expression to extract the commit hash from the DEPS file.
	SecondaryRepoRegEx string `json:"secondary_repo_regex" optional:"true"`
}

// ingesterConfig is the configuration for a single ingester.
type ingesterConfig struct {
	// Type describes the backend type of the ingester.
	Type string `json:"type"`
	// Source is where the ingester will read files from.
	Source gcsSourceConfig `json:"gcs_source"`
	// ExtraParams help configure the ingester and are specific to the backend type.
	ExtraParams map[string]string `json:"extra_configuration"`
}

// gcsSourceConfig is the configuration needed to ingest from files in a GCS bucket.
type gcsSourceConfig struct {
	Bucket string `json:"bucket"`
	Prefix string `json:"prefix"`
}

func main() {
	// Command line flags.
	var (
		commonInstanceConfig = flag.String("common_instance_config", "", "Path to the json5 file containing the configuration that needs to be the same across all services for a given instance.")
		thisConfig           = flag.String("config", "", "Path to the json5 file containing the configuration specific to baseline server.")
		hang                 = flag.Bool("hang", false, "Stop and do nothing after reading the flags. Good for debugging containers.")
	)

	// Parse the options. So we can configure logging.
	flag.Parse()

	if *hang {
		sklog.Info("Hanging")
		select {}
	}

	var isc ingestionServerConfig
	if err := config.LoadFromJSON5(&isc, commonInstanceConfig, thisConfig); err != nil {
		sklog.Fatalf("Reading config: %s", err)
	}
	sklog.Infof("Loaded config %#v", isc)

	common.InitWithMust(
		"gold-ingestion",
		common.PrometheusOpt(&isc.PromPort),
	)
	// We expect there to be a lot of ingestion work, so we sample 1% of them to avoid incurring
	// too much overhead.
	if err := tracing.Initialize(0.01, isc.SQLDatabaseName); err != nil {
		sklog.Fatalf("Could not set up tracing: %s", err)
	}

	ctx := context.Background()

	if isc.SQLDatabaseName == "" {
		sklog.Fatalf("Must have SQL database config")
	}
	url := sql.GetConnectionURL(isc.SQLConnection, isc.SQLDatabaseName)
	conf, err := pgxpool.ParseConfig(url)
	if err != nil {
		sklog.Fatalf("error getting postgres config %s: %s", url, err)
	}

	conf.MaxConns = maxSQLConnections
	sqlDB, err := pgxpool.ConnectConfig(ctx, conf)
	if err != nil {
		sklog.Fatalf("error connecting to the database: %s", err)
	}
	ingestionStore := sqlingestionstore.New(sqlDB)
	sklog.Infof("Using new SQL ingestion store")

	// Instantiate the secondary repo if one was specified.
	// TODO(kjlubick): skbug.com/9553
	if isc.SecondaryRepoURL != "" {
		sklog.Fatalf("Not yet implemented to have a secondary repo url")
	}

	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		sklog.Fatalf("Could not create GCS Client")
	}
	primaryBranchProcessor, src, err := getPrimaryBranchIngester(ctx, isc.PrimaryBranchConfig, gcsClient, sqlDB)
	if err != nil {
		sklog.Fatalf("Setting up primary branch ingestion: %s", err)
	}
	sourcesToScan := []ingestion.FileSearcher{src}

	pss := &pubSubSource{
		IngestionStore:         ingestionStore,
		PrimaryBranchProcessor: primaryBranchProcessor,
		PrimaryBranchStreamingLiveness: metrics2.NewLiveness("gold_ingestion", map[string]string{
			"metric": "since_last_successful_streaming_result",
			"source": "primary_branch",
		}),
		SuccessCounter: metrics2.GetCounter("gold_ingestion_success"),
		FailedCounter:  metrics2.GetCounter("gold_ingestion_failure"),
	}

	go func() {
		// Wait at least 5 seconds for the pubsub connection to be initialized before saying
		// we are healthy.
		time.Sleep(5 * time.Second)
		http.HandleFunc("/healthz", httputils.ReadyHandleFunc)
		sklog.Fatal(http.ListenAndServe(isc.ReadyPort, nil))
	}()

	startBackupPolling(ctx, isc, sourcesToScan, pss)
	startMetrics(ctx, pss)

	sklog.Fatalf("Listening for files to ingest %s", listen(ctx, isc, pss))
}

func getPrimaryBranchIngester(ctx context.Context, conf ingesterConfig, gcsClient *storage.Client, db *pgxpool.Pool) (ingestion.Processor, ingestion.FileSearcher, error) {
	src := &ingestion.GCSSource{
		Client: gcsClient,
		Bucket: conf.Source.Bucket,
		Prefix: conf.Source.Prefix,
	}
	if ok := src.Validate(); !ok {
		return nil, nil, skerr.Fmt("Invalid GCS Source %#v", src)
	}

	var primaryBranchProcessor ingestion.Processor
	if conf.Type == ingestion_processors.SQLPrimaryBranch {
		sqlProcessor := ingestion_processors.PrimaryBranchSQL(src, conf.ExtraParams, db)
		sqlProcessor.MonitorCacheMetrics(ctx)
		primaryBranchProcessor = sqlProcessor
		sklog.Infof("Configured SQL primary branch ingestion")
	} else {
		return nil, nil, skerr.Fmt("unknown ingestion backend: %q", conf.Type)
	}
	return primaryBranchProcessor, src, nil
}

// listen begins listening to the PubSub topic with the configured PubSub subscription. It will
// fail if the topic or subscription have not been created or PubSub fails.
func listen(ctx context.Context, isc ingestionServerConfig, p *pubSubSource) error {
	psc, err := pubsub.NewClient(ctx, isc.PubsubProjectID)
	if err != nil {
		return skerr.Wrapf(err, "initializing pubsub client for project %s", isc.PubsubProjectID)
	}

	// Check that the topic exists. Fail if it does not.
	t := psc.Topic(isc.IngestionFilesTopic)
	if exists, err := t.Exists(ctx); err != nil {
		return skerr.Wrapf(err, "checking for existing topic %s", isc.IngestionFilesTopic)
	} else if !exists {
		return skerr.Fmt("Diff work topic %s does not exist in project %s", isc.IngestionFilesTopic, isc.PubsubProjectID)
	}

	// Check that the subscription exists. Fail if it does not.
	sub := psc.Subscription(isc.IngestionSubscription)
	if exists, err := sub.Exists(ctx); err != nil {
		return skerr.Wrapf(err, "checking for existing subscription %s", isc.IngestionSubscription)
	} else if !exists {
		return skerr.Fmt("subscription %s does not exist in project %s", isc.IngestionSubscription, isc.PubsubProjectID)
	}

	// This is a limit of how many messages to fetch when PubSub has no work. Waiting for PubSub
	// to give us messages can take a second or two, so we choose a small, but not too small
	// batch size.
	if isc.PubSubFetchSize == 0 {
		sub.ReceiveSettings.MaxOutstandingMessages = 10
	} else {
		sub.ReceiveSettings.MaxOutstandingMessages = isc.PubSubFetchSize
	}

	if isc.FilesProcessedInParallel == 0 {
		sub.ReceiveSettings.NumGoroutines = 4
	} else {
		sub.ReceiveSettings.NumGoroutines = isc.FilesProcessedInParallel
	}

	// Blocks until context cancels or PubSub fails in a non retryable way.
	return skerr.Wrap(sub.Receive(ctx, p.ingestFromPubSubMessage))
}

type pubSubSource struct {
	IngestionStore         ingestion.Store
	PrimaryBranchProcessor ingestion.Processor
	// PrimaryBranchStreamingLiveness lets us have a metric to monitor the successful
	// streaming of data. It will be reset after each successful ingestion of a file from
	// the primary branch.
	PrimaryBranchStreamingLiveness metrics2.Liveness

	SuccessCounter metrics2.Counter
	FailedCounter  metrics2.Counter

	// busy is either 0 or non-zero depending on if this ingestion is working or not. This
	// allows us to gather data on wall-clock utilization.
	busy int64
}

// ingestFromPubSubMessage takes in a PubSub message and looks for a fileName specified as
// the "objectId" Attribute on the message. This is how file names are provided from GCS
// on file changes. https://cloud.google.com/storage/docs/pubsub-notifications#attributes
// It will either Nack or Ack the message depending on if there was a retryable error or not.
func (p *pubSubSource) ingestFromPubSubMessage(ctx context.Context, msg *pubsub.Message) {
	ctx, span := trace.StartSpan(ctx, "ingestion_ingestFromPubSubMessage")
	defer span.End()
	atomic.AddInt64(&p.busy, 1)
	fileName := msg.Attributes["objectId"]
	if shouldAck := p.ingestFile(ctx, fileName); shouldAck {
		msg.Ack()
	} else {
		msg.Nack()
	}
	atomic.AddInt64(&p.busy, -1)
}

// ingestFile ingests the file and returns true if the ingestion was successful or it got
// a non-retryable error. It returns false if it got a retryable error.
func (p *pubSubSource) ingestFile(ctx context.Context, name string) bool {
	if !strings.HasSuffix(name, ".json") {
		return true
	}
	if p.PrimaryBranchProcessor.HandlesFile(name) {
		err := p.PrimaryBranchProcessor.Process(ctx, name)
		if skerr.Unwrap(err) == ingestion.ErrRetryable {
			sklog.Warningf("Got retryable error for primary branch data for file %s", name)
			p.FailedCounter.Inc(1)
			return false
		}
		// TODO(kjlubick) Processors should mark the SourceFiles table as ingested, not here.
		if err := p.IngestionStore.SetIngested(ctx, name, now.Now(ctx)); err != nil {
			sklog.Errorf("Could not write to ingestion store: %s", err)
			// We'll continue anyway. The IngestionStore is not a big deal.
		}
		if err != nil {
			sklog.Errorf("Got non-retryable error for primary branch data for file %s: %s", name, err)
			p.FailedCounter.Inc(1)
			return true
		}
		p.PrimaryBranchStreamingLiveness.Reset()
		p.SuccessCounter.Inc(1)
		return true
	}
	// TODO(kjlubick) Processors should mark the SourceFiles table as ingested, not here.
	if err := p.IngestionStore.SetIngested(ctx, name, time.Now()); err != nil {
		sklog.Errorf("Could not write to ingestion store: %s", err)
		// We'll continue anyway. The IngestionStore is not a big deal.
	}
	p.SuccessCounter.Inc(1)
	return true
}

func startBackupPolling(ctx context.Context, isc ingestionServerConfig, sourcesToScan []ingestion.FileSearcher, pss *pubSubSource) {
	if isc.BackupPollInterval.Duration <= 0 {
		sklog.Infof("Skipping backup polling")
		return
	}

	pollingLiveness := metrics2.NewLiveness("gold_ingestion", map[string]string{
		"metric": "since_last_successful_poll",
		"source": "combined",
	})

	go util.RepeatCtx(ctx, isc.BackupPollInterval.Duration, func(ctx context.Context) {
		ctx, span := trace.StartSpan(ctx, "ingestion_backupPollingCycle", trace.WithSampler(trace.AlwaysSample()))
		defer span.End()
		startTime, endTime := getTimesToPoll(ctx, isc.BackupPollScope.Duration)
		totalIgnored, totalProcessed := 0, 0
		sklog.Infof("Starting backup polling for %d sources in time range [%s,%s]", len(sourcesToScan), startTime, endTime)
		for _, src := range sourcesToScan {
			ignored, processed := 0, 0
			files := src.SearchForFiles(ctx, startTime, endTime)
			for _, f := range files {
				ok, err := pss.IngestionStore.WasIngested(ctx, f)
				if err != nil {
					sklog.Errorf("Could not check ingestion store: %s", err)
				}
				if ok {
					ignored++
					continue
				}
				processed++
				pss.ingestFile(ctx, f)
			}
			srcName := "<unknown>"
			// Failure to do this can cause a race condition in tests.
			if stringer, ok := src.(fmt.Stringer); ok {
				srcName = stringer.String()
			}
			sklog.Infof("backup polling for %s processed/ignored: %d/%d", srcName, processed, ignored)
			totalIgnored += ignored
			totalProcessed += processed
		}
		pollingLiveness.Reset()
		sklog.Infof("Total backup polling [%s,%s] processed/ignored: %d/%d", startTime, endTime, totalProcessed, totalIgnored)
	})
}

func getTimesToPoll(ctx context.Context, duration time.Duration) (time.Time, time.Time) {
	endTS := now.Now(ctx).UTC()
	return endTS.Add(-duration), endTS
}

func startMetrics(ctx context.Context, pss *pubSubSource) {
	// This metric will let us get a sense of how well-utilized this processor is. It reads the
	// busy int of the processor (which is 0 when not busy) and increments the counter if the
	// int is non-zero.
	// Because we are updating the counter once per second, we can use rate() [which computes deltas
	// per second] on this counter to get a number between 0 and 1 to indicate wall-clock
	// utilization. Hopefully, this lets us know if we need to add more replicas.
	go func() {
		busy := metrics2.GetCounter("goldingestion_busy_pulses")
		for range time.Tick(time.Second) {
			if err := ctx.Err(); err != nil {
				return
			}
			i := atomic.LoadInt64(&pss.busy)
			if i > 0 {
				busy.Inc(1)
			}
		}
	}()
}
