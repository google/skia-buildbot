package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"go.skia.org/infra/go/util"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

const (
	// This is how many commits from the beginning of history we are. The schema should work fine if there's a huge
	// block of missing data from the beginning, since no repo uses Gold from its inceception.
	fakeCommitOffset = 40000
	// 100 tests * 100 traces/test = 10k traces
	// 10k traces * 1000 commits * 0.6 data points / commit = 6M data points
	// There could be a few less traces than this if a test happens to randomly generate non-distinct
	// traces (<1% chance)
	numTests          = 100
	maxSQLConnections = 24

	proportionDigestsPostive   = 0.7
	proportionDigestsNegative  = 0.1
	proportionDigestsUntriaged = 0.2

	probabilityPositiveIgnored  = 0.01
	probabilityNegativeIgnored  = 0.5
	probabilityUntriagedIgnored = 0.8
)

func main() {
	rand.Seed(time.Now().UnixNano())
	local := flag.Bool("local", true, "Spin up a local instance of cockroachdb. If false, will connect to local port 26257.")
	port := flag.String("port", "", "Port on localhost to connect to. Only set if --local=false")

	flag.Parse()
	if *local {
		if *port != "" {
			sklog.Fatalf("Must not set port if local=true")
		}
		storageDir, err := sqltest.StartLocalCockroachDB()
		if err != nil {
			sklog.Fatalf("Could not start local cockroachdb: %s", err)
		}
		sklog.Infof("Check out localhost:8080 and %s for storage", storageDir)
		*port = "26257"
	} else {
		sklog.Infof("Not using local db. Make sure you ran kubectl port-forward gold-cockroachdb-0 26234:26234")
	}

	if err := createBenchmarkDBAndTables(*port); err != nil {
		sklog.Fatalf("Could not initialize db/tables: %s", err)
	}

	ctx := context.Background()
	conf, err := pgxpool.ParseConfig("postgresql://root@localhost:" + *port + "/benchmark_db?sslmode=disable")
	if err != nil {
		sklog.Fatalf("error getting postgress config: %s", err)
	}

	conf.MaxConns = maxSQLConnections
	db, err := pgxpool.ConnectConfig(ctx, conf)
	if err != nil {
		sklog.Fatalf("error connecting to the database: %s", err)
	}
	defer db.Close()

	if err = os.RemoveAll("out_tsv"); err != nil {
		sklog.Fatalf("Cleaning old out_tsv folder")
	}
	if err = os.MkdirAll("out_tsv", 0777); err != nil {
		sklog.Fatalf("Making output tsv folder")
	}

	defer timer.New("filling data").Stop()
	if err := fillData(ctx, db, fakeCommitOffset); err != nil {
		sklog.Fatalf("Could not fill with data: %s", err)
	}

	if err := uploadAndImport(ctx, *port); err != nil {
		sklog.Fatalf("Could not import tsv files: %s", err)
	}

	sklog.Infof("Done.")
}

func createBenchmarkDBAndTables(port string) error {
	out, err := exec.Command("cockroach", "sql", "--insecure", "--host=localhost:"+port,
		`--execute=DROP DATABASE IF EXISTS benchmark_db;`).CombinedOutput()
	if err != nil {
		return skerr.Wrapf(err, "dropping previous database: %s", out)
	}

	out, err = exec.Command("cockroach", "sql", "--insecure", "--host=localhost:"+port,
		`--execute=CREATE DATABASE IF NOT EXISTS benchmark_db;`).CombinedOutput()
	if err != nil {
		return skerr.Wrapf(err, "creating database: %s", out)
	}

	out, err = exec.Command("cockroach", "sql", "--insecure", "--host=localhost:"+port,
		"--database=benchmark_db", // Connect to benchmark_db that we just made
		"--execute="+sql.CockroachDBSchema,
	).CombinedOutput()
	if err != nil {
		return skerr.Wrapf(err, "creating tables: %s", out)
	}
	sklog.Infof("benchmark DB setup")
	return nil
}

func fillData(ctx context.Context, db *pgxpool.Pool, commitOffset int) error {
	eg, errCtx := errgroup.WithContext(ctx)

	for i := 0; i < numTests; i++ {
		if err := errCtx.Err(); err != nil {
			return skerr.Wrap(err)
		}
		fakeFile := "gs://skia-gold-benchmark/dm-json-v1/2020/10/31/13/dm" + strconv.Itoa(rand.Int()) + ".json"
		tracePairs := generateTracesForTest(randomTestSettings{
			Corpus:               "gm",
			TestName:             "blend_modes_" + strconv.Itoa(rand.Int()),
			NumCommits:           1000,
			MinAdditionalKeys:    8,
			MaxAdditionalKeys:    11,
			MaxAdditionalOptions: 4,
			MinTraceDensity:      0.2,
			MaxTraceDensity:      0.99,
			NumTraces:            100,
			MinTraceFlakiness:    2,
			MaxTraceFlakiness:    20,
			TraceDigestOverlap:   0.50,
			GlobalDigestOverlap:  0.05,
		})
		if err := storePrimaryBranchTraceData(ctx, errCtx, db, eg, tracePairs, fakeFile, commitOffset); err != nil {
			return skerr.Wrap(err)
		}
	}

	if err := eg.Wait(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func storePrimaryBranchTraceData(ctx, errCtx context.Context, db *pgxpool.Pool, eg *errgroup.Group, pairs []tiling.TracePair, sourceFile string, commitOffset int) error {
	sfh := md5.Sum([]byte(sourceFile))
	sourceFileHash := sfh[:] // convert array to slice

	// tracesToStore, optsToStore, groupingsToStore, paramset := storeToTraceValues(ctx, db, eg, pairs, sourceFileHash, commitOffset)
	tracesToStore, optsToStore, groupingsToStore, paramset, err := writeTraceValuesToTSV(ctx, pairs, sourceFileHash, commitOffset)
	if err != nil {
		return skerr.Wrap(err)
	}
	storeToKeyValueTable(errCtx, db, eg, insertTraces, tracesToStore)
	storeToKeyValueTable(errCtx, db, eg, insertOptions, optsToStore)
	storeToKeyValueTable(errCtx, db, eg, insertGroupings, groupingsToStore)
	//storeToPrimaryBranchParams(errCtx, db, eg, paramset, commitOffset, commitOffset+len(pairs[0].Trace.Digests))
	err = writePrimaryBranchParamsToTSV(ctx, paramset, commitOffset, commitOffset+len(pairs[0].Trace.Digests))
	if err != nil {
		return skerr.Wrap(err)
	}
	storeToSourceFiles(errCtx, db, eg, sourceFile, sourceFileHash)
	exp, err := storeToExpectations(ctx, errCtx, db, eg, pairs)
	if err != nil {
		return skerr.Wrap(err)
	}
	ignoredTraces := updateTracesWithIgnored(errCtx, db, eg, pairs, exp)
	storeToValuesAtHead(errCtx, db, eg, pairs, exp, ignoredTraces, commitOffset)
	//storeToDiffMetrics(errCtx, db, eg, exp)
	err = writeDiffMetricsToTSV(ctx, exp)
	if err != nil {
		return skerr.Wrap(err)
	}

	return nil
}

func storeToTraceValues(ctx context.Context, db *pgxpool.Pool, eg *errgroup.Group, pairs []tiling.TracePair, sourceFileHash []byte, commitOffset int) (map[string]string, map[string]string, map[string]string, paramtools.ParamSet) {
	// Actual max is 65535. Having a really large batch size has not been problematic yet, although
	// it can be if there is a lot of DB contention.
	const maxPlaceholders = 65000
	numCommits := pairs[0].Trace.Len()
	const traceValuesStatement = `INSERT INTO TraceValues (trace_id, shard, commit_id, grouping_id, digest, options_id, source_file_id) VALUES `
	const valuesPerRow = 7

	arguments := make([]interface{}, 0, maxPlaceholders)
	writeCurrentValues := func() {
		if len(arguments) == 0 {
			return
		}
		argCopy := make([]interface{}, len(arguments))
		copy(argCopy, arguments)
		arguments = arguments[:0]
		eg.Go(func() error {
			sklog.Infof("Inserting %d rows to TraceValues", len(argCopy)/valuesPerRow)
			statement := traceValuesStatement + sql.ValuesPlaceholders(valuesPerRow, len(argCopy)/valuesPerRow)
			statement += " RETURNING NOTHING"
			_, err := db.Exec(ctx, statement, argCopy...)
			if err != nil {
				return skerr.Wrapf(err, "inserting TraceValues")
			}
			return nil
		})
	}

	keysToStore := map[string]string{}      // maps string(traceHash) => keysJSON
	optsToStore := map[string]string{}      // maps string(optsHash) => optsJSON
	groupingsToStore := map[string]string{} // maps string(groupingHash) => groupingJSON
	paramset := paramtools.ParamSet{}

	for _, tp := range pairs {
		// Check to see if we have enough space
		if len(arguments)+numCommits*valuesPerRow > maxPlaceholders {
			writeCurrentValues()
		}
		keysJSON, traceHash := sql.SerializeMap(tp.Trace.Keys())
		keysToStore[string(traceHash)] = keysJSON
		optsJSON, optsHash := sql.SerializeMap(tp.Trace.Options())
		optsToStore[string(optsHash)] = optsJSON
		grouping := groupingFor(tp.Trace.Keys())
		groupingJSON, groupingHash := sql.SerializeMap(grouping)
		groupingsToStore[string(groupingHash)] = groupingJSON
		paramset.AddParams(tp.Trace.KeysAndOptions())
		// Add arguments for all digests in this trace.
		for i := range tp.Trace.Digests {
			digest := tp.Trace.Digests[i]
			if digest == tiling.MissingDigest {
				continue
			}
			digestBytes, err := sql.DigestToBytes(digest)
			if err != nil {
				panic(err)
			}
			arguments = append(arguments, traceHash, sql.ComputeTraceValueShard(traceHash), commitOffset+i,
				groupingHash, digestBytes, optsHash, sourceFileHash)
		}
	}
	writeCurrentValues()
	return keysToStore, optsToStore, groupingsToStore, paramset
}

func writeTraceValuesToTSV(ctx context.Context, pairs []tiling.TracePair, sourceFileHash []byte, commitOffset int) (map[string]string, map[string]string, map[string]string, paramtools.ParamSet, error) {
	path := filepath.Join("out_tsv", "tracevalues_"+strconv.Itoa(rand.Int())+".tsv")
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, nil, nil, skerr.Wrap(err)
	}
	defer util.Close(f)

	var buf strings.Builder
	writeTraceValue := func(traceID, shard []byte, commitID int, digest types.Digest, grouping, options, sourceFile []byte) {
		buf.WriteString(`\x`)
		buf.WriteString(hex.EncodeToString(traceID))
		buf.WriteRune('\t')
		buf.WriteString(`\x`)
		buf.WriteString(hex.EncodeToString(shard))
		buf.WriteRune('\t')
		buf.WriteString(strconv.Itoa(commitID))
		buf.WriteRune('\t')
		buf.WriteString(`\x`)
		buf.WriteString(string(digest))
		buf.WriteRune('\t')
		buf.WriteString(`\x`)
		buf.WriteString(hex.EncodeToString(grouping))
		buf.WriteRune('\t')
		buf.WriteString(`\x`)
		buf.WriteString(hex.EncodeToString(options))
		buf.WriteRune('\t')
		buf.WriteString(`\x`)
		buf.WriteString(hex.EncodeToString(sourceFile))
		buf.WriteRune('\n')
	}

	keysToStore := map[string]string{}      // maps string(traceHash) => keysJSON
	optsToStore := map[string]string{}      // maps string(optsHash) => optsJSON
	groupingsToStore := map[string]string{} // maps string(groupingHash) => groupingJSON
	paramset := paramtools.ParamSet{}

	for _, tp := range pairs {
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, nil, skerr.Wrap(err)
		}
		keysJSON, traceHash := sql.SerializeMap(tp.Trace.Keys())
		keysToStore[string(traceHash)] = keysJSON
		optsJSON, optsHash := sql.SerializeMap(tp.Trace.Options())
		optsToStore[string(optsHash)] = optsJSON
		grouping := groupingFor(tp.Trace.Keys())
		groupingJSON, groupingHash := sql.SerializeMap(grouping)
		groupingsToStore[string(groupingHash)] = groupingJSON
		paramset.AddParams(tp.Trace.KeysAndOptions())
		// Add arguments for all digests in this trace.
		for i := range tp.Trace.Digests {
			digest := tp.Trace.Digests[i]
			if digest == tiling.MissingDigest {
				continue
			}
			writeTraceValue(traceHash, sql.ComputeTraceValueShard(traceHash), commitOffset+i,
				digest, groupingHash, optsHash, sourceFileHash)
		}
		if _, err := f.WriteString(buf.String()); err != nil {
			return nil, nil, nil, nil, skerr.Wrap(err)
		}
		buf.Reset()
	}
	return keysToStore, optsToStore, groupingsToStore, paramset, nil
}

const insertTraces = `INSERT INTO Traces (trace_id, keys) VALUES `
const insertGroupings = `INSERT INTO Groupings (grouping_id, keys) VALUES `
const insertOptions = `INSERT INTO Options (options_id, keys) VALUES `

func storeToKeyValueTable(ctx context.Context, db *pgxpool.Pool, eg *errgroup.Group, insert string, toCreate map[string]string) {
	if len(toCreate) == 0 {
		return
	}
	const maxPlaceholders = 65000
	const valuesPerRow = 2

	arguments := make([]interface{}, 0, maxPlaceholders)
	for id, json := range toCreate {
		arguments = append(arguments, []byte(id), json)
	}
	if len(arguments) == 0 {
		return
	}
	if len(arguments) > maxPlaceholders {
		panic("need to do batching")
	}
	eg.Go(func() error {
		sklog.Infof("Inserting %d keys", len(arguments)/valuesPerRow)
		statement := insert + sql.ValuesPlaceholders(valuesPerRow, len(arguments)/valuesPerRow)
		// ON CONFLICT DO NOTHING because if the rows already exist, then the data we are writing
		// is immutable.
		statement += ` ON CONFLICT DO NOTHING;`
		_, err := db.Exec(ctx, statement, arguments...)
		if err != nil {
			return skerr.Wrapf(err, "inserting to table %s", insert)
		}
		return nil
	})
}

// storeToPrimaryBranchParams writes the given params as if they were seen on every 50th commit in
// the given range. This is much sparser than actual data, but it shouldn't matter too much and will
// be much faster when loading in data.
func storeToPrimaryBranchParams(ctx context.Context, db *pgxpool.Pool, eg *errgroup.Group, paramset paramtools.ParamSet, startCommit, endCommit int) {
	const maxPlaceholders = 1000 // When this number was higher, there was a lot of contention
	const primaryBranchParamsStatement = `INSERT INTO PrimaryBranchParams (key, value, commit_id) VALUES `
	const valuesPerRow = 3

	arguments := make([]interface{}, 0, maxPlaceholders)
	writeCurrentValues := func() {
		if len(arguments) == 0 {
			return
		}
		argCopy := make([]interface{}, len(arguments))
		copy(argCopy, arguments)
		arguments = arguments[:0]
		eg.Go(func() error {
			statement := primaryBranchParamsStatement + sql.ValuesPlaceholders(valuesPerRow, len(argCopy)/valuesPerRow)
			// ON CONFLICT DO NOTHING because the rows are immutable once written.
			statement += ` ON CONFLICT DO NOTHING;`
			_, err := db.Exec(ctx, statement, argCopy...)
			if err != nil {
				return skerr.Wrapf(err, "inserting PrimaryBranchParams")
			}
			return nil
		})
	}

	for commit := startCommit; commit < endCommit; commit += 50 {
		for key, values := range paramset {
			if len(arguments)+valuesPerRow*len(values) > maxPlaceholders {
				writeCurrentValues()
			}
			for _, value := range values {
				arguments = append(arguments, key, value, commit)
			}
		}
	}
	writeCurrentValues()
}

var writtenPrimaryBranchParams = map[[2]string]struct{}{}

func writePrimaryBranchParamsToTSV(ctx context.Context, paramset paramtools.ParamSet, startCommit, endCommit int) error {
	path := filepath.Join("out_tsv", "params_"+strconv.Itoa(rand.Int())+".tsv")
	f, err := os.Create(path)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.Close(f)

	paramset.Normalize()
	var rows [][]string
	for key, values := range paramset {
		for _, value := range values {
			// We've already written this in a previous test, so we don't need to again.
			if _, ok := writtenPrimaryBranchParams[[2]string{key, value}]; ok {
				continue
			}
			rows = append(rows, []string{key, value})
			writtenPrimaryBranchParams[[2]string{key, value}] = emptyStruct
		}
	}
	// Sort by Key to make sure our data is sorted by primary key:
	// https://www.cockroachlabs.com/docs/v20.1/import#performance
	sort.Slice(rows, func(i, j int) bool {
		return rows[i][0] < rows[j][0]
	})

	var buf strings.Builder
	for commit := startCommit; commit < endCommit; commit++ {
		if err := ctx.Err(); err != nil {
			return skerr.Wrap(err)
		}
		c := strconv.Itoa(commit)
		for _, row := range rows {
			buf.WriteString(c)
			buf.WriteRune('\t')
			buf.WriteString(row[0]) // key
			buf.WriteRune('\t')
			buf.WriteString(row[1]) // value
			buf.WriteRune('\n')
		}
		if _, err := f.WriteString(buf.String()); err != nil {
			return skerr.Wrap(err)
		}
		buf.Reset()
	}
	return nil
}

func storeToSourceFiles(ctx context.Context, db *pgxpool.Pool, eg *errgroup.Group, file string, hash []byte) {
	eg.Go(func() error {
		_, err := db.Exec(ctx,
			`INSERT INTO SourceFiles (source_file_id, source_file, last_ingested) VALUES ($1, $2, $3)`,
			hash, file, time.Now())
		return skerr.Wrapf(err, "storing to source files")
	})
}

type expectationResult struct {
	Expectations map[expectations.Label][]types.Digest
	RecordID     string
}

func (e *expectationResult) Classify(digest types.Digest) expectations.Label {
	for label, digests := range e.Expectations {
		for _, d := range digests {
			if d == digest {
				return label
			}
		}
	}
	panic("Unknown digest " + digest)
}

// storeToExpectations randomly assigns labels to the digests in the given trace pairs. Note that
// all trace pairs are expected to be from the same grouping (corpus + test), so the logic is much
// simpler than if pairs could be from multiple groupings.
func storeToExpectations(ctx, errCtx context.Context, db *pgxpool.Pool, eg *errgroup.Group, pairs []tiling.TracePair) (expectationResult, error) {
	alreadyTriaged := types.DigestSet{}

	exp := map[expectations.Label][]types.Digest{}
	for _, tp := range pairs {
		for _, digest := range tp.Trace.Digests {
			if digest == tiling.MissingDigest {
				continue
			}
			if _, ok := alreadyTriaged[digest]; ok {
				continue
			}
			label := randomExpectationLabel()
			exp[label] = append(exp[label], digest)
			alreadyTriaged[digest] = true
		}
	}

	grouping := groupingFor(pairs[0].Trace.Keys())
	_, groupingHash := sql.SerializeMap(grouping)

	row := db.QueryRow(ctx,
		`INSERT INTO ExpectationRecords (user_name, time, num_changes) VALUES ($1, $2, $3) RETURNING expectation_record_id`,
		"user"+strconv.Itoa(rand.Intn(10)), time.Now(), len(exp[expectations.Positive])+len(exp[expectations.Negative]))
	recordUUID := ""
	err := row.Scan(&recordUUID)
	if err != nil {
		return expectationResult{}, skerr.Wrapf(err, "getting new UUID")
	}

	eg.Go(func() error {
		const insertDeltas = `INSERT INTO ExpectationDeltas (expectation_record_id, grouping_id, digest, label_before, label_after) VALUES `
		const valuesPerRow = 5
		total := len(exp[expectations.Positive]) + len(exp[expectations.Negative])
		if total*valuesPerRow > 65000 {
			panic("need to implement batching")
		}
		arguments := make([]interface{}, 0, total*valuesPerRow)
		statement := insertDeltas + sql.ValuesPlaceholders(valuesPerRow, total)
		for _, digest := range exp[expectations.Positive] {
			digestBytes, err := sql.DigestToBytes(digest)
			if err != nil {
				panic(err)
			}
			arguments = append(arguments, recordUUID, groupingHash, digestBytes, sql.LabelUntriaged, sql.LabelPositive)
		}
		for _, digest := range exp[expectations.Negative] {
			digestBytes, err := sql.DigestToBytes(digest)
			if err != nil {
				panic(err)
			}
			arguments = append(arguments, recordUUID, groupingHash, digestBytes, sql.LabelUntriaged, sql.LabelNegative)
		}
		statement += ` RETURNING NOTHING;`
		_, err := db.Exec(errCtx, statement, arguments...)
		return skerr.Wrapf(err, "inserting ExpectationDeltas")
	})

	eg.Go(func() error {
		const insertExpectations = `INSERT INTO Expectations (expectation_record_id, grouping_id, digest, label) VALUES `
		const valuesPerRow = 4
		total := len(exp[expectations.Positive]) + len(exp[expectations.Negative]) + len(exp[expectations.Untriaged])
		if total*valuesPerRow > 65000 {
			panic("need to implement batching")
		}
		arguments := make([]interface{}, 0, total*valuesPerRow)
		statement := insertExpectations + sql.ValuesPlaceholders(valuesPerRow, total)
		for _, digest := range exp[expectations.Positive] {
			digestBytes, err := sql.DigestToBytes(digest)
			if err != nil {
				panic(err)
			}
			arguments = append(arguments, recordUUID, groupingHash, digestBytes, sql.LabelPositive)
		}
		for _, digest := range exp[expectations.Negative] {
			digestBytes, err := sql.DigestToBytes(digest)
			if err != nil {
				panic(err)
			}
			arguments = append(arguments, recordUUID, groupingHash, digestBytes, sql.LabelNegative)
		}
		for _, digest := range exp[expectations.Untriaged] {
			digestBytes, err := sql.DigestToBytes(digest)
			if err != nil {
				panic(err)
			}
			arguments = append(arguments, recordUUID, groupingHash, digestBytes, sql.LabelUntriaged)
		}
		statement += ` RETURNING NOTHING;`
		_, err := db.Exec(errCtx, statement, arguments...)
		return skerr.Wrapf(err, "inserting Expectations")
	})

	return expectationResult{
		Expectations: exp,
		RecordID:     recordUUID,
	}, nil
}

// updateTracesWithIgnored probabilistically ignores traces depending on their value at head.
func updateTracesWithIgnored(ctx context.Context, db *pgxpool.Pool, eg *errgroup.Group, pairs []tiling.TracePair, exp expectationResult) []sql.TraceID {
	var ignoredTraces []sql.TraceID
	var notIgnoredTraces []sql.TraceID

	for _, tp := range pairs {
		t := tp.Trace
		_, th := sql.SerializeMap(t.Keys())
		for i := len(t.Digests) - 1; i >= 0; i-- {
			digest := t.Digests[i]
			if digest == tiling.MissingDigest {
				continue
			}
			switch exp.Classify(digest) {
			case expectations.Positive:
				if rand.Float32() < probabilityPositiveIgnored {
					ignoredTraces = append(ignoredTraces, th)
				} else {
					notIgnoredTraces = append(notIgnoredTraces, th)
				}
			case expectations.Negative:
				if rand.Float32() < probabilityNegativeIgnored {
					ignoredTraces = append(ignoredTraces, th)
				} else {
					notIgnoredTraces = append(notIgnoredTraces, th)
				}
			case expectations.Untriaged:
				if rand.Float32() < probabilityUntriagedIgnored {
					ignoredTraces = append(ignoredTraces, th)
				} else {
					notIgnoredTraces = append(notIgnoredTraces, th)
				}
			}
			break // we only base ignore rules based on the most recent non-empty data point.
		}
	}

	cases := [][]sql.TraceID{notIgnoredTraces, ignoredTraces}
	for i, traces := range cases {
		if len(traces) == 0 {
			continue
		}
		func(state bool, traceIDs []sql.TraceID) {
			if len(traceIDs) > 65000 {
				panic("need batching")
			}
			arguments := make([]interface{}, len(traceIDs))
			for i := range traceIDs {
				arguments[i] = traceIDs[i]
			}
			eg.Go(func() error {
				updateTraces := `UPDATE Traces SET matches_any_ignore_rule = ` + strconv.FormatBool(state) + ` WHERE trace_id IN `
				updateTraces += sql.ValuesPlaceholders(len(traceIDs), 1)
				updateTraces += ` RETURNING NOTHING;`
				_, err := db.Exec(ctx, updateTraces, arguments...)
				return skerr.Wrapf(err, "Updating traces")
			})
		}(i == 0, traces)
	}
	return ignoredTraces
}

func storeToValuesAtHead(ctx context.Context, db *pgxpool.Pool, eg *errgroup.Group, pairs []tiling.TracePair, exp expectationResult, ignoredTraces []sql.TraceID, offset int) {
	const insert = `INSERT INTO ValuesAtHead
(trace_id, most_recent_commit_id, digest, grouping_id, keys, expectation_label,
expectation_record_id, matches_any_ignore_rule) VALUES `
	const valuesPerRow = 8
	const maxValues = 65000

	arguments := make([]interface{}, 0, maxValues)
	for _, tp := range pairs {
		trace := tp.Trace
		keysJSON, traceHash := sql.SerializeMap(trace.Keys())
		grouping := groupingFor(trace.Keys())
		_, groupingHash := sql.SerializeMap(grouping)
		mostRecentCommit := len(trace.Digests) - 1
		for ; mostRecentCommit >= 0; mostRecentCommit-- {
			if trace.Digests[mostRecentCommit] != tiling.MissingDigest {
				break
			}
		}
		digest := trace.Digests[mostRecentCommit]
		digestBytes, err := sql.DigestToBytes(digest)
		if err != nil {
			panic(err)
		}
		label := sql.ConvertLabelFromString(exp.Classify(digest))
		isIgnored := false
		for _, ignoredTrace := range ignoredTraces {
			if ignoredTrace.Equals(traceHash) {
				isIgnored = true
				break
			}
		}
		arguments = append(arguments, traceHash, offset+mostRecentCommit, digestBytes)
		arguments = append(arguments, groupingHash, keysJSON, label)
		arguments = append(arguments, exp.RecordID, isIgnored)
	}
	if len(arguments) > maxValues {
		panic("need to do batching")
	}

	eg.Go(func() error {
		statement := insert + sql.ValuesPlaceholders(valuesPerRow, len(pairs))
		statement += ` RETURNING NOTHING;`
		_, err := db.Exec(ctx, statement, arguments...)
		return skerr.Wrapf(err, "Inserting ValuesAtHead")
	})
}

type diffMetricFingerprint string

var diffsStored = map[diffMetricFingerprint]struct{}{}
var emptyStruct = struct{}{}

func storeToDiffMetrics(ctx context.Context, db *pgxpool.Pool, eg *errgroup.Group, er expectationResult) {
	exp := er.Expectations
	var digests []types.Digest
	digests = append(digests, exp[expectations.Positive]...)
	digests = append(digests, exp[expectations.Negative]...)
	digests = append(digests, exp[expectations.Untriaged]...)
	sklog.Infof("Generating diffs for %d digests, about %d rows", len(digests), len(digests)*len(digests))

	sort.Slice(digests, func(i, j int) bool {
		return digests[i] < digests[j]
	})

	const maxPlaceholders = 5000
	const insertDiffMetrics = `INSERT INTO DiffMetrics (left_digest, right_digest, num_diff_pixels, pixel_diff_percent,
         max_channel_diff, max_rgba_diff, dimensions_differ) VALUES `
	const valuesPerRow = 7
	arguments := make([]interface{}, 0, maxPlaceholders)
	writeCurrentValues := func() {
		if len(arguments) == 0 {
			return
		}
		argCopy := make([]interface{}, len(arguments))
		copy(argCopy, arguments)
		arguments = arguments[:0]
		eg.Go(func() error {
			sklog.Infof("Inserting %d rows to DiffMetrics", len(argCopy)/valuesPerRow)
			statement := insertDiffMetrics + sql.ValuesPlaceholders(valuesPerRow, len(argCopy)/valuesPerRow)
			statement += " ON CONFLICT DO NOTHING RETURNING NOTHING"
			_, err := db.Exec(ctx, statement, argCopy...)
			if err != nil {
				return skerr.Wrapf(err, "inserting DiffMetrics")
			}
			return nil
		})
	}

	for i, left := range digests {
		if err := ctx.Err(); err != nil {
			return
		}
		leftBytes, err := sql.DigestToBytes(left)
		if err != nil {
			panic(err)
		}
		for _, right := range digests[i+1:] {
			if len(arguments)+2*valuesPerRow > maxPlaceholders {
				writeCurrentValues()
			}
			rightBytes, err := sql.DigestToBytes(right)
			if err != nil {
				panic(err)
			}
			// There could be duplicates when the global digests are involved. This check dramatically speeds up
			// loading becuase (I believe) cockroach db doesn't have to retry the big inserts when there are not
			// any colision and it can stick to the fast path.
			fingerprint := generateFingerprint(left, right)
			if _, ok := diffsStored[fingerprint]; ok {
				continue
			}
			diffsStored[fingerprint] = emptyStruct
			numDiffPixels := r(1, 1000000)
			pixelDiffPercent := rand.Float32()
			maxChannelDiff := r(1, 255)
			rgbaDiff := []int{r(0, maxChannelDiff), r(0, maxChannelDiff), r(0, maxChannelDiff), maxChannelDiff}
			rand.Shuffle(len(rgbaDiff), func(i, j int) {
				rgbaDiff[i], rgbaDiff[j] = rgbaDiff[j], rgbaDiff[i]
			})
			dimsDiffer := rand.Float32() < 0.1
			// Insert left and right version
			arguments = append(arguments, leftBytes, rightBytes, numDiffPixels, pixelDiffPercent,
				maxChannelDiff, rgbaDiff, dimsDiffer)
			// Insert right and left version
			arguments = append(arguments, rightBytes, leftBytes, numDiffPixels, pixelDiffPercent,
				maxChannelDiff, rgbaDiff, dimsDiffer)
		}
	}
	writeCurrentValues()
}

func writeDiffMetricsToTSV(ctx context.Context, er expectationResult) error {
	path := filepath.Join("out_tsv", "diffmetrics_"+strconv.Itoa(rand.Int())+".tsv")
	f, err := os.Create(path)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.Close(f)

	exp := er.Expectations
	var digests []types.Digest
	digests = append(digests, exp[expectations.Positive]...)
	digests = append(digests, exp[expectations.Negative]...)
	digests = append(digests, exp[expectations.Untriaged]...)
	sklog.Infof("Generating diffs for %d digests, about %d rows", len(digests), len(digests)*len(digests))

	sort.Slice(digests, func(i, j int) bool {
		return digests[i] < digests[j]
	})

	var buf strings.Builder
	writeDiff := func(left, right types.Digest, numDiffPixels int, percentDiffPixels float32, maxChannelDiff int, rgbaDiff []int, dimensionsDiffer bool) {
		buf.WriteString(`\x`)
		buf.WriteString(string(left))
		buf.WriteRune('\t')
		buf.WriteString(`\x`)
		buf.WriteString(string(right))
		buf.WriteRune('\t')
		buf.WriteString(strconv.Itoa(numDiffPixels))
		buf.WriteRune('\t')
		buf.WriteString(strconv.FormatFloat(float64(percentDiffPixels), 'f', 5, 32))
		buf.WriteRune('\t')
		buf.WriteString(strconv.Itoa(maxChannelDiff))
		buf.WriteRune('\t')
		buf.WriteString(fmt.Sprintf("ARRAY[%d,%d,%d,%d]", rgbaDiff[0], rgbaDiff[1], rgbaDiff[2], rgbaDiff[3]))
		buf.WriteRune('\t')
		buf.WriteString(strconv.FormatBool(dimensionsDiffer))
		buf.WriteRune('\n')
	}
	for i, left := range digests {
		if err := ctx.Err(); err != nil {
			return skerr.Wrap(err)
		}
		for _, right := range digests[i+1:] {
			// There could be duplicates when the global digests are involved.
			fingerprint := generateFingerprint(left, right)
			if _, ok := diffsStored[fingerprint]; ok {
				continue
			}
			diffsStored[fingerprint] = emptyStruct
			numDiffPixels := r(1, 1000000)
			pixelDiffPercent := rand.Float32()
			maxChannelDiff := r(1, 255)
			rgbaDiff := []int{r(0, maxChannelDiff), r(0, maxChannelDiff), r(0, maxChannelDiff), maxChannelDiff}
			rand.Shuffle(len(rgbaDiff), func(i, j int) {
				rgbaDiff[i], rgbaDiff[j] = rgbaDiff[j], rgbaDiff[i]
			})
			dimsDiffer := rand.Float32() < 0.1
			// Insert left and right version
			writeDiff(left, right, numDiffPixels, pixelDiffPercent,
				maxChannelDiff, rgbaDiff, dimsDiffer)
			// Insert right and left version
			writeDiff(right, left, numDiffPixels, pixelDiffPercent,
				maxChannelDiff, rgbaDiff, dimsDiffer)
		}
		if _, err := f.WriteString(buf.String()); err != nil {
			return skerr.Wrap(err)
		}
		buf.Reset()
	}
	return nil
}

func generateFingerprint(left types.Digest, right types.Digest) diffMetricFingerprint {
	if left < right {
		return diffMetricFingerprint(left + right)
	}
	return diffMetricFingerprint(right + left)
}

func randomExpectationLabel() expectations.Label {
	r := rand.Float32()
	if r < proportionDigestsPostive {
		return expectations.Positive
	}
	r -= proportionDigestsPostive
	if r < proportionDigestsNegative {
		return expectations.Negative
	}
	return expectations.Untriaged
}

func groupingFor(keys map[string]string) map[string]string {
	return map[string]string{
		types.CorpusField:     keys[types.CorpusField],
		types.PrimaryKeyField: keys[types.PrimaryKeyField],
	}
}

func uploadAndImport(ctx context.Context, port string) error {
	// Compress tsv files with gzip (working around https://github.com/cockroachdb/cockroach/issues/56152)
	tsvFiles, err := filepath.Glob("out_tsv/*.tsv")
	if err != nil {
		return skerr.Wrap(err)
	}
	t := timer.New("gzipping and uploading data")
	// pigz uses all available cores and --fast is fine for TSV files.
	cmd := append([]string{"--fast"}, tsvFiles...)
	err = exec.Command("pigz", cmd...).Run()
	if err != nil {
		return skerr.Wrap(err)
	}
	zippedFiles, err := filepath.Glob("out_tsv/*.tsv.gz")
	if err != nil {
		return skerr.Wrap(err)
	}

	// upload gzipped files to GCS
	cmd = append([]string{"-m", "cp"}, zippedFiles...)
	cmd = append(cmd, "gs://skia-gold-benchmark-data/schemaV4/")

	err = exec.Command("gsutil", cmd...).Run()
	if err != nil {
		return skerr.Wrap(err)
	}
	t.Stop()

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		paramFiles := getImportableFiles(zippedFiles, "params")
		sklog.Infof("Importing params from %d files", len(paramFiles))
		out, err := exec.Command("cockroach", "sql", "--insecure", "--host=localhost:"+port,
			"--database=benchmark_db", // Connect to benchmark_db that we just made
			`--execute=
DROP TABLE PrimaryBranchParams;
IMPORT TABLE PrimaryBranchParams (
  commit_id INT4 NOT NULL,
  key STRING NOT NULL,
  value STRING NOT NULL,
  PRIMARY KEY (commit_id, key, value)
) CSV DATA ('`+strings.Join(paramFiles, "','")+`')
 WITH delimiter = e'\t';`,
		).CombinedOutput()
		if err != nil {
			return skerr.Wrapf(err, "importing PrimaryBranchParams from file %s: %s", paramFiles, out)
		}
		sklog.Infof("Imported params: \n%s", out)
		return nil
	})

	eg.Go(func() error {
		diffMetricFiles := getImportableFiles(zippedFiles, "diffmetrics")
		sklog.Infof("Importing DiffMetrics from %d files", len(diffMetricFiles))
		out, err := exec.Command("cockroach", "sql", "--insecure", "--host=localhost:"+port,
			"--database=benchmark_db", // Connect to benchmark_db that we just made
			`--execute=
DROP TABLE DiffMetrics;
IMPORT TABLE DiffMetrics (
  left_digest BYTES NOT NULL,
  right_digest BYTES NOT NULL,
  num_diff_pixels INT4,
  pixel_diff_percent FLOAT4,
  max_channel_diff INT2,
  max_rgba_diff INT2[],
  dimensions_differ BOOL,
  PRIMARY KEY (left_digest, right_digest)
) CSV DATA ('`+strings.Join(diffMetricFiles, "','")+`')
 WITH delimiter = e'\t';`,
		).CombinedOutput()
		if err != nil {
			return skerr.Wrapf(err, "importing DiffMetrics from file %s: %s", diffMetricFiles, out)
		}
		sklog.Infof("Imported diff metrics: \n%s", out)
		return nil
	})

	eg.Go(func() error {
		traceValueFiles := getImportableFiles(zippedFiles, "tracevalues")
		sklog.Infof("Importing TraceValues from %d files", len(traceValueFiles))
		out, err := exec.Command("cockroach", "sql", "--insecure", "--host=localhost:"+port,
			"--database=benchmark_db", // Connect to benchmark_db that we just made
			`--execute=
DROP TABLE TraceValues;
IMPORT TABLE TraceValues (
  trace_id BYTES NOT NULL,
  shard BYTES NOT NULL,
  commit_id INT4,
  digest BYTES NOT NULL,
  grouping_id BYTES NOT NULL,
  options_id BYTES NOT NULL,
  source_file_id BYTES NOT NULL,

  INDEX grouping_digest_commit_idx (grouping_id, digest, commit_id DESC),
  INDEX grouping_commit_digest_idx (grouping_id, commit_id DESC, digest),
  INDEX trace_commit_idx (trace_id, commit_id) STORING (digest),
  PRIMARY KEY (shard, commit_id, trace_id)
) CSV DATA ('`+strings.Join(traceValueFiles, "','")+`')
 WITH delimiter = e'\t';`,
		).CombinedOutput()
		if err != nil {
			return skerr.Wrapf(err, "importing TraceValues from file %s: %s", traceValueFiles, out)
		}
		sklog.Infof("Imported TraceValues: \n%s", out)
		return nil
	})

	return skerr.Wrap(eg.Wait())
}

func getImportableFiles(allFiles []string, prefix string) []string {
	var rv []string
	for _, f := range allFiles {
		if !strings.HasPrefix(f, "out_tsv/"+prefix) {
			continue
		}
		f = strings.TrimPrefix(f, "out_tsv/")
		rv = append(rv, "gs://skia-gold-benchmark-data/schemaV4/"+f)
	}
	return rv
}
