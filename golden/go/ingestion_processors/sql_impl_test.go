package ingestion_processors

import (
	"context"
	"crypto/md5"
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

func TestSQLProcess(t *testing.T) {
	unittest.LargeTest(t)

	dbURL, deleteDB := sqltest.MakeLocalCockroachDBForTesting(t)
	defer deleteDB() // uncomment this to explore the results more easily

	ingester := testSQLProcessor(t, dbURL)

	t.Run("AllNewData_Success", func(t *testing.T) {
		subTest_AllNewData_Success(t, ingester)
	})

	t.Run("DataFromPreviousCommitExists", func(t *testing.T) {
		subTest_DataFromPreviousCommitExists_Success(t, ingester)
	})

	t.Run("OverwriteTraceValues", func(t *testing.T) {
		subTest_OverwriteTraceValues_Success(t, ingester)
	})

	t.Run("StoresManyValues", func(t *testing.T) {
		subTest_StoresManyValues_Success(t, ingester)
	})

	// Error cases

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
	// Store data needed by this test.
	storeCommitData(t, p, three_devices.MakeTestCommits())

	// Process the file.
	const fakeFileLocation = "gs://bucket/path/to/file.json"
	const commitNumberOfThisFile = 2
	fsResult := ingestion_mocks.MockResultFileLocationWithContent(fakeFileLocation, []byte(twoResultsFromThreeDevices), time.Time{})

	assert.NoError(t, p.Process(context.Background(), fsResult))
	assertTraceCountMetricIs(t, p, 2)

	// Assert the data made it into the proper tables.
	db := p.(*sqlProcessor).db

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

	assert.Fail(t, "need to assert stuff")
}

func subTest_DataFromPreviousCommitExists_Success(t *testing.T, ingester ingestion.Processor) {
	// store data explicitly

	assert.Fail(t, "not impl")
}

func subTest_OverwriteTraceValues_Success(t *testing.T, ingester ingestion.Processor) {
	assert.Fail(t, "not impl")
}

func subTest_StoresManyValues_Success(t *testing.T, ingester ingestion.Processor) {
	TODO ingest the big file that we were seeing errors on
}

// storeCommitData writes the given commits into the appropriate commit table
func storeCommitData(t *testing.T, ingester ingestion.Processor, commits []tiling.Commit) {
	db := ingester.(*sqlProcessor).db

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

// assertKeyValueMapIsStored asserts the given map is in the KeyValueMaps table and returns
// the primary key (i.e. the md5 hash of the map).
func assertKeyValueMapIsStored(t *testing.T, db *pgxpool.Pool, m map[string]string) []byte {
	mJSON, expectedHash, err := sql.SerializeMap(m)
	require.NoError(t, err)

	row := db.QueryRow(context.Background(), `
SELECT keys_hash FROM KeyValueMaps WHERE keys = $1;`, mJSON)

	var actualKeyHashPK []byte
	require.NoError(t, row.Scan(&actualKeyHashPK), "Expected one row where keys = %s", mJSON)
	assert.Equal(t, expectedHash, actualKeyHashPK)
	return actualKeyHashPK
}

func assertSourceFileIsIngested(t *testing.T, db *pgxpool.Pool, filePath string, expectedTime time.Time) []byte {
	row := db.QueryRow(context.Background(), `
SELECT source_file_hash, last_ingested FROM SourceFiles WHERE source_file = $1;`, filePath)

	expectedHash := md5.Sum([]byte(filePath))

	var actualSourceFilePK []byte
	var ingestedTime time.Time
	require.NoError(t, row.Scan(&actualSourceFilePK, &ingestedTime), "Expected one row where source_file = %s", filePath)
	assert.Equal(t, expectedHash[:], actualSourceFilePK)
	ingestedTime = ingestedTime.UTC()
	assert.Equal(t, expectedTime, ingestedTime, "Time %s != %s", expectedTime, ingestedTime)
	return actualSourceFilePK
}

func assertDigestOnTraceAtCommit(t *testing.T, db *pgxpool.Pool, digest types.Digest, traceHashFK []byte, commitNumber int) {
	row := db.QueryRow(context.Background(), `
SELECT encode(digest, 'hex') FROM TraceValues WHERE trace_hash = $1 AND commit_number = $2;`,
		traceHashFK, commitNumber)

	var storedDigest types.Digest
	err := row.Scan(&storedDigest)
	require.NoError(t, err, "expected one row where trace_hash = %v and commit_number = %d", traceHashFK, commitNumber)

	assert.Equal(t, digest, storedDigest, "Incorrect digest where trace_hash = %v and commit_number = %d", traceHashFK, commitNumber)
}

func assertOptionsGroupingSourceFileOnTraceAtCommit(t *testing.T, db *pgxpool.Pool, optionsFK, groupingFK, sourceFileFK, traceFK []byte, commitNumber int) {
	row := db.QueryRow(context.Background(), `
SELECT options_hash, grouping_hash, source_file_hash FROM TraceValues WHERE trace_hash = $1 AND commit_number = $2;`,
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

func assertExpectationsStored(t *testing.T, db *pgxpool.Pool, groupingFK []byte, digest types.Digest, expectedLabel expectations.Label) {
	digestBytes, err := sql.DigestToBytes(digest)
	require.NoError(t, err)

	row := db.QueryRow(context.Background(), `
SELECT label FROM Expectations WHERE grouping_hash = $1 AND digest = $2;`,
		groupingFK, digestBytes)

	var storedLabel sql.ExpectationsLabel
	err = row.Scan(&storedLabel)
	require.NoError(t, err, "Expected one row where grouping_hash = %v and digest = %v", groupingFK, digestBytes)

	assert.Equal(t, sql.ConvertLabelFromString(expectedLabel), storedLabel)
}
