// The diffcalculator executable listens to the Pub/Sub topic and processes diffs based on the
// messages passed in. For an overview of Pub/Sub, see https://cloud.google.com/pubsub/docs
package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/pubsub"
	gstorage "cloud.google.com/go/storage"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/reconnectingmemcached"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diff/worker"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/tracing"
	"go.skia.org/infra/golden/go/types"
)

const (
	// An arbitrary amount.
	maxSQLConnections = 20

	// The GCS folder that contains the images, named by their digests.
	imgFolder = "dm-images-v1"
)

type diffCalculatorConfig struct {
	config.Common

	// DiffCacheNamespace is a namespace for differentiating the DiffCache entities. The instance
	// name is fine here.
	DiffCacheNamespace string `json:"diff_cache_namespace" optional:"true"`

	// DiffWorkSubscription is the subscription name used by all replicas of the diffcalculator.
	// By setting the subscriber ID to be the same on all instances of the diffcalculator,
	// only one of the replicas will get each event (usually). We like our subscription names
	// to be unique and keyed to the instance, for easier following up on "Why are there so many
	// backed up messages?"
	DiffWorkSubscription string `json:"diff_work_subscription"`

	// MemcachedServer is the address in the form dns_name:port
	// (e.g. gold-memcached-0.gold-memcached:11211).
	MemcachedServer string `json:"memcached_server" optional:"true"`

	// PubSubFetchSize is how many worker messages to ask PubSub for. This defaults to 10, but for
	// instances that have many tests, but most of the messages result in no-ops, this can be
	// higher for better utilization and throughput.
	PubSubFetchSize int `json:"pubsub_fetch_size" optional:"true"`
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

	var dcc diffCalculatorConfig
	if err := config.LoadFromJSON5(&dcc, commonInstanceConfig, thisConfig); err != nil {
		sklog.Fatalf("Reading config: %s", err)
	}
	sklog.Infof("Loaded config %#v", dcc)

	// Set up the logging options.
	logOpts := []common.Opt{
		common.PrometheusOpt(&dcc.PromPort),
	}

	common.InitWithMust("diffcalculator", logOpts...)
	// We expect there to be a lot of diff work, so we sample 1% of them by default
	// to avoid incurring too much overhead.
	tp := 0.01
	if dcc.TracingProportion > tp {
		tp = dcc.TracingProportion
	}
	if err := tracing.Initialize(tp); err != nil {
		sklog.Fatalf("Could not set up tracing: %s", err)
	}

	ctx := context.Background()

	db := mustInitSQLDatabase(ctx, dcc)
	gis := mustMakeGCSImageSource(ctx, dcc)
	diffcache := mustMakeDiffCache(ctx, dcc)
	sqlProcessor := &processor{
		calculator:  worker.New(db, gis, diffcache, dcc.WindowSize),
		ackCounter:  metrics2.GetCounter("diffcalculator_ack"),
		nackCounter: metrics2.GetCounter("diffcalculator_nack"),
	}

	go func() {
		// Wait at least 5 seconds for the pubsub connection to be initialized before saying
		// we are healthy.
		time.Sleep(5 * time.Second)
		http.HandleFunc("/healthz", httputils.ReadyHandleFunc)
		sklog.Fatal(http.ListenAndServe(dcc.ReadyPort, nil))
	}()

	startMetrics(ctx, sqlProcessor)

	sklog.Fatalf("Listening for work %s", listen(ctx, dcc, sqlProcessor))
}

func mustInitSQLDatabase(ctx context.Context, dcc diffCalculatorConfig) *pgxpool.Pool {
	if dcc.SQLDatabaseName == "" {
		sklog.Fatalf("Must have SQL Database Information")
	}
	url := sql.GetConnectionURL(dcc.SQLConnection, dcc.SQLDatabaseName)
	conf, err := pgxpool.ParseConfig(url)
	if err != nil {
		sklog.Fatalf("error getting postgres config %s: %s", url, err)
	}

	conf.MaxConns = maxSQLConnections
	db, err := pgxpool.ConnectConfig(ctx, conf)
	if err != nil {
		sklog.Fatalf("error connecting to the database: %s", err)
	}
	sklog.Infof("Connected to SQL database %s", dcc.SQLDatabaseName)
	return db
}

func mustMakeGCSImageSource(ctx context.Context, dcc diffCalculatorConfig) worker.ImageSource {
	// Reads credentials from the env variable GOOGLE_APPLICATION_CREDENTIALS.
	storageClient, err := gstorage.NewClient(ctx)
	if err != nil {
		sklog.Fatalf("Making GCS Image source: %s", storageClient)
	}
	return &gcsImageDownloader{
		client: storageClient,
		bucket: dcc.GCSBucket,
	}
}

// TODO(kjlubick) maybe deduplicate with storage.GCSClient
type gcsImageDownloader struct {
	client *gstorage.Client
	bucket string
}

// GetImage downloads the image with the corresponding digest (name) from GCS.
func (g *gcsImageDownloader) GetImage(ctx context.Context, digest types.Digest) ([]byte, error) {
	// intentionally using path because gcs is forward slashes
	imgPath := path.Join(imgFolder, string(digest)+".png")
	r, err := g.client.Bucket(g.bucket).Object(imgPath).NewReader(ctx)
	if err != nil {
		// If not image not found, this error path will be taken.
		return nil, skerr.Wrap(err)
	}
	defer util.Close(r)
	b, err := ioutil.ReadAll(r)
	return b, skerr.Wrap(err)
}

const failureReconnectLimit = 100

type memcachedDiffCache struct {
	client reconnectingmemcached.Client

	// namespace is the string to add to each key to avoid conflicts with more than one
	// gold instance.
	namespace string
}

func newMemcacheDiffCache(server, namespace string) (*memcachedDiffCache, error) {
	m := &memcachedDiffCache{
		client: reconnectingmemcached.NewClient(reconnectingmemcached.Options{
			Servers:                      []string{server},
			Timeout:                      time.Second,
			MaxIdleConnections:           4,
			AllowedFailuresBeforeHealing: failureReconnectLimit,
		}),
		namespace: namespace,
	}
	return m, m.client.Ping()
}

func key(namespace string, left, right types.Digest) string {
	return namespace + string(left+"_"+right)
}

// RemoveAlreadyComputedDiffs implements the DiffCache interface.
func (m *memcachedDiffCache) RemoveAlreadyComputedDiffs(ctx context.Context, left types.Digest, rightDigests []types.Digest) []types.Digest {
	ctx, span := trace.StartSpan(ctx, "memcached_removeDiffs")
	defer span.End()
	keys := make([]string, 0, len(rightDigests))
	for _, right := range rightDigests {
		if left == right {
			continue // this is never computed
		}
		keys = append(keys, key(m.namespace, left, right))
	}
	alreadyCalculated, ok := m.client.GetMulti(keys)
	if !ok {
		return rightDigests // on an error, assume all need to be queried from DB.
	}
	if len(alreadyCalculated) == len(keys) {
		return nil // common case, everything has been computed already.
	}
	// Go through all the inputs. For each one that is not in the returned value of "already been
	// calculated", add it to return value of "needs lookup/calculation".
	rv := make([]types.Digest, 0, len(rightDigests)-len(alreadyCalculated))
	for _, right := range rightDigests {
		if left == right {
			continue // this is never computed
		}
		if _, ok := alreadyCalculated[key(m.namespace, left, right)]; !ok {
			rv = append(rv, right)
		}
	}
	return rv
}

// StoreDiffComputed implements the DiffCache interface.
func (m *memcachedDiffCache) StoreDiffComputed(_ context.Context, left, right types.Digest) {
	m.client.Set(&memcache.Item{
		Key:   key(m.namespace, left, right),
		Value: []byte{0x01},
	})
}

// noopDiffCache pretends the memcached instance always does not have what we are looking up.
// It is useful for corp-instances where we do not have memcached setup.
type noopDiffCache struct{}

func (n noopDiffCache) RemoveAlreadyComputedDiffs(_ context.Context, _ types.Digest, right []types.Digest) []types.Digest {
	return right
}

func (n noopDiffCache) StoreDiffComputed(_ context.Context, _, _ types.Digest) {}

func mustMakeDiffCache(_ context.Context, dcc diffCalculatorConfig) worker.DiffCache {
	if dcc.MemcachedServer == "" || dcc.DiffCacheNamespace == "" {
		sklog.Infof("not using memcached")
		return noopDiffCache{}
	}
	dc, err := newMemcacheDiffCache(dcc.MemcachedServer, dcc.DiffCacheNamespace)
	if err != nil {
		sklog.Fatalf("Could not ping memcached server %s: %s", dcc.MemcachedServer, err)
	}
	return dc
}

func listen(ctx context.Context, dcc diffCalculatorConfig, p *processor) error {
	psc, err := pubsub.NewClient(ctx, dcc.PubsubProjectID)
	if err != nil {
		return skerr.Wrapf(err, "initializing pubsub client for project %s", dcc.PubsubProjectID)
	}

	// Check that the topic exists. Fail if it does not.
	t := psc.Topic(dcc.DiffWorkTopic)
	if exists, err := t.Exists(ctx); err != nil {
		return skerr.Wrapf(err, "checking for existing topic %s", dcc.DiffWorkTopic)
	} else if !exists {
		return skerr.Fmt("Diff work topic %s does not exist in project %s", dcc.DiffWorkTopic, dcc.PubsubProjectID)
	}

	// Check that the subscription exists. Fail if it does not.
	sub := psc.Subscription(dcc.DiffWorkSubscription)
	if exists, err := sub.Exists(ctx); err != nil {
		return skerr.Wrapf(err, "checking for existing subscription %s", dcc.DiffWorkSubscription)
	} else if !exists {
		return skerr.Fmt("subscription %s does not exist in project %s", dcc.DiffWorkSubscription, dcc.PubsubProjectID)
	}

	// This is a limit of how many messages to fetch when PubSub has no work. Waiting for PubSub
	// to give us messages can take a second or two, so we choose a small, but not too small
	// batch size.
	if dcc.PubSubFetchSize == 0 {
		sub.ReceiveSettings.MaxOutstandingMessages = 10
	} else {
		sub.ReceiveSettings.MaxOutstandingMessages = dcc.PubSubFetchSize
	}

	// This process will handle one message at a time. This allows us to more finely control the
	// scaling up as necessary.
	sub.ReceiveSettings.NumGoroutines = 1

	// Blocks until context cancels or pubsub fails in a non retryable way.
	return skerr.Wrap(sub.Receive(ctx, p.processPubSubMessage))
}

type processor struct {
	calculator  diff.Calculator
	ackCounter  metrics2.Counter
	nackCounter metrics2.Counter
	// busy is either 1 or 0 depending on if this processor is working or not. This allows us
	// to gather data on wall-clock utilization.
	busy int64
	// PubSub sometimes gives us more than one messages at a time. This mutex ensures that
	// we only really process one at a time, which makes sure we don't overload our CPU estimate
	// and we avoid cache thrashing.
	oneMessageAtATime sync.Mutex
}

// processPubSubMessage processes the data in the given pubsub message and acks or nacks it
// as appropriate.
func (p *processor) processPubSubMessage(ctx context.Context, msg *pubsub.Message) {
	p.oneMessageAtATime.Lock()
	defer p.oneMessageAtATime.Unlock()
	ctx, span := trace.StartSpan(ctx, "processFromPubSub")
	defer span.End()
	atomic.StoreInt64(&p.busy, 1)
	if shouldAck := p.processMessage(ctx, msg.Data); shouldAck {
		msg.Ack()
		p.ackCounter.Inc(1)
	} else {
		msg.Nack()
		p.nackCounter.Inc(1)
	}
	atomic.StoreInt64(&p.busy, 0)
}

// processMessage reads the bytes as JSON and calls CalculateDiffs if those bytes were valid.
// We have this as its own function to make it easier to test (it's hard to instantiate a valid
// pubsub message without the emulator because there are private members that need initializing).
// It returns a bool that represents whether the message should be Ack'd (not retried) or Nack'd
// (retried later).
func (p *processor) processMessage(ctx context.Context, msgData []byte) bool {
	defer metrics2.FuncTimer().Stop()
	var wm diff.WorkerMessage
	if err := json.Unmarshal(msgData, &wm); err != nil {
		sklog.Errorf("Invalid message passed in: %s\n%s", err, string(msgData))
		return true // ack this message so no other subscriber gets it (it will still be invalid).
	}
	if wm.Version != diff.WorkerMessageVersion {
		return true // This is an old or a new message, skip it.
	}
	// Prevent our workers from getting starved out with long-running tasks. Cancel them, an
	// requeue them. CalculateDiffs should be streaming results, so we get some partial progress.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	err := p.calculator.CalculateDiffs(ctx, wm.Grouping, wm.AdditionalLeft, wm.AdditionalRight)
	if err != nil {
		sklog.Errorf("Calculating diffs for %v: %s", wm, err)
		return false // Let this be tried again.
	}
	return true // successfully processed.
}

func startMetrics(ctx context.Context, p *processor) {
	// This metric will let us get a sense of how well-utilized this processor is. It reads the
	// busy int of the processor (which is 0 or 1) and increments the counter with that value.
	// Because we are updating the counter once per second, we can use rate() [which computes deltas
	// per second] on this counter to get a number between 0 and 1 to indicate wall-clock
	// utilization. Hopefully, this lets us know if we need to add more replicas.
	go func() {
		busy := metrics2.GetCounter("diffcalculator_busy_pulses")
		for range time.Tick(time.Second) {
			if err := ctx.Err(); err != nil {
				return
			}
			busy.Inc(atomic.LoadInt64(&p.busy))
		}
	}()
}
