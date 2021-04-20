package search2

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
)

var changelistTSForIOS = time.Date(2020, time.December, 10, 4, 5, 6, 0, time.UTC)
var changelistTSForNewTests = time.Date(2020, time.December, 12, 9, 20, 33, 0, time.UTC)

func TestNewAndUntriagedSummaryForCL_OnePatchset_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db)
	require.NoError(t, s.StartCacheProcess(ctx, time.Minute, 100))
	rv, err := s.NewAndUntriagedSummaryForCL(ctx, dks.GerritCRS, dks.ChangelistIDThatAttemptsToFixIOS)
	require.NoError(t, err)
	assert.Equal(t, NewAndUntriagedSummary{
		ChangelistID: dks.ChangelistIDThatAttemptsToFixIOS,
		PatchsetSummaries: []PatchsetNewAndUntriagedSummary{{
			NewImages: 2, // DigestC07Unt_CL and DigestC06Pos_CL
			// Only 1 of the two CLs "new images" is untriaged, so that's what we report.
			NewUntriagedImages: 1,
			// In addition to DigestC07Unt_CL, this PS produces DigestC05Unt and DigestB01Pos
			// (the latter is incorrectly triaged as untriaged on this CL). C05 was produced
			// by a ignored trace, so it shouldn't be counted.
			TotalUntriagedImages: 2,
			PatchsetID:           dks.PatchSetIDFixesIPadButNotIPhone,
			PatchsetOrder:        3,
		}},
		LastUpdated: changelistTSForIOS,
	}, rv)
}

func TestNewAndUntriagedSummaryForCL_TwoPatchsets_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db)
	require.NoError(t, s.StartCacheProcess(ctx, time.Minute, 100))
	rv, err := s.NewAndUntriagedSummaryForCL(ctx, dks.GerritInternalCRS, dks.ChangelistIDThatAddsNewTests)
	require.NoError(t, err)

	assert.Equal(t, NewAndUntriagedSummary{
		ChangelistID: dks.ChangelistIDThatAddsNewTests,
		// Should be sorted by PatchsetOrder
		PatchsetSummaries: []PatchsetNewAndUntriagedSummary{{
			// One grouping (Text-Seven) produced one image that had not been seen on that grouping
			// before (DigestBlank). This digest *had* been seen on the primary branch in a
			// different grouping, but that should not prevent us from letting a developer know.
			NewImages:          1,
			NewUntriagedImages: 1,
			// Two circle tests are producing DigestC03Unt and DigestC04Unt
			TotalUntriagedImages: 3,
			PatchsetID:           dks.PatchsetIDAddsNewCorpus,
			PatchsetOrder:        1,
		}, {
			// Two groupings (Text-Seven and Round-RoundRect) produced 1 and 3 new digests
			// respectively. DigestE03Unt_CL remains untriaged.
			NewImages:          4,
			NewUntriagedImages: 1,
			// Two circle tests are producing DigestC03Unt and DigestC04Unt
			TotalUntriagedImages: 3,
			PatchsetID:           dks.PatchsetIDAddsNewCorpusAndTest,
			PatchsetOrder:        4,
		}},
		LastUpdated: changelistTSForNewTests,
	}, rv)
}

func TestNewAndUntriagedSummaryForCL_NoNewDataForPS_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	b := dks.RawBuilder()
	const clID = "new_cl"
	const ps1ID = "first_docs_ps"
	const ps2ID = "second_docs_ps"
	cl := b.AddChangelist(clID, dks.GerritCRS, dks.UserFour, "Update docs", schema.StatusAbandoned)
	ps1 := cl.AddPatchset(ps1ID, "5555555555555555555555555555555555555555", 2)
	ps2 := cl.AddPatchset(ps2ID, "6666666666666666666666666666666666666666", 12)

	ps1.DataWithCommonKeys(paramtools.Params{
		dks.OSKey:     dks.AndroidOS,
		dks.DeviceKey: dks.WalleyeDevice,
	}).Digests(dks.DigestA07Pos, dks.DigestB01Pos, dks.DigestC01Pos).
		Keys([]paramtools.Params{
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.SquareTest},
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.TriangleTest},
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.RoundCorpus, types.PrimaryKeyField: dks.CircleTest},
		}).OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob("tryjob 1", dks.BuildBucketCIS, "My-Test", "whatever", "2021-03-30T23:59:59Z")

	ps2.DataWithCommonKeys(paramtools.Params{
		dks.OSKey:     dks.AndroidOS,
		dks.DeviceKey: dks.WalleyeDevice,
	}).Digests(dks.DigestA07Pos, dks.DigestB01Pos, dks.DigestC01Pos).
		Keys([]paramtools.Params{
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.SquareTest},
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.TriangleTest},
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.RoundCorpus, types.PrimaryKeyField: dks.CircleTest},
		}).OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob("tryjob 2", dks.BuildBucketCIS, "My-Test", "whatever", "2021-04-01T02:03:04Z")

	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, b.Build()))

	s := New(db)
	require.NoError(t, s.StartCacheProcess(ctx, time.Minute, 100))
	rv, err := s.NewAndUntriagedSummaryForCL(ctx, dks.GerritCRS, clID)
	require.NoError(t, err)

	assert.Equal(t, NewAndUntriagedSummary{
		ChangelistID: clID,
		// Should be sorted by PatchsetOrder
		PatchsetSummaries: []PatchsetNewAndUntriagedSummary{{
			NewImages:            0,
			NewUntriagedImages:   0,
			TotalUntriagedImages: 0,
			PatchsetID:           ps1ID,
			PatchsetOrder:        2,
		}, {
			NewImages:            0,
			NewUntriagedImages:   0,
			TotalUntriagedImages: 0,
			PatchsetID:           ps2ID,
			PatchsetOrder:        12,
		}},
		LastUpdated: time.Date(2021, time.April, 1, 2, 3, 4, 0, time.UTC),
	}, rv)
}

func TestNewAndUntriagedSummaryForCL_CLDoesNotExist_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db)
	require.NoError(t, s.StartCacheProcess(ctx, time.Minute, 100))
	_, err := s.NewAndUntriagedSummaryForCL(ctx, dks.GerritInternalCRS, "does not exist")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestNewAndUntriagedSummaryForCL_NewDeviceAdded_DigestsOnPrimaryBranchNotCounted(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	b := dks.RawBuilder()
	const clID = "new_cl"
	const ps1ID = "ps has bug with circle"
	const ps2ID = "ps has different bug with circle"
	const ps3ID = "ps resolved bug with circle"
	cl := b.AddChangelist(clID, dks.GerritCRS, dks.UserFour, "Add new device", schema.StatusLanded)
	ps1 := cl.AddPatchset(ps1ID, "5555555555555555555555555555555555555555", 2)
	ps2 := cl.AddPatchset(ps2ID, "6666666666666666666666666666666666666666", 4)
	ps3 := cl.AddPatchset(ps3ID, "7777777777777777777777777777777777777777", 7)

	ps1.DataWithCommonKeys(paramtools.Params{
		dks.OSKey:     dks.AndroidOS,
		dks.DeviceKey: "brand new device",
	}).Digests(dks.DigestA07Pos, dks.DigestB01Pos,
		// This digest has been triaged on another CL, but that shouldn't impact this CL - it
		// should be counted and reported as untriaged.
		dks.DigestC06Pos_CL,
	).
		Keys([]paramtools.Params{
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.SquareTest},
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.TriangleTest},
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.RoundCorpus, types.PrimaryKeyField: dks.CircleTest},
		}).OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob("tryjob 1", dks.BuildBucketCIS, "My-Test", "whatever", "2021-03-30T00:00:00Z")

	ps2.DataWithCommonKeys(paramtools.Params{
		dks.OSKey:     dks.AndroidOS,
		dks.DeviceKey: "brand new device",
	}).Digests(dks.DigestA07Pos, dks.DigestB01Pos,
		// This digest will be triaged as negative on this CL.
		dks.DigestC07Unt_CL,
	).
		Keys([]paramtools.Params{
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.SquareTest},
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.TriangleTest},
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.RoundCorpus, types.PrimaryKeyField: dks.CircleTest},
		}).OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob("tryjob 2", dks.BuildBucketCIS, "My-Test", "whatever", "2021-03-30T12:00:00Z")

	cl.AddTriageEvent(dks.UserFour, "2021-03-30T13:00:00Z").
		ExpectationsForGrouping(paramtools.Params{
			types.CorpusField: dks.RoundCorpus, types.PrimaryKeyField: dks.CircleTest}).
		Negative(dks.DigestC07Unt_CL)

	ps3.DataWithCommonKeys(paramtools.Params{
		dks.OSKey:     dks.AndroidOS,
		dks.DeviceKey: "brand new device",
	}).Digests(dks.DigestA07Pos, dks.DigestB01Pos, dks.DigestC01Pos).
		Keys([]paramtools.Params{
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.SquareTest},
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.TriangleTest},
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.RoundCorpus, types.PrimaryKeyField: dks.CircleTest},
		}).OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob("tryjob 3", dks.BuildBucketCIS, "My-Test", "whatever", "2021-04-01T02:03:04Z")

	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, b.Build()))

	s := New(db)
	require.NoError(t, s.StartCacheProcess(ctx, time.Minute, 100))
	rv, err := s.NewAndUntriagedSummaryForCL(ctx, dks.GerritCRS, clID)
	require.NoError(t, err)

	assert.Equal(t, NewAndUntriagedSummary{
		ChangelistID: clID,
		// Should be sorted by PatchsetOrder
		PatchsetSummaries: []PatchsetNewAndUntriagedSummary{{
			NewImages:            1,
			NewUntriagedImages:   1,
			TotalUntriagedImages: 1,
			PatchsetID:           ps1ID,
			PatchsetOrder:        2,
		}, {
			NewImages:            1,
			NewUntriagedImages:   0,
			TotalUntriagedImages: 0,
			PatchsetID:           ps2ID,
			PatchsetOrder:        4,
		}, {
			NewImages:            0,
			NewUntriagedImages:   0,
			TotalUntriagedImages: 0,
			PatchsetID:           ps3ID,
			PatchsetOrder:        7,
		}},
		LastUpdated: time.Date(2021, time.April, 1, 2, 3, 4, 0, time.UTC),
	}, rv)
}

func TestNewAndUntriagedSummaryForCL_IgnoreRulesRespected(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	b := dks.RawBuilder()
	const clID = "make everything blank"
	const ps1ID = "blankity_blanket"
	const ps2ID = "blankity_blanket_taimen"
	cl := b.AddChangelist(clID, dks.GerritCRS, dks.UserFour, "Experiment with alpha=0", schema.StatusOpen)
	ps1 := cl.AddPatchset(ps1ID, "5555555555555555555555555555555555555555", 1)
	ps2 := cl.AddPatchset(ps2ID, "5555555555555555555555555555555555555555", 2)

	// Force BlankDigest to be untriaged on the primary branch.
	b.AddTriageEvent(dks.UserFour, "2021-03-30T00:00:00Z").
		ExpectationsForGrouping(paramtools.Params{
			types.CorpusField: dks.RoundCorpus, types.PrimaryKeyField: dks.CircleTest,
		}).Triage(dks.DigestBlank, schema.LabelNegative, schema.LabelUntriaged)

	ps1.DataWithCommonKeys(paramtools.Params{
		dks.OSKey:        dks.AndroidOS,
		dks.ColorModeKey: dks.RGBColorMode,
	}).Digests(dks.DigestBlank, dks.DigestBlank, dks.DigestBlank, dks.DigestBlank, dks.DigestBlank, dks.DigestBlank).
		Keys([]paramtools.Params{
			// Blank has been seen on the triangle test before, so this shouldn't be counted
			// except in the TotalUntriaged count.
			{dks.DeviceKey: dks.WalleyeDevice, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.TriangleTest},
			// Blank has not been seen on the square or circle test before, so this should count
			// as 2 new images (one per grouping).
			{dks.DeviceKey: dks.WalleyeDevice, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.SquareTest},
			{dks.DeviceKey: dks.WalleyeDevice, types.CorpusField: dks.RoundCorpus, types.PrimaryKeyField: dks.CircleTest},
			// Again, not counted because blank has been seen on triangle.
			{dks.DeviceKey: "new device", types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.TriangleTest},
			// We count "new digest + grouping", so since we've already counted Blank for each
			// of these, we shouldn't add to the new images anymore.
			{dks.DeviceKey: "new device", types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.SquareTest},
			{dks.DeviceKey: "new device", types.CorpusField: dks.RoundCorpus, types.PrimaryKeyField: dks.CircleTest},
		}).OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob("tryjob 1", dks.BuildBucketCIS, "My-Test", "whatever", "2021-03-30T00:00:00Z")

	ps2.DataWithCommonKeys(paramtools.Params{
		dks.OSKey:        dks.AndroidOS,
		dks.ColorModeKey: dks.RGBColorMode,
	}).Digests(dks.DigestBlank, dks.DigestBlank, dks.DigestBlank).
		Keys([]paramtools.Params{
			// Again, not counted (except in TotalUntriaged) because blank has been seen for
			// the triangle test on primary branch.
			{dks.DeviceKey: dks.TaimenDevice, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.TriangleTest},
			// Should be ignored, so they shouldn't be added to the new image count.
			{dks.DeviceKey: dks.TaimenDevice, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.SquareTest},
			{dks.DeviceKey: dks.TaimenDevice, types.CorpusField: dks.RoundCorpus, types.PrimaryKeyField: dks.CircleTest},
		}).OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob("tryjob 2", dks.BuildBucketCIS, "My-Test", "whatever", "2021-03-30T00:00:00Z")

	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, b.Build()))

	s := New(db)
	require.NoError(t, s.StartCacheProcess(ctx, time.Minute, 100))
	rv, err := s.NewAndUntriagedSummaryForCL(ctx, dks.GerritCRS, clID)
	require.NoError(t, err)

	assert.Equal(t, NewAndUntriagedSummary{
		ChangelistID: clID,
		// Should be sorted by PatchsetOrder
		PatchsetSummaries: []PatchsetNewAndUntriagedSummary{{
			NewImages:            2,
			NewUntriagedImages:   2,
			TotalUntriagedImages: 3,
			PatchsetID:           ps1ID,
			PatchsetOrder:        1,
		}, {
			NewImages:            0,
			NewUntriagedImages:   0,
			TotalUntriagedImages: 1,
			PatchsetID:           ps2ID,
			PatchsetOrder:        2,
		}},
		LastUpdated: time.Date(2021, time.March, 30, 0, 0, 0, 0, time.UTC),
	}, rv)
}

func TestNewAndUntriagedSummaryForCL_TriageStatusAffectsAllPS(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	b := dks.RawBuilder()
	const clID = "new_cl"
	const ps1ID = "ps has bug with circle"
	const ps2ID = "ps has different bug with circle"
	const ps3ID = "still bug with circle"
	cl := b.AddChangelist(clID, dks.GerritCRS, dks.UserFour, "Add new device", schema.StatusLanded)
	ps1 := cl.AddPatchset(ps1ID, "5555555555555555555555555555555555555555", 2)
	ps2 := cl.AddPatchset(ps2ID, "6666666666666666666666666666666666666666", 4)
	ps3 := cl.AddPatchset(ps3ID, "7777777777777777777777777777777777777777", 7)

	ps1.DataWithCommonKeys(paramtools.Params{
		dks.OSKey:     dks.AndroidOS,
		dks.DeviceKey: "brand new device",
	}).Digests(dks.DigestA07Pos, dks.DigestB01Pos, dks.DigestC07Unt_CL).
		Keys([]paramtools.Params{
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.SquareTest},
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.TriangleTest},
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.RoundCorpus, types.PrimaryKeyField: dks.CircleTest},
		}).OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob("tryjob 1", dks.BuildBucketCIS, "My-Test", "whatever", "2021-03-30T00:00:00Z")

	ps2.DataWithCommonKeys(paramtools.Params{
		dks.OSKey:     dks.AndroidOS,
		dks.DeviceKey: dks.WalleyeDevice,
	}).Digests(dks.DigestA07Pos, dks.DigestB01Pos, dks.DigestC07Unt_CL).
		Keys([]paramtools.Params{
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.SquareTest},
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.TriangleTest},
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.RoundCorpus, types.PrimaryKeyField: dks.CircleTest},
		}).OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob("tryjob 2", dks.BuildBucketCIS, "My-Test", "whatever", "2021-03-30T12:00:00Z")

	ps3.DataWithCommonKeys(paramtools.Params{
		dks.OSKey:     dks.AndroidOS,
		dks.DeviceKey: dks.WalleyeDevice,
	}).Digests(dks.DigestA07Pos, dks.DigestB01Pos, dks.DigestC07Unt_CL).
		Keys([]paramtools.Params{
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.SquareTest},
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.TriangleTest},
			{dks.ColorModeKey: dks.RGBColorMode, types.CorpusField: dks.RoundCorpus, types.PrimaryKeyField: dks.CircleTest},
		}).OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob("tryjob 3", dks.BuildBucketCIS, "My-Test", "whatever", "2021-04-01T02:03:04Z")

	// The digest was triaged negative after data from the third PS was ingested, but it should
	// retroactively apply to all PS in the CL because we don't care about time.
	cl.AddTriageEvent(dks.UserFour, "2021-04-03T00:00:00Z").
		ExpectationsForGrouping(paramtools.Params{
			types.CorpusField: dks.RoundCorpus, types.PrimaryKeyField: dks.CircleTest}).
		Negative(dks.DigestC07Unt_CL)

	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, b.Build()))

	s := New(db)
	require.NoError(t, s.StartCacheProcess(ctx, time.Minute, 100))
	rv, err := s.NewAndUntriagedSummaryForCL(ctx, dks.GerritCRS, clID)
	require.NoError(t, err)

	assert.Equal(t, NewAndUntriagedSummary{
		ChangelistID: clID,
		// Should be sorted by PatchsetOrder
		PatchsetSummaries: []PatchsetNewAndUntriagedSummary{{
			NewImages:            1,
			NewUntriagedImages:   0,
			TotalUntriagedImages: 0,
			PatchsetID:           ps1ID,
			PatchsetOrder:        2,
		}, {
			NewImages:            1,
			NewUntriagedImages:   0,
			TotalUntriagedImages: 0,
			PatchsetID:           ps2ID,
			PatchsetOrder:        4,
		}, {
			NewImages:            1,
			NewUntriagedImages:   0,
			TotalUntriagedImages: 0,
			PatchsetID:           ps3ID,
			PatchsetOrder:        7,
		}},
		LastUpdated: time.Date(2021, time.April, 1, 2, 3, 4, 0, time.UTC),
	}, rv)
}

func TestNewAndUntriagedSummaryForCL_MultipleThreadsAtOnce_NoRaces(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db)
	// Update the caches aggressively to be writing to the shared cache while reading from it.
	require.NoError(t, s.StartCacheProcess(ctx, 100*time.Millisecond, 100))

	wg := sync.WaitGroup{}
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rv, err := s.NewAndUntriagedSummaryForCL(ctx, dks.GerritCRS, dks.ChangelistIDThatAttemptsToFixIOS)
			require.NoError(t, err)
			assert.Equal(t, NewAndUntriagedSummary{
				ChangelistID: dks.ChangelistIDThatAttemptsToFixIOS,
				PatchsetSummaries: []PatchsetNewAndUntriagedSummary{{
					NewImages: 2, // DigestC07Unt_CL and DigestC06Pos_CL
					// Only 1 of the two CLs "new images" is untriaged, so that's what we report.
					NewUntriagedImages: 1,
					// In addition to DigestC07Unt_CL, this PS produces DigestC05Unt and DigestB01Pos
					// (the latter is incorrectly triaged as untriaged on this CL). C05 was produced
					// by a ignored trace, so it shouldn't be counted.
					TotalUntriagedImages: 2,
					PatchsetID:           dks.PatchSetIDFixesIPadButNotIPhone,
					PatchsetOrder:        3,
				}},
				LastUpdated: changelistTSForIOS,
			}, rv)
		}()
	}
	wg.Wait()
}

func TestChangelistLastUpdated_ReturnsLatestTS(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	s := New(db)
	ts, err := s.ChangelistLastUpdated(ctx, dks.GerritInternalCRS, dks.ChangelistIDThatAddsNewTests)
	require.NoError(t, err)
	assert.Equal(t, changelistTSForNewTests, ts)
}

// waitForSystemTime waits for a time greater than the duration mentioned in "AS OF SYSTEM TIME"
// clauses in queries. This way, the queries will be accurate.
func waitForSystemTime() {
	time.Sleep(150 * time.Millisecond)
}
