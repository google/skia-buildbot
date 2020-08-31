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
	t.Run("DataFromPreviousCommitExists_Success", func(t *testing.T) {
		subTest_DataFromFutureCommitExists_Success(t, ingester)
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
	const commitFKOfThisFile = 2
	jsonResult := ingestion_mocks.MockResultFileLocationWithContent(fakeFileLocation, []byte(twoResultsFromThreeDevices), time.Time{})

	assert.NoError(t, p.Process(context.Background(), jsonResult))
	assertTraceCountMetricIs(t, p, 2)

	// Assert the data made it into the proper tables.
	// 2 keys
	alphaTracePK := assertTraceIsStored(t, db, map[string]string{
		"device":              three_devices.AnglerDevice,
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.AlphaTest),
	})
	betaTracePK := assertTraceIsStored(t, db, map[string]string{
		"device":              three_devices.AnglerDevice,
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.BetaTest),
	})
	// 1 option
	pngOptionsPK := assertOptionsAreStored(t, db, map[string]string{
		"ext": "png",
	})
	// 2 groupings
	alphaGroupingPK := assertGroupingIsStored(t, db, map[string]string{
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.AlphaTest),
	})
	betaGroupingPK := assertGroupingIsStored(t, db, map[string]string{
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.BetaTest),
	})

	filePK := assertSourceFileIsIngested(t, db, fakeFileLocation, fakeNow)

	assertDigestOnTraceAtCommit(t, db, three_devices.AlphaNegativeDigest, alphaTracePK, commitFKOfThisFile)
	assertDigestOnTraceAtCommit(t, db, three_devices.BetaPositiveDigest, betaTracePK, commitFKOfThisFile)

	assertTraceHasNullIgnoreStatus(t, db, alphaTracePK)
	assertTraceHasNullIgnoreStatus(t, db, betaTracePK)

	assertTraceHasMostRecentCommit(t, db, alphaTracePK, commitFKOfThisFile)
	assertTraceHasMostRecentCommit(t, db, betaTracePK, commitFKOfThisFile)

	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, pngOptionsPK, alphaGroupingPK, filePK, alphaTracePK, commitFKOfThisFile)
	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, pngOptionsPK, betaGroupingPK, filePK, betaTracePK, commitFKOfThisFile)

	// When ingesting, if the expectations don't exist, we want to set them to be untriaged so as to
	// simplify future queries.
	assertExpectationsStored(t, db, alphaGroupingPK, three_devices.AlphaNegativeDigest, expectations.Untriaged)
	assertExpectationsStored(t, db, betaGroupingPK, three_devices.BetaPositiveDigest, expectations.Untriaged)

	assertPrimaryBranchParamsAreOnCommit(t, db, []keyValue{
		{key: "device", value: three_devices.AnglerDevice},
		{key: types.CorpusField, value: three_devices.GMCorpus},
		{key: types.PrimaryKeyField, value: string(three_devices.AlphaTest)},
		{key: types.PrimaryKeyField, value: string(three_devices.BetaTest)},
		{key: "ext", value: "png"},
	}, commitFKOfThisFile)

	assertCommitHasData(t, db, commitFKOfThisFile)
}

func subTest_DataFromPreviousCommitExists_Success(t *testing.T, p ingestion.Processor) {
	sqltest.RemoveOldDataAndResetSchema(t)
	clearCachesAndMetrics(p)
	db := p.(*sqlProcessor).db
	// Store data needed by this test. We are pretending the alpha test data was stored in commit 1
	// (but not the beta test).
	const previousCommitFK = 1
	storeCommitData(t, db, three_devices.MakeTestCommits())
	storeCommitHasData(t, db, previousCommitFK)
	alphaTracePK := storeTrace(t, db, map[string]string{
		"device":              three_devices.AnglerDevice,
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.AlphaTest),
	}, previousCommitFK)
	pngOptionsPK := storeOptions(t, db, map[string]string{
		"ext": "png",
	})
	alphaGroupingPK := storeGrouping(t, db, map[string]string{
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.AlphaTest),
	})
	previousCommitParamKV := []keyValue{
		{key: "device", value: three_devices.AnglerDevice},
		{key: types.CorpusField, value: three_devices.GMCorpus},
		{key: types.PrimaryKeyField, value: string(three_devices.AlphaTest)},
		{key: "ext", value: "png"},
	}
	oldFilePK := storeSourceFile(t, db, "gs://bucket/path/to/oldfile.json", fakeNow.Add(-time.Hour))
	storeTraceValue(t, db, three_devices.AlphaNegativeDigest, alphaTracePK, previousCommitFK, pngOptionsPK, alphaGroupingPK, oldFilePK)
	storeExpectation(t, db, alphaGroupingPK, three_devices.AlphaNegativeDigest, expectations.Negative)
	storePrimaryBranchParams(t, db, previousCommitFK, previousCommitParamKV)
	p.(*sqlProcessor).traceCounter.Inc(1)

	// Process the file.
	const fakeFileLocation = "gs://bucket/path/to/file.json"
	const commitFK = 2
	jsonResult := ingestion_mocks.MockResultFileLocationWithContent(fakeFileLocation, []byte(twoResultsFromThreeDevices), time.Time{})

	assert.NoError(t, p.Process(context.Background(), jsonResult))
	assertTraceCountMetricIs(t, p, 3) // 1 previously + 2 new traces in the file

	// Assert the data made it into the proper tables (and that the previous data still exists)
	betaTracePK := assertTraceIsStored(t, db, map[string]string{
		"device":              three_devices.AnglerDevice,
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.BetaTest),
	})
	betaGroupingPK := assertGroupingIsStored(t, db, map[string]string{
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.BetaTest),
	})

	filePK := assertSourceFileIsIngested(t, db, fakeFileLocation, fakeNow)

	assertDigestOnTraceAtCommit(t, db, three_devices.AlphaNegativeDigest, alphaTracePK, previousCommitFK)
	assertDigestOnTraceAtCommit(t, db, three_devices.AlphaNegativeDigest, alphaTracePK, commitFK)
	assertDigestOnTraceAtCommit(t, db, three_devices.BetaPositiveDigest, betaTracePK, commitFK)

	assertTraceHasMostRecentCommit(t, db, alphaTracePK, commitFK)
	assertTraceHasMostRecentCommit(t, db, betaTracePK, commitFK)

	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, pngOptionsPK, alphaGroupingPK, oldFilePK, alphaTracePK, previousCommitFK)
	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, pngOptionsPK, alphaGroupingPK, filePK, alphaTracePK, commitFK)
	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, pngOptionsPK, betaGroupingPK, filePK, betaTracePK, commitFK)

	// The alpha test should still be triaged negative (since it was before ingestion), but the beta
	// one should be marked untriaged (since it didn't have an expectation entry prior)
	assertExpectationsStored(t, db, alphaGroupingPK, three_devices.AlphaNegativeDigest, expectations.Negative)
	assertExpectationsStored(t, db, betaGroupingPK, three_devices.BetaPositiveDigest, expectations.Untriaged)

	assertPrimaryBranchParamsAreOnCommit(t, db, previousCommitParamKV, previousCommitFK)
	assertPrimaryBranchParamsAreOnCommit(t, db, []keyValue{
		{key: "device", value: three_devices.AnglerDevice},
		{key: types.CorpusField, value: three_devices.GMCorpus},
		{key: types.PrimaryKeyField, value: string(three_devices.AlphaTest)},
		{key: types.PrimaryKeyField, value: string(three_devices.BetaTest)},
		{key: "ext", value: "png"},
	}, commitFK)

	assertCommitHasData(t, db, commitFK)
}

func subTest_DataFromFutureCommitExists_Success(t *testing.T, p ingestion.Processor) {
	sqltest.RemoveOldDataAndResetSchema(t)
	clearCachesAndMetrics(p)
	db := p.(*sqlProcessor).db
	// Store data needed by this test. We are pretending the alpha test data was stored in commit 3
	// (but not the beta test).
	const futureCommitFK = 3
	storeCommitData(t, db, three_devices.MakeTestCommits())
	storeCommitHasData(t, db, futureCommitFK)
	alphaTracePK := storeTrace(t, db, map[string]string{
		"device":              three_devices.AnglerDevice,
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.AlphaTest),
	}, futureCommitFK)
	pngOptionsPK := storeOptions(t, db, map[string]string{
		"ext": "png",
	})
	alphaGroupingPK := storeGrouping(t, db, map[string]string{
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.AlphaTest),
	})
	futureCommitParamKV := []keyValue{
		{key: "device", value: three_devices.AnglerDevice},
		{key: types.CorpusField, value: three_devices.GMCorpus},
		{key: types.PrimaryKeyField, value: string(three_devices.AlphaTest)},
		{key: "ext", value: "png"},
	}
	futureFilePK := storeSourceFile(t, db, "gs://bucket/path/to/oldfile.json", fakeNow.Add(-time.Hour))
	storeTraceValue(t, db, three_devices.AlphaNegativeDigest, alphaTracePK, futureCommitFK, pngOptionsPK, alphaGroupingPK, futureFilePK)
	storeExpectation(t, db, alphaGroupingPK, three_devices.AlphaNegativeDigest, expectations.Negative)
	storePrimaryBranchParams(t, db, futureCommitFK, futureCommitParamKV)
	p.(*sqlProcessor).traceCounter.Inc(1)

	// Process the file.
	const fakeFileLocation = "gs://bucket/path/to/file.json"
	const commitFK = 2
	jsonResult := ingestion_mocks.MockResultFileLocationWithContent(fakeFileLocation, []byte(twoResultsFromThreeDevices), time.Time{})

	assert.NoError(t, p.Process(context.Background(), jsonResult))
	assertTraceCountMetricIs(t, p, 3) // 1 previously + 2 new traces in the file

	// Assert the data made it into the proper tables (and that the previous data still exists)
	betaTracePK := assertTraceIsStored(t, db, map[string]string{
		"device":              three_devices.AnglerDevice,
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.BetaTest),
	})
	betaGroupingPK := assertGroupingIsStored(t, db, map[string]string{
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.BetaTest),
	})

	filePK := assertSourceFileIsIngested(t, db, fakeFileLocation, fakeNow)

	assertDigestOnTraceAtCommit(t, db, three_devices.AlphaNegativeDigest, alphaTracePK, futureCommitFK)
	assertDigestOnTraceAtCommit(t, db, three_devices.AlphaNegativeDigest, alphaTracePK, commitFK)
	assertDigestOnTraceAtCommit(t, db, three_devices.BetaPositiveDigest, betaTracePK, commitFK)

	// Make sure the future commit is still known as the most recent commit for alpha trace since
	// it already had data stored.
	assertTraceHasMostRecentCommit(t, db, alphaTracePK, futureCommitFK)
	assertTraceHasMostRecentCommit(t, db, betaTracePK, commitFK)

	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, pngOptionsPK, alphaGroupingPK, futureFilePK, alphaTracePK, futureCommitFK)
	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, pngOptionsPK, alphaGroupingPK, filePK, alphaTracePK, commitFK)
	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, pngOptionsPK, betaGroupingPK, filePK, betaTracePK, commitFK)

	// The alpha test should still be triaged negative (since it was before ingestion), but the beta
	// one should be marked untriaged (since it didn't have an expectation entry prior)
	assertExpectationsStored(t, db, alphaGroupingPK, three_devices.AlphaNegativeDigest, expectations.Negative)
	assertExpectationsStored(t, db, betaGroupingPK, three_devices.BetaPositiveDigest, expectations.Untriaged)

	assertPrimaryBranchParamsAreOnCommit(t, db, futureCommitParamKV, futureCommitFK)
	assertPrimaryBranchParamsAreOnCommit(t, db, []keyValue{
		{key: "device", value: three_devices.AnglerDevice},
		{key: types.CorpusField, value: three_devices.GMCorpus},
		{key: types.PrimaryKeyField, value: string(three_devices.AlphaTest)},
		{key: types.PrimaryKeyField, value: string(three_devices.BetaTest)},
		{key: "ext", value: "png"},
	}, commitFK)

	assertCommitHasData(t, db, commitFK)
}

func subTest_OverwriteTraceValues_Success(t *testing.T, p ingestion.Processor) {
	sqltest.RemoveOldDataAndResetSchema(t)
	clearCachesAndMetrics(p)
	db := p.(*sqlProcessor).db
	const previousFileCommitFK = 2
	// Store data needed by this test. We are pretending AlphaPositiveDigest was stored previously
	// on commit 2
	storeCommitData(t, db, three_devices.MakeTestCommits())
	storeCommitHasData(t, db, previousFileCommitFK)
	alphaTracePK := storeTrace(t, db, map[string]string{
		"device":              three_devices.AnglerDevice,
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.AlphaTest),
	}, previousFileCommitFK)
	pngOptionsPK := storeOptions(t, db, map[string]string{
		"ext": "png",
	})
	alphaGroupingPK := storeGrouping(t, db, map[string]string{
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.AlphaTest),
	})
	previousParamKV := []keyValue{
		{key: "device", value: three_devices.AnglerDevice},
		{key: types.CorpusField, value: three_devices.GMCorpus},
		{key: types.PrimaryKeyField, value: string(three_devices.AlphaTest)},
		{key: "ext", value: "png"},
	}
	oldFilePK := storeSourceFile(t, db, "gs://bucket/path/to/oldfile.json", fakeNow.Add(-time.Hour))
	storeTraceValue(t, db, three_devices.AlphaPositiveDigest, alphaTracePK, previousFileCommitFK, pngOptionsPK, alphaGroupingPK, oldFilePK)
	storeExpectation(t, db, alphaGroupingPK, three_devices.AlphaPositiveDigest, expectations.Positive)
	storePrimaryBranchParams(t, db, previousFileCommitFK, previousParamKV)
	p.(*sqlProcessor).traceCounter.Inc(1)

	// Process the file.
	const fakeFileLocation = "gs://bucket/path/to/file.json"
	const commitFK = 2
	// Let's make it explicit that we are writing on the same commit as before.
	require.Equal(t, previousFileCommitFK, commitFK)
	jsonResult := ingestion_mocks.MockResultFileLocationWithContent(fakeFileLocation, []byte(twoResultsFromThreeDevices), time.Time{})

	assert.NoError(t, p.Process(context.Background(), jsonResult))
	assertTraceCountMetricIs(t, p, 3) // 1 previously + 2 new traces in the file

	// Assert the data made it into the proper tables (and the previous data was overwritten).
	betaTracePK := assertTraceIsStored(t, db, map[string]string{
		"device":              three_devices.AnglerDevice,
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.BetaTest),
	})
	betaGroupingPK := assertGroupingIsStored(t, db, map[string]string{
		types.CorpusField:     three_devices.GMCorpus,
		types.PrimaryKeyField: string(three_devices.BetaTest),
	})

	filePK := assertSourceFileIsIngested(t, db, fakeFileLocation, fakeNow)

	assertDigestOnTraceAtCommit(t, db, three_devices.AlphaNegativeDigest, alphaTracePK, commitFK)
	assertDigestOnTraceAtCommit(t, db, three_devices.BetaPositiveDigest, betaTracePK, commitFK)

	assertTraceHasMostRecentCommit(t, db, alphaTracePK, commitFK)
	assertTraceHasMostRecentCommit(t, db, betaTracePK, commitFK)

	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, pngOptionsPK, alphaGroupingPK, filePK, alphaTracePK, commitFK)
	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, pngOptionsPK, betaGroupingPK, filePK, betaTracePK, commitFK)

	// Both should be marked untriaged since this is the first time seeing both digests
	assertExpectationsStored(t, db, alphaGroupingPK, three_devices.AlphaNegativeDigest, expectations.Untriaged)
	assertExpectationsStored(t, db, betaGroupingPK, three_devices.BetaPositiveDigest, expectations.Untriaged)

	assertPrimaryBranchParamsAreOnCommit(t, db, []keyValue{
		{key: "device", value: three_devices.AnglerDevice},
		{key: types.CorpusField, value: three_devices.GMCorpus},
		{key: types.PrimaryKeyField, value: string(three_devices.AlphaTest)},
		{key: types.PrimaryKeyField, value: string(three_devices.BetaTest)},
		{key: "ext", value: "png"},
	}, commitFK)
	assertCommitHasData(t, db, commitFK)
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
	const commitFKOfThisFile = 3
	const troublesomeDigest = "f83371a093397e6a86fc117b583b1533"
	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(fileLocation)
	require.NoError(t, err)

	assert.NoError(t, p.Process(context.Background(), fsResult))
	assertTraceCountMetricIs(t, p, 4796)

	// Spot check that the troublesome data point made it in
	tracePK := assertTraceIsStored(t, db, map[string]string{
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
	optionsPK := assertOptionsAreStored(t, db, map[string]string{
		"ext":         "png",
		"gamut":       "untagged",
		"transfer_fn": "untagged",
		"color_type":  "BGRA_8888",
		"alpha_type":  "Premul",
		"color_depth": "8888",
	})
	groupingPK := assertGroupingIsStored(t, db, map[string]string{
		types.CorpusField:     "colorImage",
		types.PrimaryKeyField: "mandrill.wbmp",
	})
	assert.Equal(t, "804485e766fe007e258573df00a79437", hex.EncodeToString(tracePK))

	filePK := assertSourceFileIsIngested(t, db, fileLocation, fakeNow)
	assertDigestOnTraceAtCommit(t, db, troublesomeDigest, tracePK, commitFKOfThisFile)
	assertOptionsGroupingSourceFileOnTraceAtCommit(t, db, optionsPK, groupingPK, filePK, tracePK, commitFKOfThisFile)
	assertExpectationsStored(t, db, groupingPK, troublesomeDigest, expectations.Untriaged)

	// Spot check these values (chosen somewhat arbitrarily)
	assertPrimaryBranchParamsSubsetIsOnCommit(t, db, []keyValue{
		{key: types.CorpusField, value: "colorImage"},
		{key: types.PrimaryKeyField, value: "mandrill.wbmp"},
		{key: types.PrimaryKeyField, value: "baseline_restart_jpeg.jpg_0.125"},
		{key: "ext", value: "png"},
		{key: "color_type", value: "BGRA_8888"},
	}, commitFKOfThisFile)

	assertCommitHasData(t, db, commitFKOfThisFile)
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
	statement := "INSERT INTO Commits (commit_id, git_hash, commit_time, author, subject) VALUES "
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
	s.keyValueCache.Purge()
	s.traceCounter.Reset()
}

func assertTraceCountMetricIs(t *testing.T, p ingestion.Processor, n int64) {
	s := p.(*sqlProcessor)

	assert.Equal(t, n, s.traceCounter.Get())
}

func storeTrace(t *testing.T, db *pgxpool.Pool, m map[string]string, mostRecentCommitFK int) []byte {
	return storeKeyValueMap(t, db, `
INSERT INTO Traces (trace_id, keys, most_recent_commit_id) VALUES ($1, $2, $3)`, m, mostRecentCommitFK)
}

func storeOptions(t *testing.T, db *pgxpool.Pool, m map[string]string) []byte {
	return storeKeyValueMap(t, db, `INSERT INTO Options (options_id, keys) VALUES ($1, $2)`, m)
}

func storeGrouping(t *testing.T, db *pgxpool.Pool, m map[string]string) []byte {
	return storeKeyValueMap(t, db, `INSERT INTO Groupings (grouping_id, keys) VALUES ($1, $2)`, m)
}

// storeKeyValueMap stores the given map in the appropriate table and returns the primary key
// (i.e. the md5 hash of the map). If the map is already stored, this will fail (to alert test
// Author that preconditions may not have met expectations).
func storeKeyValueMap(t *testing.T, db *pgxpool.Pool, statement string, m map[string]string, extra ...interface{}) []byte {
	mJSON, keysHashPK, err := sql.SerializeMap(m)
	require.NoError(t, err)

	args := []interface{}{keysHashPK, mJSON}
	args = append(args, extra...)

	_, err = db.Exec(context.Background(), statement, args...)
	require.NoError(t, err)
	return keysHashPK
}

// assertTraceIsStored asserts the given map is in the Traces table and returns
// the primary key (i.e. the md5 hash of the keys).
func assertTraceIsStored(t *testing.T, db *pgxpool.Pool, m map[string]string) []byte {
	return assertKeyAndHashAreStored(t, db, `SELECT trace_id FROM Traces WHERE keys = $1;`, m)
}

// assertOptionsAreStored asserts the given map is in the Options table and returns
// the primary key (i.e. the md5 hash of the keys).
func assertOptionsAreStored(t *testing.T, db *pgxpool.Pool, m map[string]string) []byte {
	return assertKeyAndHashAreStored(t, db, `SELECT options_id FROM Options WHERE keys = $1;`, m)
}

// assertGroupingIsStored asserts the given map is in the Grouping table and returns
// the primary key (i.e. the md5 hash of the keys).
func assertGroupingIsStored(t *testing.T, db *pgxpool.Pool, m map[string]string) []byte {
	return assertKeyAndHashAreStored(t, db, `SELECT grouping_id FROM Groupings WHERE keys = $1;`, m)
}

// assertKeyAndHashAreStored asserts the given map is in the appropriate table and returns
// the primary key (i.e. the md5 hash of the map).
func assertKeyAndHashAreStored(t *testing.T, db *pgxpool.Pool, statement string, m map[string]string) []byte {
	mJSON, expectedHash, err := sql.SerializeMap(m)
	require.NoError(t, err)

	row := db.QueryRow(context.Background(), statement, mJSON)

	var actualPK []byte
	require.NoError(t, row.Scan(&actualPK), "Expected one row where keys = %s", mJSON)
	assert.Equal(t, expectedHash, actualPK)
	return actualPK
}

func assertTraceHasNullIgnoreStatus(t *testing.T, db *pgxpool.Pool, tracePK []byte) {
	row := db.QueryRow(context.Background(), `
SELECT COUNT(*) FROM Traces where trace_id = $1 AND matches_any_ignore_rule IS NULL`, tracePK)

	var count int
	require.NoError(t, row.Scan(&count), "Expected one row where trace_id = %v", tracePK)
	assert.Equal(t, 1, count)
}

func assertTraceHasMostRecentCommit(t *testing.T, db *pgxpool.Pool, tracePK []byte, expectedCommitFK int) {
	row := db.QueryRow(context.Background(), `
SELECT most_recent_commit_id FROM Traces where trace_id = $1`, tracePK)

	var actualCommitFK int
	require.NoError(t, row.Scan(&actualCommitFK), "Expected one row where trace_id = %v", tracePK)
	assert.Equal(t, expectedCommitFK, actualCommitFK, "incorrect for trace %x", tracePK)
}

// storeSourceFile stores the given source file into the SourceFiles table and returns the primary
// key (i.e. the md5 hash of the file path). If the source file is already stored, this will fail
// (to alert test Author that preconditions may not have met expectations).
func storeSourceFile(t *testing.T, db *pgxpool.Pool, filePath string, ingestedTime time.Time) []byte {
	sourceFilePK := md5.Sum([]byte(filePath))
	_, err := db.Exec(context.Background(),
		`INSERT INTO SourceFiles (source_file_id, source_file, last_ingested) VALUES ($1, $2, $3)`,
		sourceFilePK[:], filePath, ingestedTime)
	require.NoError(t, err)
	return sourceFilePK[:]
}

// assertSourceFileIsIngested asserts the given source file was stored in the SourceFiles table
// and returns the primary key (i.e. the md5 hash of the file path)
func assertSourceFileIsIngested(t *testing.T, db *pgxpool.Pool, filePath string, expectedTime time.Time) []byte {
	row := db.QueryRow(context.Background(),
		`SELECT source_file_id, last_ingested FROM SourceFiles WHERE source_file = $1;`, filePath)

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
func storeTraceValue(t *testing.T, db *pgxpool.Pool, digest types.Digest, traceHashFK []byte, commitFK int, optionsFK, groupingFK, sourceFileFK []byte) {
	digestBytes, err := sql.DigestToBytes(digest)
	require.NoError(t, err)

	_, err = db.Exec(context.Background(), `
INSERT INTO TraceValues (trace_id, shard, commit_id, grouping_id, digest, 
  options_id, source_file_id) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		traceHashFK, sql.ComputeTraceValueShard(traceHashFK), commitFK, groupingFK, digestBytes,
		optionsFK, sourceFileFK)
	require.NoError(t, err)
}

func assertDigestOnTraceAtCommit(t *testing.T, db *pgxpool.Pool, digest types.Digest, traceHashFK []byte, commitFK int) {
	row := db.QueryRow(context.Background(),
		`SELECT encode(digest, 'hex') FROM TraceValues WHERE trace_id = $1 AND commit_id = $2;`,
		traceHashFK, commitFK)

	var storedDigest types.Digest
	err := row.Scan(&storedDigest)
	require.NoError(t, err, "expected one row where trace_id = %v and commit_id = %d", traceHashFK, commitFK)

	assert.Equal(t, digest, storedDigest, "Incorrect digest where trace_id = %v and commit_id = %d", traceHashFK, commitFK)
}

func assertOptionsGroupingSourceFileOnTraceAtCommit(t *testing.T, db *pgxpool.Pool, optionsFK, groupingFK, sourceFileFK, traceFK []byte, commitFK int) {
	row := db.QueryRow(context.Background(),
		`SELECT options_id, grouping_id, source_file_id FROM TraceValues WHERE trace_id = $1 AND commit_id = $2;`,
		traceFK, commitFK)

	var actualOptionsFK []byte
	var actualGroupingFK []byte
	var actualSourceFileFK []byte
	err := row.Scan(&actualOptionsFK, &actualGroupingFK, &actualSourceFileFK)
	require.NoError(t, err, "expected one row where trace_id = %v and commit_id = %d", traceFK, commitFK)

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
		`INSERT INTO Expectations (grouping_id, digest, label) VALUES ($1, $2, $3);`,
		groupingFK, digestBytes, labelInt)
	require.NoError(t, err)
}

func assertExpectationsStored(t *testing.T, db *pgxpool.Pool, groupingFK []byte, digest types.Digest, expectedLabel expectations.Label) {
	digestBytes, err := sql.DigestToBytes(digest)
	require.NoError(t, err)

	row := db.QueryRow(context.Background(),
		`SELECT label FROM Expectations WHERE grouping_id = $1 AND digest = $2;`,
		groupingFK, digestBytes)

	var storedLabel sql.ExpectationsLabel
	err = row.Scan(&storedLabel)
	require.NoError(t, err, "Expected one row where grouping_id = %v and digest = %v", groupingFK, digestBytes)

	assert.Equal(t, sql.ConvertLabelFromString(expectedLabel), storedLabel)
}

type keyValue struct {
	key   string
	value string
}

func storePrimaryBranchParams(t *testing.T, db *pgxpool.Pool, commitFK int, toStore []keyValue) {
	statement := `INSERT INTO PrimaryBranchParams (key, value, commit_id) VALUES `
	const valuesPerRow = 3
	vp, err := sql.ValuesPlaceholders(valuesPerRow, len(toStore))
	require.NoError(t, err)
	statement += vp
	arguments := make([]interface{}, 0, valuesPerRow*len(toStore))
	for _, kv := range toStore {
		arguments = append(arguments, kv.key, kv.value, commitFK)
	}
	_, err = db.Exec(context.Background(), statement, arguments...)
	require.NoError(t, err, "running statement %s", statement)
}

func assertPrimaryBranchParamsAreOnCommit(t *testing.T, db *pgxpool.Pool, expected []keyValue, commitFK int) {
	rows, err := db.Query(context.Background(), `
SELECT key, value FROM PrimaryBranchParams WHERE commit_id = $1`, commitFK)
	require.NoError(t, err)
	defer rows.Close()

	var results []keyValue
	for rows.Next() {
		kv := keyValue{}
		err := rows.Scan(&kv.key, &kv.value)
		require.NoError(t, err)
		results = append(results, kv)
	}
	assert.ElementsMatch(t, expected, results)
}

func assertPrimaryBranchParamsSubsetIsOnCommit(t *testing.T, db *pgxpool.Pool, values []keyValue, commitFK int) {
	for _, kv := range values {
		row := db.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM PrimaryBranchParams WHERE key = $1 AND value = $2 AND commit_id = $3`,
			kv.key, kv.value, commitFK)
		count := 0
		require.NoError(t, row.Scan(&count), "finding %q: %q on commit %d", kv.key, kv.value, commitFK)
		assert.Equal(t, 1, count, "expected exactly one row of %q: %q on commit %d", kv.key, kv.value, commitFK)
	}
}

func storeCommitHasData(t *testing.T, db *pgxpool.Pool, commitPK int) {
	_, err := db.Exec(context.Background(), `UPDATE Commits SET has_data = true WHERE commit_id = $1`, commitPK)
	require.NoError(t, err)
}

func assertCommitHasData(t *testing.T, db *pgxpool.Pool, commitPK int) {
	row := db.QueryRow(context.Background(), `SELECT COUNT(*) FROM commits WHERE commit_id = $1 AND has_data = true;`, commitPK)

	count := -1
	err := row.Scan(&count)
	require.NoError(t, err, "Expected one row where commit_id = %d AND has_data = true", commitPK)
	assert.Equal(t, 1, count, "Expected one row where commit_id = %d AND has_data = true", commitPK)
}
