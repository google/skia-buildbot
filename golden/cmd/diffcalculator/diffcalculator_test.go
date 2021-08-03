package main

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diff/mocks"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
)

func TestComputeDiffsForPrimaryBranch_WorkAvailable_Success(t *testing.T) {
	unittest.LargeTest(t)

	fakeNow := ts("2021-02-02T02:30:00Z")

	existingData := schema.Tables{PrimaryBranchDiffCalculationWork: []schema.PrimaryBranchDiffCalculationRow{
		{
			GroupingID:           h(alphaGrouping), // available for work
			LastCalculated:       ts("2021-02-02T02:15:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
		},
		{
			GroupingID:           h(betaGrouping), // Too recently computed
			LastCalculated:       ts("2021-02-02T02:29:50Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
		},
		{
			GroupingID:           h(gammaGrouping), // another worker has it "leased"
			LastCalculated:       ts("2021-02-02T02:25:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:37:00Z"),
		},
		{
			GroupingID:           h(deltaGrouping), // available for work (oldest)
			LastCalculated:       ts("2021-02-02T02:12:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
		},
	}, Groupings: makeGroupingRows(alphaGrouping, betaGrouping, gammaGrouping, deltaGrouping)}

	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	mc := &mocks.Calculator{}
	mc.On("CalculateDiffs", testutils.AnyContext, ps(deltaGrouping), noDigests).Return(nil)

	s := processorForTest(mc, db)

	shouldSleep, err := s.computeDiffsForPrimaryBranch(ctx)
	require.NoError(t, err)
	assert.False(t, shouldSleep)

	mc.AssertExpectations(t)

	actualWork := sqltest.GetAllRows(ctx, t, db, "PrimaryBranchDiffCalculationWork", &schema.PrimaryBranchDiffCalculationRow{})
	assert.Contains(t, actualWork, schema.PrimaryBranchDiffCalculationRow{
		GroupingID:           h(deltaGrouping),
		LastCalculated:       ts("2021-02-02T02:30:00Z"), // Diff calculated time (fakeNow)
		CalculationLeaseEnds: ts("2021-02-02T02:40:00Z"), // This is the timeout + fakeNow
	})
}

func TestComputeDiffsForPrimaryBranch_NoWorkAvailable_ShouldSleep(t *testing.T) {
	unittest.LargeTest(t)

	fakeNow := ts("2021-02-02T02:30:00Z")

	rowsThatShouldBeUnchanged := []schema.PrimaryBranchDiffCalculationRow{
		{
			GroupingID:           h(alphaGrouping), // another worker has it "leased"
			LastCalculated:       ts("2021-02-02T02:15:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:34:00Z"),
		},
		{
			GroupingID:           h(betaGrouping), // Too recently computed
			LastCalculated:       ts("2021-02-02T02:29:50Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
		},
		{
			GroupingID:           h(gammaGrouping), // another worker has it "leased"
			LastCalculated:       ts("2021-02-02T02:25:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:37:00Z"),
		},
		{
			GroupingID:           h(deltaGrouping), // Too recently computed
			LastCalculated:       ts("2021-02-02T02:29:45Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
		},
	}
	existingData := schema.Tables{PrimaryBranchDiffCalculationWork: rowsThatShouldBeUnchanged, Groupings: makeGroupingRows(alphaGrouping, betaGrouping, gammaGrouping, deltaGrouping)}

	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	s := processorForTest(nil, db)

	shouldSleep, err := s.computeDiffsForPrimaryBranch(ctx)
	require.NoError(t, err)
	assert.True(t, shouldSleep)

	// We shouldn't have leased any work
	actualWork := sqltest.GetAllRows(ctx, t, db, "PrimaryBranchDiffCalculationWork", &schema.PrimaryBranchDiffCalculationRow{})
	assert.ElementsMatch(t, rowsThatShouldBeUnchanged, actualWork)
}

func TestComputeDiffsForPrimaryBranch_DiffComputationFails_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)

	fakeNow := ts("2021-02-02T02:30:00Z")

	existingData := schema.Tables{PrimaryBranchDiffCalculationWork: []schema.PrimaryBranchDiffCalculationRow{
		{
			GroupingID:           h(alphaGrouping), // available for work
			LastCalculated:       ts("2021-02-02T02:15:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
		},
		{
			GroupingID:           h(betaGrouping), // Too recently computed
			LastCalculated:       ts("2021-02-02T02:29:50Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
		},
		{
			GroupingID:           h(gammaGrouping), // another worker has it "leased"
			LastCalculated:       ts("2021-02-02T02:25:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:37:00Z"),
		},
		{
			GroupingID:           h(deltaGrouping), // available for work (oldest)
			LastCalculated:       ts("2021-02-02T02:12:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
		},
	}, Groupings: makeGroupingRows(alphaGrouping, betaGrouping, gammaGrouping, deltaGrouping)}

	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	mc := &mocks.Calculator{}
	mc.On("CalculateDiffs", testutils.AnyContext, ps(deltaGrouping), noDigests).Return(errors.New("timeout"))

	s := processorForTest(mc, db)

	shouldSleep, err := s.computeDiffsForPrimaryBranch(ctx)
	require.Error(t, err)
	assert.False(t, shouldSleep)

	mc.AssertExpectations(t)

	actualWork := sqltest.GetAllRows(ctx, t, db, "PrimaryBranchDiffCalculationWork", &schema.PrimaryBranchDiffCalculationRow{})
	assert.Contains(t, actualWork, schema.PrimaryBranchDiffCalculationRow{
		GroupingID:           h(deltaGrouping),
		LastCalculated:       ts("2021-02-02T02:12:00Z"), // unchanged b/c not successful
		CalculationLeaseEnds: ts("2021-02-02T02:40:00Z"), // This is the timeout + fakeNow
	})
}

func TestComputeDiffsForSecondaryBranch_WorkAvailable_Success(t *testing.T) {
	unittest.LargeTest(t)

	fakeNow := ts("2021-02-02T02:30:00Z")
	expectedDigests := []types.Digest{dks.DigestE02Pos_CL, dks.DigestE03Unt_CL}
	existingData := schema.Tables{SecondaryBranchDiffCalculationWork: []schema.SecondaryBranchDiffCalculationRow{
		{
			BranchName:           "gerrit_whatever",
			GroupingID:           h(alphaGrouping), // available for work
			LastCalculated:       ts("2021-02-02T02:15:00Z"),
			LastUpdated:          ts("2021-02-02T02:20:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestC06Pos_CL},
		},
		{
			BranchName:           "gerrit_whatever",
			GroupingID:           h(betaGrouping), // no updates since last calculation
			LastCalculated:       ts("2021-02-02T02:26:00Z"),
			LastUpdated:          ts("2021-02-02T02:20:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestE03Unt_CL},
		},
		{
			BranchName:           "gerrit_anything",
			GroupingID:           h(gammaGrouping), // another worker has it "leased"
			LastCalculated:       ts("2021-02-02T02:20:00Z"),
			LastUpdated:          ts("2021-02-02T02:25:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:37:00Z"),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestD01Pos_CL},
		},
		{
			BranchName:           "gerrit_anything",
			GroupingID:           h(deltaGrouping), // available for work (oldest)
			LastCalculated:       ts("2021-02-02T02:12:00Z"),
			LastUpdated:          ts("2021-02-02T02:16:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
			DigestsNotOnPrimary:  expectedDigests,
		},
	}, Groupings: makeGroupingRows(alphaGrouping, betaGrouping, gammaGrouping, deltaGrouping)}

	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	mc := &mocks.Calculator{}
	mc.On("CalculateDiffs", testutils.AnyContext, ps(deltaGrouping), expectedDigests).Return(nil)

	s := processorForTest(mc, db)

	shouldSleep, err := s.computeDiffsForSecondaryBranch(ctx)
	require.NoError(t, err)
	assert.False(t, shouldSleep)

	mc.AssertExpectations(t)

	actualWork := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchDiffCalculationWork", &schema.SecondaryBranchDiffCalculationRow{})
	assert.Contains(t, actualWork, schema.SecondaryBranchDiffCalculationRow{
		BranchName:           "gerrit_anything",
		GroupingID:           h(deltaGrouping),
		LastCalculated:       ts("2021-02-02T02:30:00Z"), // Diff calculated time (fakeNow)
		LastUpdated:          ts("2021-02-02T02:16:00Z"), // unchanged
		CalculationLeaseEnds: ts("2021-02-02T02:40:00Z"), // This is the timeout + fakeNow
		DigestsNotOnPrimary:  expectedDigests,
	})
}

func TestComputeDiffsForSecondaryBranch_NoWorkAvailable_ShouldSleep(t *testing.T) {
	unittest.LargeTest(t)

	fakeNow := ts("2021-02-02T02:30:00Z")
	rowsThatShouldBeUnchanged := []schema.SecondaryBranchDiffCalculationRow{
		{
			BranchName:           "gerrit_whatever",
			GroupingID:           h(alphaGrouping), // no updates since last calculation
			LastCalculated:       ts("2021-02-02T02:25:00Z"),
			LastUpdated:          ts("2021-02-02T02:20:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestC06Pos_CL},
		},
		{
			BranchName:           "gerrit_whatever",
			GroupingID:           h(betaGrouping), // no updates since last calculation
			LastCalculated:       ts("2021-02-02T02:26:00Z"),
			LastUpdated:          ts("2021-02-02T02:20:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestE03Unt_CL},
		},
		{
			BranchName:           "gerrit_anything",
			GroupingID:           h(gammaGrouping), // another worker has it "leased"
			LastCalculated:       ts("2021-02-02T02:20:00Z"),
			LastUpdated:          ts("2021-02-02T02:25:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:37:00Z"),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestD01Pos_CL},
		},
		{
			BranchName:           "gerrit_anything",
			GroupingID:           h(deltaGrouping), // another worker has it "leased"
			LastCalculated:       ts("2021-02-02T02:12:00Z"),
			LastUpdated:          ts("2021-02-02T02:16:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:34:00Z"),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestE02Pos_CL, dks.DigestE03Unt_CL},
		},
	}

	existingData := schema.Tables{SecondaryBranchDiffCalculationWork: rowsThatShouldBeUnchanged, Groupings: makeGroupingRows(alphaGrouping, betaGrouping, gammaGrouping, deltaGrouping)}

	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	s := processorForTest(nil, db)

	shouldSleep, err := s.computeDiffsForSecondaryBranch(ctx)
	require.NoError(t, err)
	assert.True(t, shouldSleep)

	// We shouldn't have leased any work
	actualWork := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchDiffCalculationWork", &schema.SecondaryBranchDiffCalculationRow{})
	assert.ElementsMatch(t, rowsThatShouldBeUnchanged, actualWork)
}

func TestComputeDiffsForSecondaryBranch_DiffComputationFails_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)

	fakeNow := ts("2021-02-02T02:30:00Z")
	expectedDigests := []types.Digest{dks.DigestE02Pos_CL, dks.DigestE03Unt_CL}
	existingData := schema.Tables{SecondaryBranchDiffCalculationWork: []schema.SecondaryBranchDiffCalculationRow{
		{
			BranchName:           "gerrit_whatever",
			GroupingID:           h(alphaGrouping), // available for work
			LastCalculated:       ts("2021-02-02T02:15:00Z"),
			LastUpdated:          ts("2021-02-02T02:20:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestC06Pos_CL},
		},
		{
			BranchName:           "gerrit_whatever",
			GroupingID:           h(betaGrouping), // no updates since last calculation
			LastCalculated:       ts("2021-02-02T02:26:00Z"),
			LastUpdated:          ts("2021-02-02T02:20:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestE03Unt_CL},
		},
		{
			BranchName:           "gerrit_anything",
			GroupingID:           h(gammaGrouping), // another worker has it "leased"
			LastCalculated:       ts("2021-02-02T02:20:00Z"),
			LastUpdated:          ts("2021-02-02T02:25:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:37:00Z"),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestD01Pos_CL},
		},
		{
			BranchName:           "gerrit_anything",
			GroupingID:           h(deltaGrouping), // available for work (oldest)
			LastCalculated:       ts("2021-02-02T02:12:00Z"),
			LastUpdated:          ts("2021-02-02T02:16:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
			DigestsNotOnPrimary:  expectedDigests,
		},
	}, Groupings: makeGroupingRows(alphaGrouping, betaGrouping, gammaGrouping, deltaGrouping)}

	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	mc := &mocks.Calculator{}
	mc.On("CalculateDiffs", testutils.AnyContext, ps(deltaGrouping), expectedDigests).Return(errors.New("timeout"))

	s := processorForTest(mc, db)

	shouldSleep, err := s.computeDiffsForSecondaryBranch(ctx)
	require.Error(t, err)
	assert.False(t, shouldSleep)

	mc.AssertExpectations(t)

	actualWork := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchDiffCalculationWork", &schema.SecondaryBranchDiffCalculationRow{})
	assert.Contains(t, actualWork, schema.SecondaryBranchDiffCalculationRow{
		BranchName:           "gerrit_anything",
		GroupingID:           h(deltaGrouping),
		LastCalculated:       ts("2021-02-02T02:12:00Z"), // unchanged
		LastUpdated:          ts("2021-02-02T02:16:00Z"), // unchanged
		CalculationLeaseEnds: ts("2021-02-02T02:40:00Z"), // This is the timeout + fakeNow
		DigestsNotOnPrimary:  expectedDigests,
	})
}

func TestHighContentionSecondaryBranch_WorkAvailable_Success(t *testing.T) {
	unittest.LargeTest(t)

	fakeNow := ts("2021-02-02T02:30:00Z")
	alphaDigests := []types.Digest{dks.DigestC06Pos_CL}
	deltaDigests := []types.Digest{dks.DigestE02Pos_CL, dks.DigestE03Unt_CL}
	existingData := schema.Tables{SecondaryBranchDiffCalculationWork: []schema.SecondaryBranchDiffCalculationRow{
		{
			BranchName:           "gerrit_whatever",
			GroupingID:           h(alphaGrouping), // available for work
			LastCalculated:       ts("2021-02-02T02:15:00Z"),
			LastUpdated:          ts("2021-02-02T02:20:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
			DigestsNotOnPrimary:  alphaDigests,
		},
		{
			BranchName:           "gerrit_whatever",
			GroupingID:           h(betaGrouping), // no updates since last calculation
			LastCalculated:       ts("2021-02-02T02:26:00Z"),
			LastUpdated:          ts("2021-02-02T02:20:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestE03Unt_CL},
		},
		{
			BranchName:           "gerrit_anything",
			GroupingID:           h(gammaGrouping), // another worker has it "leased"
			LastCalculated:       ts("2021-02-02T02:20:00Z"),
			LastUpdated:          ts("2021-02-02T02:25:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:37:00Z"),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestD01Pos_CL},
		},
		{
			BranchName:           "gerrit_anything",
			GroupingID:           h(deltaGrouping), // available for work (oldest)
			LastCalculated:       ts("2021-02-02T02:12:00Z"),
			LastUpdated:          ts("2021-02-02T02:16:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
			DigestsNotOnPrimary:  deltaDigests,
		},
	}, Groupings: makeGroupingRows(alphaGrouping, betaGrouping, gammaGrouping, deltaGrouping)}

	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()

	// The diffs to calculate will be randomly picked from the available work.
	mc := &mocks.Calculator{}
	called := ""
	mc.On("CalculateDiffs", testutils.AnyContext, ps(alphaGrouping), alphaDigests).Return(nil).Run(func(_ mock.Arguments) {
		called = "alpha"
	}).Maybe()
	mc.On("CalculateDiffs", testutils.AnyContext, ps(deltaGrouping), deltaDigests).Return(nil).Run(func(_ mock.Arguments) {
		called = "delta"
	}).Maybe()

	s := processorForTest(mc, db)

	shouldSleep, err := s.highContentionSecondaryBranch(ctx)
	require.NoError(t, err)
	assert.False(t, shouldSleep)

	mc.AssertExpectations(t)
	assert.NotEmpty(t, called)

	actualWork := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchDiffCalculationWork", &schema.SecondaryBranchDiffCalculationRow{})
	if called == "alpha" {
		assert.Contains(t, actualWork, schema.SecondaryBranchDiffCalculationRow{
			BranchName:           "gerrit_whatever",
			GroupingID:           h(alphaGrouping),
			LastCalculated:       ts("2021-02-02T02:30:00Z"), // Diff calculated time (fakeNow)
			LastUpdated:          ts("2021-02-02T02:20:00Z"), // unchanged
			CalculationLeaseEnds: ts("2021-02-02T02:40:00Z"), // This is the timeout + fakeNow
			DigestsNotOnPrimary:  alphaDigests,
		})
	} else {
		assert.Contains(t, actualWork, schema.SecondaryBranchDiffCalculationRow{
			BranchName:           "gerrit_anything",
			GroupingID:           h(deltaGrouping),
			LastCalculated:       ts("2021-02-02T02:30:00Z"), // Diff calculated time (fakeNow)
			LastUpdated:          ts("2021-02-02T02:16:00Z"), // unchanged
			CalculationLeaseEnds: ts("2021-02-02T02:40:00Z"), // This is the timeout + fakeNow
			DigestsNotOnPrimary:  deltaDigests,
		})
	}
}

func TestHighContentionSecondaryBranch_NoWorkAvailable_ShouldSleep(t *testing.T) {
	unittest.LargeTest(t)

	fakeNow := ts("2021-02-02T02:30:00Z")
	rowsThatShouldBeUnchanged := []schema.SecondaryBranchDiffCalculationRow{
		{
			BranchName:           "gerrit_whatever",
			GroupingID:           h(alphaGrouping), // no updates since last calculation
			LastCalculated:       ts("2021-02-02T02:25:00Z"),
			LastUpdated:          ts("2021-02-02T02:20:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestC06Pos_CL},
		},
		{
			BranchName:           "gerrit_whatever",
			GroupingID:           h(betaGrouping), // no updates since last calculation
			LastCalculated:       ts("2021-02-02T02:26:00Z"),
			LastUpdated:          ts("2021-02-02T02:20:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestE03Unt_CL},
		},
		{
			BranchName:           "gerrit_anything",
			GroupingID:           h(gammaGrouping), // another worker has it "leased"
			LastCalculated:       ts("2021-02-02T02:20:00Z"),
			LastUpdated:          ts("2021-02-02T02:25:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:37:00Z"),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestD01Pos_CL},
		},
		{
			BranchName:           "gerrit_anything",
			GroupingID:           h(deltaGrouping), // another worker has it "leased"
			LastCalculated:       ts("2021-02-02T02:12:00Z"),
			LastUpdated:          ts("2021-02-02T02:16:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:34:00Z"),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestE02Pos_CL, dks.DigestE03Unt_CL},
		},
	}

	existingData := schema.Tables{SecondaryBranchDiffCalculationWork: rowsThatShouldBeUnchanged, Groupings: makeGroupingRows(alphaGrouping, betaGrouping, gammaGrouping, deltaGrouping)}

	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()

	s := processorForTest(nil, db)

	shouldSleep, err := s.highContentionSecondaryBranch(ctx)
	require.NoError(t, err)
	assert.True(t, shouldSleep)

	// We shouldn't have leased any work
	actualWork := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchDiffCalculationWork", &schema.SecondaryBranchDiffCalculationRow{})
	assert.ElementsMatch(t, rowsThatShouldBeUnchanged, actualWork)
}

func TestHighContentionSecondaryBranch_DiffComputationFails_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)

	fakeNow := ts("2021-02-02T02:30:00Z")
	alphaDigests := []types.Digest{dks.DigestC06Pos_CL}
	deltaDigests := []types.Digest{dks.DigestE02Pos_CL, dks.DigestE03Unt_CL}
	existingData := schema.Tables{SecondaryBranchDiffCalculationWork: []schema.SecondaryBranchDiffCalculationRow{
		{
			BranchName:           "gerrit_whatever",
			GroupingID:           h(alphaGrouping), // available for work
			LastCalculated:       ts("2021-02-02T02:15:00Z"),
			LastUpdated:          ts("2021-02-02T02:20:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
			DigestsNotOnPrimary:  alphaDigests,
		},
		{
			BranchName:           "gerrit_whatever",
			GroupingID:           h(betaGrouping), // no updates since last calculation
			LastCalculated:       ts("2021-02-02T02:26:00Z"),
			LastUpdated:          ts("2021-02-02T02:20:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:14:00Z"),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestE03Unt_CL},
		},
		{
			BranchName:           "gerrit_anything",
			GroupingID:           h(gammaGrouping), // another worker has it "leased"
			LastCalculated:       ts("2021-02-02T02:20:00Z"),
			LastUpdated:          ts("2021-02-02T02:25:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:37:00Z"),
			DigestsNotOnPrimary:  []types.Digest{dks.DigestD01Pos_CL},
		},
		{
			BranchName:           "gerrit_anything",
			GroupingID:           h(deltaGrouping), // another worker has it "leased"
			LastCalculated:       ts("2021-02-02T02:12:00Z"),
			LastUpdated:          ts("2021-02-02T02:16:00Z"),
			CalculationLeaseEnds: ts("2021-02-02T02:34:00Z"),
			DigestsNotOnPrimary:  deltaDigests,
		},
	}, Groupings: makeGroupingRows(alphaGrouping, betaGrouping, gammaGrouping, deltaGrouping)}

	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()

	mc := &mocks.Calculator{}
	mc.On("CalculateDiffs", testutils.AnyContext, ps(alphaGrouping), alphaDigests).Return(errors.New("timeout")).Maybe()
	mc.On("CalculateDiffs", testutils.AnyContext, ps(deltaGrouping), deltaDigests).Return(errors.New("timeout")).Maybe()

	s := processorForTest(mc, db)

	shouldSleep, err := s.highContentionSecondaryBranch(ctx)
	require.Error(t, err)
	assert.False(t, shouldSleep)

	mc.AssertExpectations(t)

	actualWork := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchDiffCalculationWork", &schema.SecondaryBranchDiffCalculationRow{})
	assert.Contains(t, actualWork, schema.SecondaryBranchDiffCalculationRow{
		BranchName:           "gerrit_whatever",
		GroupingID:           h(alphaGrouping),
		LastCalculated:       ts("2021-02-02T02:15:00Z"), // unchanged
		LastUpdated:          ts("2021-02-02T02:20:00Z"), // unchanged
		CalculationLeaseEnds: ts("2021-02-02T02:40:00Z"), // This is the timeout + fakeNow
		DigestsNotOnPrimary:  alphaDigests,
	})
}

func processorForTest(c diff.Calculator, db *pgxpool.Pool) *processor {
	cache, err := lru.New(100)
	if err != nil {
		panic(err)
	}
	return &processor{
		db:             db,
		calculator:     c,
		groupingCache:  cache,
		primaryCounter: fakeCounter{},
		clsCounter:     fakeCounter{},
	}
}

func makeGroupingRows(groupings ...string) []schema.GroupingRow {
	rv := make([]schema.GroupingRow, 0, len(groupings))
	for _, g := range groupings {
		rv = append(rv, schema.GroupingRow{
			GroupingID: h(g),
			Keys:       ps(g),
		})
	}
	return rv
}

const (
	alphaGrouping = `{"name":"alpha","source_type":"corpus_one"}`
	betaGrouping  = `{"name":"beta","source_type":"corpus_two"}`
	gammaGrouping = `{"name":"gamma","source_type":"corpus_one"}`
	deltaGrouping = `{"name":"delta","source_type":"corpus_two"}`
)

var (
	noDigests []types.Digest
)

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

func ps(s string) paramtools.Params {
	var rv paramtools.Params
	if err := json.Unmarshal([]byte(s), &rv); err != nil {
		panic(err)
	}
	return rv
}

type fakeCounter struct{}

func (fakeCounter) Dec(_ int64)   {}
func (fakeCounter) Delete() error { return nil }
func (fakeCounter) Get() int64    { return 0 }
func (fakeCounter) Inc(_ int64)   {}
func (fakeCounter) Reset()        {}

// waitForSystemTime waits for a time greater than the duration mentioned in "AS OF SYSTEM TIME"
// clauses in queries. This way, the queries will be accurate.
func waitForSystemTime() {
	time.Sleep(150 * time.Millisecond)
}
