package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
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
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

const (
	// This is how many commits from the beginning of history we are. The schema should work fine if there's a huge
	// block of missing data from the beginning, since no repo uses Gold from its inceception.
	fakeCommitOffset = 40000
	// As of November 2020, Skia had 1.4M traces across 3300 tests. This averages to ~424 traces
	// per test, although those are *not* evenly distributed. For benchmarking, we'll round up and
	// pretend they are consistent.
	tracesPerTest = 500
	// 20 tests * 500 traces/test = 10k traces
	// 10k traces * 1000 commits * 0.6 data points / commit = 6M data points
	numTests = 1

	// Each CL will generate the same traces as the primary branch, with a random chance to produce
	// untriaged data instead. Additionally, they will each add some number of new traces on
	// top of the ones from the primary branch.
	numCLs         = 30
	newTracesOnCLs = 10

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
	port := flag.String("port", "", "Port on localhost to connect to. Only set if --local=false")
	skipGeneration := flag.Bool("skip_generation", false, "If true, won't generate new data")
	skipUpload := flag.Bool("skip_upload", false, "If true, won't re-compress or re-upload new data")

	flag.Parse()
	sklog.Infof("Not using local db. Make sure you ran kubectl port-forward gold-cockroachdb-0 26234:26234")

	if !*skipGeneration {
		if err := createBenchmarkDBAndTables(*port); err != nil {
			sklog.Fatalf("Could not initialize db/tables: %s", err)
		}
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

	if !*skipGeneration {
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
	}

	if err := uploadAndImport(ctx, *skipUpload); err != nil {
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
	corpora := []string{"corpus1", "corpus2", "corpus3", "corpus4", "corpus5"}

	for i := 0; i < numTests; i++ {
		if err := errCtx.Err(); err != nil {
			break
		}
		corpus := corpora[i%len(corpora)]
		fakeFile := "gs://skia-gold-benchmark/dm-json-v1/2020/10/31/13/dm" + strconv.Itoa(rand.Int()) + ".json"
		tracePairs := generateTracesForTest(randomTestSettings{
			Corpus:               corpus,
			TestName:             "test_" + corpus + strconv.Itoa(rand.Int()),
			NumCommits:           1000,
			MinAdditionalKeys:    8,
			MaxAdditionalKeys:    11,
			MaxAdditionalOptions: 4,
			MinTraceDensity:      0.2,
			MaxTraceDensity:      1,
			NumTraces:            tracesPerTest,
		})
		if err := storePrimaryBranchTraceData(ctx, errCtx, db, eg, tracePairs, fakeFile, commitOffset); err != nil {
			return skerr.Wrap(err)
		}

		for j := 0; j < numCLs; j++ {
			numUntriagedFromPrimary := r(0, tracesPerTest/10)
			fakeFile := "gs://skia-gold-benchmark/trybot/dm-json-v1/2020/10/31/13/dm" + strconv.Itoa(rand.Int()) + ".json"
			clID := fmt.Sprintf("gerrit_%d", j+1000)
			psID := fmt.Sprintf("ps_%d", j+1)
			if err := storeChangeListTraceData(ctx, errCtx, db, eg, clID, psID, fakeFile, tracePairs, numUntriagedFromPrimary); err != nil {
				return skerr.Wrap(err)
			}
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

	optsToStore, groupingsToStore, paramset, err := writeTraceValuesToTSV(ctx, pairs, sourceFileHash, commitOffset)
	if err != nil {
		return skerr.Wrap(err)
	}
	storeToKeyValueTable(errCtx, db, eg, insertOptions, optsToStore)
	storeToKeyValueTable(errCtx, db, eg, insertGroupings, groupingsToStore)
	err = writePrimaryBranchParamsToTSV(ctx, paramset, commitOffset, commitOffset+len(pairs[0].Trace.Digests))
	if err != nil {
		return skerr.Wrap(err)
	}
	storeToSourceFiles(errCtx, db, eg, sourceFile, sourceFileHash)
	exp, err := storeToExpectations(ctx, errCtx, db, eg, pairs)
	if err != nil {
		return skerr.Wrap(err)
	}
	ignoredTraces := storeToTraces(errCtx, db, eg, pairs, exp)
	storeToValuesAtHead(errCtx, db, eg, pairs, exp, ignoredTraces, commitOffset)
	err = writeDiffMetricsToTSV(ctx, exp)
	if err != nil {
		return skerr.Wrap(err)
	}

	return nil
}

func storeChangeListTraceData(ctx, errCtx context.Context, db *pgxpool.Pool, eg *errgroup.Group, clID, psID, sourceFile string, pairs []tiling.TracePair, numUntriaged int) error {
	sfh := md5.Sum([]byte(sourceFile))
	sourceFileHash := sfh[:] // convert array to slice

	// generate new traces using some CL-only params for easy distinction
	data, paramset := convertToCLData(pairs)
	newData, newKeys := copyAndMutateWithCLKeys(data[0:newTracesOnCLs])
	data = append(data, newData...)
	// randomly produce n new digests
	newDigests, allDigests := sprinkleWithNewDigests(data, numUntriaged)

	// store new+old traces in SecondaryBranchTraces (TSV)
	err := writeSecondaryTraceValuesToTSV(ctx, data, sourceFileHash, clID, psID)
	if err != nil {
		return skerr.Wrap(err)
	}
	// Intentionally no new groupings or options
	// Store params to SecondaryBranchParams (TSV)
	paramset.AddParamSet(newKeys)
	err = writeSecondaryBranchParamsToTSV(ctx, paramset, clID, psID)
	if err != nil {
		return skerr.Wrap(err)
	}
	// produce new diff metrics and store to DiffMetrics (TSV)
	err = writeRandomDiffMetricsToTSV(ctx, newDigests.Keys(), allDigests.Keys())
	if err != nil {
		return skerr.Wrap(err)
	}

	err = storeToSecondaryBranchExpectations(ctx, errCtx, db, eg, clID, newDigests.Keys(), data)
	if err != nil {
		return skerr.Wrap(err)
	}
	storeToChangeListPatchset(errCtx, db, eg, clID, psID)
	return nil
}

func writeTraceValuesToTSV(ctx context.Context, pairs []tiling.TracePair, sourceFileHash []byte, commitOffset int) (map[string]string, map[string]string, paramtools.ParamSet, error) {
	path := filepath.Join("out_tsv", "tracevalues_"+strconv.Itoa(rand.Int())+".tsv")
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, nil, skerr.Wrap(err)
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

	optsToStore := map[string]string{}      // maps string(optsHash) => optsJSON
	groupingsToStore := map[string]string{} // maps string(groupingHash) => groupingJSON
	paramset := paramtools.ParamSet{}

	for _, tp := range pairs {
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, skerr.Wrap(err)
		}
		_, traceID := sql.SerializeMap(tp.Trace.Keys())
		optsJSON, optionsID := sql.SerializeMap(tp.Trace.Options())
		optsToStore[string(optionsID)] = optsJSON
		grouping := groupingFor(tp.Trace.Keys())
		groupingJSON, groupingID := sql.SerializeMap(grouping)
		groupingsToStore[string(groupingID)] = groupingJSON
		paramset.AddParams(tp.Trace.KeysAndOptions())
		// Add arguments for all digests in this trace.
		for i := range tp.Trace.Digests {
			digest := tp.Trace.Digests[i]
			if digest == tiling.MissingDigest {
				continue
			}
			writeTraceValue(traceID, sql.ComputeTraceValueShard(traceID), commitOffset+i,
				digest, groupingID, optionsID, sourceFileHash)
		}
		// TODO(kjlubick) sort these so the would be sorted by primary key. The docs say that could
		// help speed up importing.
		if _, err := f.WriteString(buf.String()); err != nil {
			return nil, nil, nil, skerr.Wrap(err)
		}
		buf.Reset()
	}
	return optsToStore, groupingsToStore, paramset, nil
}

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
	_, groupingID := sql.SerializeMap(grouping)

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
		if total == 0 {
			return nil
		}
		arguments := make([]interface{}, 0, total*valuesPerRow)
		statement := insertDeltas + sql.ValuesPlaceholders(valuesPerRow, total)
		for _, digest := range exp[expectations.Positive] {
			digestBytes, err := sql.DigestToBytes(digest)
			if err != nil {
				panic(err)
			}
			arguments = append(arguments, recordUUID, groupingID, digestBytes, sql.LabelUntriaged, sql.LabelPositive)
		}
		for _, digest := range exp[expectations.Negative] {
			digestBytes, err := sql.DigestToBytes(digest)
			if err != nil {
				panic(err)
			}
			arguments = append(arguments, recordUUID, groupingID, digestBytes, sql.LabelUntriaged, sql.LabelNegative)
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
		if total == 0 {
			return nil
		}
		arguments := make([]interface{}, 0, total*valuesPerRow)
		statement := insertExpectations + sql.ValuesPlaceholders(valuesPerRow, total)
		for _, digest := range exp[expectations.Positive] {
			digestBytes, err := sql.DigestToBytes(digest)
			if err != nil {
				panic(err)
			}
			arguments = append(arguments, recordUUID, groupingID, digestBytes, sql.LabelPositive)
		}
		for _, digest := range exp[expectations.Negative] {
			digestBytes, err := sql.DigestToBytes(digest)
			if err != nil {
				panic(err)
			}
			arguments = append(arguments, recordUUID, groupingID, digestBytes, sql.LabelNegative)
		}
		for _, digest := range exp[expectations.Untriaged] {
			digestBytes, err := sql.DigestToBytes(digest)
			if err != nil {
				panic(err)
			}
			arguments = append(arguments, recordUUID, groupingID, digestBytes, sql.LabelUntriaged)
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

// storeToTraces probabilistically ignores traces depending on their value at head.
func storeToTraces(ctx context.Context, db *pgxpool.Pool, eg *errgroup.Group, pairs []tiling.TracePair, exp expectationResult) []sql.TraceID {
	var ignoredTraces []sql.TraceID

	const insertTraceStatement = `INSERT INTO Traces (trace_id, grouping_id, keys, matches_any_ignore_rule) VALUES `

	const maxPlaceholders = 65000
	const valuesPerRow = 4
	arguments := make([]interface{}, 0, maxPlaceholders)

	// We cache the status to be consistent. i.e. any traces in the same test that produce the same
	// digest should have the same ignore status.
	ignoreStatusForDigest := map[types.Digest]bool{}

	for _, tp := range pairs {
		t := tp.Trace
		traceKeys, traceID := sql.SerializeMap(t.Keys())
		grouping := groupingFor(tp.Trace.Keys())
		_, groupingID := sql.SerializeMap(grouping)
		ignoredStatus := false
		head := tp.Trace.AtHead()
		if head == tiling.MissingDigest {
			panic("Empty trace?") // should never happen
		}
		if isIgnored, ok := ignoreStatusForDigest[head]; ok {
			if isIgnored {
				ignoredTraces = append(ignoredTraces, traceID)
				ignoredStatus = true
			}
		} else {
			switch exp.Classify(head) {
			case expectations.Positive:
				if rand.Float32() < probabilityPositiveIgnored {
					ignoredTraces = append(ignoredTraces, traceID)
					ignoredStatus = true
				}
			case expectations.Negative:
				if rand.Float32() < probabilityNegativeIgnored {
					ignoredTraces = append(ignoredTraces, traceID)
					ignoredStatus = true
				}
			case expectations.Untriaged:
				if rand.Float32() < probabilityUntriagedIgnored {
					ignoredTraces = append(ignoredTraces, traceID)
					ignoredStatus = true
				}
			}
			ignoreStatusForDigest[head] = ignoredStatus
		}
		arguments = append(arguments, traceID, groupingID, traceKeys, ignoredStatus)
	}

	if len(arguments) == 0 {
		return ignoredTraces
	}
	if len(arguments) > maxPlaceholders {
		panic("need to do batching")
	}

	eg.Go(func() error {
		sklog.Infof("Inserting %d traces", len(arguments)/valuesPerRow)
		statement := insertTraceStatement + sql.ValuesPlaceholders(valuesPerRow, len(arguments)/valuesPerRow)
		_, err := db.Exec(ctx, statement, arguments...)
		if err != nil {
			return skerr.Wrapf(err, "inserting to Traces")
		}
		return nil
	})

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

var emptyStruct = struct{}{}

func writeDiffMetricsToTSV(ctx context.Context, er expectationResult) error {
	exp := er.Expectations
	var digests []types.Digest
	digests = append(digests, exp[expectations.Positive]...)
	digests = append(digests, exp[expectations.Negative]...)
	digests = append(digests, exp[expectations.Untriaged]...)
	sklog.Infof("Generating diffs for %d digests, about %d rows", len(digests), len(digests)*len(digests))

	return writeRandomDiffMetricsToTSV(ctx, digests, digests)
}

func writeRandomDiffMetricsToTSV(ctx context.Context, leftDigests, rightDigests []types.Digest) error {
	path := filepath.Join("out_tsv", "diffmetrics_"+strconv.Itoa(rand.Int())+".tsv")
	f, err := os.Create(path)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.Close(f)
	sort.Slice(leftDigests, func(i, j int) bool {
		return leftDigests[i] < leftDigests[j]
	})
	sort.Slice(rightDigests, func(i, j int) bool {
		return rightDigests[i] < rightDigests[j]
	})

	now := time.Now().Format("2006-01-02 15:04:05-07:00")
	lines := make([]string, 0, len(leftDigests)*len(rightDigests))
	var buf strings.Builder
	writeDiff := func(left, right types.Digest, numDiffPixels int, percentDiffPixels float32, maxChannelDiff int, rgbaDiff [4]int, dimensionsDiffer bool) {
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
		combined := diff.CombinedDiffMetric(rgbaDiff, percentDiffPixels)
		buf.WriteString(strconv.FormatFloat(float64(combined), 'f', 5, 32))
		buf.WriteRune('\t')
		buf.WriteString(strconv.FormatBool(dimensionsDiffer))
		buf.WriteRune('\t')
		buf.WriteString(now)
		buf.WriteRune('\n')
		lines = append(lines, buf.String())
		buf.Reset()
	}
	digestsSeen := map[string]bool{}
	alreadyDid := func(left, right types.Digest) bool {
		return digestsSeen[string(left+right)]
	}
	markDone := func(left, right types.Digest) {
		digestsSeen[string(left+right)] = true
		digestsSeen[string(right+left)] = true
	}

	for _, left := range leftDigests {
		if err := ctx.Err(); err != nil {
			return skerr.Wrap(err)
		}
		for _, right := range rightDigests {
			if left == right || alreadyDid(left, right) {
				continue
			}
			markDone(left, right)
			numDiffPixels := r(1, 1000000)
			pixelDiffPercent := rand.Float32()
			maxChannelDiff := r(1, 255)
			rgbaDiff := [4]int{r(0, maxChannelDiff), r(0, maxChannelDiff), r(0, maxChannelDiff), maxChannelDiff}
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
	}
	// Sorting this alphabetically should be the same as sorting by primary key and that is
	// supposed to speed up ingestion.
	sort.Strings(lines)
	if _, err := f.WriteString(strings.Join(lines, "")); err != nil {
		return skerr.Wrap(err)
	}
	return nil
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

var prefixToTable = map[string]string{
	"params": `DROP TABLE PrimaryBranchParams;
IMPORT TABLE PrimaryBranchParams (
  commit_id INT4 NOT NULL,
  key STRING NOT NULL,
  value STRING NOT NULL,
  PRIMARY KEY (commit_id, key, value)
)`,
	"diffmetrics": `DROP TABLE DiffMetrics;
IMPORT TABLE DiffMetrics (
  left_digest BYTES NOT NULL,
  right_digest BYTES NOT NULL,
  num_diff_pixels INT4 NOT NULL,
  pixel_diff_percent FLOAT4 NOT NULL,
  max_channel_diff INT2 NOT NULL,
  max_rgba_diff INT2[] NOT NULL,
  combined_metric FLOAT4 NOT NULL,
  dimensions_differ BOOL NOT NULL,
  updated_ts TIMESTAMP WITH TIME ZONE NOT NULL,
  PRIMARY KEY (left_digest, right_digest)
)`,
	"tracevalues": `DROP TABLE TraceValues;
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
)`,
	"secondarytracevalues": `DROP TABLE SecondaryBranchValues;
IMPORT TABLE SecondaryBranchValues (
  secondary_branch_trace_id BYTES NOT NULL,
  branch_name STRING NOT NULL,
  version_name STRING NOT NULL,
  digest BYTES NOT NULL,
  grouping_id BYTES NOT NULL,
  options_id BYTES NOT NULL,
  source_file_id BYTES NOT NULL,
  tryjob_id STRING,
  PRIMARY KEY (branch_name, version_name, secondary_branch_trace_id)
)`,
	"secondaryparams": `DROP TABLE SecondaryBranchParams;
IMPORT TABLE SecondaryBranchParams (
  branch_name STRING NOT NULL,
  version_name STRING NOT NULL,
  key STRING NOT NULL,
  value STRING NOT NULL,
  PRIMARY KEY (branch_name, version_name, key, value)
)`,
}

func uploadAndImport(ctx context.Context, skipUpload bool) error {
	var zippedFiles []string
	var err error
	if !skipUpload {
		t := timer.New("gzipping and uploading data")
		for prefix := range prefixToTable {
			err := combineAndCompress(ctx, prefix)
			if err != nil {
				return skerr.Wrapf(err, "combining %s", prefix)
			}
		}

		zippedFiles, err = filepath.Glob("out_tsv/*.tsv.gz")
		if err != nil {
			return skerr.Wrap(err)
		}

		cmd := append([]string{"-m", "cp"}, zippedFiles...)
		cmd = append(cmd, "gs://skia-gold-benchmark-data/schemaV4/")

		out, err := exec.Command("gsutil", cmd...).CombinedOutput()
		if err != nil {
			sklog.Error(string(out))
			return skerr.Wrapf(err, "Uploading the files using gsutil. Make sure you are logged in.")
		}
		t.Stop()
	} else {
		zippedFiles, err = filepath.Glob("out_tsv/*.tsv.gz")
		if err != nil {
			return skerr.Wrap(err)
		}
	}

	eg, ctx := errgroup.WithContext(ctx)

	for prefix, statement := range prefixToTable {
		files := getImportableFiles(zippedFiles, prefix)
		sklog.Infof("Importing %s from %d files", prefix, len(files))
		importStatement := statement + `CSV DATA ('` + strings.Join(files, "','") + `')
 WITH delimiter = e'\t';`
		sklog.Info(importStatement)
		thisPrefix := prefix
		eg.Go(func() error {
			out, err := exec.Command("kubectl", "run",
				"gold-cockroachdb-import-"+thisPrefix,
				"--restart=Never", "--image=cockroachdb/cockroach:v20.2.0",
				"--", "sql",
				"--insecure", "--host=gold-cockroachdb:26234", "--database=benchmark_db",
				"--execute="+importStatement,
			).CombinedOutput()
			if err != nil {
				return skerr.Wrapf(err, "importing from files %s: %s", files, out)
			}
			sklog.Infof("Imported from %s and others\n%s", files[0], out)
			return nil
		})
	}

	return skerr.Wrap(eg.Wait())
}

// combineAndCompress combines the tsv files with the given prefix and then gzips them. We combine
// them into a few large files to work around a limit with IMPORT in cockroachDB, which can't handle
// several thousand small files, but can handle bigger files just fine. It then gzips these large
// files
func combineAndCompress(ctx context.Context, prefix string) error {
	// This is set to avoid having too many files in a single command.
	const finalNumberOfZippedFiles = 200

	// Compress tsv files with gzip (working around https://github.com/cockroachdb/cockroach/issues/56152)
	tsvFiles, err := filepath.Glob("out_tsv/" + prefix + "*.tsv")
	if err != nil {
		return skerr.Wrap(err)
	}

	chunkSize := len(tsvFiles) / finalNumberOfZippedFiles
	if chunkSize < 1 {
		chunkSize = 1
	}
	// Combine files - we append a random number to the end because Cockroachdb caches files
	// downloaded from GCS that appear to have the same name.
	err = util.ChunkIterParallel(ctx, len(tsvFiles), chunkSize, func(_ context.Context, startIdx int, endIdx int) error {
		files := tsvFiles[startIdx:endIdx]
		joined := strings.Join(files, " ")
		bashCmd := fmt.Sprintf("cat %s > out_tsv/combined_%s_%d_%d.tsv", joined, prefix, startIdx, rand.Int())
		out, err := exec.Command("bash", "-c", bashCmd).Output()
		if err != nil {
			sklog.Error(out)
			return skerr.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return skerr.Wrap(err)
	}

	combinedFiles, err := filepath.Glob("out_tsv/combined_" + prefix + "*.tsv")
	if err != nil {
		return skerr.Wrap(err)
	}
	// pigz uses all available cores and --fast is fine for TSV files.
	cmd := append([]string{"--fast"}, combinedFiles...)
	err = exec.Command("pigz", cmd...).Run()
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func getImportableFiles(allFiles []string, prefix string) []string {
	var rv []string
	for _, f := range allFiles {
		if !strings.HasPrefix(f, "out_tsv/combined_"+prefix) {
			continue
		}
		f = strings.TrimPrefix(f, "out_tsv/")
		rv = append(rv, "gs://skia-gold-benchmark-data/schemaV4/"+f)
	}
	return rv
}

type clDataPoint struct {
	Digest  types.Digest
	Keys    map[string]string
	Options map[string]string
}

func convertToCLData(pairs []tiling.TracePair) ([]clDataPoint, paramtools.ParamSet) {
	data := make([]clDataPoint, 0, len(pairs)+newTracesOnCLs)
	ps := paramtools.ParamSet{}
	for _, tp := range pairs {
		head := tp.Trace.AtHead()
		cp := clDataPoint{
			Digest:  head,
			Keys:    tp.Trace.Keys(),
			Options: tp.Trace.Options(),
		}
		data = append(data, cp)
		ps.AddParams(tp.Trace.KeysAndOptions())
	}
	return data, ps
}

func copyAndMutateWithCLKeys(data []clDataPoint) ([]clDataPoint, paramtools.ParamSet) {
	newData := make([]clDataPoint, 0, len(data))
	ps := paramtools.ParamSet{}
	for _, d := range data {
		cp := clDataPoint{
			Digest:  d.Digest,
			Keys:    copyStringMap(d.Keys),
			Options: d.Options,
		}
		for k := range cp.Keys {
			if k == types.PrimaryKeyField || k == types.CorpusField {
				continue // don't want to overwrite these two
			}
			v := fmt.Sprintf("cl_value_%d", r(0, 1000))
			cp.Keys[k] = v
			ps.AddParams(map[string]string{k: v})
			break
		}
		newData = append(newData, cp)
	}
	return newData, ps
}

func copyStringMap(m map[string]string) map[string]string {
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

func sprinkleWithNewDigests(data []clDataPoint, untriaged int) (types.DigestSet, types.DigestSet) {
	newDigests := make(types.DigestSet, untriaged)
	allDigests := types.DigestSet{}
	idxs := make([]int, len(data))
	for i := range idxs {
		idxs[i] = i
	}
	rand.Shuffle(len(idxs), func(i, j int) {
		idxs[i], idxs[j] = idxs[j], idxs[i]
	})
	for _, idx := range idxs[0:untriaged] {
		newDigest := randomDigest(data[idx].Keys[types.PrimaryKeyField])
		allDigests[data[idx].Digest] = true
		newDigests[newDigest] = true
		data[idx].Digest = newDigest
	}
	for _, d := range data {
		allDigests[d.Digest] = true
	}
	return newDigests, allDigests
}

func writeSecondaryTraceValuesToTSV(ctx context.Context, data []clDataPoint, sourceFile []byte, clID, psID string) error {
	path := filepath.Join("out_tsv", "secondarytracevalues_"+strconv.Itoa(rand.Int())+".tsv")
	f, err := os.Create(path)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.Close(f)

	const tryjobID = "buildbucket_1234"

	lines := make([]string, 0, len(data))
	var buf strings.Builder
	writeValue := func(traceID []byte, digest types.Digest, grouping, options []byte) {
		buf.WriteString(`\x`)
		buf.WriteString(hex.EncodeToString(traceID))
		buf.WriteRune('\t')
		buf.WriteString(clID)
		buf.WriteRune('\t')
		buf.WriteString(psID)
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
		buf.WriteRune('\t')
		buf.WriteString(tryjobID)
		buf.WriteRune('\n')
		lines = append(lines, buf.String())
		buf.Reset()
	}

	for _, d := range data {
		if err := ctx.Err(); err != nil {
			return skerr.Wrap(err)
		}
		_, traceID := sql.SerializeMap(d.Keys)
		_, optionsID := sql.SerializeMap(d.Options)
		grouping := groupingFor(d.Keys)
		_, groupingID := sql.SerializeMap(grouping)
		writeValue(traceID, d.Digest, groupingID, optionsID)
	}

	// Sorting this alphabetically should be the same as sorting by primary key and that is
	// supposed to speed up ingestion.
	sort.Strings(lines)
	if _, err := f.WriteString(strings.Join(lines, "")); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func writeSecondaryBranchParamsToTSV(ctx context.Context, paramset paramtools.ParamSet, clID, psID string) error {
	path := filepath.Join("out_tsv", "secondaryparams_"+strconv.Itoa(rand.Int())+".tsv")
	f, err := os.Create(path)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.Close(f)

	paramset.Normalize()
	lines := make([]string, 0, paramset.Size())
	var buf strings.Builder
	for k, values := range paramset {
		if err := ctx.Err(); err != nil {
			return skerr.Wrap(err)
		}
		for _, v := range values {
			buf.WriteString(clID)
			buf.WriteRune('\t')
			buf.WriteString(psID)
			buf.WriteRune('\t')
			buf.WriteString(k)
			buf.WriteRune('\t')
			buf.WriteString(v)
			buf.WriteRune('\n')
			lines = append(lines, buf.String())
			buf.Reset()
		}
	}
	// Sorting this alphabetically should be the same as sorting by primary key and that is
	// supposed to speed up ingestion.
	sort.Strings(lines)
	if _, err := f.WriteString(strings.Join(lines, "")); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func storeToSecondaryBranchExpectations(ctx context.Context, errCtx context.Context, db *pgxpool.Pool, eg *errgroup.Group, clID string, newDigests []types.Digest, data []clDataPoint) error {
	// Randomly triage some of the digests
	toTriage := r(0, len(newDigests)+1)
	if toTriage == 0 {
		return nil
	}
	// Triage the first n of them 50/50 to positive or negative.
	newDigests = newDigests[:toTriage]
	statuses := make([]sql.ExpectationsLabel, toTriage)
	for i := range statuses {
		statuses[i] = sql.ExpectationsLabel(r(1, 3))
	}
	// By construction, all digests here are for the same grouping.
	_, groupingID := sql.SerializeMap(groupingFor(data[0].Keys))

	row := db.QueryRow(ctx,
		`INSERT INTO ExpectationRecords (user_name, branch_name, time, num_changes) VALUES ($1, $2, $3, $4) RETURNING expectation_record_id`,
		"cl_user"+strconv.Itoa(rand.Intn(10)), clID, time.Now(), toTriage)
	recordUUID := ""
	err := row.Scan(&recordUUID)
	if err != nil {
		return skerr.Wrapf(err, "getting new UUID")
	}

	eg.Go(func() error {
		const insertDeltas = `INSERT INTO ExpectationDeltas (expectation_record_id, grouping_id, digest, label_before, label_after) VALUES `
		const valuesPerRow = 5
		arguments := make([]interface{}, 0, toTriage*valuesPerRow)
		statement := insertDeltas + sql.ValuesPlaceholders(valuesPerRow, toTriage)
		for i, digest := range newDigests {
			digestBytes, err := sql.DigestToBytes(digest)
			if err != nil {
				panic(err)
			}
			arguments = append(arguments, recordUUID, groupingID, digestBytes, sql.LabelUntriaged, statuses[i])
		}
		statement += ` RETURNING NOTHING;`
		_, err := db.Exec(errCtx, statement, arguments...)
		return skerr.Wrapf(err, "inserting ExpectationDeltas")
	})

	eg.Go(func() error {
		const insertExpectations = `INSERT INTO SecondaryBranchExpectations (branch_name, expectation_record_id, grouping_id, digest, label) VALUES `
		const valuesPerRow = 5
		arguments := make([]interface{}, 0, toTriage*valuesPerRow)
		statement := insertExpectations + sql.ValuesPlaceholders(valuesPerRow, toTriage)
		for i, digest := range newDigests {
			digestBytes, err := sql.DigestToBytes(digest)
			if err != nil {
				panic(err)
			}
			arguments = append(arguments, clID, recordUUID, groupingID, digestBytes, statuses[i])
		}
		statement += ` RETURNING NOTHING;`
		_, err := db.Exec(errCtx, statement, arguments...)
		return skerr.Wrapf(err, "inserting SecondaryBranchExpectations")
	})
	return nil
}

func storeToChangeListPatchset(ctx context.Context, db *pgxpool.Pool, eg *errgroup.Group, clID, psID string) {
	eg.Go(func() error {
		const insertChangeLists = `
INSERT INTO Changelists (changelist_id, system, status, owner, updated, subject) VALUES 
($1, $2, $3, $4, $5, $6)
ON CONFLICT DO NOTHING`

		_, err := db.Exec(ctx, insertChangeLists, clID, "gerrit", sql.StatusOpen,
			"somebody"+strconv.Itoa(rand.Intn(5)), time.Now(), "Interesting changes")
		return skerr.Wrapf(err, "inserting Changelists")
	})

	eg.Go(func() error {
		const insertChangeLists = `
INSERT INTO Patchsets (patchset_id, system, changelist_id, ps_order, git_hash) VALUES 
($1, $2, $3, $4, $5)
ON CONFLICT DO NOTHING`

		_, err := db.Exec(ctx, insertChangeLists, psID, "gerrit", clID, 1, "whatever")
		return skerr.Wrapf(err, "inserting Patchsets")
	})

}
