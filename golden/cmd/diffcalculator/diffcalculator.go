// The diffcalculator executable listens to the Pub/Sub topic and processes diffs based on the
// messages passed in. For an overview of Pub/Sub, see https://cloud.google.com/pubsub/docs
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"path"

	"cloud.google.com/go/pubsub"
	gstorage "cloud.google.com/go/storage"
	"github.com/jackc/pgx/v4/pgxpool"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diff/worker"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/types"
)

const (
	// This subscription ID doesn't have to be unique instance by instance
	// because the unique topic id it is listening to will suffice.
	// By setting the subscriber ID to be the same on all instances of the diff worker,
	// only one of the diff workers will get each event (usually).
	subscriptionID = "gold-image-diffs"

	// An arbitrary amount.
	maxSQLConnections = 20

	// The GCS folder that contains the images, named by their digests.
	imgFolder = "dm-images-v1"
)

type diffCalculatorConfig struct {
	config.Common

	// PubsubEventTopic the event topic used for diff work
	DiffWorkTopic string `json:"diff_work_topic"`

	// Metrics service address (e.g., ':10110')
	PromPort string `json:"prom_port"`

	// Project ID that houses the pubsub topics
	PubsubProjectID string `json:"pubsub_project_id"`

	// TileToProcess is how many tiles of commits we should use as the number of available digests
	// to diff.
	TilesToProcess int `json:"tiles_to_process"`
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
	ctx := context.Background()

	db := mustInitSQLDatabase(ctx, dcc)
	gis := mustMakeGCSImageSource(ctx, dcc)
	sqlProcessor := processor{
		worker.New(db, gis, dcc.TilesToProcess),
	}

	sID := subscriptionID
	if dcc.Local {
		// This allows us to have an independent diffcalculator when running locally.
		sID += "-local"
	}
	sklog.Fatalf("Listening for work %s", listen(ctx, dcc, sID, sqlProcessor))
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

func listen(ctx context.Context, dcc diffCalculatorConfig, subscriberID string, p processor) error {
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

	subName := fmt.Sprintf("%s+%s", dcc.DiffWorkTopic, subscriberID)
	// Check that the subscription exists. Fail if it does not.
	sub := psc.Subscription(subName)
	if exists, err := sub.Exists(ctx); err != nil {
		return skerr.Wrapf(err, "checking for existing subscription %s", subName)
	} else if !exists {
		return skerr.Fmt("subscription %s does not exist in project %s", subName, dcc.PubsubProjectID)
	}

	// This process will handle one message at a time. This allows us to more finely control the
	// scaling up as necessary.
	sub.ReceiveSettings.MaxOutstandingMessages = 1
	sub.ReceiveSettings.NumGoroutines = 1

	// Blocks until context cancels or pubsub fails in a non retryable way.
	return skerr.Wrap(sub.Receive(ctx, p.processPubSubMessage))
}

type processor struct {
	calculator diff.Calculator
}

// processPubSubMessage processes the data in the given pubsub message and acks or nacks it
// as appropriate.
func (p processor) processPubSubMessage(ctx context.Context, msg *pubsub.Message) {
	if shouldAck := p.processMessage(ctx, msg.Data); shouldAck {
		msg.Ack()
	} else {
		msg.Nack()
	}
}

// processMessage reads the bytes as JSON and calls CalculateDiffs if those bytes were valid.
// We have this as its own function to make it easier to test (it's hard to instantiate a valid
// pubsub message without the emulator because there are private members that need initializing).
// It returns a bool that represents whether the message should be Ack'd (not retried) or Nack'd
// (retried later).
func (p processor) processMessage(ctx context.Context, msgData []byte) bool {
	var wm diff.WorkerMessage
	if err := json.Unmarshal(msgData, &wm); err != nil {
		sklog.Errorf("Invalid message passed in: %s\n%s", err, string(msgData))
		return true // ack this message so no other subscriber gets it (it will still be invalid).
	}
	err := p.calculator.CalculateDiffs(ctx, wm.Grouping, wm.AdditionalDigests)
	if err != nil {
		sklog.Errorf("Calculating diffs for %v: %s", wm, err)
		return false // Let this be tried again.
	}
	return true // successfully processed.
}
