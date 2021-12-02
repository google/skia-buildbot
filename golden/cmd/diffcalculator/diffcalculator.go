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
	"sync/atomic"
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

	calculateCLDataProportion = 0.8

	primaryBranchStalenessThreshold = time.Minute

	diffCalculationTimeout = 10 * time.Minute

	groupingCacheSize = 100_000
)

type diffCalculatorConfig struct {
	config.Common

	// HighContentionMode indicates to use fewer transactions when getting diff work. This can help
	// for instances with high amounts of secondary branches.
	HighContentionMode bool `json:"high_contention_mode"`
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
	if err := tracing.Initialize(tp, dcc.SQLDatabaseName); err != nil {
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
		calculator:         worker.New(db, gis, dcc.WindowSize),
		db:                 db,
		groupingCache:      gc,
		primaryCounter:     metrics2.GetCounter("diffcalculator_primarybranch_processed"),
		clsCounter:         metrics2.GetCounter("diffcalculator_cls_processed"),
		highContentionMode: dcc.HighContentionMode,
	}
	sqlProcessor.startMetrics(ctx)

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
	var secondaryShouldSleepUntil time.Time
	var primaryShouldSleepUntil time.Time
	const sleepDuration = 10 * time.Second
	sqlProcessor.setBusy(true)
	for {
		err := ctx.Err()
		if err != nil {
			return skerr.Wrapf(err, "context had error")
		}
		n := now.Now(ctx)
		if secondaryShouldSleepUntil.After(n) && primaryShouldSleepUntil.After(n) {
			// Neither has data, so sleep. This prevents us from slamming the SQL DB during periods
			// we are not busy
			sklog.Infof("No diffs to calculate, sleeping")
			sqlProcessor.setBusy(false)
			time.Sleep(sleepDuration)
		} else if secondaryShouldSleepUntil.Before(n) && !primaryShouldSleepUntil.Before(n) {
			// Both primary and secondary have data, so randomly choose one. We randomly choose
			// to avoid starving one of our "queues" if both are full.
			if rand.Float32() < calculateCLDataProportion {
				shouldSleep, err := sqlProcessor.computeDiffsForSecondaryBranch(ctx)
				if err != nil {
					sklog.Errorf("Error computing diffs for secondary: %s", err)
					continue
				}
				if shouldSleep {
					secondaryShouldSleepUntil = now.Now(ctx).Add(sleepDuration)
				}
			} else {
				shouldSleep, err := sqlProcessor.computeDiffsForPrimaryBranch(ctx)
				if err != nil {
					sklog.Errorf("Error computing diffs on primary branch: %s", err)
					continue
				}
				if shouldSleep {
					primaryShouldSleepUntil = now.Now(ctx).Add(sleepDuration)
				}
			}
		} else if secondaryShouldSleepUntil.Before(n) {
			shouldSleep, err := sqlProcessor.computeDiffsForSecondaryBranch(ctx)
			if err != nil {
				sklog.Errorf("Error computing diffs for secondary: %s", err)
				continue
			}
			if shouldSleep {
				secondaryShouldSleepUntil = now.Now(ctx).Add(sleepDuration)
			}
		} else {
			shouldSleep, err := sqlProcessor.computeDiffsForPrimaryBranch(ctx)
			if err != nil {
				sklog.Errorf("Error computing diffs on primary branch: %s", err)
				continue
			}
			if shouldSleep {
				primaryShouldSleepUntil = now.Now(ctx).Add(sleepDuration)
			}
		}
		sqlProcessor.setBusy(true)
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

	// busy is either 1 or 0 depending on if this processor is working or not. This allows us
	// to gather data on wall-clock utilization.
	busy               int64
	highContentionMode bool
}

// computeDiffsForPrimaryBranch fetches the grouping which has not had diff computation happen
// in the longest time and that some other process is not currently working on.
// The boolean value returned is true if there is no work available.
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
	err = crdbpgx.ExecuteTx(ctx, p.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const doneStatement = `UPDATE PrimaryBranchDiffCalculationWork
SET last_calculated_ts = $2 WHERE grouping_id = $1`
		if _, err := tx.Exec(ctx, doneStatement, groupingID, now.Now(ctx)); err != nil {
			return err // don't wrap, might be retried
		}
		return nil
	})
	if err != nil {
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

// computeDiffsForSecondaryBranch fetches the grouping for a branch which has not had diff
// computation happen since data was uploaded to that CL and that some other process is not
// currently working on. The boolean value returned is true if there is no work available.
func (p *processor) computeDiffsForSecondaryBranch(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	ctx, span := trace.StartSpan(ctx, "diffcalculator_computeDiffsForSecondaryBranch")
	defer span.End()
	if p.highContentionMode {
		return p.highContentionSecondaryBranch(ctx)
	}
	return p.lowContentionSecondaryBranch(ctx)
}

// highContentionSecondaryBranch finds a grouping for a branch to compute that is probabilistically
// not being worked on by another process and computes the diffs for it.
func (p *processor) highContentionSecondaryBranch(ctx context.Context) (bool, error) {
	ctx, span := trace.StartSpan(ctx, "highContentionSecondaryBranch")
	defer span.End()
	var groupingID schema.GroupingID
	var additionalDigests []types.Digest
	var branchName string

	ts := now.Now(ctx)
	// By not doing the select and update below in the same transaction, we work around cases where
	// there are many rows in SecondaryBranchDiffCalculationWork, causing the queries to take a
	// long time and causing many retries. At the same time, we want to limit the number of workers
	// who are wasting time by computing diffs for the same branch+grouping, so we select one of
	// the top candidates randomly.
	const selectStatement = `WITH
PossibleWork AS (
	SELECT branch_name, grouping_id, digests
	FROM SecondaryBranchDiffCalculationWork
	AS OF SYSTEM TIME '-0.1s'
	WHERE calculation_lease_ends < $1 AND last_calculated_ts < last_updated_ts
	ORDER BY last_calculated_ts ASC
	LIMIT 50 -- We choose 50 to reduce the chance of multiple workers picking the same one.
)
SELECT * FROM PossibleWork
AS OF SYSTEM TIME '-0.1s'
ORDER BY random() LIMIT 1
`
	row := p.db.QueryRow(ctx, selectStatement, ts)
	var digests []string
	if err := row.Scan(&branchName, &groupingID, &digests); err != nil {
		if err == pgx.ErrNoRows {
			// We've calculated data for every CL past the "last_updated_ts" time.
			return true, nil
		}
		return false, skerr.Wrap(err)
	}
	additionalDigests = convertType(digests)
	if len(additionalDigests) == 0 {
		return true, nil
	}

	err := crdbpgx.ExecuteTx(ctx, p.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const updateStatement = `UPDATE SecondaryBranchDiffCalculationWork
SET calculation_lease_ends = $3 WHERE branch_name = $1 AND grouping_id = $2`
		if _, err := tx.Exec(ctx, updateStatement, branchName, groupingID, ts.Add(diffCalculationTimeout)); err != nil {
			return err // don't wrap, might be retried
		}
		return nil
	})
	if err != nil {
		return false, skerr.Wrap(err)
	}

	grouping, err := p.expandGrouping(ctx, groupingID)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	if err := p.calculator.CalculateDiffs(ctx, grouping, additionalDigests); err != nil {
		return false, skerr.Wrap(err)
	}
	err = crdbpgx.ExecuteTx(ctx, p.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const doneStatement = `UPDATE SecondaryBranchDiffCalculationWork
SET last_calculated_ts = $3 WHERE branch_name = $1 AND grouping_id = $2`
		if _, err := tx.Exec(ctx, doneStatement, branchName, groupingID, now.Now(ctx)); err != nil {
			return err // don't wrap, might be retried
		}
		return nil
	})
	if err != nil {
		return false, skerr.Wrap(err)
	}
	p.clsCounter.Inc(1)
	return false, nil
}

// lowContentionSecondaryBranch finds a grouping for a branch to compute that is guaranteed not
// to be worked on by another process and computes the diffs for it.
func (p *processor) lowContentionSecondaryBranch(ctx context.Context) (bool, error) {
	ctx, span := trace.StartSpan(ctx, "lowContentionSecondaryBranch")
	defer span.End()
	var groupingID schema.GroupingID
	var additionalDigests []types.Digest
	var branchName string

	// By finding the best work candidate and setting the lease time in the same transaction, we
	// are confident that no other worker will be computing the same diffs at the same time.
	// This does not work well if SecondaryBranchDiffCalculationWork has too many rows in it;
	// see highContentionSecondaryBranch for an alternative.
	err := crdbpgx.ExecuteTx(ctx, p.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		ts := now.Now(ctx)
		const selectStatement = `SELECT branch_name, grouping_id, digests
FROM SecondaryBranchDiffCalculationWork
WHERE calculation_lease_ends < $1 AND last_calculated_ts < last_updated_ts
ORDER BY last_calculated_ts ASC
LIMIT 1`
		row := tx.QueryRow(ctx, selectStatement, ts)
		var digests []string
		if err := row.Scan(&branchName, &groupingID, &digests); err != nil {
			if err == pgx.ErrNoRows {
				// We've calculated data for every CL past the "last_updated_ts" time.
				return nil
			}
			return err // don't wrap - might be retried
		}
		additionalDigests = convertType(digests)

		const updateStatement = `UPDATE SecondaryBranchDiffCalculationWork
SET calculation_lease_ends = $3 WHERE branch_name = $1 AND grouping_id = $2`
		if _, err := tx.Exec(ctx, updateStatement, branchName, groupingID, ts.Add(diffCalculationTimeout)); err != nil {
			return err // don't wrap, might be retried
		}
		return nil
	})
	if err != nil {
		return false, skerr.Wrap(err)
	}
	if len(additionalDigests) == 0 {
		return true, nil
	}
	grouping, err := p.expandGrouping(ctx, groupingID)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	if err := p.calculator.CalculateDiffs(ctx, grouping, additionalDigests); err != nil {
		return false, skerr.Wrap(err)
	}
	err = crdbpgx.ExecuteTx(ctx, p.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const doneStatement = `UPDATE SecondaryBranchDiffCalculationWork
SET last_calculated_ts = $3 WHERE branch_name = $1 AND grouping_id = $2`
		if _, err := tx.Exec(ctx, doneStatement, branchName, groupingID, now.Now(ctx)); err != nil {
			return err // don't wrap, might be retried
		}
		return nil
	})
	if err != nil {
		return false, skerr.Wrap(err)
	}
	p.clsCounter.Inc(1)
	return false, nil
}

func (p *processor) setBusy(b bool) {
	if b {
		atomic.StoreInt64(&p.busy, 1)
	} else {
		atomic.StoreInt64(&p.busy, 0)
	}
}

// convertType turns a slice of strings into a slice of types.Digest. pgx cannot assign one to
// the other when reading from the DB, so we take the []string that is stored in the SQLDB and
// convert it to []types.Digest needed elsewhere.
func convertType(digests []string) []types.Digest {
	rv := make([]types.Digest, 0, len(digests))
	for _, d := range digests {
		rv = append(rv, types.Digest(d))
	}
	return rv
}

func (p *processor) startMetrics(ctx context.Context) {
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
