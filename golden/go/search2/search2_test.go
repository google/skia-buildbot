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
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/search/frontend"
	"go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/sql"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
	web_frontend "go.skia.org/infra/golden/go/web/frontend"
)

// These are the later of the times of the last ingested data or last triage action for the
// given CL.
var changelistTSForIOS = time.Date(2020, time.December, 10, 5, 0, 2, 0, time.UTC)
var changelistTSForNewTests = time.Date(2020, time.December, 12, 9, 31, 32, 0, time.UTC)

func TestNewAndUntriagedSummaryForCL_OnePatchset_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 100)
	require.NoError(t, s.StartCacheProcess(ctx, time.Minute, 100))
	rv, err := s.NewAndUntriagedSummaryForCL(ctx, sql.Qualify(dks.GerritCRS, dks.ChangelistIDThatAttemptsToFixIOS))
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

	s := New(db, 100)
	require.NoError(t, s.StartCacheProcess(ctx, time.Minute, 100))
	rv, err := s.NewAndUntriagedSummaryForCL(ctx, sql.Qualify(dks.GerritInternalCRS, dks.ChangelistIDThatAddsNewTests))
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

	s := New(db, 100)
	require.NoError(t, s.StartCacheProcess(ctx, time.Minute, 100))
	rv, err := s.NewAndUntriagedSummaryForCL(ctx, sql.Qualify(dks.GerritCRS, clID))
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

	s := New(db, 100)
	require.NoError(t, s.StartCacheProcess(ctx, time.Minute, 100))
	_, err := s.NewAndUntriagedSummaryForCL(ctx, sql.Qualify(dks.GerritInternalCRS, "does not exist"))
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

	s := New(db, 100)
	require.NoError(t, s.StartCacheProcess(ctx, time.Minute, 100))
	rv, err := s.NewAndUntriagedSummaryForCL(ctx, sql.Qualify(dks.GerritCRS, clID))
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

	s := New(db, 100)
	require.NoError(t, s.StartCacheProcess(ctx, time.Minute, 100))
	rv, err := s.NewAndUntriagedSummaryForCL(ctx, sql.Qualify(dks.GerritCRS, clID))
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
		FromTryjob("tryjob 3", dks.BuildBucketCIS, "My-Test", "whatever", "2021-04-05T02:03:04Z")

	// The digest was triaged negative after data from PS 2 but before PS 3 was ingested. We want
	// to see that the latest data impacts the timestamp.
	cl.AddTriageEvent(dks.UserFour, "2021-04-03T00:00:00Z").
		ExpectationsForGrouping(paramtools.Params{
			types.CorpusField: dks.RoundCorpus, types.PrimaryKeyField: dks.CircleTest}).
		Negative(dks.DigestC07Unt_CL)

	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, b.Build()))

	s := New(db, 100)
	require.NoError(t, s.StartCacheProcess(ctx, time.Minute, 100))
	rv, err := s.NewAndUntriagedSummaryForCL(ctx, sql.Qualify(dks.GerritCRS, clID))
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
		LastUpdated: time.Date(2021, time.April, 5, 2, 3, 4, 0, time.UTC),
	}, rv)
}

func TestNewAndUntriagedSummaryForCL_MultipleThreadsAtOnce_NoRaces(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 100)
	// Update the caches aggressively to be writing to the shared cache while reading from it.
	require.NoError(t, s.StartCacheProcess(ctx, 100*time.Millisecond, 100))

	wg := sync.WaitGroup{}
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rv, err := s.NewAndUntriagedSummaryForCL(ctx, sql.Qualify(dks.GerritCRS, dks.ChangelistIDThatAttemptsToFixIOS))
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

func TestChangelistLastUpdated_ValidCL_ReturnsLatestTS(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	s := New(db, 100)
	ts, err := s.ChangelistLastUpdated(ctx, sql.Qualify(dks.GerritInternalCRS, dks.ChangelistIDThatAddsNewTests))
	require.NoError(t, err)
	assert.Equal(t, changelistTSForNewTests, ts)
}

func TestChangelistLastUpdated_NonExistantCL_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	s := New(db, 100)
	_, err := s.ChangelistLastUpdated(ctx, sql.Qualify(dks.GerritInternalCRS, "does not exist"))
	require.Error(t, err)
}

func TestSearch_UntriagedDigestsAtHead_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 100)
	res, err := s.Search(ctx, &query.Search{
		OnlyIncludeDigestsProducedAtHead: true,
		IncludePositiveDigests:           false,
		IncludeNegativeDigests:           false,
		IncludeUntriagedDigests:          true,
		Sort:                             query.SortDescending,
		IncludeIgnoredTraces:             false,
		TraceValues: paramtools.ParamSet{
			types.CorpusField: []string{dks.RoundCorpus},
		},
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
	})
	require.NoError(t, err)
	assert.Equal(t, &frontend.SearchResponse{
		Results: []*frontend.SearchResult{{
			Digest: dks.DigestC05Unt,
			Test:   dks.CircleTest,
			Status: expectations.Untriaged,
			ParamSet: paramtools.ParamSet{
				dks.ColorModeKey:      []string{dks.GreyColorMode, dks.RGBColorMode},
				types.CorpusField:     []string{dks.RoundCorpus},
				dks.DeviceKey:         []string{dks.IPadDevice, dks.IPhoneDevice},
				dks.OSKey:             []string{dks.IOS}, // Note: Android + Taimen are ignored
				types.PrimaryKeyField: []string{dks.CircleTest},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
					ID:            "0b61c8d85467fc95b1306128ceb2ef6d",
					DigestIndices: []int{-1, 2, -1, -1, 2, -1, -1, 0, -1, -1},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.GreyColorMode,
						types.CorpusField:     dks.RoundCorpus,
						dks.DeviceKey:         dks.IPhoneDevice,
						dks.OSKey:             dks.IOS,
						types.PrimaryKeyField: dks.CircleTest,
						"ext":                 "png",
					},
				}, {
					ID:            "22b530e029c22e396c5a24c0900c9ed5",
					DigestIndices: []int{1, -1, 1, -1, 1, -1, 1, -1, 0, -1},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.RoundCorpus,
						dks.DeviceKey:         dks.IPhoneDevice,
						dks.OSKey:             dks.IOS,
						types.PrimaryKeyField: dks.CircleTest,
						"ext":                 "png",
					},
				}, {
					ID:            "273119ca291863331e906fe71bde0e7d",
					DigestIndices: []int{1, 1, 1, 1, 1, 1, 1, 0, 0, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.RoundCorpus,
						dks.DeviceKey:         dks.IPadDevice,
						dks.OSKey:             dks.IOS,
						types.PrimaryKeyField: dks.CircleTest,
						"ext":                 "png",
					},
				}, {
					ID:            "3b44c31afc832ef9d1a2d25a5b873152",
					DigestIndices: []int{2, 2, 2, 2, 2, 2, 2, 0, 0, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.GreyColorMode,
						types.CorpusField:     dks.RoundCorpus,
						dks.DeviceKey:         dks.IPadDevice,
						dks.OSKey:             dks.IOS,
						types.PrimaryKeyField: dks.CircleTest,
						"ext":                 "png",
					},
				}},
				Digests: []frontend.DigestStatus{
					{Digest: dks.DigestC05Unt, Status: expectations.Untriaged},
					{Digest: dks.DigestC01Pos, Status: expectations.Positive},
					{Digest: dks.DigestC02Pos, Status: expectations.Positive},
				},
				TotalDigests: 3,
			},
			RefDiffs: map[common.RefClosest]*frontend.SRDiffDigest{
				common.PositiveRef: {
					CombinedMetric: 4.9783297, QueryMetric: 4.9783297, PixelDiffPercent: 68.75, NumDiffPixels: 44,
					MaxRGBADiffs: [4]int{40, 149, 100, 0},
					DimDiffer:    false,
					Digest:       dks.DigestC01Pos,
					Status:       expectations.Positive,
					ParamSet: paramtools.ParamSet{
						dks.ColorModeKey:      []string{dks.RGBColorMode},
						types.CorpusField:     []string{dks.RoundCorpus},
						dks.DeviceKey:         []string{dks.QuadroDevice, dks.IPadDevice, dks.IPhoneDevice, dks.WalleyeDevice},
						dks.OSKey:             []string{dks.AndroidOS, dks.Windows10dot2OS, dks.IOS},
						types.PrimaryKeyField: []string{dks.CircleTest},
						"ext":                 []string{"png"},
					},
				},
				common.NegativeRef: nil,
			},
			ClosestRef: common.PositiveRef,
		}, {
			Digest: dks.DigestC03Unt,
			Test:   dks.CircleTest,
			Status: expectations.Untriaged,
			ParamSet: paramtools.ParamSet{
				dks.ColorModeKey:      []string{dks.RGBColorMode},
				types.CorpusField:     []string{dks.RoundCorpus},
				dks.DeviceKey:         []string{dks.QuadroDevice},
				dks.OSKey:             []string{dks.Windows10dot3OS},
				types.PrimaryKeyField: []string{dks.CircleTest},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
					ID:            "9156c4774e7d90db488b6aadf416ff8e",
					DigestIndices: []int{-1, -1, -1, 0, 0, 0, 0, 0, 0, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.RoundCorpus,
						dks.DeviceKey:         dks.QuadroDevice,
						dks.OSKey:             dks.Windows10dot3OS,
						types.PrimaryKeyField: dks.CircleTest,
						"ext":                 "png",
					},
				}},
				Digests: []frontend.DigestStatus{{
					Digest: dks.DigestC03Unt, Status: expectations.Untriaged,
				}},
				TotalDigests: 1,
			},
			RefDiffs: map[common.RefClosest]*frontend.SRDiffDigest{
				common.PositiveRef: {
					CombinedMetric: 0.89245414, QueryMetric: 0.89245414, PixelDiffPercent: 50, NumDiffPixels: 32,
					MaxRGBADiffs: [4]int{1, 7, 4, 0},
					DimDiffer:    false,
					Digest:       dks.DigestC01Pos,
					Status:       expectations.Positive,
					ParamSet: paramtools.ParamSet{
						dks.ColorModeKey:      []string{dks.RGBColorMode},
						types.CorpusField:     []string{dks.RoundCorpus},
						dks.DeviceKey:         []string{dks.QuadroDevice, dks.IPadDevice, dks.IPhoneDevice, dks.WalleyeDevice},
						dks.OSKey:             []string{dks.AndroidOS, dks.Windows10dot2OS, dks.IOS},
						types.PrimaryKeyField: []string{dks.CircleTest},
						"ext":                 []string{"png"},
					},
				},
				common.NegativeRef: nil,
			},
			ClosestRef: common.PositiveRef,
		}, {
			Digest: dks.DigestC04Unt,
			Test:   dks.CircleTest,
			Status: expectations.Untriaged,
			ParamSet: paramtools.ParamSet{
				dks.ColorModeKey:      []string{dks.GreyColorMode},
				types.CorpusField:     []string{dks.RoundCorpus},
				dks.DeviceKey:         []string{dks.QuadroDevice},
				dks.OSKey:             []string{dks.Windows10dot3OS},
				types.PrimaryKeyField: []string{dks.CircleTest},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
					ID:            "0310e06f2b4c328cccbac480b5433390",
					DigestIndices: []int{-1, -1, -1, 0, 0, 0, 0, 0, 0, -1},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.GreyColorMode,
						types.CorpusField:     dks.RoundCorpus,
						dks.DeviceKey:         dks.QuadroDevice,
						dks.OSKey:             dks.Windows10dot3OS,
						types.PrimaryKeyField: dks.CircleTest,
						"ext":                 "png",
					},
				}},
				Digests: []frontend.DigestStatus{{
					Digest: dks.DigestC04Unt, Status: expectations.Untriaged,
				}},
				TotalDigests: 1,
			},
			RefDiffs: map[common.RefClosest]*frontend.SRDiffDigest{
				common.PositiveRef: {
					CombinedMetric: 0.17843534, QueryMetric: 0.17843534, PixelDiffPercent: 3.125, NumDiffPixels: 2,
					MaxRGBADiffs: [4]int{3, 3, 3, 0},
					DimDiffer:    false,
					Digest:       dks.DigestC02Pos,
					Status:       expectations.Positive,
					ParamSet: paramtools.ParamSet{
						dks.ColorModeKey:      []string{dks.GreyColorMode},
						types.CorpusField:     []string{dks.RoundCorpus},
						dks.DeviceKey:         []string{dks.QuadroDevice, dks.IPadDevice, dks.IPhoneDevice, dks.WalleyeDevice},
						dks.OSKey:             []string{dks.AndroidOS, dks.Windows10dot2OS, dks.IOS},
						types.PrimaryKeyField: []string{dks.CircleTest},
						"ext":                 []string{"png"},
					},
				},
				common.NegativeRef: nil,
			},
			ClosestRef: common.PositiveRef,
		}},
		Offset:  0,
		Size:    3,
		Commits: kitchenSinkCommits,
		BulkTriageData: web_frontend.TriageRequestData{
			dks.CircleTest: {
				dks.DigestC03Unt: expectations.Positive,
				dks.DigestC04Unt: expectations.Positive,
				dks.DigestC05Unt: expectations.Positive,
			},
		},
	}, res)
}

func TestSearch_IncludeIgnoredAtHead_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 100)
	res, err := s.Search(ctx, &query.Search{
		OnlyIncludeDigestsProducedAtHead: true,
		IncludePositiveDigests:           false,
		IncludeNegativeDigests:           false,
		IncludeUntriagedDigests:          true,
		Sort:                             query.SortDescending,
		IncludeIgnoredTraces:             true,
		TraceValues: paramtools.ParamSet{
			types.CorpusField: []string{dks.RoundCorpus},
		},
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
	})
	require.NoError(t, err)
	require.Len(t, res.Results, 3)
	assert.Equal(t, frontend.TraceGroup{
		Traces: []frontend.Trace{{
			ID:            "0b61c8d85467fc95b1306128ceb2ef6d",
			DigestIndices: []int{-1, 2, -1, -1, 2, -1, -1, 0, -1, -1},
			Params: paramtools.Params{
				dks.ColorModeKey:      dks.GreyColorMode,
				types.CorpusField:     dks.RoundCorpus,
				dks.DeviceKey:         dks.IPhoneDevice,
				dks.OSKey:             dks.IOS,
				types.PrimaryKeyField: dks.CircleTest,
				"ext":                 "png",
			},
		}, {
			ID:            "22b530e029c22e396c5a24c0900c9ed5",
			DigestIndices: []int{1, -1, 1, -1, 1, -1, 1, -1, 0, -1},
			Params: paramtools.Params{
				dks.ColorModeKey:      dks.RGBColorMode,
				types.CorpusField:     dks.RoundCorpus,
				dks.DeviceKey:         dks.IPhoneDevice,
				dks.OSKey:             dks.IOS,
				types.PrimaryKeyField: dks.CircleTest,
				"ext":                 "png",
			},
		}, {
			ID:            "273119ca291863331e906fe71bde0e7d",
			DigestIndices: []int{1, 1, 1, 1, 1, 1, 1, 0, 0, 0},
			Params: paramtools.Params{
				dks.ColorModeKey:      dks.RGBColorMode,
				types.CorpusField:     dks.RoundCorpus,
				dks.DeviceKey:         dks.IPadDevice,
				dks.OSKey:             dks.IOS,
				types.PrimaryKeyField: dks.CircleTest,
				"ext":                 "png",
			},
		}, {
			ID:            "3b44c31afc832ef9d1a2d25a5b873152",
			DigestIndices: []int{2, 2, 2, 2, 2, 2, 2, 0, 0, 0},
			Params: paramtools.Params{
				dks.ColorModeKey:      dks.GreyColorMode,
				types.CorpusField:     dks.RoundCorpus,
				dks.DeviceKey:         dks.IPadDevice,
				dks.OSKey:             dks.IOS,
				types.PrimaryKeyField: dks.CircleTest,
				"ext":                 "png",
			},
		}, {
			// This trace matches an ignore rule, but should be visible due to the search
			// terms
			ID:            "902ac9eee937cd11b4ccc81d535ff33f",
			DigestIndices: []int{-1, -1, -1, -1, -1, -1, 0, 0, 0, 0},
			Params: paramtools.Params{
				dks.ColorModeKey:      dks.RGBColorMode,
				types.CorpusField:     dks.RoundCorpus,
				dks.DeviceKey:         dks.TaimenDevice,
				dks.OSKey:             dks.AndroidOS,
				types.PrimaryKeyField: dks.CircleTest,
				"ext":                 "png",
			},
		}},
		Digests: []frontend.DigestStatus{
			{Digest: dks.DigestC05Unt, Status: expectations.Untriaged},
			{Digest: dks.DigestC01Pos, Status: expectations.Positive},
			{Digest: dks.DigestC02Pos, Status: expectations.Positive},
		},
		TotalDigests: 3,
	}, res.Results[0].TraceGroup)
	assert.Equal(t, paramtools.ParamSet{
		dks.ColorModeKey:      []string{dks.GreyColorMode, dks.RGBColorMode},
		types.CorpusField:     []string{dks.RoundCorpus},
		dks.DeviceKey:         []string{dks.IPadDevice, dks.IPhoneDevice, dks.TaimenDevice},
		dks.OSKey:             []string{dks.AndroidOS, dks.IOS},
		types.PrimaryKeyField: []string{dks.CircleTest},
		"ext":                 []string{"png"},
	}, res.Results[0].ParamSet)
	// Other two results should be the same as they didn't have any ignored data. We spot-check
	// that here.
	assert.Equal(t, paramtools.ParamSet{
		dks.ColorModeKey:      []string{dks.RGBColorMode},
		types.CorpusField:     []string{dks.RoundCorpus},
		dks.DeviceKey:         []string{dks.QuadroDevice},
		dks.OSKey:             []string{dks.Windows10dot3OS},
		types.PrimaryKeyField: []string{dks.CircleTest},
		"ext":                 []string{"png"},
	}, res.Results[1].ParamSet)
	assert.Equal(t, paramtools.ParamSet{
		dks.ColorModeKey:      []string{dks.GreyColorMode},
		types.CorpusField:     []string{dks.RoundCorpus},
		dks.DeviceKey:         []string{dks.QuadroDevice},
		dks.OSKey:             []string{dks.Windows10dot3OS},
		types.PrimaryKeyField: []string{dks.CircleTest},
		"ext":                 []string{"png"},
	}, res.Results[2].ParamSet)
}

func TestSearch_RespectMinMaxRGBAFilter_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 100)
	res, err := s.Search(ctx, &query.Search{
		OnlyIncludeDigestsProducedAtHead: true,
		IncludePositiveDigests:           false,
		IncludeNegativeDigests:           false,
		IncludeUntriagedDigests:          true,
		Sort:                             query.SortDescending,
		IncludeIgnoredTraces:             false,
		TraceValues: paramtools.ParamSet{
			types.CorpusField: []string{dks.RoundCorpus},
		},
		RGBAMinFilter: 4,
		RGBAMaxFilter: 20,
	})
	require.NoError(t, err)
	// The other two results are removed because they are above or below the filter.
	assert.Equal(t, &frontend.SearchResponse{
		Results: []*frontend.SearchResult{{
			Digest: dks.DigestC03Unt,
			Test:   dks.CircleTest,
			Status: expectations.Untriaged,
			ParamSet: paramtools.ParamSet{
				dks.ColorModeKey:      []string{dks.RGBColorMode},
				types.CorpusField:     []string{dks.RoundCorpus},
				dks.DeviceKey:         []string{dks.QuadroDevice},
				dks.OSKey:             []string{dks.Windows10dot3OS},
				types.PrimaryKeyField: []string{dks.CircleTest},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
					ID:            "9156c4774e7d90db488b6aadf416ff8e",
					DigestIndices: []int{-1, -1, -1, 0, 0, 0, 0, 0, 0, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.RoundCorpus,
						dks.DeviceKey:         dks.QuadroDevice,
						dks.OSKey:             dks.Windows10dot3OS,
						types.PrimaryKeyField: dks.CircleTest,
						"ext":                 "png",
					},
				}},
				Digests: []frontend.DigestStatus{{
					Digest: dks.DigestC03Unt, Status: expectations.Untriaged,
				}},
				TotalDigests: 1,
			},
			RefDiffs: map[common.RefClosest]*frontend.SRDiffDigest{
				common.PositiveRef: {
					CombinedMetric: 0.89245414, QueryMetric: 0.89245414, PixelDiffPercent: 50, NumDiffPixels: 32,
					MaxRGBADiffs: [4]int{1, 7, 4, 0},
					DimDiffer:    false,
					Digest:       dks.DigestC01Pos,
					Status:       expectations.Positive,
					ParamSet: paramtools.ParamSet{
						dks.ColorModeKey:      []string{dks.RGBColorMode},
						types.CorpusField:     []string{dks.RoundCorpus},
						dks.DeviceKey:         []string{dks.QuadroDevice, dks.IPadDevice, dks.IPhoneDevice, dks.WalleyeDevice},
						dks.OSKey:             []string{dks.AndroidOS, dks.Windows10dot2OS, dks.IOS},
						types.PrimaryKeyField: []string{dks.CircleTest},
						"ext":                 []string{"png"},
					},
				},
				common.NegativeRef: nil,
			},
			ClosestRef: common.PositiveRef,
		}},
		Offset:  0,
		Size:    1,
		Commits: kitchenSinkCommits,
		BulkTriageData: web_frontend.TriageRequestData{
			dks.CircleTest: {
				dks.DigestC03Unt: expectations.Positive,
			},
		},
	}, res)
}

func TestSearch_RespectLimitOffsetOrder_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 100)
	res, err := s.Search(ctx, &query.Search{
		OnlyIncludeDigestsProducedAtHead: true,
		IncludePositiveDigests:           true,
		IncludeNegativeDigests:           false,
		IncludeUntriagedDigests:          false,
		Sort:                             query.SortAscending,
		IncludeIgnoredTraces:             false,
		TraceValues: paramtools.ParamSet{
			types.CorpusField: []string{dks.CornersCorpus},
		},
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
		Offset:        3, // Carefully selected to return one result from square and triangle each.
		Limit:         2,
	})
	require.NoError(t, err)
	assert.Equal(t, &frontend.SearchResponse{
		Results: []*frontend.SearchResult{{
			Digest: dks.DigestA08Pos,
			Test:   dks.SquareTest,
			Status: expectations.Positive,
			ParamSet: paramtools.ParamSet{
				dks.ColorModeKey:             []string{dks.RGBColorMode},
				types.CorpusField:            []string{dks.CornersCorpus},
				dks.DeviceKey:                []string{dks.WalleyeDevice},
				dks.OSKey:                    []string{dks.AndroidOS},
				types.PrimaryKeyField:        []string{dks.SquareTest},
				"ext":                        []string{"png"},
				"image_matching_algorithm":   []string{"fuzzy"},
				"fuzzy_max_different_pixels": []string{"2"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
					ID:            "4686a4134535ad178b67325f5f2f613a",
					DigestIndices: []int{-1, -1, -1, -1, -1, 4, 3, 2, 1, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:             dks.RGBColorMode,
						types.CorpusField:            dks.CornersCorpus,
						dks.DeviceKey:                dks.WalleyeDevice,
						dks.OSKey:                    dks.AndroidOS,
						types.PrimaryKeyField:        dks.SquareTest,
						"ext":                        "png",
						"image_matching_algorithm":   "fuzzy",
						"fuzzy_max_different_pixels": "2",
					},
				}},
				Digests: []frontend.DigestStatus{
					{Digest: dks.DigestA08Pos, Status: expectations.Positive},
					{Digest: dks.DigestA07Pos, Status: expectations.Positive},
					{Digest: dks.DigestA06Unt, Status: expectations.Untriaged},
					{Digest: dks.DigestA01Pos, Status: expectations.Positive},
					{Digest: dks.DigestA05Unt, Status: expectations.Untriaged},
				},
				TotalDigests: 5,
			},
			RefDiffs: map[common.RefClosest]*frontend.SRDiffDigest{
				common.PositiveRef: {
					CombinedMetric: 0.15655607, QueryMetric: 0.15655607, PixelDiffPercent: 3.125, NumDiffPixels: 2,
					MaxRGBADiffs: [4]int{4, 0, 0, 0},
					DimDiffer:    false,
					Digest:       dks.DigestA01Pos,
					Status:       expectations.Positive,
					ParamSet: paramtools.ParamSet{
						dks.ColorModeKey:             []string{dks.RGBColorMode},
						types.CorpusField:            []string{dks.CornersCorpus},
						dks.DeviceKey:                []string{dks.QuadroDevice, dks.IPadDevice, dks.IPhoneDevice, dks.TaimenDevice, dks.WalleyeDevice},
						dks.OSKey:                    []string{dks.AndroidOS, dks.Windows10dot2OS, dks.Windows10dot3OS, dks.IOS},
						types.PrimaryKeyField:        []string{dks.SquareTest},
						"ext":                        []string{"png"},
						"image_matching_algorithm":   []string{"fuzzy"},
						"fuzzy_max_different_pixels": []string{"2"},
					},
				},
				common.NegativeRef: {
					CombinedMetric: 10, QueryMetric: 10, PixelDiffPercent: 100, NumDiffPixels: 64,
					MaxRGBADiffs: [4]int{255, 255, 255, 255},
					DimDiffer:    false,
					Digest:       dks.DigestA09Neg,
					Status:       expectations.Negative,
					// Even though this is ignored, we are free to show it on the right side
					// (just not a part of the actual results).
					ParamSet: paramtools.ParamSet{
						dks.ColorModeKey:      []string{dks.RGBColorMode},
						types.CorpusField:     []string{dks.CornersCorpus},
						dks.DeviceKey:         []string{dks.TaimenDevice},
						dks.OSKey:             []string{dks.AndroidOS},
						types.PrimaryKeyField: []string{dks.SquareTest},
						"ext":                 []string{"png"},
					},
				},
			},
			ClosestRef: common.PositiveRef,
		}, {
			Digest: dks.DigestB01Pos,
			Test:   dks.TriangleTest,
			Status: expectations.Positive,
			ParamSet: paramtools.ParamSet{
				dks.ColorModeKey:  []string{dks.RGBColorMode},
				types.CorpusField: []string{dks.CornersCorpus},
				// Of note - this test is *not* ignored for the taimen device
				dks.DeviceKey:         []string{dks.QuadroDevice, dks.IPadDevice, dks.IPhoneDevice, dks.TaimenDevice, dks.WalleyeDevice},
				dks.OSKey:             []string{dks.AndroidOS, dks.Windows10dot2OS, dks.Windows10dot3OS, dks.IOS},
				types.PrimaryKeyField: []string{dks.TriangleTest},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
					ID:            "1a16cbc8805378f0a6ef654a035d86c4",
					DigestIndices: []int{-1, -1, -1, -1, -1, -1, 0, 0, 0, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.TaimenDevice,
						dks.OSKey:             dks.AndroidOS,
						types.PrimaryKeyField: dks.TriangleTest,
						"ext":                 "png",
					},
				}, {
					ID:            "555f149dfe944816076a57c633578dbc",
					DigestIndices: []int{-1, -1, -1, 0, 0, 0, 0, 0, 0, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.QuadroDevice,
						dks.OSKey:             dks.Windows10dot3OS,
						types.PrimaryKeyField: dks.TriangleTest,
						"ext":                 "png",
					},
				}, {
					ID:            "7346d80b7d5d1087fd61ae40098f4277",
					DigestIndices: []int{2, 0, 0, -1, -1, -1, -1, -1, -1, -1},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.QuadroDevice,
						dks.OSKey:             dks.Windows10dot2OS,
						types.PrimaryKeyField: dks.TriangleTest,
						"ext":                 "png",
					},
				}, {
					ID:            "760c2db998331eafd3023f4b6d135b06",
					DigestIndices: []int{1, -1, 2, -1, 2, -1, 1, -1, 0, -1},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.IPhoneDevice,
						dks.OSKey:             dks.IOS,
						types.PrimaryKeyField: dks.TriangleTest,
						"ext":                 "png",
					},
				}, {
					ID:            "8fe41dfab19e0a291f37964416432128",
					DigestIndices: []int{1, 1, 2, 1, 1, 2, 1, 0, 0, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.IPadDevice,
						dks.OSKey:             dks.IOS,
						types.PrimaryKeyField: dks.TriangleTest,
						"ext":                 "png",
					},
				}, {
					ID:            "ab734e10b7aed9d06a91f46d14746270",
					DigestIndices: []int{-1, -1, -1, -1, -1, 0, 0, 0, 0, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.WalleyeDevice,
						dks.OSKey:             dks.AndroidOS,
						types.PrimaryKeyField: dks.TriangleTest,
						"ext":                 "png",
					},
				}},
				Digests: []frontend.DigestStatus{
					{Digest: dks.DigestB01Pos, Status: expectations.Positive},
					{Digest: dks.DigestB03Neg, Status: expectations.Negative},
					{Digest: dks.DigestBlank, Status: expectations.Untriaged},
				},
				TotalDigests: 3,
			},
			RefDiffs: map[common.RefClosest]*frontend.SRDiffDigest{
				common.PositiveRef: {
					CombinedMetric: 1.9362538, QueryMetric: 1.9362538, PixelDiffPercent: 43.75, NumDiffPixels: 28,
					MaxRGBADiffs: [4]int{11, 5, 42, 0},
					DimDiffer:    false,
					Digest:       dks.DigestB02Pos,
					Status:       expectations.Positive,
					ParamSet: paramtools.ParamSet{
						dks.ColorModeKey:      []string{dks.GreyColorMode},
						types.CorpusField:     []string{dks.CornersCorpus},
						dks.DeviceKey:         []string{dks.QuadroDevice, dks.IPadDevice, dks.IPhoneDevice, dks.WalleyeDevice},
						dks.OSKey:             []string{dks.AndroidOS, dks.Windows10dot2OS, dks.Windows10dot3OS, dks.IOS},
						types.PrimaryKeyField: []string{dks.TriangleTest},
						"ext":                 []string{"png"},
					},
				},
				common.NegativeRef: {
					CombinedMetric: 2.9445405, QueryMetric: 2.9445405, PixelDiffPercent: 10.9375, NumDiffPixels: 7,
					MaxRGBADiffs: [4]int{250, 244, 197, 51},
					DimDiffer:    false,
					Digest:       dks.DigestB03Neg,
					Status:       expectations.Negative,
					ParamSet: paramtools.ParamSet{
						dks.ColorModeKey:      []string{dks.RGBColorMode},
						types.CorpusField:     []string{dks.CornersCorpus},
						dks.DeviceKey:         []string{dks.IPadDevice, dks.IPhoneDevice},
						dks.OSKey:             []string{dks.IOS},
						types.PrimaryKeyField: []string{dks.TriangleTest},
						"ext":                 []string{"png"},
					},
				},
			},
			ClosestRef: common.PositiveRef,
		}},
		Offset:  3,
		Size:    6,
		Commits: kitchenSinkCommits,
		BulkTriageData: web_frontend.TriageRequestData{
			dks.SquareTest: {
				dks.DigestA01Pos: expectations.Positive,
				dks.DigestA02Pos: expectations.Positive,
				dks.DigestA03Pos: expectations.Positive,
				dks.DigestA08Pos: expectations.Positive,
			}, dks.TriangleTest: {
				dks.DigestB02Pos: expectations.Positive,
				dks.DigestB01Pos: expectations.Positive,
			},
		},
	}, res)
}

func TestMakeTraceGroup_TwoMostlyStableTraces_Success(t *testing.T) {
	unittest.SmallTest(t)

	ctx := context.WithValue(context.Background(), commitToIdxKey, map[schema.CommitID]int{
		"10": 0,
		"11": 1,
		"12": 2,
		"17": 3,
		"20": 4,
	})
	ctx = context.WithValue(ctx, actualWindowLengthKey, 5)
	inputData := []traceDigestCommit{
		{traceID: schema.TraceID{0xaa}, commitID: "10", digest: dks.DigestA01Pos},
		{traceID: schema.TraceID{0xaa}, commitID: "11", digest: dks.DigestA01Pos},
		{traceID: schema.TraceID{0xaa}, commitID: "12", digest: dks.DigestA01Pos},
		{traceID: schema.TraceID{0xaa}, commitID: "13", digest: dks.DigestA01Pos},
		{traceID: schema.TraceID{0xaa}, commitID: "20", digest: dks.DigestA01Pos},

		{traceID: schema.TraceID{0xbb}, commitID: "10", digest: dks.DigestA05Unt},
		{traceID: schema.TraceID{0xbb}, commitID: "12", digest: dks.DigestA01Pos},
		{traceID: schema.TraceID{0xbb}, commitID: "17", digest: dks.DigestA05Unt},
		{traceID: schema.TraceID{0xbb}, commitID: "20", digest: dks.DigestA01Pos},
	}

	tg, err := makeTraceGroup(ctx, inputData, dks.DigestA01Pos)
	require.NoError(t, err)
	for i := range tg.Traces {
		tg.Traces[i].RawTrace = nil // We don't want to do assertions on this.
	}
	assert.Equal(t, frontend.TraceGroup{
		TotalDigests: 2, // saw 2 distinct digests
		Digests: []frontend.DigestStatus{
			{Digest: dks.DigestA01Pos},
			{Digest: dks.DigestA05Unt},
		},
		Traces: []frontend.Trace{{
			ID:            "aa",
			DigestIndices: []int{0, 0, 0, -1, 0},
		}, {
			ID:            "bb",
			DigestIndices: []int{1, -1, 0, 1, 0},
		}},
	}, tg)
}

func TestMakeTraceGroup_OneFlakyTrace_PrioritizeShowingMostUsedDigests(t *testing.T) {
	unittest.SmallTest(t)

	ctx := context.WithValue(context.Background(), commitToIdxKey, map[schema.CommitID]int{
		"10": 0,
		"11": 1,
		"12": 2,
		"17": 3,
		"19": 4,
		"20": 5,
		"21": 6,
		"22": 7,
		"23": 8,
		"24": 9,
		"25": 10,
		"26": 11,
		"27": 12,
		"28": 13,
		"29": 14,
		"30": 15,
		"31": 16,
		"32": 17,
	})
	ctx = context.WithValue(ctx, actualWindowLengthKey, 18)
	inputData := []traceDigestCommit{
		{traceID: schema.TraceID{0xaa}, commitID: "10", digest: "dC"},
		{traceID: schema.TraceID{0xaa}, commitID: "11", digest: "dC"},
		{traceID: schema.TraceID{0xaa}, commitID: "12", digest: "dC"},
		{traceID: schema.TraceID{0xaa}, commitID: "17", digest: "dB"},
		{traceID: schema.TraceID{0xaa}, commitID: "20", digest: "dB"},
		{traceID: schema.TraceID{0xaa}, commitID: "21", digest: "dA"},
		{traceID: schema.TraceID{0xaa}, commitID: "22", digest: "d9"},
		{traceID: schema.TraceID{0xaa}, commitID: "23", digest: "d8"},
		{traceID: schema.TraceID{0xaa}, commitID: "24", digest: "d7"},
		{traceID: schema.TraceID{0xaa}, commitID: "25", digest: "d6"},
		{traceID: schema.TraceID{0xaa}, commitID: "26", digest: "d6"},
		{traceID: schema.TraceID{0xaa}, commitID: "27", digest: "d5"},
		{traceID: schema.TraceID{0xaa}, commitID: "28", digest: "d4"},
		{traceID: schema.TraceID{0xaa}, commitID: "29", digest: "d3"},
		{traceID: schema.TraceID{0xaa}, commitID: "30", digest: "d2"},
		{traceID: schema.TraceID{0xaa}, commitID: "31", digest: "d1"},
		{traceID: schema.TraceID{0xaa}, commitID: "32", digest: "d0"},
	}

	tg, err := makeTraceGroup(ctx, inputData, "d0")
	require.NoError(t, err)
	for i := range tg.Traces {
		tg.Traces[i].RawTrace = nil // We don't want to do assertions on this.
	}
	assert.Equal(t, frontend.TraceGroup{
		TotalDigests: 13, // saw 13 distinct digests
		Digests: []frontend.DigestStatus{
			{Digest: "d0"},
			{Digest: "d1"},
			{Digest: "d2"},
			{Digest: "d3"},
			{Digest: "dC"},
			{Digest: "d6"},
			{Digest: "dB"},
			{Digest: "d4"},
			{Digest: "d5"}, // All others combined with this one
		},
		Traces: []frontend.Trace{{
			ID:            "aa",
			DigestIndices: []int{4, 4, 4, 6, -1, 6, 8, 8, 8, 8, 5, 5, 8, 7, 3, 2, 1, 0},
		}},
	}, tg)
}

var kitchenSinkCommits = makeKitchenSinkCommits()

func makeKitchenSinkCommits() []web_frontend.Commit {
	data := dks.Build()
	convert := func(row schema.GitCommitRow) web_frontend.Commit {
		return web_frontend.Commit{
			CommitTime: row.CommitTime.Unix(),
			Hash:       row.GitHash,
			Author:     row.AuthorEmail,
			Subject:    row.Subject,
		}
	}
	return []web_frontend.Commit{
		convert(data.GitCommits[0]),
		convert(data.GitCommits[1]),
		convert(data.GitCommits[2]),
		convert(data.GitCommits[3]),
		convert(data.GitCommits[4]), // There are 3 commits w/o data here
		convert(data.GitCommits[8]),
		convert(data.GitCommits[9]),
		convert(data.GitCommits[10]),
		convert(data.GitCommits[11]),
		convert(data.GitCommits[12]),
	}
}

// waitForSystemTime waits for a time greater than the duration mentioned in "AS OF SYSTEM TIME"
// clauses in queries. This way, the queries will be accurate.
func waitForSystemTime() {
	time.Sleep(150 * time.Millisecond)
}
