// The diffcalculator executable listens to the Pub/Sub topic and processes diffs based on the
// messages passed in. For an overview of Pub/Sub, see https://cloud.google.com/pubsub/docs
package main

import (
	"context"
	"flag"
	"io/ioutil"
	"math/rand"
	"net/http"
	"path"
	"time"

	gstorage "cloud.google.com/go/storage"
	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diff/worker"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/tracing"
	"go.skia.org/infra/golden/go/types"
)

const (
	// An arbitrary amount.
	maxSQLConnections = 20

	// The GCS folder that contains the images, named by their digests.
	imgFolder = "dm-images-v1"

	calculateCLDataProportion = 0.7

	primaryBranchStalenessThreshold = time.Minute

	diffCalculationTimeout = 10 * time.Minute

	groupingCacheSize = 100_000
)

type diffCalculatorConfig struct {
	config.Common
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
	gc, err := lru.New(groupingCacheSize)
	if err != nil {
		sklog.Fatalf("Could not initialize cache: %s", err)
	}

	sqlProcessor := &processor{
		calculator:     worker.NewV2(db, gis, dcc.WindowSize),
		db:             db,
		groupingCache:  gc,
		primaryCounter: metrics2.GetCounter("diffcalculator_primarybranch_processed"),
		clsCounter:     metrics2.GetCounter("diffcalculator_cls_processed"),
	}

	go func() {
		// Wait at least 5 seconds for the db connection to be initialized before saying
		// we are healthy.
		time.Sleep(5 * time.Second)
		http.HandleFunc("/healthz", httputils.ReadyHandleFunc)
		sklog.Fatal(http.ListenAndServe(dcc.ReadyPort, nil))
	}()

	sklog.Fatalf("Stopped while polling for work %s", beginPolling(ctx, sqlProcessor))
}

// beginPolling will continuously try to find work to compute either from CLs or the primary branch.
func beginPolling(ctx context.Context, sqlProcessor *processor) error {
	rand.Seed(time.Now().UnixNano())
	for {
		if err := ctx.Err(); err != nil {
			return skerr.Wrap(err)
		}
		if rand.Float32() < calculateCLDataProportion {
			if err := sqlProcessor.computeDiffsForCL(ctx); err != nil {
				sklog.Errorf("Error computing diffs for CL: %s", err)
				continue
			}
		} else {
			shouldSleep, err := sqlProcessor.computeDiffsForPrimaryBranch(ctx)
			if err != nil {
				sklog.Errorf("Error computing diffs on primary branch: %s", err)
				continue
			}
			if shouldSleep {
				// TODO(kjlubick) make sure we don't poll as fast as possible when there is
				// 		no work to do.
			}
		}
	}
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

type processor struct {
	db             *pgxpool.Pool
	calculator     diff.Calculator
	groupingCache  *lru.Cache
	primaryCounter metrics2.Counter
	clsCounter     metrics2.Counter
}

// computeDiffsForPrimaryBranch fetches the grouping which has not had diff computation happen
// in the longest time and that some other process is not currently working on.
func (p *processor) computeDiffsForPrimaryBranch(ctx context.Context) (bool, error) {
	// Prevent our workers from getting starved out with long-running tasks. Cancel them, an
	// requeue them. CalculateDiffs should be streaming results, so we get some partial progress.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	ctx, span := trace.StartSpan(ctx, "diffcalculator_computeDiffsForPrimaryBranch")
	defer span.End()

	hasWork := false
	var groupingID schema.GroupingID

	err := crdbpgx.ExecuteTx(ctx, p.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		ts := now.Now(ctx)
		const selectStatement = `SELECT grouping_id
FROM PrimaryBranchDiffCalculationWork
WHERE calculation_lease_ends < $1 AND last_calculated_ts < $2
ORDER BY last_calculated_ts ASC
LIMIT 1`
		row := tx.QueryRow(ctx, selectStatement, ts, ts.Add(-1*primaryBranchStalenessThreshold))

		if err := row.Scan(&groupingID); err != nil {
			if err == pgx.ErrNoRows {
				// We've calculated data for the entire primary branch to better than the threshold,
				// so we return because there's nothing to do right now.
				return nil
			}
			return err // don't wrap - might be retried
		}

		const updateStatement = `UPDATE PrimaryBranchDiffCalculationWork
SET calculation_lease_ends = $2 WHERE grouping_id = $1`
		if _, err := tx.Exec(ctx, updateStatement, groupingID, ts.Add(diffCalculationTimeout)); err != nil {
			return err // don't wrap, might be retried
		}
		hasWork = true
		return nil
	})
	if err != nil {
		return false, skerr.Wrap(err)
	}
	if !hasWork {
		return true, nil
	}
	grouping, err := p.expandGrouping(ctx, groupingID)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	if err := p.calculator.CalculateDiffs(ctx, grouping, nil); err != nil {
		return false, skerr.Wrap(err)
	}
	p.primaryCounter.Inc(1)
	return false, nil
}

// expandGrouping returns the params associated with the grouping id. It will use the cache - if
// there is a cache miss, it will look it up, add it to the cache and return it.
func (p *processor) expandGrouping(ctx context.Context, groupingID schema.GroupingID) (paramtools.Params, error) {
	ctx, span := trace.StartSpan(ctx, "expandGrouping")
	defer span.End()
	var groupingKeys paramtools.Params
	if gk, ok := p.groupingCache.Get(string(groupingID)); ok {
		return gk.(paramtools.Params), nil
	} else {
		const statement = `SELECT keys FROM Groupings WHERE grouping_id = $1`
		row := p.db.QueryRow(ctx, statement, groupingID)
		if err := row.Scan(&groupingKeys); err != nil {
			return nil, skerr.Wrap(err)
		}
		p.groupingCache.Add(string(groupingID), groupingKeys)
	}
	return groupingKeys, nil
}

func (p *processor) computeDiffsForCL(ctx context.Context) error {
	return skerr.Fmt("Not impl")
}
