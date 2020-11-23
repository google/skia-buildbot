package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/google/uuid"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"io/ioutil"
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
	// block of missing data from the beginning, since no repo uses Gold from its inception.
	fakeCommitOffset = 40000
	// As of November 2020, Skia had 1.4M traces across 3300 tests. This averages to ~424 traces
	// per test, although those are *not* evenly distributed. For benchmarking, we'll round up and
	// pretend they are consistent.
	tracesPerTest = 500
	// 20 tests * 500 traces/test = 10k traces
	// 10k traces * 1000 commits * 0.6 data points / commit = 6M data points
	numTests = 30

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

	sqlNull = "NULL"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	port := flag.String("port", "", "Port on localhost to connect to.")
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
	db.Close()

	if !*skipGeneration {
		if err = os.RemoveAll("out_tsv"); err != nil {
			sklog.Fatalf("Cleaning old out_tsv folder")
		}
		if err = os.MkdirAll("out_tsv", 0777); err != nil {
			sklog.Fatalf("Making output tsv folder")
		}

		defer timer.New("filling data").Stop()
		if err := fillData(ctx, fakeCommitOffset); err != nil {
			sklog.Fatalf("Could not fill with data: %s", err)
		}
	}

	if err := uploadAndImport(ctx, *skipUpload); err != nil {
		sklog.Fatalf("Could not import tsv files: %s", err)
	}

	sklog.Infof("Done.  Run k get pods | grep import to see the status")
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

var globalOptions = map[string][]byte{}
var globalGroupings = map[string][]byte{}
var globalSourceFiles = map[string]sql.SourceFileID{}

func fillData(ctx context.Context, commitOffset int) error {
	corpora := []string{"corpus1", "corpus2", "corpus3", "corpus4", "corpus5"}

	for i := 0; i < numTests; i++ {
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
		if err := storePrimaryBranchTraceData(ctx, tracePairs, fakeFile, commitOffset); err != nil {
			return skerr.Wrap(err)
		}

		for j := 0; j < numCLs; j++ {
			numUntriagedFromPrimary := r(0, tracesPerTest/10)
			fakeFile := "gs://skia-gold-benchmark/trybot/dm-json-v1/2020/10/31/13/dm" + strconv.Itoa(rand.Int()) + ".json"
			clID := fmt.Sprintf("gerrit_%d", j+1000)
			psID := fmt.Sprintf("ps_%d", j+1)
			if err := storeChangeListTraceData(ctx, clID, psID, fakeFile, tracePairs, numUntriagedFromPrimary); err != nil {
				return skerr.Wrap(err)
			}
		}
	}

	if err := writeKeyValuesToTSV(ctx, globalGroupings, "groupings_"); err != nil {
		return skerr.Wrap(err)
	}
	if err := writeKeyValuesToTSV(ctx, globalOptions, "options_"); err != nil {
		return skerr.Wrap(err)
	}
	if err := writeSourceFilesToTSV(ctx, globalSourceFiles); err != nil {
		return skerr.Wrap(err)
	}
	for j := 0; j < numCLs; j++ {
		clID := fmt.Sprintf("gerrit_%d", j+1000)
		psID := fmt.Sprintf("ps_%d", j+1)
		if err := writeChangelistPatchsetToTSV(ctx, clID, psID); err != nil {
			return skerr.Wrap(err)
		}
	}

	return nil
}

func storePrimaryBranchTraceData(ctx context.Context, pairs []tiling.TracePair, sourceFile string, commitOffset int) error {
	sfh := md5.Sum([]byte(sourceFile))
	sourceFileHash := sfh[:] // convert array to slice
	globalSourceFiles[sourceFile] = sourceFileHash

	paramset, err := writeTraceValuesToTSV(ctx, pairs, sourceFileHash, commitOffset)
	if err != nil {
		return skerr.Wrap(err)
	}
	err = writePrimaryBranchParamsToTSV(ctx, paramset, commitOffset, commitOffset+len(pairs[0].Trace.Digests))
	if err != nil {
		return skerr.Wrap(err)
	}
	exp, err := writeExpectationsToTSV(ctx, pairs)
	if err != nil {
		return skerr.Wrap(err)
	}
	ignoredTraces, err := writeTracesToTSV(ctx, pairs, exp)
	if err != nil {
		return skerr.Wrap(err)
	}
	err = writeValuesAtHeadToTSV(ctx, pairs, exp, ignoredTraces, commitOffset)
	if err != nil {
		return skerr.Wrap(err)
	}
	err = writeDiffMetricsToTSV(ctx, exp)
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func storeChangeListTraceData(ctx context.Context, clID, psID, sourceFile string, pairs []tiling.TracePair, numUntriaged int) error {
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

	err = writeSecondaryBranchExpectationsToTSV(ctx, clID, newDigests.Keys(), data)
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func writeTraceValuesToTSV(ctx context.Context, pairs []tiling.TracePair, sourceFileHash []byte, commitOffset int) (paramtools.ParamSet, error) {
	path := filepath.Join("out_tsv", "tracevalues_"+strconv.Itoa(rand.Int())+".tsv")
	f, err := os.Create(path)
	if err != nil {
		return nil, skerr.Wrap(err)
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

	paramset := paramtools.ParamSet{}

	for _, tp := range pairs {
		if err := ctx.Err(); err != nil {
			return nil, skerr.Wrap(err)
		}
		_, traceID := sql.SerializeMap(tp.Trace.Keys())
		optsJSON, optionsID := sql.SerializeMap(tp.Trace.Options())
		globalOptions[optsJSON] = optionsID
		grouping := groupingFor(tp.Trace.Keys())
		groupingJSON, groupingID := sql.SerializeMap(grouping)
		globalGroupings[groupingJSON] = groupingID
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
			return nil, skerr.Wrap(err)
		}
		buf.Reset()
	}
	return paramset, nil
}

var writtenPrimaryBranchParams = map[[2]string]struct{}{}

func writePrimaryBranchParamsToTSV(_ context.Context, paramset paramtools.ParamSet, startCommit, endCommit int) error {
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

	lines := make([]string, 0, len(rows)*(endCommit-startCommit))
	var buf strings.Builder
	for commit := startCommit; commit < endCommit; commit++ {
		c := strconv.Itoa(commit)
		for _, row := range rows {
			buf.WriteString(c)
			buf.WriteRune('\t')
			buf.WriteString(row[0]) // key
			buf.WriteRune('\t')
			buf.WriteString(row[1]) // value
			buf.WriteRune('\n')
		}
		lines = append(lines, buf.String())
		buf.Reset()
	}

	path := filepath.Join("out_tsv", "params_"+strconv.Itoa(rand.Int())+".tsv")
	if err := ioutil.WriteFile(path, []byte(strings.Join(lines, "")), 0666); err != nil {
		return skerr.Wrap(err)
	}
	return nil
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

// writeExpectationsToTSV randomly assigns labels to the digests in the given trace pairs. Note that
// all trace pairs are expected to be from the same grouping (corpus + test), so the logic is much
// simpler than if pairs could be from multiple groupings.
func writeExpectationsToTSV(_ context.Context, pairs []tiling.TracePair) (expectationResult, error) {
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

	// Create the record that added these expectations
	recordUUID := uuid.New().String()

	numTriaged := len(exp[expectations.Positive]) + len(exp[expectations.Negative])
	totalExp := numTriaged + len(exp[expectations.Untriaged])

	path := filepath.Join("out_tsv", "expectation-records_"+strconv.Itoa(rand.Int())+".tsv")
	// expectation_record_id, branch_name (null), user_name, time, num_changes
	err := ioutil.WriteFile(path, []byte(fmt.Sprintf("%s\tNULL\t%s\t%s\t%d\n",
		recordUUID, "user"+strconv.Itoa(rand.Intn(10)), sql.FormatTime(time.Now()), numTriaged)),
		0666)
	if err != nil {
		return expectationResult{}, skerr.Wrap(err)
	}

	// Write the deltas
	lines := make([]string, 0, totalExp)
	var buf strings.Builder
	const untriaged = '0'
	const positive = '1'
	const negative = '2'
	writeDelta := func(digest types.Digest, labelAfter rune) {
		buf.WriteString(uuid.New().String()) // generate random id for this delta
		buf.WriteRune('\t')
		buf.WriteString(recordUUID)
		buf.WriteRune('\t')
		buf.WriteString(`\x`)
		buf.WriteString(hex.EncodeToString(groupingID))
		buf.WriteRune('\t')
		buf.WriteString(`\x`)
		buf.WriteString(string(digest))
		buf.WriteRune('\t')
		buf.WriteRune(untriaged) // label_before
		buf.WriteRune('\t')
		buf.WriteRune(labelAfter) // label_after
		buf.WriteRune('\n')
		lines = append(lines, buf.String())
		buf.Reset()
	}
	for _, digest := range exp[expectations.Positive] {
		writeDelta(digest, positive)
	}
	for _, digest := range exp[expectations.Negative] {
		writeDelta(digest, negative)
	}
	sort.Strings(lines)
	path = filepath.Join("out_tsv", "expectation-deltas_"+strconv.Itoa(rand.Int())+".tsv")
	if err := ioutil.WriteFile(path, []byte(strings.Join(lines, "")), 0666); err != nil {
		return expectationResult{}, skerr.Wrap(err)
	}

	// Now write the expectations to the primary branch
	lines = lines[:0]
	writeExpectation := func(digest types.Digest, label rune, recordID string) {
		buf.WriteString(hex.EncodeToString(groupingID))
		buf.WriteRune('\t')
		buf.WriteString(`\x`)
		buf.WriteString(string(digest))
		buf.WriteRune('\t')
		buf.WriteRune(label)
		buf.WriteRune('\t')
		buf.WriteString(recordID)
		buf.WriteRune('\n')
		lines = append(lines, buf.String())
		buf.Reset()
	}
	for _, digest := range exp[expectations.Positive] {
		writeExpectation(digest, positive, recordUUID)
	}
	for _, digest := range exp[expectations.Negative] {
		writeExpectation(digest, negative, recordUUID)
	}
	for _, digest := range exp[expectations.Untriaged] {
		writeExpectation(digest, untriaged, sqlNull)
	}
	sort.Strings(lines)
	path = filepath.Join("out_tsv", "expectations_"+strconv.Itoa(rand.Int())+".tsv")
	if err := ioutil.WriteFile(path, []byte(strings.Join(lines, "")), 0666); err != nil {
		return expectationResult{}, skerr.Wrap(err)
	}

	return expectationResult{
		Expectations: exp,
		RecordID:     recordUUID,
	}, nil
}

// writeTracesToTSV probabilistically ignores traces depending on their value at head.
// It returns the trace ids which are ignored.
func writeTracesToTSV(_ context.Context, pairs []tiling.TracePair, exp expectationResult) ([]sql.TraceID, error) {
	var ignoredTraces []sql.TraceID

	grouping := groupingFor(pairs[0].Trace.Keys())
	// we know all the traces have the same grouping, since they were created from the same test.
	_, groupingBytes := sql.SerializeMap(grouping)
	groupingID := hex.EncodeToString(groupingBytes)

	// We cache the status to be consistent. i.e. any traces in the same test that produce the same
	// digest should have the same ignore status.
	ignoreStatusForDigest := map[types.Digest]bool{}

	lines := make([]string, 0, len(pairs))
	var buf strings.Builder
	writeValue := func(traceID sql.TraceID, keys string, ignored bool) {
		buf.WriteString(`\x`)
		buf.WriteString(hex.EncodeToString(traceID))
		buf.WriteRune('\t')
		buf.WriteString(`\x`)
		buf.WriteString(groupingID)
		buf.WriteRune('\t')
		buf.WriteString(keys)
		buf.WriteRune('\t')
		buf.WriteString(strconv.FormatBool(ignored))
		buf.WriteRune('\n')
		lines = append(lines, buf.String())
		buf.Reset()
	}

	for _, tp := range pairs {
		t := tp.Trace
		traceKeys, traceID := sql.SerializeMap(t.Keys())
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
		writeValue(traceID, traceKeys, ignoredStatus)
	}

	sort.Strings(lines)
	path := filepath.Join("out_tsv", "primarytraces_"+strconv.Itoa(rand.Int())+".tsv")
	if err := ioutil.WriteFile(path, []byte(strings.Join(lines, "")), 0666); err != nil {
		return nil, skerr.Wrap(err)
	}

	return ignoredTraces, nil
}

func writeValuesAtHeadToTSV(_ context.Context, pairs []tiling.TracePair, exp expectationResult, ignoredTraces []sql.TraceID, offset int) error {
	grouping := groupingFor(pairs[0].Trace.Keys())
	// we know all the traces have the same grouping, since they were created from the same test.
	_, groupingBytes := sql.SerializeMap(grouping)
	groupingID := hex.EncodeToString(groupingBytes)

	lines := make([]string, 0, len(pairs))
	var buf strings.Builder
	writeValueAtHead := func(traceID sql.TraceID, mostRecentCommit int, digest types.Digest, options sql.OptionsID,
		keys string, label sql.ExpectationsLabel, recordID string, ignored bool) {
		buf.WriteString(`\x`)
		buf.WriteString(hex.EncodeToString(traceID))
		buf.WriteRune('\t')
		buf.WriteString(strconv.Itoa(mostRecentCommit))
		buf.WriteRune('\t')
		buf.WriteString(`\x`)
		buf.WriteString(string(digest))
		buf.WriteRune('\t')
		buf.WriteString(`\x`)
		buf.WriteString(hex.EncodeToString(options))
		buf.WriteRune('\t')
		buf.WriteString(`\x`)
		buf.WriteString(groupingID)
		buf.WriteRune('\t')
		buf.WriteString(keys)
		buf.WriteRune('\t')
		buf.WriteString(strconv.Itoa(int(label)))
		buf.WriteRune('\t')
		buf.WriteString(recordID)
		buf.WriteRune('\t')
		buf.WriteString(strconv.FormatBool(ignored))
		buf.WriteRune('\n')
		lines = append(lines, buf.String())
		buf.Reset()
	}

	for _, tp := range pairs {
		trace := tp.Trace
		keysJSON, traceID := sql.SerializeMap(trace.Keys())
		_, optionsID := sql.SerializeMap(trace.Options())
		mostRecentCommit := trace.LastIndex()
		digest := trace.AtHead()
		if mostRecentCommit == -1 || digest == tiling.MissingDigest {
			panic("empty trace")
		}
		label := sql.ConvertLabelFromString(exp.Classify(digest))
		recordID := sqlNull
		if label != sql.LabelUntriaged {
			recordID = exp.RecordID
		}
		isIgnored := false
		for _, ignoredTrace := range ignoredTraces {
			if ignoredTrace.Equals(traceID) {
				isIgnored = true
				break
			}
		}
		writeValueAtHead(traceID, offset+mostRecentCommit, digest, optionsID, keysJSON, label, recordID, isIgnored)
	}
	sort.Strings(lines)
	path := filepath.Join("out_tsv", "values-at-head_"+strconv.Itoa(rand.Int())+".tsv")
	if err := ioutil.WriteFile(path, []byte(strings.Join(lines, "")), 0666); err != nil {
		return skerr.Wrap(err)
	}
	return nil
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

	now := sql.FormatTime(time.Now())
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
	const tryjobID = "buildbucket_1234"

	lines := make([]string, 0, len(data))
	var buf strings.Builder
	writeValue := func(traceID sql.TraceID, digest types.Digest, grouping sql.GroupingID, options sql.OptionsID) {
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
	path := filepath.Join("out_tsv", "secondarytracevalues_"+strconv.Itoa(rand.Int())+".tsv")
	if err := ioutil.WriteFile(path, []byte(strings.Join(lines, "")), 0666); err != nil {
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

func writeSecondaryBranchExpectationsToTSV(_ context.Context, clID string, newDigests []types.Digest, data []clDataPoint) error {
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
	_, groupingBytes := sql.SerializeMap(groupingFor(data[0].Keys))
	groupingID := hex.EncodeToString(groupingBytes)

	// Create the record that added these expectations
	recordUUID := uuid.New().String()

	path := filepath.Join("out_tsv", "expectation-records_"+strconv.Itoa(rand.Int())+".tsv")
	// expectation_record_id, branch_name (CL_ID), user_name, time, num_changes
	err := ioutil.WriteFile(path, []byte(fmt.Sprintf("%s\t%s\t%s\t%s\t%d\n",
		recordUUID, clID, "user"+strconv.Itoa(rand.Intn(10)), sql.FormatTime(time.Now()), toTriage)),
		0666)
	if err != nil {
		return nil
	}

	// Write the deltas
	lines := make([]string, 0, toTriage)
	var buf strings.Builder
	writeDelta := func(digest types.Digest, labelAfter sql.ExpectationsLabel) {
		buf.WriteString(uuid.New().String()) // generate random id for this delta
		buf.WriteRune('\t')
		buf.WriteString(recordUUID)
		buf.WriteRune('\t')
		buf.WriteString(`\x`)
		buf.WriteString(groupingID)
		buf.WriteRune('\t')
		buf.WriteString(`\x`)
		buf.WriteString(string(digest))
		buf.WriteRune('\t')
		// label_before (this isn't correct, but that shouldn't really impact things)
		buf.WriteRune('0')
		buf.WriteRune('\t')
		buf.WriteString(strconv.Itoa(int(labelAfter))) // label_after
		buf.WriteRune('\n')
		lines = append(lines, buf.String())
		buf.Reset()
	}
	for i, digest := range newDigests {
		writeDelta(digest, statuses[i])
	}
	sort.Strings(lines)
	path = filepath.Join("out_tsv", "expectation-deltas_"+strconv.Itoa(rand.Int())+".tsv")
	if err := ioutil.WriteFile(path, []byte(strings.Join(lines, "")), 0666); err != nil {
		return skerr.Wrap(err)
	}

	// Now write the expectations to the primary branch
	lines = lines[:0]
	writeSecondaryExpectation := func(digest types.Digest, label sql.ExpectationsLabel) {
		buf.WriteString(clID)
		buf.WriteRune('\t')
		buf.WriteString(groupingID)
		buf.WriteRune('\t')
		buf.WriteString(`\x`)
		buf.WriteString(string(digest))
		buf.WriteRune('\t')
		buf.WriteString(strconv.Itoa(int(label)))
		buf.WriteRune('\t')
		buf.WriteString(recordUUID)
		buf.WriteRune('\n')
		lines = append(lines, buf.String())
		buf.Reset()
	}
	for i, digest := range newDigests {
		writeSecondaryExpectation(digest, statuses[i])
	}
	sort.Strings(lines)
	path = filepath.Join("out_tsv", "secondaryexpectations_"+strconv.Itoa(rand.Int())+".tsv")
	if err := ioutil.WriteFile(path, []byte(strings.Join(lines, "")), 0666); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func writeChangelistPatchsetToTSV(_ context.Context, clID, psID string) error {
	path := filepath.Join("out_tsv", "changelists_"+strconv.Itoa(rand.Int())+".tsv")
	// changelist_id, system, status, owner, updated, subject
	err := ioutil.WriteFile(path, []byte(fmt.Sprintf("%s\tgerrit\t0\t%s\t%s\tsome changes\n",
		clID, "somebody"+strconv.Itoa(rand.Intn(5)), sql.FormatTime(time.Now()))),
		0666)
	if err != nil {
		return skerr.Wrap(err)
	}
	path = filepath.Join("out_tsv", "patchsets_"+strconv.Itoa(rand.Int())+".tsv")
	// patchset_id, system, changelist_id, ps_order, git_hash
	err = ioutil.WriteFile(path, []byte(fmt.Sprintf("%s\tgerrit\t%s\t1\twhatever\n",
		psID, clID)),
		0666)
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func writeKeyValuesToTSV(_ context.Context, jsonToIDs map[string][]byte, filePrefix string) error {
	lines := make([]string, 0, len(jsonToIDs))
	var buf strings.Builder
	writeEntry := func(id []byte, jsonStr string) {
		buf.WriteString(`\x`)
		buf.WriteString(hex.EncodeToString(id))
		buf.WriteRune('\t')
		buf.WriteString(jsonStr)
		buf.WriteRune('\n')
		lines = append(lines, buf.String())
		buf.Reset()
	}
	for j, id := range jsonToIDs {
		writeEntry(id, j)
	}
	sort.Strings(lines)
	path := filepath.Join("out_tsv", filePrefix+strconv.Itoa(rand.Int())+".tsv")
	if err := ioutil.WriteFile(path, []byte(strings.Join(lines, "")), 0666); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func writeSourceFilesToTSV(_ context.Context, nameToIDs map[string]sql.SourceFileID) error {
	lines := make([]string, 0, len(nameToIDs))
	var buf strings.Builder
	now := sql.FormatTime(time.Now())
	writeEntry := func(id []byte, name string) {
		buf.WriteString(`\x`)
		buf.WriteString(hex.EncodeToString(id))
		buf.WriteRune('\t')
		buf.WriteString(name)
		buf.WriteRune('\t')
		buf.WriteString(now)
		buf.WriteRune('\n')
		lines = append(lines, buf.String())
		buf.Reset()
	}
	for name, id := range nameToIDs {
		writeEntry(id, name)
	}
	sort.Strings(lines)
	path := filepath.Join("out_tsv", "source-files"+strconv.Itoa(rand.Int())+".tsv")
	if err := ioutil.WriteFile(path, []byte(strings.Join(lines, "")), 0666); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

var prefixToTable = map[string]string{
	"changelists": `DROP TABLE Changelists;
IMPORT TABLE Changelists (
  changelist_id STRING PRIMARY KEY,
  system STRING NOT NULL,
  status INT2,
  owner STRING,
  updated TIMESTAMP WITH TIME ZONE,
  subject STRING
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
	"expectation-deltas": `DROP TABLE ExpectationDeltas;
IMPORT TABLE ExpectationDeltas (
  expectation_delta_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  expectation_record_id UUID,
  grouping_id BYTES,
  digest BYTES NOT NULL,
  label_before SMALLINT,
  label_after SMALLINT
)`,
	"expectation-records": `DROP TABLE ExpectationRecords;
IMPORT TABLE ExpectationRecords (
  expectation_record_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  branch_name STRING,
  user_name STRING,
  time TIMESTAMP WITH TIME ZONE,
  num_changes INT4
)`,
	"expectations": `DROP TABLE Expectations;
IMPORT TABLE Expectations (
  grouping_id BYTES NOT NULL,
  digest BYTES NOT NULL,
  label SMALLINT NOT NULL,
  expectation_record_id UUID,
  INDEX group_label_idx (grouping_id, label) STORING (expectation_record_id),
  PRIMARY KEY (grouping_id, digest)
)`,
	"groupings": `DROP TABLE Groupings;
IMPORT TABLE Groupings (
  grouping_id BYTES PRIMARY KEY,
  keys JSONB NOT NULL,
  INVERTED INDEX keys_idx (keys)
)`,
	"options": `DROP TABLE Options;
IMPORT TABLE Options (
  options_id BYTES PRIMARY KEY,
  keys JSONB NOT NULL,
  INVERTED INDEX keys_idx (keys)
)`,
	"params": `DROP TABLE PrimaryBranchParams;
IMPORT TABLE PrimaryBranchParams (
  commit_id INT4 NOT NULL,
  key STRING NOT NULL,
  value STRING NOT NULL,
  PRIMARY KEY (commit_id, key, value)
)`,
	"patchsets": `DROP TABLE Patchsets;
IMPORT TABLE Patchsets (
  patchset_id STRING PRIMARY KEY,
  system STRING NOT NULL,
  changelist_id STRING,
  ps_order INT2,
  git_hash STRING
)`,
	"primarytraces": `DROP TABLE Traces;
IMPORT TABLE Traces (
  trace_id BYTES PRIMARY KEY,
  grouping_id BYTES NOT NULL,
  keys JSONB NOT NULL,
  matches_any_ignore_rule BOOL,
  INVERTED INDEX keys_idx (keys),
  INDEX ignored_grouping_idx (matches_any_ignore_rule, grouping_id)
)`,
	"secondaryexpectations": `DROP TABLE SecondaryBranchExpectations;
IMPORT TABLE SecondaryBranchExpectations (
  branch_name STRING NOT NULL,
  grouping_id BYTES,
  digest BYTES NOT NULL,
  label SMALLINT NOT NULL,
  expectation_record_id UUID,
  PRIMARY KEY (branch_name, grouping_id, digest)
)`,
	"secondaryparams": `DROP TABLE SecondaryBranchParams;
IMPORT TABLE SecondaryBranchParams (
  branch_name STRING NOT NULL,
  version_name STRING NOT NULL,
  key STRING NOT NULL,
  value STRING NOT NULL,
  PRIMARY KEY (branch_name, version_name, key, value)
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
	"source-files": `DROP TABLE SourceFiles;
IMPORT TABLE SourceFiles (
  source_file_id BYTES PRIMARY KEY,
  source_file STRING NOT NULL,
  last_ingested TIMESTAMP WITH TIME ZONE NOT NULL
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
  INDEX trace_commit_digest_idx (trace_id, commit_id, digest) STORING (options_id),
  PRIMARY KEY (shard, commit_id, trace_id)
)`,
	"values-at-head": `DROP TABLE ValuesAtHead;
IMPORT TABLE ValuesAtHead (
  trace_id BYTES NOT NULL PRIMARY KEY,
  most_recent_commit_id INT4,
  digest BYTES NOT NULL,
  options_id BYTES NOT NULL,
  grouping_id BYTES NOT NULL,
  keys JSONB NOT NULL,
  expectation_label SMALLINT NOT NULL,
  expectation_record_id UUID,
  matches_any_ignore_rule BOOL,

  INVERTED INDEX keys_idx (keys),
  FAMILY f1 (most_recent_commit_id, digest),
  FAMILY f2 (options_id, matches_any_ignore_rule, expectation_label, expectation_record_id),
  FAMILY f3 (keys, grouping_id)
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
		if len(files) == 0 {
			return skerr.Fmt("Got no files for prefix %s", prefix)
		}
		sklog.Infof("Importing %s from %d files", prefix, len(files))
		importStatement := statement + `CSV DATA ('` + strings.Join(files, "','") + `')
 WITH delimiter = e'\t', nullif = '` + sqlNull + `';`
		thisPrefix := prefix
		eg.Go(func() error {
			// Spin up a small pod in the cluster that will connect and run the sql import.
			// This should be resilient to network disconnects between the machine running this
			// executable and the cluster. It will return after the pod is scheduled, not after
			// the task is complete.
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

	eg.Go(func() error {
		if err = os.RemoveAll("out_tsv"); err != nil {
			return skerr.Wrapf(err, "Cleaning up data after upload")
		}
		return ioutil.WriteFile("created_files.txt", []byte(strings.Join(zippedFiles, "\n")), 0666)
	})

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
	if len(tsvFiles) == 0 {
		return skerr.Fmt("Got no TSV files for prefix %s", prefix)
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
