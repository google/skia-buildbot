package ingestion_processors

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/ingestion"
	ingestion_mocks "go.skia.org/infra/golden/go/ingestion/mocks"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/sqltest"
	three_devices "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

// Setting up a cockroachdb instance takes several seconds. To make our tests faster,
// we re-use the same instance for multiple tests. Each subtest is responsible for deleting
// old data and setting up whatever existing data is needed.
func TestSQLProcess(t *testing.T) {
	unittest.LargeTest(t)

	dbURL, deleteDB := sqltest.MakeLocalCockroachDBForTesting(t)
	defer deleteDB() // uncomment this to explore the results more easily

	ingester := testSQLProcessor(t, dbURL)

	t.Run("AllNewData_Success", func(t *testing.T) {
		subTest_AllNewData_Success(t, ingester)
	})
	t.Run("DataFromPreviousCommitExists_Success", func(t *testing.T) {
		subTest_DataFromPreviousCommitExists_Success(t, ingester)
	})
	t.Run("OverwriteTraceValues_Success", func(t *testing.T) {
		subTest_OverwriteTraceValues_Success(t, ingester)
	})
	t.Run("StoresManyValuesIncludingDuplicates_Success", func(t *testing.T) {
		subTest_StoresManyValuesIncludingDuplicates_Success(t, ingester)
	})

	// Error cases
	t.Run("FileWithNoResults_Ignored", func(t *testing.T) {
		subTest_FileWithNoResults_Ignored(t, ingester)
	})
	t.Run("FileWithInvalidJSON_Error", func(t *testing.T) {
		subTest_FileWithInvalidJSON_Error(t, ingester)
	})
	t.Run("FileWithUnknownCommit_Error", func(t *testing.T) {
		subTest_FileWithUnknownCommit_Error(t, ingester)
	})
}

const twoResultsFromThreeDevices = `{
  "gitHash": "bbbbb829a2384b001cc12b0c2613c756454a1f6a",
  "key": {
    "device": "angler"
  },
  "results": [
    {
      "key": {
        "source_type": "gm",
        "name": "test_alpha"
      },
      "options": {
        "ext": "png"
      },
      "md5": "11115ffee6ae2fec3ad71c777531578f"
    },
    {
      "key": {
        "source_type": "gm",
        "name": "test_beta"
      },
      "options": {
        "ext": "png"
      },
      "md5": "4444e0910d750195b448797616e091ad"
    }
  ]
}`

func subTest_AllNewData_Success(t *testing.T, p ingestion.Processor) {
	sqltest.RemoveOldDataAndResetSchema(t)
	clearCachesAndMetrics(p)
	db := p.(*sqlProcessor).db
	// Store data needed by this test.
	storeCommitData(t, db, three_devices.MakeTestCommits())

	// Process the file.
	const fakeFileLocation = "gs://bucket/path/to/file.json"
	const commitNumberOfThisFile = 2
	jsonResult := ingestion_mocks.MockResultFileLocationWithContent(fakeFileLocation, []byte(twoResultsFromThreeDevices), time.Time{})

	assert.NoError(t, p.Process(context.Background(), jsonResult))
	assertTraceCountMetricIs(t, p, 2)

	// Assert the data made it into the proper tables.
	// 2 keys
	alphaTracePK := assertKeyValueMapIsStored(t, db, map[string]string{
		"device":              three_devices.AnglerDevice,
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.AlphaTest),
	})
	betaTracePK := assertKeyValueMapIsStored(t, db, map[string]string{
		"device":              three_devices.AnglerDevice,
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.BetaTest),
	})
	// 1 option
	pngOptionsPK := assertKeyValueMapIsStored(t, db, map[string]string{
		"ext": "png",
	})
	// 2 groupings
	alphaGroupingPK := assertKeyValueMapIsStored(t, db, map[string]string{
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.AlphaTest),
	})
	betaGroupingPK := assertKeyValueMapIsStored(t, db, map[string]string{
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.BetaTest),
	})

	filePK := assertSourceFileIsIngested(t, db, fakeFileLocation, fakeNow)

	assertDigestOnTraceAtCommit(t, db, three_devices.AlphaNegativeDigest, alphaTracePK, commitNumberOfThisFile)
	assertDigestOnTraceAtCommit(t, db, three_devices.BetaPositiveDigest, betaTracePK, commitNumberOfThisFile)

	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, pngOptionsPK, alphaGroupingPK, filePK, alphaTracePK, commitNumberOfThisFile)
	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, pngOptionsPK, betaGroupingPK, filePK, betaTracePK, commitNumberOfThisFile)

	// When ingesting, if the expectations don't exist, we want to set them to be untriaged so as to
	// simplify future queries.
	assertExpectationsStored(t, db, alphaGroupingPK, three_devices.AlphaNegativeDigest, expectations.Untriaged)
	assertExpectationsStored(t, db, betaGroupingPK, three_devices.BetaPositiveDigest, expectations.Untriaged)
}

func subTest_DataFromPreviousCommitExists_Success(t *testing.T, p ingestion.Processor) {
	sqltest.RemoveOldDataAndResetSchema(t)
	clearCachesAndMetrics(p)
	db := p.(*sqlProcessor).db
	// Store data needed by this test. We are pretending the alpha data was stored in commit 1
	const previousCommitNumber = 1
	storeCommitData(t, db, three_devices.MakeTestCommits())
	alphaTracePK := storeKeyValueMap(t, db, map[string]string{
		"device":              three_devices.AnglerDevice,
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.AlphaTest),
	})
	pngOptionsPK := storeKeyValueMap(t, db, map[string]string{
		"ext": "png",
	})
	alphaGroupingPK := storeKeyValueMap(t, db, map[string]string{
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.AlphaTest),
	})
	oldFilePK := storeSourceFile(t, db, "gs://bucket/path/to/oldfile.json", fakeNow.Add(-time.Hour))
	storeTraceValue(t, db, three_devices.AlphaNegativeDigest, alphaTracePK, previousCommitNumber, pngOptionsPK, alphaGroupingPK, oldFilePK)
	storeExpectation(t, db, alphaGroupingPK, three_devices.AlphaNegativeDigest, expectations.Negative)
	p.(*sqlProcessor).traceCounter.Inc(1)

	// Process the file.
	const fakeFileLocation = "gs://bucket/path/to/file.json"
	const commitNumberOfThisFile = 2
	jsonResult := ingestion_mocks.MockResultFileLocationWithContent(fakeFileLocation, []byte(twoResultsFromThreeDevices), time.Time{})

	assert.NoError(t, p.Process(context.Background(), jsonResult))
	assertTraceCountMetricIs(t, p, 3) // 1 previously + 2 new traces in the file

	// Assert the data made it into the proper tables (and that the previous data still exists)
	betaTracePK := assertKeyValueMapIsStored(t, db, map[string]string{
		"device":              three_devices.AnglerDevice,
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.BetaTest),
	})
	betaGroupingPK := assertKeyValueMapIsStored(t, db, map[string]string{
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.BetaTest),
	})

	filePK := assertSourceFileIsIngested(t, db, fakeFileLocation, fakeNow)

	assertDigestOnTraceAtCommit(t, db, three_devices.AlphaNegativeDigest, alphaTracePK, previousCommitNumber)
	assertDigestOnTraceAtCommit(t, db, three_devices.AlphaNegativeDigest, alphaTracePK, commitNumberOfThisFile)
	assertDigestOnTraceAtCommit(t, db, three_devices.BetaPositiveDigest, betaTracePK, commitNumberOfThisFile)

	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, pngOptionsPK, alphaGroupingPK, oldFilePK, alphaTracePK, previousCommitNumber)
	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, pngOptionsPK, alphaGroupingPK, filePK, alphaTracePK, commitNumberOfThisFile)
	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, pngOptionsPK, betaGroupingPK, filePK, betaTracePK, commitNumberOfThisFile)

	// The alpha test should still be triaged negative (since it was before ingestion), but the beta
	// one should be marked untriaged (since it didn't have an expectation entry prior)
	assertExpectationsStored(t, db, alphaGroupingPK, three_devices.AlphaNegativeDigest, expectations.Negative)
	assertExpectationsStored(t, db, betaGroupingPK, three_devices.BetaPositiveDigest, expectations.Untriaged)
}

func subTest_OverwriteTraceValues_Success(t *testing.T, p ingestion.Processor) {
	sqltest.RemoveOldDataAndResetSchema(t)
	clearCachesAndMetrics(p)
	db := p.(*sqlProcessor).db
	const commitNumberOfPreviousFile = 2
	// Store data needed by this test. We are pretending AlphaPositiveDigest was stored previously
	// on commit 2
	storeCommitData(t, db, three_devices.MakeTestCommits())
	alphaTracePK := storeKeyValueMap(t, db, map[string]string{
		"device":              three_devices.AnglerDevice,
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.AlphaTest),
	})
	pngOptionsPK := storeKeyValueMap(t, db, map[string]string{
		"ext": "png",
	})
	alphaGroupingPK := storeKeyValueMap(t, db, map[string]string{
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.AlphaTest),
	})
	oldFilePK := storeSourceFile(t, db, "gs://bucket/path/to/oldfile.json", fakeNow.Add(-time.Hour))
	storeTraceValue(t, db, three_devices.AlphaPositiveDigest, alphaTracePK, commitNumberOfPreviousFile, pngOptionsPK, alphaGroupingPK, oldFilePK)
	storeExpectation(t, db, alphaGroupingPK, three_devices.AlphaPositiveDigest, expectations.Positive)
	p.(*sqlProcessor).traceCounter.Inc(1)

	// Process the file.
	const fakeFileLocation = "gs://bucket/path/to/file.json"
	const commitNumberOfThisFile = 2
	require.Equal(t, commitNumberOfPreviousFile, commitNumberOfThisFile)
	jsonResult := ingestion_mocks.MockResultFileLocationWithContent(fakeFileLocation, []byte(twoResultsFromThreeDevices), time.Time{})

	assert.NoError(t, p.Process(context.Background(), jsonResult))
	assertTraceCountMetricIs(t, p, 3) // 1 previously + 2 new traces in the file

	// Assert the data made it into the proper tables (and the previous data was overwritten).
	betaTracePK := assertKeyValueMapIsStored(t, db, map[string]string{
		"device":              three_devices.AnglerDevice,
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.BetaTest),
	})
	betaGroupingPK := assertKeyValueMapIsStored(t, db, map[string]string{
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.BetaTest),
	})

	filePK := assertSourceFileIsIngested(t, db, fakeFileLocation, fakeNow)

	assertDigestOnTraceAtCommit(t, db, three_devices.AlphaNegativeDigest, alphaTracePK, commitNumberOfThisFile)
	assertDigestOnTraceAtCommit(t, db, three_devices.BetaPositiveDigest, betaTracePK, commitNumberOfThisFile)

	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, pngOptionsPK, alphaGroupingPK, filePK, alphaTracePK, commitNumberOfThisFile)
	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, pngOptionsPK, betaGroupingPK, filePK, betaTracePK, commitNumberOfThisFile)

	// Both should be marked untriaged since this is the first time seeing both digests
	assertExpectationsStored(t, db, alphaGroupingPK, three_devices.AlphaNegativeDigest, expectations.Untriaged)
	assertExpectationsStored(t, db, betaGroupingPK, three_devices.BetaPositiveDigest, expectations.Untriaged)
}

func subTest_StoresManyValuesIncludingDuplicates_Success(t *testing.T, p ingestion.Processor) {
	sqltest.RemoveOldDataAndResetSchema(t)
	clearCachesAndMetrics(p)
	db := p.(*sqlProcessor).db
	// Store data needed by this test.
	storeCommitData(t, db, three_devices.MakeTestCommits())

	// This file is based off of production data from Skia. It has many results and was causing a
	// problem in early versions because the trace with hash 804485e766fe007e258573df00a79437
	// is in there twice (drawing the same digest both times). It also tests the batch logic.
	const fileLocation = "testdata/big.json"
	const commitNumberOfThisFile = 3
	const troublesomeDigest = "f83371a093397e6a86fc117b583b1533"
	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(fileLocation)
	require.NoError(t, err)

	assert.NoError(t, p.Process(context.Background(), fsResult))
	assertTraceCountMetricIs(t, p, 4796)

	// Spot check that the troublesome data point made it in
	tracePK := assertKeyValueMapIsStored(t, db, map[string]string{
		"arch":             "x86_64",
		"compiler":         "Clang",
		"config":           "8888",
		"configuration":    "Debug",
		"cpu_or_gpu":       "CPU",
		"cpu_or_gpu_value": "AVX2",
		"extra_config":     "SafeStack",
		"model":            "GCE",
		"name":             "mandrill.wbmp",
		"os":               "Debian10",
		"source_options":   "decode_native",
		"source_type":      "colorImage",
		"style":            "default",
	})
	optionsPK := assertKeyValueMapIsStored(t, db, map[string]string{
		"ext":         "png",
		"gamut":       "untagged",
		"transfer_fn": "untagged",
		"color_type":  "BGRA_8888",
		"alpha_type":  "Premul",
		"color_depth": "8888",
	})
	groupingPK := assertKeyValueMapIsStored(t, db, map[string]string{
		types.CorpusField:     "colorImage",
		types.PrimaryKeyField: "mandrill.wbmp",
	})
	assert.Equal(t, "804485e766fe007e258573df00a79437", hex.EncodeToString(tracePK))

	filePK := assertSourceFileIsIngested(t, db, fileLocation, fakeNow)
	assertDigestOnTraceAtCommit(t, db, troublesomeDigest, tracePK, commitNumberOfThisFile)
	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, optionsPK, groupingPK, filePK, tracePK, commitNumberOfThisFile)
	assertExpectationsStored(t, db, groupingPK, troublesomeDigest, expectations.Untriaged)
}

func subTest_FileWithNoResults_Ignored(t *testing.T, p ingestion.Processor) {
	sqltest.RemoveOldDataAndResetSchema(t)
	clearCachesAndMetrics(p)
	db := p.(*sqlProcessor).db
	// Store data needed by this test.
	storeCommitData(t, db, three_devices.MakeTestCommits())

	// Process the file.
	const fakeFileLocation = "gs://bucket/path/to/file.json"
	jsonResult := ingestion_mocks.MockResultFileLocationWithContent(fakeFileLocation, []byte(`{
  "gitHash": "bbbbb829a2384b001cc12b0c2613c756454a1f6a",
  "key": {
    "device": "angler"
  },
  "results": []
}`), time.Time{})

	err := p.Process(context.Background(), jsonResult)
	assert.Error(t, err)
	assert.Equal(t, ingestion.IgnoreResultsFileErr, err)
	assertTraceCountMetricIs(t, p, 0)
}

func subTest_FileWithInvalidJSON_Error(t *testing.T, p ingestion.Processor) {
	sqltest.RemoveOldDataAndResetSchema(t)
	clearCachesAndMetrics(p)
	db := p.(*sqlProcessor).db
	// Store data needed by this test.
	storeCommitData(t, db, three_devices.MakeTestCommits())

	// Process the file.
	const fakeFileLocation = "gs://bucket/path/to/file.json"
	jsonResult := ingestion_mocks.MockResultFileLocationWithContent(fakeFileLocation, []byte(`{
  "gitHash": "bbbbb829a2384b001cc12b0c2613c756454a1f6a",
  "key": {
    "device": "angler"
  },
  "results": [
    {
      "key": {
        "source_type": "this is invalid jSON
      },
      "options": {
        "ext": "png"
      },
      "md5": "11115ffee6ae2fec3ad71c777531578f"
    }
  ]
}`), time.Time{})

	err := p.Process(context.Background(), jsonResult)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "could not parse json")
	assertTraceCountMetricIs(t, p, 0)
}

func subTest_FileWithUnknownCommit_Error(t *testing.T, p ingestion.Processor) {
	sqltest.RemoveOldDataAndResetSchema(t)
	clearCachesAndMetrics(p)
	db := p.(*sqlProcessor).db
	// Store data needed by this test.
	storeCommitData(t, db, three_devices.MakeTestCommits())

	// Process the file.
	const fakeFileLocation = "gs://bucket/path/to/file.json"
	jsonResult := ingestion_mocks.MockResultFileLocationWithContent(fakeFileLocation, []byte(`{
  "gitHash": "0000000000000000000000000000000000000000",
  "key": {
    "device": "angler"
  },
  "results": [
    {
      "key": {
        "source_type": "gm",
        "name": "test_alpha"
      },
      "options": {
        "ext": "png"
      },
      "md5": "11115ffee6ae2fec3ad71c777531578f"
    }
  ]
}`), time.Time{})

	err := p.Process(context.Background(), jsonResult)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "could not determine branch for 0000000000000000000000000000000000000000")
	assertTraceCountMetricIs(t, p, 0)
}

// storeCommitData writes the given commits into the appropriate commit table
func storeCommitData(t *testing.T, db *pgxpool.Pool, commits []tiling.Commit) {
	const valuesPerRow = 5
	statement := "INSERT INTO Commits (commit_number, git_hash, commit_time, author, subject) VALUES "
	values, err := sql.ValuesPlaceholders(valuesPerRow, len(commits))
	require.NoError(t, err)
	statement += values
	arguments := make([]interface{}, 0, valuesPerRow*len(commits))
	for i, c := range commits {
		arguments = append(arguments, i+1)
		arguments = append(arguments, c.Hash)
		arguments = append(arguments, c.CommitTime)
		arguments = append(arguments, c.Author)
		arguments = append(arguments, c.Subject)
	}

	_, err = db.Exec(context.Background(), statement, arguments...)
	require.NoError(t, err, "inserting commits with statement %s", statement)
}

var fakeNow = time.Date(2020, time.August, 4, 4, 4, 4, 0, time.UTC)

func testSQLProcessor(t *testing.T, dbURL string) ingestion.Processor {
	cfg := ingestion.Config{
		ExtraParams: map[string]string{
			sqlConnectionURL: dbURL,
		},
	}
	ingester, err := newSQLProcessor(context.Background(), nil, cfg, nil)
	require.NoError(t, err)
	ingester.(*sqlProcessor).now = func() time.Time {
		return fakeNow
	}
	return ingester
}

func clearCachesAndMetrics(p ingestion.Processor) {
	s := p.(*sqlProcessor)
	s.keysOptionsCache.Purge()
	s.traceCounter.Reset()
}

func assertTraceCountMetricIs(t *testing.T, p ingestion.Processor, n int64) {
	s := p.(*sqlProcessor)

	assert.Equal(t, n, s.traceCounter.Get())
}

// storeKeyValueMap stores the given map in the KeyValueMaps table and returns the primary key
// (i.e. the md5 hash of the map). If the map is already stored, this will fail (to alert test
// Author that preconditions may not have met expectations).
func storeKeyValueMap(t *testing.T, db *pgxpool.Pool, m map[string]string) []byte {
	mJSON, keysHashPK, err := sql.SerializeMap(m)
	require.NoError(t, err)

	_, err = db.Exec(context.Background(),
		`INSERT INTO KeyValueMaps (keys_hash, keys) VALUES ($1, $2)`,
		keysHashPK, mJSON)
	require.NoError(t, err)
	return keysHashPK
}

// assertKeyValueMapIsStored asserts the given map is in the KeyValueMaps table and returns
// the primary key (i.e. the md5 hash of the map).
func assertKeyValueMapIsStored(t *testing.T, db *pgxpool.Pool, m map[string]string) []byte {
	mJSON, expectedHash, err := sql.SerializeMap(m)
	require.NoError(t, err)

	row := db.QueryRow(context.Background(),
		`SELECT keys_hash FROM KeyValueMaps WHERE keys = $1;`, mJSON)

	var actualKeyHashPK []byte
	require.NoError(t, row.Scan(&actualKeyHashPK), "Expected one row where keys = %s", mJSON)
	assert.Equal(t, expectedHash, actualKeyHashPK)
	return actualKeyHashPK
}

// storeSourceFile stores the given source file into the SourceFiles table and returns the primary
// key (i.e. the md5 hash of the file path). If the source file is already stored, this will fail
// (to alert test Author that preconditions may not have met expectations).
func storeSourceFile(t *testing.T, db *pgxpool.Pool, filePath string, ingestedTime time.Time) []byte {
	sourceFilePK := md5.Sum([]byte(filePath))
	_, err := db.Exec(context.Background(),
		`INSERT INTO SourceFiles (source_file_hash, source_file, last_ingested) VALUES ($1, $2, $3)`,
		sourceFilePK[:], filePath, ingestedTime)
	require.NoError(t, err)
	return sourceFilePK[:]
}

// assertSourceFileIsIngested asserts the given source file was stored in the SourceFiles table
// and returns the primary key (i.e. the md5 hash of the file path)
func assertSourceFileIsIngested(t *testing.T, db *pgxpool.Pool, filePath string, expectedTime time.Time) []byte {
	row := db.QueryRow(context.Background(),
		`SELECT source_file_hash, last_ingested FROM SourceFiles WHERE source_file = $1;`, filePath)

	expectedHash := md5.Sum([]byte(filePath))

	var actualSourceFilePK []byte
	var ingestedTime time.Time
	require.NoError(t, row.Scan(&actualSourceFilePK, &ingestedTime), "Expected one row where source_file = %s", filePath)
	assert.Equal(t, expectedHash[:], actualSourceFilePK)
	ingestedTime = ingestedTime.UTC()
	assert.Equal(t, expectedTime, ingestedTime, "Time %s != %s", expectedTime, ingestedTime)
	return actualSourceFilePK
}

// storeTraceValue stores a row in the TraceValues table. If the row already exists, this will
// fail (to alert test Author that preconditions may not have met expectations).
func storeTraceValue(t *testing.T, db *pgxpool.Pool, digest types.Digest, traceHashFK []byte, commitNumber int, optionsFK, groupingFK, sourceFileFK []byte) {
	digestBytes, err := sql.DigestToBytes(digest)
	require.NoError(t, err)

	_, err = db.Exec(context.Background(), `
INSERT INTO TraceValues (trace_hash, shard, commit_number, grouping_hash, digest, 
  options_hash, source_file_hash) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		traceHashFK, sql.ComputeTraceValueShard(traceHashFK), commitNumber, groupingFK, digestBytes,
		optionsFK, sourceFileFK)
	require.NoError(t, err)
}

func assertDigestOnTraceAtCommit(t *testing.T, db *pgxpool.Pool, digest types.Digest, traceHashFK []byte, commitNumber int) {
	row := db.QueryRow(context.Background(),
		`SELECT encode(digest, 'hex') FROM TraceValues WHERE trace_hash = $1 AND commit_number = $2;`,
		traceHashFK, commitNumber)

	var storedDigest types.Digest
	err := row.Scan(&storedDigest)
	require.NoError(t, err, "expected one row where trace_hash = %v and commit_number = %d", traceHashFK, commitNumber)

	assert.Equal(t, digest, storedDigest, "Incorrect digest where trace_hash = %v and commit_number = %d", traceHashFK, commitNumber)
}

func assertOptionsGroupingSourceFileOnTraceAtCommit(t *testing.T, db *pgxpool.Pool, optionsFK, groupingFK, sourceFileFK, traceFK []byte, commitNumber int) {
	row := db.QueryRow(context.Background(),
		`SELECT options_hash, grouping_hash, source_file_hash FROM TraceValues WHERE trace_hash = $1 AND commit_number = $2;`,
		traceFK, commitNumber)

	var actualOptionsFK []byte
	var actualGroupingFK []byte
	var actualSourceFileFK []byte
	err := row.Scan(&actualOptionsFK, &actualGroupingFK, &actualSourceFileFK)
	require.NoError(t, err, "expected one row where trace_hash = %v and commit_number = %d", traceFK, commitNumber)

	assert.Equal(t, optionsFK, actualOptionsFK)
	assert.Equal(t, groupingFK, actualGroupingFK)
	assert.Equal(t, sourceFileFK, actualSourceFileFK)
}

// storeExpectation stores a row in the ExpectationsTable. If the row already exists, this will
// fail (to alert test Author that preconditions may not have met expectations).
func storeExpectation(t *testing.T, db *pgxpool.Pool, groupingFK []byte, digest types.Digest, label expectations.Label) {
	digestBytes, err := sql.DigestToBytes(digest)
	require.NoError(t, err)

	labelInt := sql.ConvertLabelFromString(label)

	_, err = db.Exec(context.Background(),
		`INSERT INTO Expectations (grouping_hash, digest, label) VALUES ($1, $2, $3)`,
		groupingFK, digestBytes, labelInt)
	require.NoError(t, err)
}

func assertExpectationsStored(t *testing.T, db *pgxpool.Pool, groupingFK []byte, digest types.Digest, expectedLabel expectations.Label) {
	digestBytes, err := sql.DigestToBytes(digest)
	require.NoError(t, err)

	row := db.QueryRow(context.Background(),
		`SELECT label FROM Expectations WHERE grouping_hash = $1 AND digest = $2;`,
		groupingFK, digestBytes)

	var storedLabel sql.ExpectationsLabel
	err = row.Scan(&storedLabel)
	require.NoError(t, err, "Expected one row where grouping_hash = %v and digest = %v", groupingFK, digestBytes)

	assert.Equal(t, sql.ConvertLabelFromString(expectedLabel), storedLabel)
}
