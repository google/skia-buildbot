package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"go.skia.org/infra/go/gcs"

	"go.skia.org/infra/go/testutils"

	"go.skia.org/infra/go/gcs/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/now"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
)

func TestGatherFromPrimaryBranch_NoExistingWork_AllWorkAdded(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()
	g := diffWorkGatherer{
		windowSize: 100,
		db:         db,
	}
	require.NoError(t, g.gatherFromPrimaryBranch(ctx))
	actualWork := sqltest.GetAllRows(ctx, t, db, "PrimaryBranchDiffCalculationWork", &schema.PrimaryBranchDiffCalculationRow{})
	assert.ElementsMatch(t, []schema.PrimaryBranchDiffCalculationRow{
		{
			GroupingID:           dks.SquareGroupingID,
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
		},
		{
			GroupingID:           dks.TriangleGroupingID,
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
		},
		{
			GroupingID:           dks.CircleGroupingID,
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
		},
	}, actualWork)
}

func TestGatherFromPrimaryBranch_SomeExistingWork_AllWorkAdded(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	sentinelTime := ts("2021-02-02T02:15:00Z")
	existingData := dks.Build()
	existingData.PrimaryBranchDiffCalculationWork = []schema.PrimaryBranchDiffCalculationRow{
		{
			GroupingID:           dks.SquareGroupingID,
			LastCalculated:       sentinelTime,
			CalculationLeaseEnds: sentinelTime,
		},
		{
			GroupingID:           dks.TriangleGroupingID,
			LastCalculated:       sentinelTime,
			CalculationLeaseEnds: sentinelTime,
		},
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()

	g := diffWorkGatherer{
		windowSize: 100,
		db:         db,
	}
	require.NoError(t, g.gatherFromPrimaryBranch(ctx))
	actualWork := sqltest.GetAllRows(ctx, t, db, "PrimaryBranchDiffCalculationWork", &schema.PrimaryBranchDiffCalculationRow{})
	assert.ElementsMatch(t, []schema.PrimaryBranchDiffCalculationRow{
		{
			GroupingID:           dks.SquareGroupingID,
			LastCalculated:       sentinelTime,
			CalculationLeaseEnds: sentinelTime,
		},
		{
			GroupingID:           dks.TriangleGroupingID,
			LastCalculated:       sentinelTime,
			CalculationLeaseEnds: sentinelTime,
		},
		{
			GroupingID:           dks.CircleGroupingID,
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
		},
	}, actualWork)
}

func TestGatherFromPrimaryBranch_NoNewWork_NothingChanged(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	sentinelTime := ts("2021-02-02T02:15:00Z")
	existingData := dks.Build()
	existingData.PrimaryBranchDiffCalculationWork = []schema.PrimaryBranchDiffCalculationRow{
		{
			GroupingID:           dks.SquareGroupingID,
			LastCalculated:       sentinelTime,
			CalculationLeaseEnds: sentinelTime,
		},
		{
			GroupingID:           dks.TriangleGroupingID,
			LastCalculated:       sentinelTime,
			CalculationLeaseEnds: sentinelTime,
		},
		{
			GroupingID:           dks.CircleGroupingID,
			LastCalculated:       sentinelTime,
			CalculationLeaseEnds: sentinelTime,
		},
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()

	g := diffWorkGatherer{
		windowSize: 100,
		db:         db,
	}
	require.NoError(t, g.gatherFromPrimaryBranch(ctx))
	actualWork := sqltest.GetAllRows(ctx, t, db, "PrimaryBranchDiffCalculationWork", &schema.PrimaryBranchDiffCalculationRow{})
	assert.ElementsMatch(t, []schema.PrimaryBranchDiffCalculationRow{
		{
			GroupingID:           dks.SquareGroupingID,
			LastCalculated:       sentinelTime,
			CalculationLeaseEnds: sentinelTime,
		},
		{
			GroupingID:           dks.TriangleGroupingID,
			LastCalculated:       sentinelTime,
			CalculationLeaseEnds: sentinelTime,
		},
		{
			GroupingID:           dks.CircleGroupingID,
			LastCalculated:       sentinelTime,
			CalculationLeaseEnds: sentinelTime,
		},
	}, actualWork)
}

func TestGatherFromChangelists_OnlyReportsGroupingsWithDataNotOnPrimaryBranch(t *testing.T) {
	fakeNow := ts("2020-12-13T00:00:00Z")
	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)

	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	g := diffWorkGatherer{
		windowSize:       100,
		db:               db,
		mostRecentCLScan: time.Time{}, // Setting this at time.Zero will get us data from all CLS
	}
	require.NoError(t, g.gatherFromChangelists(ctx))

	actualWork := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchDiffCalculationWork", &schema.SecondaryBranchDiffCalculationRow{})
	assert.ElementsMatch(t, []schema.SecondaryBranchDiffCalculationRow{
		{
			BranchName:           "gerrit_CL_fix_ios",
			GroupingID:           dks.CircleGroupingID,
			LastUpdated:          ts("2020-12-10T04:05:06Z"),
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
			DigestsNotOnPrimary: []types.Digest{
				dks.DigestC06Pos_CL, dks.DigestC07Unt_CL,
			},
		},
		{
			BranchName:           "gerrit-internal_CL_new_tests",
			GroupingID:           dks.RoundRectGroupingID,
			LastUpdated:          ts("2020-12-12T09:20:33Z"),
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
			DigestsNotOnPrimary: []types.Digest{
				dks.DigestE01Pos_CL, dks.DigestE02Pos_CL, dks.DigestE03Unt_CL,
			},
		},
		{
			BranchName:           "gerrit-internal_CL_new_tests",
			GroupingID:           dks.TextSevenGroupingID,
			LastUpdated:          ts("2020-12-12T09:20:33Z"),
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
			DigestsNotOnPrimary: []types.Digest{
				dks.DigestBlank, dks.DigestD01Pos_CL,
			},
		},
		{
			BranchName:           "gerrit_CLmultipledatapoints",
			GroupingID:           dks.SquareGroupingID,
			LastUpdated:          ts("2020-12-12T14:00:00Z"),
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
			DigestsNotOnPrimary: []types.Digest{
				dks.DigestC01Pos, dks.DigestC03Unt, dks.DigestC04Unt,
			},
		},
	}, actualWork)
	assert.Equal(t, fakeNow, g.mostRecentCLScan)
}

func TestGatherFromChangelists_UpdatesExistingWork(t *testing.T) {
	fakeNow := ts("2020-12-12T13:13:13Z")
	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)

	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := dks.Build()
	sentinelTime := ts("2020-05-25T00:00:00Z")
	existingData.SecondaryBranchDiffCalculationWork = []schema.SecondaryBranchDiffCalculationRow{
		{
			BranchName:           "gerrit-internal_CL_new_tests",
			GroupingID:           dks.TextSevenGroupingID,
			DigestsNotOnPrimary:  []types.Digest{dks.DigestBlank},
			LastUpdated:          sentinelTime,
			LastCalculated:       sentinelTime,
			CalculationLeaseEnds: sentinelTime,
		},
		{
			BranchName:           "gerrit_CL_fix_ios",
			GroupingID:           dks.CircleGroupingID,
			DigestsNotOnPrimary:  []types.Digest{dks.DigestBlank},
			LastUpdated:          sentinelTime,
			LastCalculated:       sentinelTime,
			CalculationLeaseEnds: sentinelTime,
		},
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	g := diffWorkGatherer{
		windowSize:       100,
		db:               db,
		mostRecentCLScan: time.Time{}, // Setting this at time.Zero will get us data from all CLS
	}
	require.NoError(t, g.gatherFromChangelists(ctx))

	actualWork := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchDiffCalculationWork", &schema.SecondaryBranchDiffCalculationRow{})
	assert.ElementsMatch(t, []schema.SecondaryBranchDiffCalculationRow{
		{
			BranchName:           "gerrit_CL_fix_ios",
			GroupingID:           dks.CircleGroupingID,
			LastUpdated:          ts("2020-12-10T04:05:06Z"),
			LastCalculated:       sentinelTime, // not changed
			CalculationLeaseEnds: sentinelTime, // not changed
			DigestsNotOnPrimary: []types.Digest{
				dks.DigestC06Pos_CL, dks.DigestC07Unt_CL,
			},
		},
		{
			BranchName:           "gerrit-internal_CL_new_tests",
			GroupingID:           dks.RoundRectGroupingID,
			LastUpdated:          ts("2020-12-12T09:20:33Z"),
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
			DigestsNotOnPrimary: []types.Digest{
				dks.DigestE01Pos_CL, dks.DigestE02Pos_CL, dks.DigestE03Unt_CL,
			},
		},
		{
			BranchName:           "gerrit-internal_CL_new_tests",
			GroupingID:           dks.TextSevenGroupingID,
			LastUpdated:          ts("2020-12-12T09:20:33Z"),
			LastCalculated:       sentinelTime, // not changed
			CalculationLeaseEnds: sentinelTime, // not changed
			DigestsNotOnPrimary: []types.Digest{
				dks.DigestBlank, dks.DigestD01Pos_CL,
			},
		},
		{
			BranchName:           "gerrit_CLmultipledatapoints",
			GroupingID:           dks.SquareGroupingID,
			LastUpdated:          ts("2020-12-12T14:00:00Z"),
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
			DigestsNotOnPrimary: []types.Digest{
				dks.DigestC01Pos, dks.DigestC03Unt, dks.DigestC04Unt,
			},
		},
	}, actualWork)
	assert.Equal(t, fakeNow, g.mostRecentCLScan)
}

func TestGatherFromChangelists_DeletesOldWork(t *testing.T) {
	fakeNow := ts("2021-07-07T07:07:07Z")
	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)

	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := dks.Build()
	existingData.SecondaryBranchDiffCalculationWork = []schema.SecondaryBranchDiffCalculationRow{
		{
			BranchName:           "new_cl",
			GroupingID:           dks.TextSevenGroupingID,
			DigestsNotOnPrimary:  []types.Digest{dks.DigestBlank},
			LastUpdated:          ts("2021-07-05T00:00:00Z"), // 2 days ago
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
		},
		{
			BranchName:           "old_cl",
			GroupingID:           dks.CircleGroupingID,
			DigestsNotOnPrimary:  []types.Digest{dks.DigestBlank},
			LastUpdated:          ts("2021-07-01T00:00:00Z"), // 6 days ago,
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
		},
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	g := diffWorkGatherer{
		windowSize:       100,
		db:               db,
		mostRecentCLScan: ts("2021-07-07T00:00:00Z"),
	}
	require.NoError(t, g.gatherFromChangelists(ctx))

	actualWork := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchDiffCalculationWork", &schema.SecondaryBranchDiffCalculationRow{})
	assert.Equal(t, []schema.SecondaryBranchDiffCalculationRow{
		{
			BranchName:           "new_cl", // This should still be around
			GroupingID:           dks.TextSevenGroupingID,
			DigestsNotOnPrimary:  []types.Digest{dks.DigestBlank},
			LastUpdated:          ts("2021-07-05T00:00:00Z"),
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
		},
	}, actualWork)
	assert.Equal(t, fakeNow, g.mostRecentCLScan)
}

func TestGetAllRecentDigests_ReturnsAllRecentDigestsFromPrimaryBranch(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	digests, err := getAllRecentDigests(ctx, db, 100)
	require.NoError(t, err)
	assert.Equal(t, []types.Digest{
		dks.DigestBlank, dks.DigestA01Pos, dks.DigestA02Pos, dks.DigestA03Pos, dks.DigestA04Unt,
		dks.DigestA05Unt, dks.DigestA06Unt, dks.DigestA07Pos, dks.DigestA08Pos, dks.DigestA09Neg,
		dks.DigestB01Pos, dks.DigestB02Pos, dks.DigestB03Neg, dks.DigestB04Neg,
		dks.DigestC01Pos, dks.DigestC02Pos, dks.DigestC03Unt, dks.DigestC04Unt, dks.DigestC05Unt,
	}, digests)
}

func TestGetTuplesOfKeysToQuery_MultipleKeysAndCorpora_Success(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	keys := []string{dks.OSKey, dks.DeviceKey}
	corpora := []string{dks.CornersCorpus, dks.RoundCorpus}
	tuples, err := getTuplesOfKeysToQuery(ctx, db, keys, nil, corpora)
	require.NoError(t, err)
	assert.Equal(t, []summaryTuple{{
		Corpus: dks.CornersCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.AndroidOS},
			{Key: dks.DeviceKey, Value: dks.TaimenDevice},
		},
	}, {
		Corpus: dks.CornersCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.AndroidOS},
			{Key: dks.DeviceKey, Value: dks.WalleyeDevice},
		},
	}, {
		Corpus: dks.CornersCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.Windows10dot2OS},
			{Key: dks.DeviceKey, Value: dks.QuadroDevice},
		},
	}, {
		Corpus: dks.CornersCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.Windows10dot3OS},
			{Key: dks.DeviceKey, Value: dks.QuadroDevice},
		},
	}, {
		Corpus: dks.CornersCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.IOS},
			{Key: dks.DeviceKey, Value: dks.IPadDevice},
		},
	}, {
		Corpus: dks.CornersCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.IOS},
			{Key: dks.DeviceKey, Value: dks.IPhoneDevice},
		},
	}, {
		Corpus: dks.RoundCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.AndroidOS},
			{Key: dks.DeviceKey, Value: dks.TaimenDevice},
		},
	}, {
		Corpus: dks.RoundCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.AndroidOS},
			{Key: dks.DeviceKey, Value: dks.WalleyeDevice},
		},
	}, {
		Corpus: dks.RoundCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.Windows10dot2OS},
			{Key: dks.DeviceKey, Value: dks.QuadroDevice},
		},
	}, {
		Corpus: dks.RoundCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.Windows10dot3OS},
			{Key: dks.DeviceKey, Value: dks.QuadroDevice},
		},
	}, {
		Corpus: dks.RoundCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.IOS},
			{Key: dks.DeviceKey, Value: dks.IPadDevice},
		},
	}, {
		Corpus: dks.RoundCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.IOS},
			{Key: dks.DeviceKey, Value: dks.IPhoneDevice},
		},
	}}, tuples)
}

func TestGetTuplesOfKeysToQuery_KeysAreSometimesNull_TracesWithNullValuesNotReturned(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	data := dks.Build()
	data.Traces = append(data.Traces, schema.TraceRow{
		TraceID:    []byte("abcdef"),
		Corpus:     dks.RoundCorpus,
		GroupingID: []byte("123456"),
		Keys: map[string]string{
			types.CorpusField: dks.RoundCorpus,
			dks.OSKey:         "os set, but not device",
		},
		MatchesAnyIgnoreRule: schema.NBFalse,
	})
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, data))
	waitForSystemTime()

	keys := []string{dks.OSKey, dks.DeviceKey}
	corpora := []string{dks.RoundCorpus}
	tuples, err := getTuplesOfKeysToQuery(ctx, db, keys, nil, corpora)
	require.NoError(t, err)
	assert.Equal(t, []summaryTuple{{
		Corpus: dks.RoundCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.AndroidOS},
			{Key: dks.DeviceKey, Value: dks.TaimenDevice},
		},
	}, {
		Corpus: dks.RoundCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.AndroidOS},
			{Key: dks.DeviceKey, Value: dks.WalleyeDevice},
		},
	}, {
		Corpus: dks.RoundCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.Windows10dot2OS},
			{Key: dks.DeviceKey, Value: dks.QuadroDevice},
		},
	}, {
		Corpus: dks.RoundCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.Windows10dot3OS},
			{Key: dks.DeviceKey, Value: dks.QuadroDevice},
		},
	}, {
		Corpus: dks.RoundCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.IOS},
			{Key: dks.DeviceKey, Value: dks.IPadDevice},
		},
	}, {
		Corpus: dks.RoundCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.IOS},
			{Key: dks.DeviceKey, Value: dks.IPhoneDevice},
		},
	}}, tuples)
}

func TestGetTuplesOfKeysToQuery_IgnoredValuesCauseTracesToBeSkipped(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	keys := []string{dks.OSKey, dks.DeviceKey}
	// An example use-case is that these are old bits of hardware which we have not tested
	// on in a while and so can skip as an optimization.
	ignores := []string{dks.Windows10dot2OS, dks.WalleyeDevice, dks.IPadDevice}
	corpora := []string{dks.RoundCorpus, dks.CornersCorpus}
	tuples, err := getTuplesOfKeysToQuery(ctx, db, keys, ignores, corpora)
	require.NoError(t, err)
	assert.Equal(t, []summaryTuple{{
		Corpus: dks.CornersCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.AndroidOS},
			{Key: dks.DeviceKey, Value: dks.TaimenDevice},
		},
	}, {
		Corpus: dks.CornersCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.Windows10dot3OS},
			{Key: dks.DeviceKey, Value: dks.QuadroDevice},
		},
	}, {
		Corpus: dks.CornersCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.IOS},
			{Key: dks.DeviceKey, Value: dks.IPhoneDevice},
		},
	}, {
		Corpus: dks.RoundCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.AndroidOS},
			{Key: dks.DeviceKey, Value: dks.TaimenDevice},
		},
	}, {
		Corpus: dks.RoundCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.Windows10dot3OS},
			{Key: dks.DeviceKey, Value: dks.QuadroDevice},
		},
	}, {
		Corpus: dks.RoundCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.IOS},
			{Key: dks.DeviceKey, Value: dks.IPhoneDevice},
		},
	}}, tuples)
}

func TestGetOldestCommitInWindow_Success(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	oldest, newest, err := getWindowCommitBounds(ctx, db, 100)
	require.NoError(t, err)
	assert.Equal(t, dks.OldestCommitID, oldest)
	assert.Equal(t, dks.MostRecentCommitID, newest)

	oldest, newest, err = getWindowCommitBounds(ctx, db, 3)
	require.NoError(t, err)
	assert.Equal(t, dks.IOSFixTriangleTestsBreakCircleTestsCommitID, oldest)
	assert.Equal(t, dks.MostRecentCommitID, newest)
}

func TestGetTriageStatus_FillsInPositiveNegativeUntriagedAndCommitID(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	test := func(name string, tuple summaryTuple, expectedData summaryData) {
		t.Run(name, func(t *testing.T) {
			data, err := getTriageStatus(ctx, db, tuple, dks.OldestCommitID)
			require.NoError(t, err)
			assert.Equal(t, expectedData, data)
		})
	}

	test("all positive", summaryTuple{
		Corpus: dks.CornersCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.AndroidOS},
			{Key: dks.DeviceKey, Value: dks.WalleyeDevice},
		},
	}, summaryData{
		PositiveTraces:  4,
		NegativeTraces:  0,
		UntriagedTraces: 0,
		CommitID:        dks.MostRecentCommitID,
	})

	test("Data not at ToT", summaryTuple{
		Corpus: dks.RoundCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.IOS},
			{Key: dks.DeviceKey, Value: dks.IPhoneDevice},
		},
	}, summaryData{
		PositiveTraces:  0,
		NegativeTraces:  0,
		UntriagedTraces: 2,
		// One trace is at this commit and another is one commit older. For the purposes of
		// reporting data to Perf, we assume all data is the most recent commit for which this
		// set of keys has *any* data, effectively "rounding up" the age of older data points.
		CommitID: "0000000109",
	})

	test("some ignored traces", summaryTuple{
		Corpus: dks.CornersCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.AndroidOS},
			{Key: dks.DeviceKey, Value: dks.TaimenDevice},
		},
	}, summaryData{
		PositiveTraces: 1,
		// This set of keys does have a negative and untriaged digest produced, but those are for
		// ignored traces, so they are not counted here.
		NegativeTraces:  0,
		UntriagedTraces: 0,
		CommitID:        dks.MostRecentCommitID,
	})
}

func TestGetTriageStatus_RespectsPassedInCommitID(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	test := func(name string, tuple summaryTuple, commitID schema.CommitID, expectedData summaryData) {
		t.Run(name, func(t *testing.T) {
			data, err := getTriageStatus(ctx, db, tuple, commitID)
			require.NoError(t, err)
			assert.Equal(t, expectedData, data)
		})
	}

	test("all data", summaryTuple{
		Corpus: dks.CornersCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.Windows10dot2OS},
			{Key: dks.DeviceKey, Value: dks.QuadroDevice},
		},
	}, dks.OldestCommitID,
		summaryData{
			PositiveTraces:  4,
			NegativeTraces:  0,
			UntriagedTraces: 0,
			CommitID:        "0000000100",
		})

	// These traces don't produce any data after commit id 100, so we should get no counts
	// and nothing set for CommitID.
	test("too old", summaryTuple{
		Corpus: dks.CornersCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.Windows10dot2OS},
			{Key: dks.DeviceKey, Value: dks.QuadroDevice},
		},
	}, "0000000101",
		summaryData{
			PositiveTraces:  0,
			NegativeTraces:  0,
			UntriagedTraces: 0,
			CommitID:        "",
		})
}

func TestGetIgnoredCount_ReturnsCorrectAnswer(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	test := func(name string, tuple summaryTuple, expectedCount int) {
		t.Run(name, func(t *testing.T) {
			count, err := getIgnoredCount(ctx, db, tuple, dks.OldestCommitID)
			require.NoError(t, err)
			assert.Equal(t, expectedCount, count)
		})
	}

	test("no ignored traces", summaryTuple{
		Corpus: dks.CornersCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.AndroidOS},
			{Key: dks.DeviceKey, Value: dks.WalleyeDevice},
		},
	}, 0)

	test("some ignored traces", summaryTuple{
		Corpus: dks.CornersCorpus,
		KeyValues: []pair{
			{Key: dks.OSKey, Value: dks.AndroidOS},
			{Key: dks.DeviceKey, Value: dks.TaimenDevice},
		},
	}, 1)
}

func TestUploadDataToPerf_DataComplete_CorrectJSONUploaded(t *testing.T) {
	mockTime := time.Date(2022, time.October, 9, 8, 7, 6, 54321, time.UTC)
	ctx := now.TimeTravelingContext(mockTime)

	mc := mocks.NewGCSClient(t)
	expectedOptions := gcs.FileWriteOptions{
		ContentType: "application/json",
	}
	const expectedJSON = `{
	"version": 1,
	"git_hash": "11223344556677889900aabbccddeeff",
	"key": {
		"config": "gles",
		"cpu_or_gpu_value": "AppleA13",
		"model": "iPhone11",
		"os": "iOS",
		"source_type": "gm"
	},
	"results": [
		{
			"key": {
				"count": "gold_triaged_positive",
				"unit": "traces"
			},
			"measurement": 4
		},
		{
			"key": {
				"count": "gold_triaged_negative",
				"unit": "traces"
			},
			"measurement": 3
		},
		{
			"key": {
				"count": "gold_untriaged",
				"unit": "traces"
			},
			"measurement": 2
		},
		{
			"key": {
				"count": "gold_ignored",
				"unit": "traces"
			},
			"measurement": 1
		}
	]
}`
	mc.On("SetFileContents", testutils.AnyContext, "gold-summary-v1/2022/10/9/8/gm-iOS-iPhone11-AppleA13-gles/1665302826000054321.json",
		expectedOptions, mock.Anything).Run(func(args mock.Arguments) {
		actualBytes := args.Get(3).([]byte)
		assert.Equal(t, expectedJSON, string(actualBytes))
	}).Return(nil)

	tuple := summaryTuple{
		Corpus: "gm",
		KeyValues: []pair{
			{Key: "os", Value: "iOS"},
			{Key: "model", Value: "iPhone11"},
			{Key: "cpu_or_gpu_value", Value: "AppleA13"},
			{Key: "config", Value: "gles"},
		},
	}
	data := summaryData{
		PositiveTraces:  4,
		NegativeTraces:  3,
		UntriagedTraces: 2,
		IgnoredTraces:   1,
		CommitID:        "Should be ignored",
		GitHash:         "11223344556677889900aabbccddeeff",
	}
	require.NoError(t, uploadDataToPerf(ctx, tuple, data, mc))
}

func TestSummarizeTraces_Success(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	mockTime := time.Date(2022, time.October, 10, 10, 10, 10, 10101010, time.UTC)
	ctx = now.TimeTravelingContext(mockTime)

	uploadedFiles := map[string]string{}
	gcsSpy := mocks.NewGCSClient(t)
	gcsSpy.On("SetFileContents", testutils.AnyContext, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		actualName := args.String(1)
		actualBytes := args.Get(3).([]byte)
		uploadedFiles[actualName] = string(actualBytes)
	}).Return(nil)

	cfg := perfSummariesConfig{
		AgeOutCommits:      100,
		CorporaToSummarize: []string{dks.CornersCorpus},
		KeysToSummarize:    []string{dks.DeviceKey, dks.OSKey},
		ValuesToIgnore:     []string{dks.Windows10dot2OS},
	}
	err := summarizeTraces(ctx, db, &cfg, gcsSpy)
	require.NoError(t, err)

	// Verify the correct files got uploaded.
	var actualFilesCreated []string
	for f := range uploadedFiles {
		actualFilesCreated = append(actualFilesCreated, f)
	}
	expectedFiles := []string{
		"gold-summary-v1/2022/10/10/10/corners-QuadroP400-Windows10.3/1665396610010101010.json",
		"gold-summary-v1/2022/10/10/10/corners-iPad6,3-iOS/1665396610010101010.json",
		"gold-summary-v1/2022/10/10/10/corners-iPhone12,1-iOS/1665396610010101010.json",
		"gold-summary-v1/2022/10/10/10/corners-taimen-Android/1665396610010101010.json",
		"gold-summary-v1/2022/10/10/10/corners-walleye-Android/1665396610010101010.json",
	}
	assert.ElementsMatch(t, expectedFiles, actualFilesCreated)

	// Spot check one of the uploaded files for correct data.
	// The omitted measurements *will* be interpreted by Perf as 0.
	assert.Equal(t, `{
	"version": 1,
	"git_hash": "f4412901bfb130a8774c0c719450d1450845f471",
	"key": {
		"device": "taimen",
		"os": "Android",
		"source_type": "corners"
	},
	"results": [
		{
			"key": {
				"count": "gold_triaged_positive",
				"unit": "traces"
			},
			"measurement": 1
		},
		{
			"key": {
				"count": "gold_triaged_negative",
				"unit": "traces"
			}
		},
		{
			"key": {
				"count": "gold_untriaged",
				"unit": "traces"
			}
		},
		{
			"key": {
				"count": "gold_ignored",
				"unit": "traces"
			},
			"measurement": 1
		}
	]
}`, uploadedFiles["gold-summary-v1/2022/10/10/10/corners-taimen-Android/1665396610010101010.json"])
}

var beginningOfTime = ts("1970-01-01T00:00:00Z")

func ts(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func waitForSystemTime() {
	time.Sleep(150 * time.Millisecond)
}
