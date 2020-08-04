package ingestion_processors

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/ingestion"
	ingestion_mocks "go.skia.org/infra/golden/go/ingestion/mocks"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/sqltest"
	three_devices "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

func TestSQLProcess_TypicalFlows_Success(t *testing.T) {
	unittest.LargeTest(t)

	dbURL, deleteDB := sqltest.MakeLocalCockroachDBWithSchema(t)
	defer deleteDB() // uncomment this to explore the results more easily

	ingester := testSQLProcessor(t, dbURL)

	storeCommitData(t, ingester, three_devices.MakeTestCommits())

	t.Run("Ingesting all new data", func(t *testing.T) {
		ingestingAllNewData(t, ingester)
	})
	deleteIngestionData(t, ingester)

	t.Run("All keys already exist", func(t *testing.T) {
		allKeysAlreadyExist(t, ingester)
	})
	deleteIngestionData(t, ingester)

	t.Run("Some keys already exist", func(t *testing.T) {
		someKeysAlreadyExist(t, ingester)
	})
	deleteIngestionData(t, ingester)

	t.Run("Overwriting existing values", func(t *testing.T) {
		overwriteExistingValues(t, ingester)
	})
	deleteIngestionData(t, ingester)
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

func ingestingAllNewData(t *testing.T, p ingestion.Processor) {
	const fakeFileLocation = "gs://bucket/path/to/file.json"
	const commitNumberOfThisFile = 2
	whenever := time.Date(2020, time.August, 1, 2, 3, 4, 0, time.UTC)
	fsResult := ingestion_mocks.MockResultFileLocationWithContent(fakeFileLocation, []byte(twoResultsFromThreeDevices), whenever)

	assert.NoError(t, p.Process(context.Background(), fsResult))

	db := p.(*sqlProcessor).db

	// 2 keys
	alphaTrace := assertHasKeyValueMapAndCorrectHash(t, db, map[string]string{
		"device":              three_devices.AnglerDevice,
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.AlphaTest),
	})
	betaTrace := assertHasKeyValueMapAndCorrectHash(t, db, map[string]string{
		"device":              three_devices.AnglerDevice,
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.BetaTest),
	})
	// 1 option
	pngOptions := assertHasKeyValueMapAndCorrectHash(t, db, map[string]string{
		"ext": "png",
	})
	// 2 groupings
	alphaGrouping := assertHasKeyValueMapAndCorrectHash(t, db, map[string]string{
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.AlphaTest),
	})
	betaGrouping := assertHasKeyValueMapAndCorrectHash(t, db, map[string]string{
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.BetaTest),
	})

	fileNameHash := assertHasSourceFileIngested(t, fakeFileLocation, whenever)

	assertDigestOnTraceAtCommit(t, db, three_devices.AlphaNegativeDigest, alphaTrace, commitNumberOfThisFile)
	assertDigestOnTraceAtCommit(t, db, three_devices.BetaPositiveDigest, betaTrace, commitNumberOfThisFile)

	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, pngOptions, alphaGrouping, fileNameHash, alphaTrace, commitNumberOfThisFile)
	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, pngOptions, betaGrouping, fileNameHash, betaTrace, commitNumberOfThisFile)

	assert.Fail(t, "need to assert stuff")
}

func allKeysAlreadyExist(t *testing.T, ingester ingestion.Processor) {
	assert.Fail(t, "not impl")
}

func someKeysAlreadyExist(t *testing.T, ingester ingestion.Processor) {
	assert.Fail(t, "not impl")
}

func overwriteExistingValues(t *testing.T, ingester ingestion.Processor) {
	assert.Fail(t, "not impl")
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

// deleteIngestionData removes all rows from tables written to during ingestion.
func deleteIngestionData(t *testing.T, ingester ingestion.Processor) {
	db := ingester.(*sqlProcessor).db

	_, err := db.Exec(context.Background(), `
DELETE FROM TraceValues;
DELETE FROM KeyValueMaps;
DELETE FROM SourceFiles;
DELETE FROM Expectations;
`)
	require.NoError(t, err, "inserting commits")
}

func testSQLProcessor(t *testing.T, dbURL string) ingestion.Processor {
	cfg := ingestion.Config{
		ExtraParams: map[string]string{
			sqlConnectionURL: dbURL,
		},
	}
	ingester, err := newSQLProcessor(context.Background(), nil, cfg, nil)
	require.NoError(t, err)
	return ingester
}

func assertHasKeyValueMapAndCorrectHash(t *testing.T, db *pgxpool.Pool, m map[string]string) []byte {
	mJSON, expectedHash, err := sql.SerializeMap(m)
	require.NoError(t, err)

	row := db.QueryRow(context.Background(), `
SELECT keys_hash FROM KeyValueMaps WHERE keys = $1;`, mJSON)

	var keysHash []byte
	require.NoError(t, row.Scan(&keysHash), "Expected one row where keys = %s", mJSON)
	assert.Equal(t, keysHash, expectedHash, "Expected one row where keys = %s and the hash to be %v", mJSON, expectedHash)
	return keysHash
}

func assertHasSourceFileIngested(t *testing.T, db *pgxpool.Pool, filePath string, ingestedTime time.Time) interface{} {
	row := db.QueryRow(context.Background(), `
SELECT source_file_hash, last_ingested FROM KeyValueMaps WHERE source_file = $1;`, filePath)

	var keysHash []byte
	require.NoError(t, row.Scan(&keysHash), "Expected one row where keys = %s", mJSON)
	assert.Equal(t, keysHash, expectedHash, "Expected one row where keys = %s and the hash to be %v", mJSON, expectedHash)
	return keysHash
}

func assertDigestOnTraceAtCommit(t *testing.T, db *pgxpool.Pool, digest types.Digest, traceHash []byte, commitNumber int) {
	row := db.QueryRow(context.Background(), `
SELECT encode(digest, 'hex') FROM TraceValues WHERE trace_hash = $1 AND commit_number = $2;`,
		traceHash, commitNumber)

	var storedDigest types.Digest
	err := row.Scan(&storedDigest)
	require.NoError(t, err, "expected one row where trace_hash = %v and commit_number = %d", traceHash, commitNumber)

	assert.Equal(t, digest, storedDigest, "Incorrect digest where trace_hash = %v and commit_number = %d", traceHash, commitNumber)
}

func assertOptionsGroupingSourceFileOnTraceAtCommit(t *testing.T, db *pgxpool.Pool, options, grouping, sourceFile, traceHash []byte, commitNumber int) {
	row := db.QueryRow(context.Background(), `
SELECT options_hash, grouping_hash, source_file_hash FROM TraceValues WHERE trace_hash = $1 AND commit_number = $2;`,
		traceHash, commitNumber)

}
