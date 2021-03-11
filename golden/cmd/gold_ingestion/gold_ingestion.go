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

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo/bt_vcs"
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

	// Metrics service address (e.g., ':10110')
	PromPort string `json:"prom_port"`

	// PubSubFetchSize is how many worker messages to ask PubSub for. This defaults to 10, but for
	// instances that have many small files ingested, this can be higher for better utilization
	// and throughput.
	PubSubFetchSize int `json:"pubsub_fetch_size" optional:"true"`

	// The port to provide a web handler for /healthz
	ReadyPort string `json:"port"`

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

	// Set up the logging options.
	logOpts := []common.Opt{
		common.PrometheusOpt(&isc.PromPort),
	}

	common.InitWithMust("gold-ingestion", logOpts...)
	// We expect there to be a lot of ingestion work, so we sample 10% of them to avoid incurring
	// too much overhead.
	if err := tracing.Initialize(0.1); err != nil {
		sklog.Fatalf("Could not set up tracing: %s", err)
	}

	ctx := context.Background()

	// Initialize oauth client and start the ingesters.
	tokenSrc, err := auth.NewDefaultTokenSource(isc.Local, auth.SCOPE_USERINFO_EMAIL, storage.ScopeFullControl, pubsub.ScopePubSub, pubsub.ScopeCloudPlatform, swarming.AUTH_SCOPE, auth.SCOPE_GERRIT)
	if err != nil {
		sklog.Fatalf("Failed to auth: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(tokenSrc).With2xxOnly().WithDialTimeout(time.Second * 10).Client()

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

	// Set up the gitstore
	btConf := &bt_gitstore.BTConfig{
		InstanceID: isc.BTInstance,
		ProjectID:  isc.BTProjectID,
		TableID:    isc.GitBTTable,
		AppProfile: "gold-ingestion",
	}

	gitStore, err := bt_gitstore.New(ctx, btConf, isc.GitRepoURL)
	if err != nil {
		sklog.Fatalf("could not instantiate gitstore for %s: %s", isc.GitRepoURL, err)
	}

	// Set up VCS instance to track primary branch.
	vcs, err := bt_vcs.New(ctx, gitStore, isc.GitRepoBranch)
	if err != nil {
		sklog.Fatalf("could not instantiate BT VCS for %s", isc.GitRepoURL)
	}
	sklog.Infof("Created vcs client based on BigTable.")
	// Instantiate the secondary repo if one was specified.
	// TODO(kjlubick): skbug.com/9553
	if isc.SecondaryRepoURL != "" {
		sklog.Fatalf("Not yet implemented to have a secondary repo url")
	}

	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		sklog.Fatalf("Could not create GCS Client")
	}
	primaryBranchProcessor, src, err := getPrimaryBranchIngester(ctx, isc.PrimaryBranchConfig, gcsClient, vcs, sqlDB)
	if err != nil {
		sklog.Fatalf("Setting up primary branch ingestion: %s", err)
	}
	sourcesToScan := []ingestion.FileSearcher{src}

	var secondaryBranchLiveness metrics2.Liveness
	tryjobProcessor, src, err := getSecondaryBranchIngester(ctx, isc.SecondaryBranchConfig, gcsClient, client, sqlDB)
	if src != nil {
		sourcesToScan = append(sourcesToScan, src)
		secondaryBranchLiveness = metrics2.NewLiveness("gold_ingestion", map[string]string{
			"metric": "since_last_successful_streaming_result",
			"source": "secondary_branch",
		})
	}

	pss := &pubSubSource{
		IngestionStore:         ingestionStore,
		PrimaryBranchProcessor: primaryBranchProcessor,
		TryjobProcessor:        tryjobProcessor,
		PrimaryBranchStreamingLiveness: metrics2.NewLiveness("gold_ingestion", map[string]string{
			"metric": "since_last_successful_streaming_result",
			"source": "primary_branch",
		}),
		SecondaryBranchStreamingLiveness: secondaryBranchLiveness,
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

func getPrimaryBranchIngester(ctx context.Context, conf ingesterConfig, gcsClient *storage.Client, vcs *bt_vcs.BigTableVCS, db *pgxpool.Pool) (ingestion.Processor, ingestion.FileSearcher, error) {
	src := &ingestion.GCSSource{
		Client: gcsClient,
		Bucket: conf.Source.Bucket,
		Prefix: conf.Source.Prefix,
	}
	if ok := src.Validate(); !ok {
		return nil, nil, skerr.Fmt("Invalid GCS Source %#v", src)
	}

	var primaryBranchProcessor ingestion.Processor
	var err error
	if conf.Type == ingestion_processors.BigTableTraceStore {
		primaryBranchProcessor, err = ingestion_processors.PrimaryBranchBigTable(ctx, src, conf.ExtraParams, vcs)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		sklog.Infof("Configured BT-backed primary branch ingestion")
	} else if conf.Type == ingestion_processors.SQLPrimaryBranch {
		primaryBranchProcessor = ingestion_processors.PrimaryBranchSQL(src, conf.ExtraParams, db)
		sklog.Infof("Configured SQL primary branch ingestion")
	} else {
		return nil, nil, skerr.Fmt("unknown ingestion backend: %q", conf.Type)
	}
	return primaryBranchProcessor, src, nil
}

func getSecondaryBranchIngester(ctx context.Context, conf *ingesterConfig, gcsClient *storage.Client, hClient *http.Client, db *pgxpool.Pool) (ingestion.Processor, ingestion.FileSearcher, error) {
	if conf == nil { // not configured for secondary branch (e.g. tryjob) ingestion.
		return nil, nil, nil
	}
	src := &ingestion.GCSSource{
		Client: gcsClient,
		Bucket: conf.Source.Bucket,
		Prefix: conf.Source.Prefix,
	}
	if ok := src.Validate(); !ok {
		return nil, nil, skerr.Fmt("Invalid GCS Source %#v", src)
	}
	var sbProcessor ingestion.Processor
	var err error
	if conf.Type == ingestion_processors.SQLSecondaryBranch {
		sbProcessor, err = ingestion_processors.TryjobSQL(ctx, src, conf.ExtraParams, hClient, db)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		sklog.Infof("Configured SQL-backed secondary branch ingestion")
	} else {
		return nil, nil, skerr.Fmt("unknown ingestion backend: %q", conf.Type)
	}
	return sbProcessor, src, nil
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
	TryjobProcessor        ingestion.Processor
	// PrimaryBranchStreamingLiveness lets us have a metric to monitor the successful
	// streaming of data. It will be reset after each successful ingestion of a file from
	// the primary branch.
	PrimaryBranchStreamingLiveness metrics2.Liveness
	// SecondaryBranchStreamingLiveness lets us have a metric to monitor the successful
	// streaming of data. It will be reset after each successful ingestion of a file from
	// the secondary branch.
	SecondaryBranchStreamingLiveness metrics2.Liveness

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
			return false
		}
		// TODO(kjlubick) Processors should mark the SourceFiles table as ingested, not here.
		if err := p.IngestionStore.SetIngested(ctx, name, now(ctx)); err != nil {
			sklog.Errorf("Could not write to ingestion store: %s", err)
			// We'll continue anyway. The IngestionStore is not a big deal.
		}
		if err != nil {
			sklog.Errorf("Got non-retryable error for primary branch data for file %s: %s", name, err)
			return true
		}
		p.PrimaryBranchStreamingLiveness.Reset()
		return true
	}
	if p.TryjobProcessor == nil || !p.TryjobProcessor.HandlesFile(name) {
		sklog.Warningf("Got a file that no processor is configured for: %s", name)
		return true
	}
	err := p.TryjobProcessor.Process(ctx, name)
	if skerr.Unwrap(err) == ingestion.ErrRetryable {
		sklog.Warningf("Got retryable error for tryjob data for file %s", name)
		return false
	}
	// TODO(kjlubick) Processors should mark the SourceFiles table as ingested, not here.
	if err := p.IngestionStore.SetIngested(ctx, name, time.Now()); err != nil {
		sklog.Errorf("Could not write to ingestion store: %s", err)
		// We'll continue anyway. The IngestionStore is not a big deal.
	}
	if err != nil {
		sklog.Errorf("Got non-retryable error for tryjob data for file %s: %s", name, err)
		return true
	}
	p.SecondaryBranchStreamingLiveness.Reset()
	return true
}

func startBackupPolling(ctx context.Context, isc ingestionServerConfig, sourcesToScan []ingestion.FileSearcher, pss *pubSubSource) {
	pollingLiveness := metrics2.NewLiveness("gold_ingestion", map[string]string{
		"metric": "since_last_successful_poll",
		"source": "combined",
	})

	go util.RepeatCtx(ctx, isc.BackupPollInterval.Duration, func(ctx context.Context) {
		ctx, span := trace.StartSpan(ctx, "ingestion_backupPollingCycle")
		defer span.End()
		startTime, endTime := getTimesToPoll(ctx, isc.BackupPollScope.Duration)
		processed := int64(0)
		ignored := int64(0)

		for _, src := range sourcesToScan {
			// Failure to do this can cause a race condition in tests.
			if stringer, ok := src.(fmt.Stringer); ok {
				sklog.Infof("Performing backup scan of %s", stringer.String())
			}
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
		}
		pollingLiveness.Reset()
		sklog.Infof("Backup polling received/processed/ignored: %d/%d/%d", ignored+processed, processed, ignored)
	})
}

func getTimesToPoll(ctx context.Context, duration time.Duration) (time.Time, time.Time) {
	now := now(ctx)
	return now.Add(-duration), now
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

// overwriteNowKey is used by tests to make the time deterministic.
const overwriteNowKey = contextKey("overwriteNow")

type contextKey string

// now returns the current time or the time from the context.
func now(ctx context.Context) time.Time {
	if ts := ctx.Value(overwriteNowKey); ts != nil {
		return ts.(time.Time)
	}
	return time.Now()
}
