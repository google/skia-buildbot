package main

import (
	"context"
	"crypto/md5"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/testutils/unittest"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
)

func TestGatherFromPrimaryBranch_NoExistingWork_AllWorkAdded(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	g := diffWorkGatherer{
		windowSize: 100,
		db:         db,
	}
	require.NoError(t, g.gatherFromPrimaryBranch(ctx))
	actualWork := sqltest.GetAllRows(ctx, t, db, "PrimaryBranchDiffCalculationWork", &schema.PrimaryBranchDiffCalculationRow{})
	assert.ElementsMatch(t, []schema.PrimaryBranchDiffCalculationRow{
		{
			GroupingID:           h(squareGrouping),
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
		},
		{
			GroupingID:           h(triangleGrouping),
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
		},
		{
			GroupingID:           h(circleGrouping),
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
		},
	}, actualWork)
}

func TestGatherFromPrimaryBranch_SomeExistingWork_AllWorkAdded(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	sentinelTime := ts("2021-02-02T02:15:00Z")
	existingData := dks.Build()
	existingData.PrimaryBranchDiffCalculationWork = []schema.PrimaryBranchDiffCalculationRow{
		{
			GroupingID:           h(squareGrouping),
			LastCalculated:       sentinelTime,
			CalculationLeaseEnds: sentinelTime,
		},
		{
			GroupingID:           h(triangleGrouping),
			LastCalculated:       sentinelTime,
			CalculationLeaseEnds: sentinelTime,
		},
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	g := diffWorkGatherer{
		windowSize: 100,
		db:         db,
	}
	require.NoError(t, g.gatherFromPrimaryBranch(ctx))
	actualWork := sqltest.GetAllRows(ctx, t, db, "PrimaryBranchDiffCalculationWork", &schema.PrimaryBranchDiffCalculationRow{})
	assert.ElementsMatch(t, []schema.PrimaryBranchDiffCalculationRow{
		{
			GroupingID:           h(squareGrouping),
			LastCalculated:       sentinelTime,
			CalculationLeaseEnds: sentinelTime,
		},
		{
			GroupingID:           h(triangleGrouping),
			LastCalculated:       sentinelTime,
			CalculationLeaseEnds: sentinelTime,
		},
		{
			GroupingID:           h(circleGrouping),
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
		},
	}, actualWork)
}

func TestGatherFromChangelists_OnlyReportsGroupingsWithDataNotOnPrimaryBranch(t *testing.T) {
	unittest.LargeTest(t)
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
			GroupingID:           h(circleGrouping),
			LastUpdated:          ts("2020-12-10T04:05:06Z"),
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
			DigestsNotOnPrimary: []types.Digest{
				dks.DigestC06Pos_CL, dks.DigestC07Unt_CL,
			},
		},
		{
			BranchName:           "gerrit-internal_CL_new_tests",
			GroupingID:           h(roundRectGrouping),
			LastUpdated:          ts("2020-12-12T09:20:33Z"),
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
			DigestsNotOnPrimary: []types.Digest{
				dks.DigestE01Pos_CL, dks.DigestE02Pos_CL, dks.DigestE03Unt_CL,
			},
		},
		{
			BranchName:           "gerrit-internal_CL_new_tests",
			GroupingID:           h(textGrouping),
			LastUpdated:          ts("2020-12-12T09:20:33Z"),
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
			DigestsNotOnPrimary: []types.Digest{
				dks.DigestBlank, dks.DigestD01Pos_CL,
			},
		},
	}, actualWork)
	assert.Equal(t, fakeNow, g.mostRecentCLScan)
}

func TestGatherFromChangelists_UpdatesExistingWork(t *testing.T) {
	unittest.LargeTest(t)
	fakeNow := ts("2020-12-12T13:13:13Z")
	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)

	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := dks.Build()
	sentinelTime := ts("2020-05-25T00:00:00Z")
	existingData.SecondaryBranchDiffCalculationWork = []schema.SecondaryBranchDiffCalculationRow{
		{
			BranchName:           "gerrit-internal_CL_new_tests",
			GroupingID:           h(textGrouping),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestBlank},
			LastUpdated:          sentinelTime,
			LastCalculated:       sentinelTime,
			CalculationLeaseEnds: sentinelTime,
		},
		{
			BranchName:           "gerrit_CL_fix_ios",
			GroupingID:           h(circleGrouping),
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
			GroupingID:           h(circleGrouping),
			LastUpdated:          ts("2020-12-10T04:05:06Z"),
			LastCalculated:       sentinelTime, // not changed
			CalculationLeaseEnds: sentinelTime, // not changed
			DigestsNotOnPrimary: []types.Digest{
				dks.DigestC06Pos_CL, dks.DigestC07Unt_CL,
			},
		},
		{
			BranchName:           "gerrit-internal_CL_new_tests",
			GroupingID:           h(roundRectGrouping),
			LastUpdated:          ts("2020-12-12T09:20:33Z"),
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
			DigestsNotOnPrimary: []types.Digest{
				dks.DigestE01Pos_CL, dks.DigestE02Pos_CL, dks.DigestE03Unt_CL,
			},
		},
		{
			BranchName:           "gerrit-internal_CL_new_tests",
			GroupingID:           h(textGrouping),
			LastUpdated:          ts("2020-12-12T09:20:33Z"),
			LastCalculated:       sentinelTime, // not changed
			CalculationLeaseEnds: sentinelTime, // not changed
			DigestsNotOnPrimary: []types.Digest{
				dks.DigestBlank, dks.DigestD01Pos_CL,
			},
		},
	}, actualWork)
	assert.Equal(t, fakeNow, g.mostRecentCLScan)
}

func TestGatherFromChangelists_DeletesOldWork(t *testing.T) {
	unittest.LargeTest(t)
	fakeNow := ts("2021-07-07T07:07:07Z")
	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)

	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := dks.Build()
	existingData.SecondaryBranchDiffCalculationWork = []schema.SecondaryBranchDiffCalculationRow{
		{
			BranchName:           "new_cl",
			GroupingID:           h(textGrouping),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestBlank},
			LastUpdated:          ts("2021-07-05T00:00:00Z"), // 2 days ago
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
		},
		{
			BranchName:           "old_cl",
			GroupingID:           h(circleGrouping),
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
			GroupingID:           h(textGrouping),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestBlank},
			LastUpdated:          ts("2021-07-05T00:00:00Z"),
			LastCalculated:       beginningOfTime,
			CalculationLeaseEnds: beginningOfTime,
		},
	}, actualWork)
	assert.Equal(t, fakeNow, g.mostRecentCLScan)
}

const (
	circleGrouping    = `{"name":"circle","source_type":"round"}`
	squareGrouping    = `{"name":"square","source_type":"corners"}`
	triangleGrouping  = `{"name":"triangle","source_type":"corners"}`
	roundRectGrouping = `{"name":"round rect","source_type":"round"}`
	textGrouping      = `{"name":"seven","source_type":"text"}`
)

var beginningOfTime = ts("1970-01-01T00:00:00Z")

func ts(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// h returns the MD5 hash of the provided string.
func h(s string) []byte {
	hash := md5.Sum([]byte(s))
	return hash[:]
}
