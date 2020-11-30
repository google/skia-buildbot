package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/types"
	"golang.org/x/sync/errgroup"
	"strconv"
	"sync"
	"time"
)

const (
	maxSQLConnections = 24
	lastNCommits      = 200 // how much trace data to get
)

func main() {
	local := flag.Bool("local", true, "Spin up a local instance of cockroachdb. If false, will connect to local port 26257.")
	port := flag.String("port", "", "Port on localhost to connect to. Only set if --local=false")
	dbName := flag.String("db_name", "benchmark_db", "name of database")

	tileStarts := 1
	denseTileWidth := 10
	flag.Parse()
	if *local {
		sklog.Infof("Using local db. Assuming data is already there (see cockroach_explore)")
		*port = "26257"
		*dbName = "db_for_tests"
	} else {
		sklog.Infof("Not using local db. Make sure you ran kubectl port-forward gold-cockroachdb-0 26234:26234")
		tileStarts = 40500
		denseTileWidth = 500
	}

	ctx := context.Background()
	conf, err := pgxpool.ParseConfig("postgresql://root@localhost:" + *port + "/" + *dbName + "?sslmode=disable")
	if err != nil {
		sklog.Fatalf("error getting postgress config: %s", err)
	}

	conf.MaxConns = maxSQLConnections
	db, err := pgxpool.ConnectConfig(ctx, conf)
	if err != nil {
		sklog.Fatalf("error connecting to the database: %s", err)
	}
	defer db.Close()

	// search for all untriaged at head grouped by corpus
	maxCorpus := findBreakdownByCorpus(ctx, db)

	defer timer.New("end to end search over corpus with the most digests").Stop()
	results, err := doUntriagedSearchAtHead(ctx, db, maxCorpus, tileStarts, denseTileWidth)
	if err != nil {
		sklog.Fatalf("Error while fetching %s", err)
	}
	sklog.Infof("Have the data for %d results", len(results))
}

func findBreakdownByCorpus(ctx context.Context, db *pgxpool.Pool) string {
	defer timer.New("breakdown by corpus").Stop()
	const statement = `SELECT corpus, count(*) FROM (
  SELECT corpus FROM ValuesAtHead
  WHERE most_recent_commit_id > 0 AND matches_any_ignore_rule = false AND expectation_label = 0 
) GROUP BY corpus;`

	rows, err := db.Query(ctx, statement)
	if err != nil {
		sklog.Fatalf("Querying breakdown %s", err)
	}
	defer rows.Close()

	type corpusCountRow struct {
		corpus string
		count  int
	}
	var results []corpusCountRow
	for rows.Next() {
		r := corpusCountRow{}
		err := rows.Scan(&r.corpus, &r.count)
		if err != nil {
			sklog.Fatalf("scanning %s", err)
		}
		results = append(results, r)
	}
	rows.Close()
	if len(results) == 0 {
		sklog.Errorf("No untriaged data at head")
		return ""
	}
	sklog.Infof("Count by corpus: %v", results)
	top := results[0]
	for _, r := range results {
		if r.count > top.count {
			top = r
		}
	}
	return top.corpus
}

type DigestResult struct {
	// What was drawn
	Digest types.Digest
	// What situation was it drawn in
	Grouping string
	// What specific setups drew it
	MatchingTraces []Trace
	// Who is this close to
	ClosestMatches []ComparisonDigest

	Status TriageStatus

	groupingID  sql.GroupingID
	digestBytes sql.Digest
}

type Trace struct {
	Keys    string // JSON of keys
	History []DataPoint

	id sql.TraceID
}

type DataPoint struct {
	Digest  types.Digest
	Options string // JSON of options
}

type ComparisonDigest struct {
	Category string // "closest positive", "closest negative", "most recent"
	Digest   types.Digest
	Metrics  diff.DiffMetrics
	Status   TriageStatus
	// Summarizing the traces that drew this digest.
	SummarizedParams paramtools.ParamSet
}

type TriageStatus struct {
	Label   expectations.Label
	Triager string
	Updated time.Time

	recordID *string
}

func doUntriagedSearchAtHead(ctx context.Context, db *pgxpool.Pool, corpus string, tileStarts, denseTileWidth int) ([]DigestResult, error) {
	// find trace_id, digest, grouping, keys, options_id of untriaged, unignored traces matching the corpus
	preliminaryResults, err := findUntriagedUnignored(ctx, db, corpus, tileStarts, denseTileWidth)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	eg, ctx := errgroup.WithContext(ctx)
	// These two things can be run in parallel because they update different parts of the
	// DigestResults
	eg.Go(func() error {
		// - Get history for trace_ids. We probably don't need to do this, except upon request.
		return fillInHistory(ctx, db, preliminaryResults, tileStarts)
	})
	eg.Go(func() error {
		// - Get closest diffs for digests based on groupings
		return findComparisonDigests(ctx, db, preliminaryResults, tileStarts)
	})

	return preliminaryResults, skerr.Wrap(eg.Wait())
}

func findUntriagedUnignored(ctx context.Context, db *pgxpool.Pool, corpus string, tileStarts, denseTileWidth int) ([]DigestResult, error) {
	defer timer.New("find untriaged and unignored for corpus " + corpus).Stop()

	// This does the simplest thing possible first - if it's slow, we can try fetching the options
	// and the groupings separately.
	const statement = `
WITH
TracesWithMatchingDigestStatuses AS (
	SELECT * FROM ValuesAtHead
	WHERE most_recent_commit_id >= $1 AND expectation_label = 0 AND
          matches_any_ignore_rule = FALSE AND corpus = $2
)
SELECT trace_id, most_recent_commit_id, digest, TracesWithMatchingDigestStatuses.keys, 
  expectation_record_id, Options.keys, Groupings.grouping_id, Groupings.keys
FROM TracesWithMatchingDigestStatuses
JOIN Groupings on TracesWithMatchingDigestStatuses.grouping_id = Groupings.grouping_id
INNER JOIN Options on TracesWithMatchingDigestStatuses.options_id = Options.options_id
;
`
	rows, err := db.Query(ctx, statement, tileStarts, corpus)
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching from ValuesAtHead")
	}
	defer rows.Close()
	type valuesRow struct {
		traceID             sql.TraceID
		commitIdx           int
		digest              sql.Digest
		keys                string
		options             string
		groupingID          sql.GroupingID
		grouping            string
		expectationRecordID *string // could be NULL, so this needs to be a pointer.
	}
	var results []valuesRow
	for rows.Next() {
		r := valuesRow{}
		err := rows.Scan(&r.traceID, &r.commitIdx, &r.digest, &r.keys, &r.expectationRecordID,
			&r.options, &r.groupingID, &r.grouping)
		if err != nil {
			return nil, skerr.Wrapf(err, "reading values at head row")
		}
		results = append(results, r)
	}
	rows.Close()

	type digestAndGrouping string
	byDigestAndGrouping := map[digestAndGrouping]*DigestResult{}
	for _, row := range results {
		newTrace := Trace{
			Keys:    row.keys,
			History: make([]DataPoint, denseTileWidth),
			id:      row.traceID,
		}

		digest := internDigest(row.digest)

		newTrace.History[row.commitIdx-tileStarts] = DataPoint{
			Digest:  digest,
			Options: row.options,
		}

		key := digestAndGrouping(digest) + digestAndGrouping(row.grouping)
		dr, ok := byDigestAndGrouping[key]
		if ok {
			dr.MatchingTraces = append(dr.MatchingTraces, newTrace)
			continue
		}
		byDigestAndGrouping[key] = &DigestResult{
			Digest:         digest,
			digestBytes:    row.digest,
			Grouping:       row.grouping,
			groupingID:     row.groupingID,
			MatchingTraces: []Trace{newTrace},
			Status: TriageStatus{
				Label:    expectations.Untriaged,
				recordID: row.expectationRecordID,
			},
		}
	}

	rv := make([]DigestResult, 0, len(byDigestAndGrouping))
	for _, dr := range byDigestAndGrouping {
		rv = append(rv, *dr)
	}

	return rv, nil
}

func fillInHistory(ctx context.Context, db *pgxpool.Pool, results []DigestResult, tileStarts int) error {
	defer timer.New("find trace history").Stop()
	traceToData := map[sql.MD5Hash][]DataPoint{}
	var traceIDs []interface{}
	for i := range results {
		for j, trace := range results[i].MatchingTraces {
			traceIDs = append(traceIDs, trace.id)
			traceToData[sql.AsMD5Hash(trace.id)] = results[i].MatchingTraces[j].History
		}
	}

	statement := `
SELECT trace_id, commit_id, digest, Options.keys
FROM TraceValues 
JOIN Options ON TraceValues.options_id = Options.options_id
WHERE trace_id IN `
	statement += sql.ValuesPlaceholders(len(traceIDs), 1)
	statement += ` AND commit_id > ` + strconv.Itoa(tileStarts)
	sklog.Infof("Fetching data for %d traces", len(traceIDs))
	rows, err := db.Query(ctx, statement, traceIDs...)
	if err != nil {
		return skerr.Wrapf(err, "fetching from TraceValues")
	}
	defer rows.Close()
	var lookup sql.MD5Hash
	var rowID sql.TraceID
	var commitID int
	var digest sql.Digest
	for rows.Next() {
		var options string
		err := rows.Scan(&rowID, &commitID, &digest, &options)
		if err != nil {
			return skerr.Wrapf(err, "reading row")
		}
		copy(lookup[:], rowID)
		traceToData[lookup][commitID-tileStarts].Digest = internDigest(digest)
		traceToData[lookup][commitID-tileStarts].Options = options
	}
	rows.Close()
	return nil
}

func findComparisonDigests(ctx context.Context, db *pgxpool.Pool, results []DigestResult, starts int) error {
	defer timer.New("find closest digests").Stop()

	const statement = `
WITH
PositiveOrNegativeDigests AS (
    SELECT digest, expectation_record_id, label FROM Expectations
    WHERE grouping_id = x'%032x' AND label > 0
),
TracesOfInterest AS (
  SELECT trace_id FROM Traces@ignored_grouping_idx
  WHERE Traces.grouping_id = x'%032x'
    AND matches_any_ignore_rule = false
),
ObservedDigestsInTile AS (
    SELECT DISTINCT digest FROM TiledTraceDigests
    JOIN TracesOfInterest ON TiledTraceDigests.trace_id = TracesOfInterest.trace_id
    WHERE TiledTraceDigests.start_commit_id >= %d
),
ComparisonBetweenUntriagedAndObserved AS (
    SELECT DiffMetrics.* FROM DiffMetrics
    JOIN ObservedDigestsInTile ON DiffMetrics.right_digest = ObservedDigestsInTile.digest
    WHERE DiffMetrics.left_digest IN %s
)
SELECT DISTINCT ON (left_digest, label)
  label, encode(left_digest, 'hex') as left_digest, right_digest, 
  num_diff_pixels, pixel_diff_percent, combined_metric, dimensions_differ,  max_rgba_diff, 
  ExpectationRecords.user_name, ExpectationRecords.time
FROM
  ComparisonBetweenUntriagedAndObserved
JOIN PositiveOrNegativeDigests
  ON ComparisonBetweenUntriagedAndObserved.right_digest = PositiveOrNegativeDigests.digest
INNER LOOKUP JOIN ExpectationRecords
ON ExpectationRecords.expectation_record_id = PositiveOrNegativeDigests.expectation_record_id
ORDER BY left_digest, label, combined_metric ASC, num_diff_pixels ASC, right_digest ASC;`

	type key struct {
		grouping sql.MD5Hash
		digest   types.Digest
	}

	resultMap := map[key]*DigestResult{}
	digestsByGrouping := map[sql.MD5Hash][]interface{}{} // we can batch lookups by grouping.
	for i, r := range results {
		grouping := sql.AsMD5Hash(r.groupingID)
		k := key{grouping: grouping, digest: r.Digest}
		resultMap[k] = &results[i] // Do this so we aren't taking the pointer of the loop variable
		digestsByGrouping[grouping] = append(digestsByGrouping[grouping], r.digestBytes)
	}

	sklog.Infof("Querying for %d groupings", len(digestsByGrouping))

	eg, ctx := errgroup.WithContext(ctx)
	for g, xd := range digestsByGrouping {
		func(grouping sql.MD5Hash, digests []interface{}) {
			eg.Go(func() error {
				st := fmt.Sprintf(statement, grouping, grouping, starts, sql.ValuesPlaceholders(len(digests), 1))
				rows, err := db.Query(ctx, st, digests...)
				if err != nil {
					return skerr.Wrapf(err, "looking up %d digests in grouping %x", len(digests), grouping)
				}
				defer rows.Close()

				k := key{grouping: grouping}

				for rows.Next() {
					var label sql.ExpectationsLabel
					var leftDigest types.Digest
					var rightDigest sql.Digest
					var metrics diff.DiffMetrics
					var triageStatus TriageStatus
					var rgba []int
					err := rows.Scan(&label, &leftDigest, &rightDigest,
						&metrics.NumDiffPixels, &metrics.PixelDiffPercent, &metrics.CombinedMetric, &metrics.DimDiffer,
						&rgba,
						&triageStatus.Triager, &triageStatus.Updated)
					if err != nil {
						return skerr.Wrapf(err, "reading row")
					}
					copy(metrics.MaxRGBADiffs[:], rgba)
					triageStatus.Label = sql.ConvertLabelToString(label)
					category := "closest_positive"
					if label == sql.LabelNegative {
						category = "closest_negative"
					}
					k.digest = leftDigest
					resultMap[k].ClosestMatches = append(resultMap[k].ClosestMatches, ComparisonDigest{
						Category:         category,
						Digest:           internDigest(rightDigest),
						Metrics:          metrics,
						Status:           triageStatus,
						SummarizedParams: nil, // TODO(kjlubick)
					})
				}
				rows.Close()
				return nil
			})
		}(g, xd)
	}

	if err := eg.Wait(); err != nil {
		return skerr.Wrapf(err, "Fetching metrics")
	}
	return nil
}

// Maybe in production, these intern maps live in the context?
var internedDigests = map[[md5.Size]byte]types.Digest{}
var internedDigestsMutex sync.RWMutex

// TODO(kjlubick) probably make this take a *[16]byte for efficiency
func internDigest(digest sql.Digest) types.Digest {
	hash := sql.AsMD5Hash(digest)
	internedDigestsMutex.RLock()
	if d, ok := internedDigests[hash]; ok {
		internedDigestsMutex.RUnlock()
		return d
	}
	internedDigestsMutex.RUnlock()
	d := types.Digest(hex.EncodeToString(digest))
	internedDigestsMutex.Lock()
	defer internedDigestsMutex.Unlock()
	internedDigests[hash] = d
	return d
}
