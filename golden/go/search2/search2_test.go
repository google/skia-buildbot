package search2

import (
	"context"
	"crypto/md5"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/publicparams"
	"go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/databuilder"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/web/frontend"
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

func TestChangelistLastUpdated_NonExistentCL_ReturnsZeroTime(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	s := New(db, 100)
	ts, err := s.ChangelistLastUpdated(ctx, sql.Qualify(dks.GerritInternalCRS, "does not exist"))
	require.NoError(t, err)
	assert.True(t, ts.IsZero())
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
	assertUntriagedDigestsAtHead(t, res)
}

func TestSearch_UntriagedDigestsAtHead_WithMaterializedViews(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 10) // Otherwise there's no commit for the materialized views
	require.NoError(t, s.StartMaterializedViews(ctx, []string{dks.CornersCorpus, dks.RoundCorpus}, time.Minute))
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
	assertUntriagedDigestsAtHead(t, res)
}

func assertUntriagedDigestsAtHead(t *testing.T, res *frontend.SearchResponse) {
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
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
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
				frontend.NegativeRef: nil,
			},
			ClosestRef: frontend.PositiveRef,
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
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
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
				frontend.NegativeRef: nil,
			},
			ClosestRef: frontend.PositiveRef,
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
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
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
				frontend.NegativeRef: nil,
			},
			ClosestRef: frontend.PositiveRef,
		}},
		Offset:  0,
		Size:    3,
		Commits: kitchenSinkCommits,
		BulkTriageData: frontend.TriageRequestData{
			dks.CircleTest: {
				dks.DigestC03Unt: expectations.Positive,
				dks.DigestC04Unt: expectations.Positive,
				dks.DigestC05Unt: expectations.Positive,
			},
		},
	}, res)
}

func TestStartMaterializedViews_ViewsAreNonEmpty(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 10)
	require.NoError(t, s.StartMaterializedViews(ctx, []string{dks.CornersCorpus, dks.RoundCorpus}, time.Minute))

	assertNumRows(t, db, "mv_corners_traces", 21)
	assertNumRows(t, db, "mv_round_traces", 10)
}

func TestStartMaterializedViews_ViewsGetUpdated(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	s := New(db, 10)
	require.NoError(t, s.StartMaterializedViews(ctx, []string{dks.CornersCorpus, dks.RoundCorpus}, time.Second))
	// no data yet
	assertNumRows(t, db, "mv_corners_traces", 0)
	assertNumRows(t, db, "mv_round_traces", 0)

	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	time.Sleep(3 * time.Second) // wait for refresh to happen

	assertNumRows(t, db, "mv_corners_traces", 21)
	assertNumRows(t, db, "mv_round_traces", 10)
}

func assertNumRows(t *testing.T, db *pgxpool.Pool, tableName string, rowCount int) {
	row := db.QueryRow(context.Background(), `SELECT COUNT(*) FROM `+tableName)
	var count int
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, rowCount, count)
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
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
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
				frontend.NegativeRef: nil,
			},
			ClosestRef: frontend.PositiveRef,
		}},
		Offset:  0,
		Size:    1,
		Commits: kitchenSinkCommits,
		BulkTriageData: frontend.TriageRequestData{
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
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
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
				frontend.NegativeRef: {
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
			ClosestRef: frontend.PositiveRef,
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
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
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
				frontend.NegativeRef: {
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
			ClosestRef: frontend.PositiveRef,
		}},
		Offset:  3,
		Size:    6,
		Commits: kitchenSinkCommits,
		BulkTriageData: frontend.TriageRequestData{
			dks.SquareTest: {
				dks.DigestA01Pos: expectations.Positive,
				dks.DigestA02Pos: expectations.Positive,
				dks.DigestA03Pos: expectations.Positive,
				dks.DigestA08Pos: expectations.Positive,
			}, dks.TriangleTest: {
				dks.DigestB01Pos: expectations.Positive,
				dks.DigestB02Pos: expectations.Positive,
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
	inputData := map[schema.MD5Hash][]traceDigestCommit{
		{0xaa}: {
			{commitID: "10", digest: dks.DigestA01Pos},
			{commitID: "11", digest: dks.DigestA01Pos},
			{commitID: "12", digest: dks.DigestA01Pos},
			{commitID: "13", digest: dks.DigestA01Pos},
			{commitID: "20", digest: dks.DigestA01Pos},
		},
		{0xbb}: {
			{commitID: "10", digest: dks.DigestA05Unt},
			{commitID: "12", digest: dks.DigestA01Pos},
			{commitID: "17", digest: dks.DigestA05Unt},
			{commitID: "20", digest: dks.DigestA01Pos},
		},
	}

	tg, err := makeTraceGroup(ctx, inputData, dks.DigestA01Pos)
	require.NoError(t, err)
	for i := range tg.Traces {
		tg.Traces[i].RawTrace = nil // We don't want to do assertions on this.
	}
	assert.Equal(t, frontend.TraceGroup{
		TotalDigests: 2, // saw 2 distinct digests
		Digests: []frontend.DigestStatus{
			// We default these to untriaged. The actual values will be applied in a later step.
			{Digest: dks.DigestA01Pos, Status: expectations.Untriaged},
			{Digest: dks.DigestA05Unt, Status: expectations.Untriaged},
		},
		Traces: []frontend.Trace{{
			ID:            "aa000000000000000000000000000000",
			DigestIndices: []int{0, 0, 0, -1, 0},
		}, {
			ID:            "bb000000000000000000000000000000",
			DigestIndices: []int{1, -1, 0, 1, 0},
		}},
	}, tg)
}

func TestMakeTraceGroup_TwoNewTracesInCL_DataAppended(t *testing.T) {
	unittest.SmallTest(t)

	ctx := context.WithValue(context.Background(), commitToIdxKey, map[schema.CommitID]int{
		"10": 0,
		"11": 1,
		"12": 2,
		"17": 3,
		"20": 4,
	})
	ctx = context.WithValue(ctx, actualWindowLengthKey, 5)
	ctx = context.WithValue(ctx, qualifiedCLIDKey, "whatever") // indicate this is a CL
	inputData := map[schema.MD5Hash][]traceDigestCommit{
		{0xaa}: nil,
		{0xbb}: nil,
	}

	tg, err := makeTraceGroup(ctx, inputData, dks.DigestA01Pos)
	require.NoError(t, err)
	for i := range tg.Traces {
		tg.Traces[i].RawTrace = nil // We don't want to do assertions on this.
	}
	assert.Equal(t, frontend.TraceGroup{
		TotalDigests: 1,
		Digests: []frontend.DigestStatus{
			// We default these to untriaged. The actual values will be applied in a later step.
			{Digest: dks.DigestA01Pos, Status: expectations.Untriaged},
		},
		Traces: []frontend.Trace{{
			ID: "aa000000000000000000000000000000",
			// There are the 5 missing values for the primary commits (because no data was
			// supplied for those) and then the primary value attached to the end because this is
			// a CL.
			DigestIndices: []int{-1, -1, -1, -1, -1, 0},
		}, {
			ID:            "bb000000000000000000000000000000",
			DigestIndices: []int{-1, -1, -1, -1, -1, 0},
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
	inputData := map[schema.MD5Hash][]traceDigestCommit{
		{0xaa}: {
			{commitID: "10", digest: "dC"},
			{commitID: "11", digest: "dC"},
			{commitID: "12", digest: "dC"},
			{commitID: "17", digest: "dB"},
			{commitID: "20", digest: "dB"},
			{commitID: "21", digest: "dA"},
			{commitID: "22", digest: "d9"},
			{commitID: "23", digest: "d8"},
			{commitID: "24", digest: "d7"},
			{commitID: "25", digest: "d6"},
			{commitID: "26", digest: "d6"},
			{commitID: "27", digest: "d5"},
			{commitID: "28", digest: "d4"},
			{commitID: "29", digest: "d3"},
			{commitID: "30", digest: "d2"},
			{commitID: "31", digest: "d1"},
			{commitID: "32", digest: "d0"},
		},
	}

	tg, err := makeTraceGroup(ctx, inputData, "d0")
	require.NoError(t, err)
	for i := range tg.Traces {
		tg.Traces[i].RawTrace = nil // We don't want to do assertions on this.
	}
	assert.Equal(t, frontend.TraceGroup{
		TotalDigests: 13, // saw 13 distinct digests
		Digests: []frontend.DigestStatus{
			{Digest: "d0", Status: expectations.Untriaged},
			{Digest: "d1", Status: expectations.Untriaged},
			{Digest: "d2", Status: expectations.Untriaged},
			{Digest: "d3", Status: expectations.Untriaged},
			{Digest: "dC", Status: expectations.Untriaged},
			{Digest: "d6", Status: expectations.Untriaged},
			{Digest: "dB", Status: expectations.Untriaged},
			{Digest: "d4", Status: expectations.Untriaged},
			{Digest: "d5", Status: expectations.Untriaged}, // All others combined with this one
		},
		Traces: []frontend.Trace{{
			ID:            "aa000000000000000000000000000000",
			DigestIndices: []int{4, 4, 4, 6, -1, 6, 8, 8, 8, 8, 5, 5, 8, 7, 3, 2, 1, 0},
		}},
	}, tg)
}

func TestSearch_FilterLeftSideByKeys_Success(t *testing.T) {
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
		Sort:                             query.SortDescending,
		IncludeIgnoredTraces:             false,
		TraceValues: paramtools.ParamSet{
			types.CorpusField:     []string{dks.CornersCorpus},
			types.PrimaryKeyField: []string{dks.TriangleTest},
			dks.DeviceKey:         []string{dks.WalleyeDevice, dks.QuadroDevice},
		},
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
	})
	require.NoError(t, err)
	assertFilterLeftSideByKeys(t, res)
}

func TestSearch_FilterLeftSideByKeys_WithMaterializedViews(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 10) // Otherwise there's no commit for the materialized views
	require.NoError(t, s.StartMaterializedViews(ctx, []string{dks.CornersCorpus, dks.RoundCorpus}, time.Minute))
	res, err := s.Search(ctx, &query.Search{
		OnlyIncludeDigestsProducedAtHead: true,
		IncludePositiveDigests:           true,
		IncludeNegativeDigests:           false,
		IncludeUntriagedDigests:          false,
		Sort:                             query.SortDescending,
		IncludeIgnoredTraces:             false,
		TraceValues: paramtools.ParamSet{
			types.CorpusField:     []string{dks.CornersCorpus},
			types.PrimaryKeyField: []string{dks.TriangleTest},
			dks.DeviceKey:         []string{dks.WalleyeDevice, dks.QuadroDevice},
		},
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
	})
	require.NoError(t, err)
	assertFilterLeftSideByKeys(t, res)
}

func assertFilterLeftSideByKeys(t *testing.T, res *frontend.SearchResponse) {
	assert.Equal(t, &frontend.SearchResponse{
		Results: []*frontend.SearchResult{{
			Digest: dks.DigestB01Pos,
			Test:   dks.TriangleTest,
			Status: expectations.Positive,
			ParamSet: paramtools.ParamSet{
				dks.ColorModeKey:  []string{dks.RGBColorMode},
				types.CorpusField: []string{dks.CornersCorpus},
				// Notice these are just the devices from the filters, not all devices that
				// drew this digest.
				dks.DeviceKey:         []string{dks.QuadroDevice, dks.WalleyeDevice},
				dks.OSKey:             []string{dks.AndroidOS, dks.Windows10dot2OS, dks.Windows10dot3OS},
				types.PrimaryKeyField: []string{dks.TriangleTest},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
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
					DigestIndices: []int{1, 0, 0, -1, -1, -1, -1, -1, -1, -1},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.QuadroDevice,
						dks.OSKey:             dks.Windows10dot2OS,
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
					{Digest: dks.DigestBlank, Status: expectations.Untriaged},
				},
				TotalDigests: 2,
			},
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
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
				frontend.NegativeRef: {
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
			ClosestRef: frontend.PositiveRef,
		}, {
			Digest: dks.DigestB02Pos,
			Test:   dks.TriangleTest,
			Status: expectations.Positive,
			ParamSet: paramtools.ParamSet{
				dks.ColorModeKey:  []string{dks.GreyColorMode},
				types.CorpusField: []string{dks.CornersCorpus},
				// Notice these are just the devices from the filters, not all devices that
				// drew this digest.
				dks.DeviceKey:         []string{dks.QuadroDevice, dks.WalleyeDevice},
				dks.OSKey:             []string{dks.AndroidOS, dks.Windows10dot2OS, dks.Windows10dot3OS},
				types.PrimaryKeyField: []string{dks.TriangleTest},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
					ID:            "9a42e1337f848e4dbfa9688dda60fe7b",
					DigestIndices: []int{-1, -1, -1, -1, -1, 0, 0, -1, 0, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.GreyColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.WalleyeDevice,
						dks.OSKey:             dks.AndroidOS,
						types.PrimaryKeyField: dks.TriangleTest,
						"ext":                 "png",
					},
				}, {
					ID:            "b9c96f249f2551a5d33f264afdb23a46",
					DigestIndices: []int{-1, -1, -1, 0, 0, 0, 0, 0, 0, -1},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.GreyColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.QuadroDevice,
						dks.OSKey:             dks.Windows10dot3OS,
						types.PrimaryKeyField: dks.TriangleTest,
						"ext":                 "png",
					},
				}, {
					ID:            "c0f2834eb3408acdc799dc5190e3533e",
					DigestIndices: []int{0, 0, 0, -1, -1, -1, -1, -1, -1, -1},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.GreyColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.QuadroDevice,
						dks.OSKey:             dks.Windows10dot2OS,
						types.PrimaryKeyField: dks.TriangleTest,
						"ext":                 "png",
					},
				}},
				Digests: []frontend.DigestStatus{
					{Digest: dks.DigestB02Pos, Status: expectations.Positive},
				},
				TotalDigests: 1,
			},
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
					CombinedMetric: 1.9362538, QueryMetric: 1.9362538, PixelDiffPercent: 43.75, NumDiffPixels: 28,
					MaxRGBADiffs: [4]int{11, 5, 42, 0},
					DimDiffer:    false,
					Digest:       dks.DigestB01Pos,
					Status:       expectations.Positive,
					ParamSet: paramtools.ParamSet{
						dks.ColorModeKey:      []string{dks.RGBColorMode},
						types.CorpusField:     []string{dks.CornersCorpus},
						dks.DeviceKey:         []string{dks.QuadroDevice, dks.IPadDevice, dks.IPhoneDevice, dks.TaimenDevice, dks.WalleyeDevice},
						dks.OSKey:             []string{dks.AndroidOS, dks.Windows10dot2OS, dks.Windows10dot3OS, dks.IOS},
						types.PrimaryKeyField: []string{dks.TriangleTest},
						"ext":                 []string{"png"},
					},
				},
				frontend.NegativeRef: {
					CombinedMetric: 6.489451, QueryMetric: 6.489451, PixelDiffPercent: 53.125, NumDiffPixels: 34,
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
			ClosestRef: frontend.PositiveRef,
		}},
		Offset:  0,
		Size:    2,
		Commits: kitchenSinkCommits,
		BulkTriageData: frontend.TriageRequestData{
			dks.TriangleTest: {
				dks.DigestB01Pos: expectations.Positive,
				dks.DigestB02Pos: expectations.Positive,
			},
		},
	}, res)
}

func TestSearch_FilterLeftSideByKeysAndOptions_Success(t *testing.T) {
	t.Skip("not ready - would need frontend change for searching and removal of old fields")
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
		Sort:                             query.SortDescending,
		IncludeIgnoredTraces:             false,
		TraceValues: paramtools.ParamSet{
			types.CorpusField: []string{dks.CornersCorpus},
			dks.DeviceKey:     []string{dks.WalleyeDevice},
		},
		OptionsValues: paramtools.ParamSet{
			"image_matching_algorithm": []string{"fuzzy"},
		},
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
	})
	require.NoError(t, err)
	assert.Equal(t, &frontend.SearchResponse{
		Results: []*frontend.SearchResult{{
			Digest: dks.DigestA08Pos,
			Test:   dks.SquareTest,
			Status: expectations.Positive,
			TracesKeys: paramtools.ParamSet{
				dks.ColorModeKey:      []string{dks.RGBColorMode},
				types.CorpusField:     []string{dks.CornersCorpus},
				dks.DeviceKey:         []string{dks.WalleyeDevice},
				dks.OSKey:             []string{dks.AndroidOS},
				types.PrimaryKeyField: []string{dks.SquareTest},
			},
			TracesOptions: paramtools.ParamSet{
				"ext":                        []string{"png"},
				"image_matching_algorithm":   []string{"fuzzy"},
				"fuzzy_max_different_pixels": []string{"2"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
					ID:            "4686a4134535ad178b67325f5f2f613a",
					DigestIndices: []int{-1, -1, -1, -1, -1, 4, 3, 2, 1, 0},
					Keys: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.WalleyeDevice,
						dks.OSKey:             dks.AndroidOS,
						types.PrimaryKeyField: dks.SquareTest,
					},
					Options: paramtools.Params{
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
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
					CombinedMetric: 0.15655607, QueryMetric: 0.15655607, PixelDiffPercent: 3.125, NumDiffPixels: 2,
					MaxRGBADiffs: [4]int{4, 0, 0, 0},
					DimDiffer:    false,
					Digest:       dks.DigestA01Pos,
					Status:       expectations.Positive,
					TracesKeys: paramtools.ParamSet{
						dks.ColorModeKey:      []string{dks.RGBColorMode},
						types.CorpusField:     []string{dks.CornersCorpus},
						dks.DeviceKey:         []string{dks.QuadroDevice, dks.IPadDevice, dks.IPhoneDevice, dks.TaimenDevice, dks.WalleyeDevice},
						dks.OSKey:             []string{dks.AndroidOS, dks.Windows10dot2OS, dks.Windows10dot3OS, dks.IOS},
						types.PrimaryKeyField: []string{dks.SquareTest},
					},
					TracesOptions: paramtools.ParamSet{
						"ext":                        []string{"png"},
						"image_matching_algorithm":   []string{"fuzzy"},
						"fuzzy_max_different_pixels": []string{"2"},
					},
				},
				frontend.NegativeRef: {
					CombinedMetric: 10, QueryMetric: 10, PixelDiffPercent: 100, NumDiffPixels: 64,
					MaxRGBADiffs: [4]int{255, 255, 255, 255},
					DimDiffer:    false,
					Digest:       dks.DigestA09Neg,
					Status:       expectations.Negative,
					// Even though this is ignored, we are free to show it on the right side
					// (just not a part of the actual results).
					TracesKeys: paramtools.ParamSet{
						dks.ColorModeKey:      []string{dks.RGBColorMode},
						types.CorpusField:     []string{dks.CornersCorpus},
						dks.DeviceKey:         []string{dks.TaimenDevice},
						dks.OSKey:             []string{dks.AndroidOS},
						types.PrimaryKeyField: []string{dks.SquareTest},
					},
					TracesOptions: paramtools.ParamSet{
						"ext": []string{"png"},
					},
				},
			},
			ClosestRef: frontend.PositiveRef,
		}},
		Offset:  0,
		Size:    1,
		Commits: kitchenSinkCommits,
		BulkTriageData: frontend.TriageRequestData{
			dks.SquareTest: {
				dks.DigestA08Pos: expectations.Positive,
			},
		},
	}, res)
}

func TestSearch_FilteredAcrossAllHistory_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 100)
	res, err := s.Search(ctx, &query.Search{
		OnlyIncludeDigestsProducedAtHead: false,
		IncludePositiveDigests:           false,
		IncludeNegativeDigests:           true,
		IncludeUntriagedDigests:          true,
		Sort:                             query.SortDescending,
		IncludeIgnoredTraces:             false,
		TraceValues: paramtools.ParamSet{
			types.CorpusField: []string{dks.CornersCorpus},
			dks.OSKey:         []string{dks.IOS},
		},
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
	})
	require.NoError(t, err)
	assertFilteredAcrossAllHistory(t, res)
}

func TestSearch_FilteredAcrossAllHistory_WithMaterializedView(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 10) // Otherwise there's no commit for the materialized views
	require.NoError(t, s.StartMaterializedViews(ctx, []string{dks.CornersCorpus, dks.RoundCorpus}, time.Minute))
	res, err := s.Search(ctx, &query.Search{
		OnlyIncludeDigestsProducedAtHead: false,
		IncludePositiveDigests:           false,
		IncludeNegativeDigests:           true,
		IncludeUntriagedDigests:          true,
		Sort:                             query.SortDescending,
		IncludeIgnoredTraces:             false,
		TraceValues: paramtools.ParamSet{
			types.CorpusField: []string{dks.CornersCorpus},
			dks.OSKey:         []string{dks.IOS},
		},
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
	})
	require.NoError(t, err)
	assertFilteredAcrossAllHistory(t, res)
}

func assertFilteredAcrossAllHistory(t *testing.T, res *frontend.SearchResponse) {
	assert.Equal(t, &frontend.SearchResponse{
		Results: []*frontend.SearchResult{{
			Digest: dks.DigestBlank,
			Test:   dks.TriangleTest,
			Status: expectations.Untriaged,
			ParamSet: paramtools.ParamSet{
				dks.ColorModeKey:      []string{dks.GreyColorMode, dks.RGBColorMode},
				types.CorpusField:     []string{dks.CornersCorpus},
				dks.DeviceKey:         []string{dks.IPadDevice, dks.IPhoneDevice},
				dks.OSKey:             []string{dks.IOS},
				types.PrimaryKeyField: []string{dks.TriangleTest},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
					ID:            "47109b059f45e4f9d5ab61dd0199e2c9",
					DigestIndices: []int{4, 4, 4, 0, 4, 4, 4, 2, 2, 2},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.GreyColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.IPadDevice,
						dks.OSKey:             dks.IOS,
						types.PrimaryKeyField: dks.TriangleTest,
						"ext":                 "png",
					},
				}, {
					ID:            "760c2db998331eafd3023f4b6d135b06",
					DigestIndices: []int{3, -1, 0, -1, 0, -1, 3, -1, 1, -1},
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
					DigestIndices: []int{3, 3, 0, 3, 3, 0, 3, 1, 1, 1},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.IPadDevice,
						dks.OSKey:             dks.IOS,
						types.PrimaryKeyField: dks.TriangleTest,
						"ext":                 "png",
					},
				}, {
					ID:            "c5b4010e73321614f9049ad1985324c2",
					DigestIndices: []int{-1, 0, -1, -1, 4, -1, -1, 2, -1, -1},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.GreyColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.IPhoneDevice,
						dks.OSKey:             dks.IOS,
						types.PrimaryKeyField: dks.TriangleTest,
						"ext":                 "png",
					},
				}},
				Digests: []frontend.DigestStatus{
					{Digest: dks.DigestBlank, Status: expectations.Untriaged},
					{Digest: dks.DigestB01Pos, Status: expectations.Positive},
					{Digest: dks.DigestB02Pos, Status: expectations.Positive},
					{Digest: dks.DigestB03Neg, Status: expectations.Negative},
					{Digest: dks.DigestB04Neg, Status: expectations.Negative},
				},
				TotalDigests: 5,
			},
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
					CombinedMetric: 9.189747, QueryMetric: 9.189747, PixelDiffPercent: 90.625, NumDiffPixels: 58,
					MaxRGBADiffs: [4]int{250, 244, 197, 255},
					DimDiffer:    false,
					Digest:       dks.DigestB01Pos,
					Status:       expectations.Positive,
					ParamSet: paramtools.ParamSet{
						dks.ColorModeKey:      []string{dks.RGBColorMode},
						types.CorpusField:     []string{dks.CornersCorpus},
						dks.DeviceKey:         []string{dks.QuadroDevice, dks.IPadDevice, dks.IPhoneDevice, dks.TaimenDevice, dks.WalleyeDevice},
						dks.OSKey:             []string{dks.AndroidOS, dks.Windows10dot2OS, dks.Windows10dot3OS, dks.IOS},
						types.PrimaryKeyField: []string{dks.TriangleTest},
						"ext":                 []string{"png"},
					},
				},
				frontend.NegativeRef: {
					CombinedMetric: 9.519716, QueryMetric: 9.519716, PixelDiffPercent: 90.625, NumDiffPixels: 58,
					MaxRGBADiffs: [4]int{255, 255, 255, 255},
					DimDiffer:    true,
					Digest:       dks.DigestB04Neg,
					Status:       expectations.Negative,
					ParamSet: paramtools.ParamSet{
						dks.ColorModeKey:      []string{dks.GreyColorMode},
						types.CorpusField:     []string{dks.CornersCorpus},
						dks.DeviceKey:         []string{dks.IPadDevice, dks.IPhoneDevice},
						dks.OSKey:             []string{dks.IOS},
						types.PrimaryKeyField: []string{dks.TriangleTest},
						"ext":                 []string{"png"},
					},
				},
			},
			ClosestRef: frontend.PositiveRef,
		}, {
			Digest: dks.DigestB04Neg,
			Test:   dks.TriangleTest,
			Status: expectations.Negative,
			ParamSet: paramtools.ParamSet{
				dks.ColorModeKey:      []string{dks.GreyColorMode},
				types.CorpusField:     []string{dks.CornersCorpus},
				dks.DeviceKey:         []string{dks.IPadDevice, dks.IPhoneDevice},
				dks.OSKey:             []string{dks.IOS},
				types.PrimaryKeyField: []string{dks.TriangleTest},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
					ID:            "47109b059f45e4f9d5ab61dd0199e2c9",
					DigestIndices: []int{0, 0, 0, 2, 0, 0, 0, 1, 1, 1},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.GreyColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.IPadDevice,
						dks.OSKey:             dks.IOS,
						types.PrimaryKeyField: dks.TriangleTest,
						"ext":                 "png",
					},
				}, {
					ID:            "c5b4010e73321614f9049ad1985324c2",
					DigestIndices: []int{-1, 2, -1, -1, 0, -1, -1, 1, -1, -1},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.GreyColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.IPhoneDevice,
						dks.OSKey:             dks.IOS,
						types.PrimaryKeyField: dks.TriangleTest,
						"ext":                 "png",
					},
				}},
				Digests: []frontend.DigestStatus{
					{Digest: dks.DigestB04Neg, Status: expectations.Negative},
					{Digest: dks.DigestB02Pos, Status: expectations.Positive},
					{Digest: dks.DigestBlank, Status: expectations.Untriaged},
				},
				TotalDigests: 3,
			},
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
					CombinedMetric: 7.465255, QueryMetric: 7.465255, PixelDiffPercent: 64.0625, NumDiffPixels: 41,
					MaxRGBADiffs: [4]int{255, 255, 255, 42},
					DimDiffer:    true,
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
				frontend.NegativeRef: {
					CombinedMetric: 9.336915, QueryMetric: 9.336915, PixelDiffPercent: 100, NumDiffPixels: 64,
					MaxRGBADiffs: [4]int{255, 255, 255, 51},
					DimDiffer:    true,
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
			ClosestRef: frontend.PositiveRef,
		}, {
			Digest: dks.DigestB03Neg,
			Test:   dks.TriangleTest,
			Status: expectations.Negative,
			ParamSet: paramtools.ParamSet{
				dks.ColorModeKey:      []string{dks.RGBColorMode},
				types.CorpusField:     []string{dks.CornersCorpus},
				dks.DeviceKey:         []string{dks.IPadDevice, dks.IPhoneDevice},
				dks.OSKey:             []string{dks.IOS},
				types.PrimaryKeyField: []string{dks.TriangleTest},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
					ID:            "760c2db998331eafd3023f4b6d135b06",
					DigestIndices: []int{0, -1, 2, -1, 2, -1, 0, -1, 1, -1},
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
					DigestIndices: []int{0, 0, 2, 0, 0, 2, 0, 1, 1, 1},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.IPadDevice,
						dks.OSKey:             dks.IOS,
						types.PrimaryKeyField: dks.TriangleTest,
						"ext":                 "png",
					},
				}},
				Digests: []frontend.DigestStatus{
					{Digest: dks.DigestB03Neg, Status: expectations.Negative},
					{Digest: dks.DigestB01Pos, Status: expectations.Positive},
					{Digest: dks.DigestBlank, Status: expectations.Untriaged},
				},
				TotalDigests: 3,
			},
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
					CombinedMetric: 2.9445405, QueryMetric: 2.9445405, PixelDiffPercent: 10.9375, NumDiffPixels: 7,
					MaxRGBADiffs: [4]int{250, 244, 197, 51},
					DimDiffer:    false,
					Digest:       dks.DigestB01Pos,
					Status:       expectations.Positive,
					ParamSet: paramtools.ParamSet{
						dks.ColorModeKey:      []string{dks.RGBColorMode},
						types.CorpusField:     []string{dks.CornersCorpus},
						dks.DeviceKey:         []string{dks.QuadroDevice, dks.IPadDevice, dks.IPhoneDevice, dks.TaimenDevice, dks.WalleyeDevice},
						dks.OSKey:             []string{dks.AndroidOS, dks.Windows10dot2OS, dks.Windows10dot3OS, dks.IOS},
						types.PrimaryKeyField: []string{dks.TriangleTest},
						"ext":                 []string{"png"},
					},
				},
				frontend.NegativeRef: {
					CombinedMetric: 9.336915, QueryMetric: 9.336915, PixelDiffPercent: 100, NumDiffPixels: 64,
					MaxRGBADiffs: [4]int{255, 255, 255, 51},
					DimDiffer:    true,
					Digest:       dks.DigestB04Neg,
					Status:       expectations.Negative,
					ParamSet: paramtools.ParamSet{
						dks.ColorModeKey:      []string{dks.GreyColorMode},
						types.CorpusField:     []string{dks.CornersCorpus},
						dks.DeviceKey:         []string{dks.IPadDevice, dks.IPhoneDevice},
						dks.OSKey:             []string{dks.IOS},
						types.PrimaryKeyField: []string{dks.TriangleTest},
						"ext":                 []string{"png"},
					},
				},
			},
			ClosestRef: frontend.PositiveRef,
		}, {
			Digest: dks.DigestA04Unt,
			Test:   dks.SquareTest,
			Status: expectations.Untriaged,
			ParamSet: paramtools.ParamSet{
				dks.ColorModeKey:      []string{dks.GreyColorMode},
				types.CorpusField:     []string{dks.CornersCorpus},
				dks.DeviceKey:         []string{dks.IPadDevice},
				dks.OSKey:             []string{dks.IOS},
				types.PrimaryKeyField: []string{dks.SquareTest},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
					ID:            "796f2cc3f33fa6a9a1f4bef3aa9c48c4",
					DigestIndices: []int{2, 1, 2, 1, 2, 2, 2, 2, 0, 1},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.GreyColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.IPadDevice,
						dks.OSKey:             dks.IOS,
						types.PrimaryKeyField: dks.SquareTest,
						"ext":                 "png",
					},
				}},
				Digests: []frontend.DigestStatus{
					{Digest: dks.DigestA04Unt, Status: expectations.Untriaged},
					{Digest: dks.DigestA03Pos, Status: expectations.Positive},
					{Digest: dks.DigestA02Pos, Status: expectations.Positive},
				},
				TotalDigests: 3,
			},
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
					CombinedMetric: 0.17843534, QueryMetric: 0.17843534, PixelDiffPercent: 3.125, NumDiffPixels: 2,
					MaxRGBADiffs: [4]int{3, 3, 3, 0},
					DimDiffer:    false,
					Digest:       dks.DigestA03Pos,
					Status:       expectations.Positive,
					ParamSet: paramtools.ParamSet{
						dks.ColorModeKey:      []string{dks.GreyColorMode},
						types.CorpusField:     []string{dks.CornersCorpus},
						dks.DeviceKey:         []string{dks.QuadroDevice, dks.IPadDevice},
						dks.OSKey:             []string{dks.Windows10dot2OS, dks.Windows10dot3OS, dks.IOS},
						types.PrimaryKeyField: []string{dks.SquareTest},
						"ext":                 []string{"png"},
					},
				},
				frontend.NegativeRef: {
					CombinedMetric: 10, QueryMetric: 10, PixelDiffPercent: 100, NumDiffPixels: 64,
					MaxRGBADiffs: [4]int{255, 255, 255, 255},
					DimDiffer:    false,
					Digest:       dks.DigestA09Neg,
					Status:       expectations.Negative,
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
			ClosestRef: frontend.PositiveRef,
		}},
		Offset:  0,
		Size:    4,
		Commits: kitchenSinkCommits,
		BulkTriageData: frontend.TriageRequestData{
			dks.SquareTest: {
				dks.DigestA04Unt: expectations.Positive,
			},
			dks.TriangleTest: {
				dks.DigestBlank:  expectations.Positive,
				dks.DigestB03Neg: expectations.Positive,
				dks.DigestB04Neg: expectations.Positive,
			},
		},
	}, res)
}

func TestSearch_AcrossAllHistory_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 100)
	res, err := s.Search(ctx, &query.Search{
		OnlyIncludeDigestsProducedAtHead: false,
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
	require.Len(t, res.Results, 3)
	// Spot check this data
	assert.Equal(t, dks.DigestC05Unt, res.Results[0].Digest)
	assert.Equal(t, dks.DigestC03Unt, res.Results[1].Digest)
	assert.Equal(t, dks.DigestC04Unt, res.Results[2].Digest)
}

func TestSearch_AcrossAllHistory_WithMaterializedViews(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 10) // Otherwise there's no commit for the materialized views
	require.NoError(t, s.StartMaterializedViews(ctx, []string{dks.CornersCorpus, dks.RoundCorpus}, time.Minute))
	res, err := s.Search(ctx, &query.Search{
		OnlyIncludeDigestsProducedAtHead: false,
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
	require.Len(t, res.Results, 3)
	// Spot check this data
	assert.Equal(t, dks.DigestC05Unt, res.Results[0].Digest)
	assert.Equal(t, dks.DigestC03Unt, res.Results[1].Digest)
	assert.Equal(t, dks.DigestC04Unt, res.Results[2].Digest)
}

func TestJoinedTracesStatement_Success(t *testing.T) {
	unittest.SmallTest(t)

	statement := joinedTracesStatement([]filterSets{
		{key: "key1", values: []string{"alpha", "beta"}},
	}, "my_corpus")
	expectedCondition := `U0 AS (
	SELECT trace_id FROM Traces WHERE keys -> 'key1' = '"alpha"'
	UNION
	SELECT trace_id FROM Traces WHERE keys -> 'key1' = '"beta"'
),
JoinedTraces AS (
	SELECT trace_id FROM U0
	INTERSECT
	SELECT trace_id FROM Traces where keys -> 'source_type' = '"my_corpus"'
),
`
	assert.Equal(t, expectedCondition, statement)

	statement = joinedTracesStatement([]filterSets{
		{key: "key1", values: []string{"alpha", "beta"}},
		{key: "key2", values: []string{"gamma"}},
		{key: "key3", values: []string{"delta", "epsilon", "zeta"}},
	}, "other_corpus")
	expectedCondition = `U0 AS (
	SELECT trace_id FROM Traces WHERE keys -> 'key1' = '"alpha"'
	UNION
	SELECT trace_id FROM Traces WHERE keys -> 'key1' = '"beta"'
),
U1 AS (
	SELECT trace_id FROM Traces WHERE keys -> 'key2' = '"gamma"'
),
U2 AS (
	SELECT trace_id FROM Traces WHERE keys -> 'key3' = '"delta"'
	UNION
	SELECT trace_id FROM Traces WHERE keys -> 'key3' = '"epsilon"'
	UNION
	SELECT trace_id FROM Traces WHERE keys -> 'key3' = '"zeta"'
),
JoinedTraces AS (
	SELECT trace_id FROM U0
	INTERSECT
	SELECT trace_id FROM U1
	INTERSECT
	SELECT trace_id FROM U2
	INTERSECT
	SELECT trace_id FROM Traces where keys -> 'source_type' = '"other_corpus"'
),
`
	assert.Equal(t, expectedCondition, statement)
}

func TestJoinedTracesStatement_RemovesBadSQLCharacters(t *testing.T) {
	unittest.SmallTest(t)

	statement := joinedTracesStatement([]filterSets{
		{key: "key1", values: []string{"alpha", `beta"' OR 1=1`}},
		{key: `key2'='""' OR 1=1`, values: []string{"1"}}, // invalid keys are removed entirely.
	}, "some thing")
	expectedCondition := `U0 AS (
	SELECT trace_id FROM Traces WHERE keys -> 'key1' = '"alpha"'
	UNION
	SELECT trace_id FROM Traces WHERE keys -> 'key1' = '"beta OR 1=1"'
),
U1 AS (
	SELECT trace_id FROM Traces WHERE keys -> 'key2'='""' OR 1=1' = '"1"'
),
JoinedTraces AS (
	SELECT trace_id FROM U0
	INTERSECT
	SELECT trace_id FROM U1
	INTERSECT
	SELECT trace_id FROM Traces where keys -> 'source_type' = '"some thing"'
),
`
	assert.Equal(t, expectedCondition, statement)
}

func TestSearch_DifferentTestsDrawTheSame_SearchResultsAreSeparate(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	b := databuilder.TablesBuilder{TileWidth: 100}
	b.CommitsWithData().
		Insert("0111", "don't care", "commit 111", "2021-05-01T00:00:00Z").
		Insert("0222", "don't care", "commit 222", "2021-05-02T00:00:00Z").
		Insert("0333", "don't care", "commit 333", "2021-05-03T00:00:00Z")

	b.SetDigests(map[rune]types.Digest{
		'A': dks.DigestA01Pos,
		'b': dks.DigestA05Unt,
	})
	b.SetGroupingKeys(types.CorpusField, types.PrimaryKeyField)
	b.AddTracesWithCommonKeys(paramtools.Params{
		types.CorpusField: dks.CornersCorpus,
		dks.DeviceKey:     dks.WalleyeDevice,
	}).History(
		"Abb",
		"AAb",
	).Keys([]paramtools.Params{
		{types.PrimaryKeyField: "draw_a_square"},
		{types.PrimaryKeyField: "draw_a_square_but_faster"},
	}).OptionsAll(paramtools.Params{"ext": "png"}).
		IngestedFrom([]string{"don't care", "don't care 2", "don't care 3"}, []string{
			"2021-05-01T00:01:00Z", "2021-05-02T00:02:00Z", "2021-05-03T00:03:00Z",
		})
	b.AddTriageEvent("somebody", "2021-02-01T01:01:01Z").
		ExpectationsForGrouping(map[string]string{types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: "draw_a_square"}).
		Positive(dks.DigestA01Pos).
		ExpectationsForGrouping(map[string]string{types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: "draw_a_square_but_faster"}).
		Positive(dks.DigestA01Pos)
	b.AddIgnoreRule("user", "user", "2030-12-30T15:16:17Z", "Doesn't match anything",
		paramtools.ParamSet{
			"some key": []string{"foo bar"},
		})
	b.ComputeDiffMetricsFromImages(dks.GetImgDirectory(), "2021-05-03T06:00:00Z")
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, b.Build()))

	s := New(db, 100)
	res, err := s.Search(ctx, &query.Search{
		OnlyIncludeDigestsProducedAtHead: true,
		IncludePositiveDigests:           false,
		IncludeNegativeDigests:           false,
		IncludeUntriagedDigests:          true,
		Sort:                             query.SortDescending,
		IncludeIgnoredTraces:             false,
		TraceValues: paramtools.ParamSet{
			types.CorpusField: []string{dks.CornersCorpus},
		},
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
	})
	require.NoError(t, err)
	assert.Equal(t, &frontend.SearchResponse{
		Results: []*frontend.SearchResult{{
			Digest: dks.DigestA05Unt,
			Test:   "draw_a_square_but_faster",
			Status: expectations.Untriaged,
			ParamSet: paramtools.ParamSet{
				types.CorpusField:     []string{dks.CornersCorpus},
				types.PrimaryKeyField: []string{"draw_a_square_but_faster"},
				dks.DeviceKey:         []string{dks.WalleyeDevice},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
					ID:            "04fe79d1ad2d6d472aafdb997bbb26d3",
					DigestIndices: []int{1, 1, 0},
					Params: paramtools.Params{
						types.CorpusField:     dks.CornersCorpus,
						types.PrimaryKeyField: "draw_a_square_but_faster",
						dks.DeviceKey:         dks.WalleyeDevice,
						"ext":                 "png",
					},
				}},
				Digests: []frontend.DigestStatus{
					{Digest: dks.DigestA05Unt, Status: expectations.Untriaged},
					{Digest: dks.DigestA01Pos, Status: expectations.Positive},
				},
				TotalDigests: 2,
			},
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
					CombinedMetric: 0.20710422, QueryMetric: 0.20710422, PixelDiffPercent: 3.125, NumDiffPixels: 2,
					MaxRGBADiffs: [4]int{7, 0, 0, 0},
					DimDiffer:    false,
					Digest:       dks.DigestA01Pos,
					Status:       expectations.Positive,
					ParamSet: paramtools.ParamSet{
						types.CorpusField:     []string{dks.CornersCorpus},
						types.PrimaryKeyField: []string{"draw_a_square", "draw_a_square_but_faster"},
						dks.DeviceKey:         []string{dks.WalleyeDevice},
						"ext":                 []string{"png"},
					},
				},
				frontend.NegativeRef: nil,
			},
			ClosestRef: frontend.PositiveRef,
		}, {
			Digest: dks.DigestA05Unt,
			Test:   "draw_a_square",
			Status: "untriaged",
			ParamSet: paramtools.ParamSet{
				types.CorpusField:     []string{dks.CornersCorpus},
				types.PrimaryKeyField: []string{"draw_a_square"},
				dks.DeviceKey:         []string{dks.WalleyeDevice},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
					ID:            "9253edab166210e7198cf7e901ac87ec",
					DigestIndices: []int{1, 0, 0},
					Params: paramtools.Params{
						types.CorpusField:     dks.CornersCorpus,
						types.PrimaryKeyField: "draw_a_square",
						dks.DeviceKey:         dks.WalleyeDevice,
						"ext":                 "png",
					},
				}},
				Digests: []frontend.DigestStatus{
					{Digest: dks.DigestA05Unt, Status: expectations.Untriaged},
					{Digest: dks.DigestA01Pos, Status: expectations.Positive},
				},
				TotalDigests: 2,
			},
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
					CombinedMetric: 0.20710422, QueryMetric: 0.20710422, PixelDiffPercent: 3.125, NumDiffPixels: 2,
					MaxRGBADiffs: [4]int{7, 0, 0, 0},
					DimDiffer:    false,
					Digest:       dks.DigestA01Pos,
					Status:       expectations.Positive,
					ParamSet: paramtools.ParamSet{
						types.CorpusField:     []string{dks.CornersCorpus},
						types.PrimaryKeyField: []string{"draw_a_square", "draw_a_square_but_faster"},
						dks.DeviceKey:         []string{dks.WalleyeDevice},
						"ext":                 []string{"png"},
					},
				},
				frontend.NegativeRef: nil,
			},
			ClosestRef: frontend.PositiveRef,
		}},
		Offset: 0,
		Size:   2,
		BulkTriageData: map[types.TestName]map[types.Digest]expectations.Label{
			"draw_a_square": {
				dks.DigestA05Unt: "positive",
			},
			"draw_a_square_but_faster": {
				dks.DigestA05Unt: "positive",
			},
		},
		Commits: []frontend.Commit{{
			Subject:    "commit 111",
			ID:         "0111",
			CommitTime: 1619827200,
			Hash:       "6c505df96f1faab539199949572820b2c90f6959",
			Author:     "don't care",
		}, {
			Subject:    "commit 222",
			ID:         "0222",
			CommitTime: 1619913600,
			Hash:       "aac89c40628a35265f632940b678104349122a9f",
			Author:     "don't care",
		}, {
			Subject:    "commit 333",
			ID:         "0333",
			CommitTime: 1620000000,
			Hash:       "cb197b480a07a794b94ca9d50661db1fad2e3873",
			Author:     "don't care",
		}},
	}, res)
}

func TestSearch_RespectsPublicParams_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	matcher, err := publicparams.MatcherFromRules(publicparams.MatchingRules{
		dks.RoundCorpus: {
			dks.DeviceKey: {dks.QuadroDevice},
		},
	})
	require.NoError(t, err)

	s := New(db, 100)
	require.NoError(t, s.StartApplyingPublicParams(ctx, matcher, time.Minute))
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
	assertPublicUntriagedDigestsAtHead(t, res)
}

func TestSearch_RespectsPublicParams_WithMaterializedViews(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	matcher, err := publicparams.MatcherFromRules(publicparams.MatchingRules{
		dks.RoundCorpus: {
			dks.DeviceKey: {dks.QuadroDevice},
		},
	})
	require.NoError(t, err)

	s := New(db, 10) // Otherwise there's no commit for the materialized views
	require.NoError(t, s.StartApplyingPublicParams(ctx, matcher, time.Minute))
	require.NoError(t, s.StartMaterializedViews(ctx, []string{dks.CornersCorpus, dks.RoundCorpus}, time.Minute))
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
	assertPublicUntriagedDigestsAtHead(t, res)
}

func assertPublicUntriagedDigestsAtHead(t *testing.T, res *frontend.SearchResponse) {
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
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
					CombinedMetric: 0.89245414, QueryMetric: 0.89245414, PixelDiffPercent: 50, NumDiffPixels: 32,
					MaxRGBADiffs: [4]int{1, 7, 4, 0},
					DimDiffer:    false,
					Digest:       dks.DigestC01Pos,
					Status:       expectations.Positive,
					ParamSet: paramtools.ParamSet{
						dks.ColorModeKey:  []string{dks.RGBColorMode},
						types.CorpusField: []string{dks.RoundCorpus},
						// Android and iOS devices are not public
						dks.DeviceKey:         []string{dks.QuadroDevice},
						dks.OSKey:             []string{dks.Windows10dot2OS},
						types.PrimaryKeyField: []string{dks.CircleTest},
						"ext":                 []string{"png"},
					},
				},
				frontend.NegativeRef: nil,
			},
			ClosestRef: frontend.PositiveRef,
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
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
					CombinedMetric: 0.17843534, QueryMetric: 0.17843534, PixelDiffPercent: 3.125, NumDiffPixels: 2,
					MaxRGBADiffs: [4]int{3, 3, 3, 0},
					DimDiffer:    false,
					Digest:       dks.DigestC02Pos,
					Status:       expectations.Positive,
					ParamSet: paramtools.ParamSet{
						dks.ColorModeKey:      []string{dks.GreyColorMode},
						types.CorpusField:     []string{dks.RoundCorpus},
						dks.DeviceKey:         []string{dks.QuadroDevice},
						dks.OSKey:             []string{dks.Windows10dot2OS},
						types.PrimaryKeyField: []string{dks.CircleTest},
						"ext":                 []string{"png"},
					},
				},
				frontend.NegativeRef: nil,
			},
			ClosestRef: frontend.PositiveRef,
		}},
		Offset:  0,
		Size:    2,
		Commits: kitchenSinkCommits,
		BulkTriageData: frontend.TriageRequestData{
			dks.CircleTest: {
				dks.DigestC03Unt: expectations.Positive,
				dks.DigestC04Unt: expectations.Positive,
			},
		},
	}, res)
}

func TestSearch_RespectsRightSideFilter_Success(t *testing.T) {
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
			dks.ColorModeKey:  []string{dks.RGBColorMode},
		},
		RightTraceValues: paramtools.ParamSet{
			dks.ColorModeKey: []string{dks.GreyColorMode},
		},
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
	})
	require.NoError(t, err)
	assertRightSideTraces(t, res)
}

func assertRightSideTraces(t *testing.T, res *frontend.SearchResponse) {
	assert.Equal(t, &frontend.SearchResponse{
		Results: []*frontend.SearchResult{{
			Digest: dks.DigestC05Unt,
			Test:   dks.CircleTest,
			Status: expectations.Untriaged,
			ParamSet: paramtools.ParamSet{
				dks.ColorModeKey:      []string{dks.RGBColorMode},
				types.CorpusField:     []string{dks.RoundCorpus},
				dks.DeviceKey:         []string{dks.IPadDevice, dks.IPhoneDevice},
				dks.OSKey:             []string{dks.IOS}, // Note: Android + Taimen are ignored
				types.PrimaryKeyField: []string{dks.CircleTest},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
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
				}},
				Digests: []frontend.DigestStatus{
					{Digest: dks.DigestC05Unt, Status: expectations.Untriaged},
					{Digest: dks.DigestC01Pos, Status: expectations.Positive},
				},
				TotalDigests: 2,
			},
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
					CombinedMetric: 6.851621, QueryMetric: 6.851621, PixelDiffPercent: 100, NumDiffPixels: 64,
					MaxRGBADiffs: [4]int{141, 96, 168, 0},
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
				frontend.NegativeRef: nil,
			},
			ClosestRef: frontend.PositiveRef,
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
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
					CombinedMetric: 6.7015314, QueryMetric: 6.7015314, PixelDiffPercent: 100, NumDiffPixels: 64,
					MaxRGBADiffs: [4]int{141, 66, 168, 0},
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
				frontend.NegativeRef: nil,
			},
			ClosestRef: frontend.PositiveRef,
		}},
		Offset:  0,
		Size:    2,
		Commits: kitchenSinkCommits,
		BulkTriageData: frontend.TriageRequestData{
			dks.CircleTest: {
				dks.DigestC03Unt: expectations.Positive,
				dks.DigestC05Unt: expectations.Positive,
			},
		},
	}, res)
}

func TestObservedDigestStatement_ValidInputs_Success(t *testing.T) {
	unittest.SmallTest(t)

	statement, err := observedDigestsStatement(nil)
	require.NoError(t, err)
	assert.Equal(t, `WITH
ObservedDigestsInTile AS (
	SELECT DISTINCT digest FROM TiledTraceDigests
    WHERE grouping_id = $2 and tile_id >= $3
),`, statement)

	statement, err = observedDigestsStatement(paramtools.ParamSet{
		"some key":    []string{"a single value"},
		"another key": []string{"two", "values"},
	})
	require.NoError(t, err)
	assert.Equal(t, `WITH
U0 AS (
	SELECT trace_id FROM Traces WHERE keys -> 'another key' = '"two"'
	UNION
	SELECT trace_id FROM Traces WHERE keys -> 'another key' = '"values"'
),
U1 AS (
	SELECT trace_id FROM Traces WHERE keys -> 'some key' = '"a single value"'
),
MatchingTraces AS (
	SELECT trace_id FROM U0
	INTERSECT
	SELECT trace_id FROM U1
	INTERSECT
	SELECT trace_id FROM Traces WHERE grouping_id = $2
),
ObservedDigestsInTile AS (
	SELECT DISTINCT digest FROM TiledTraceDigests
	JOIN MatchingTraces ON TiledTraceDigests.trace_id = MatchingTraces.trace_id AND tile_id >= $3
),`, statement)
}

func TestObservedDigestStatement_InvalidInputs_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	_, err := observedDigestsStatement(paramtools.ParamSet{
		"'quote": []string{"a single value"},
	})
	require.Error(t, err)
	_, err = observedDigestsStatement(paramtools.ParamSet{
		"'quote": []string{`"a single value"`},
	})
	require.Error(t, err)
}

func TestSearch_ReturnsCLData_ShowsOnlyDataNewToPrimaryBranch(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 100)
	s.SetReviewSystemTemplates(map[string]string{
		dks.GerritCRS:         "http://example.com/public/%s",
		dks.GerritInternalCRS: "http://example.com/internal/%s",
	})
	require.NoError(t, s.StartCacheProcess(ctx, time.Minute, 100))
	res, err := s.Search(ctx, &query.Search{
		IncludePositiveDigests:  true,
		IncludeNegativeDigests:  false,
		IncludeUntriagedDigests: true,
		Sort:                    query.SortDescending,
		IncludeIgnoredTraces:    false,
		TraceValues: paramtools.ParamSet{
			types.CorpusField: []string{dks.RoundCorpus},
		},
		RGBAMinFilter:                  0,
		RGBAMaxFilter:                  255,
		ChangelistID:                   dks.ChangelistIDThatAttemptsToFixIOS,
		CodeReviewSystemID:             dks.GerritCRS,
		Patchsets:                      []int64{3}, // The first data we have for is PS order 3.
		IncludeDigestsProducedOnMaster: false,
	})
	require.NoError(t, err)
	var clCommits []frontend.Commit
	clCommits = append(clCommits, kitchenSinkCommits...)
	clCommits = append(clCommits, frontend.Commit{
		// This is the last time we ingested data for this CL.
		CommitTime:    time.Date(2020, time.December, 10, 4, 5, 6, 0, time.UTC).Unix(),
		Hash:          dks.ChangelistIDThatAttemptsToFixIOS,
		Author:        dks.UserOne,
		Subject:       "Fix iOS",
		ChangelistURL: "http://example.com/public/CL_fix_ios",
	})

	assert.Equal(t, &frontend.SearchResponse{
		Results: []*frontend.SearchResult{{
			Digest: dks.DigestC07Unt_CL,
			Test:   dks.CircleTest,
			Status: expectations.Untriaged,
			ParamSet: paramtools.ParamSet{
				dks.ColorModeKey:      []string{dks.RGBColorMode},
				types.CorpusField:     []string{dks.RoundCorpus},
				dks.DeviceKey:         []string{dks.IPhoneDevice}, // IPad is drawing correctly
				dks.OSKey:             []string{dks.IOS},
				types.PrimaryKeyField: []string{dks.CircleTest},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
					ID:            "22b530e029c22e396c5a24c0900c9ed5",
					DigestIndices: []int{2, -1, 2, -1, 2, -1, 2, -1, 1, -1, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.RoundCorpus,
						dks.DeviceKey:         dks.IPhoneDevice,
						dks.OSKey:             dks.IOS,
						types.PrimaryKeyField: dks.CircleTest,
						"ext":                 "png",
					},
				}},
				Digests: []frontend.DigestStatus{
					{Digest: dks.DigestC07Unt_CL, Status: expectations.Untriaged},
					{Digest: dks.DigestC05Unt, Status: expectations.Untriaged},
					{Digest: dks.DigestC01Pos, Status: expectations.Positive},
				},
				TotalDigests: 3,
			},
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
					CombinedMetric: 4.240108, QueryMetric: 4.240108, PixelDiffPercent: 68.75, NumDiffPixels: 44,
					MaxRGBADiffs: [4]int{77, 77, 77, 0},
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
				frontend.NegativeRef: nil,
			},
			ClosestRef: frontend.PositiveRef,
		}, {
			Digest: dks.DigestC06Pos_CL,
			Test:   dks.CircleTest,
			Status: expectations.Positive,
			ParamSet: paramtools.ParamSet{
				dks.ColorModeKey:      []string{dks.RGBColorMode},
				types.CorpusField:     []string{dks.RoundCorpus},
				dks.DeviceKey:         []string{dks.IPadDevice},
				dks.OSKey:             []string{dks.IOS},
				types.PrimaryKeyField: []string{dks.CircleTest},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
					ID:            "273119ca291863331e906fe71bde0e7d",
					DigestIndices: []int{2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.RoundCorpus,
						dks.DeviceKey:         dks.IPadDevice,
						dks.OSKey:             dks.IOS,
						types.PrimaryKeyField: dks.CircleTest,
						"ext":                 "png",
					},
				}},
				Digests: []frontend.DigestStatus{
					{Digest: dks.DigestC06Pos_CL, Status: expectations.Positive},
					{Digest: dks.DigestC05Unt, Status: expectations.Untriaged},
					{Digest: dks.DigestC01Pos, Status: expectations.Positive},
				},
				TotalDigests: 3,
			},
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
					CombinedMetric: 1.0217842, QueryMetric: 1.0217842, PixelDiffPercent: 6.25, NumDiffPixels: 4,
					MaxRGBADiffs: [4]int{15, 12, 83, 0},
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
				frontend.NegativeRef: nil,
			},
			ClosestRef: frontend.PositiveRef,
		}},
		Offset:  0,
		Size:    2,
		Commits: clCommits,
		BulkTriageData: frontend.TriageRequestData{
			dks.CircleTest: {
				dks.DigestC06Pos_CL: expectations.Positive,
				dks.DigestC07Unt_CL: expectations.Positive,
			},
		},
	}, res)
}

func TestMatchingCLTracesStatement_ValidInputs_Success(t *testing.T) {
	unittest.SmallTest(t)

	statement, err := matchingCLTracesStatement(paramtools.ParamSet{
		types.CorpusField: []string{"the corpus"},
	}, false)
	require.NoError(t, err)
	assert.Equal(t, `MatchingTraces AS (
	SELECT trace_id FROM Traces WHERE corpus = 'the corpus' AND matches_any_ignore_rule = FALSE
),`, statement)

	statement, err = matchingCLTracesStatement(paramtools.ParamSet{
		types.CorpusField: []string{"the corpus"},
		"some key":        []string{"a single value"},
		"another key":     []string{"two", "values"},
	}, false)
	require.NoError(t, err)
	assert.Equal(t, `U0 AS (
	SELECT trace_id FROM Traces WHERE keys -> 'another key' = '"two"'
	UNION
	SELECT trace_id FROM Traces WHERE keys -> 'another key' = '"values"'
),
U1 AS (
	SELECT trace_id FROM Traces WHERE keys -> 'some key' = '"a single value"'
),
MatchingTraces AS (
	SELECT trace_id FROM U0
	INTERSECT
	SELECT trace_id FROM U1
	INTERSECT
	SELECT trace_id FROM Traces WHERE corpus = 'the corpus' AND matches_any_ignore_rule = FALSE
),
`, statement)

	statement, err = matchingCLTracesStatement(paramtools.ParamSet{
		types.CorpusField: []string{"the corpus"},
		"some key":        []string{"a single value"},
		"another key":     []string{"two", "values"},
	}, true)
	require.NoError(t, err)
	assert.Equal(t, `U0 AS (
	SELECT trace_id FROM Traces WHERE keys -> 'another key' = '"two"'
	UNION
	SELECT trace_id FROM Traces WHERE keys -> 'another key' = '"values"'
),
U1 AS (
	SELECT trace_id FROM Traces WHERE keys -> 'some key' = '"a single value"'
),
MatchingTraces AS (
	SELECT trace_id FROM U0
	INTERSECT
	SELECT trace_id FROM U1
	INTERSECT
	SELECT trace_id FROM Traces WHERE corpus = 'the corpus' AND matches_any_ignore_rule IS NOT NULL
),
`, statement)
}

func TestMatchingCLTracesStatement_InvalidInputs_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	_, err := matchingCLTracesStatement(paramtools.ParamSet{
		"'quote":          []string{"a single value"},
		types.CorpusField: []string{"this is fine"},
	}, false)
	require.Error(t, err)
	_, err = matchingCLTracesStatement(paramtools.ParamSet{
		"'quote":          []string{`"a single value"`},
		types.CorpusField: []string{"this is fine"},
	}, false)
	require.Error(t, err)
	_, err = matchingCLTracesStatement(paramtools.ParamSet{
		"missing_corpus": []string{"its not here"},
	}, false)
	require.Error(t, err)
	_, err = matchingCLTracesStatement(paramtools.ParamSet{
		types.CorpusField: []string{"it's invalid"},
	}, false)
	require.Error(t, err)
}

func TestSearch_ReturnsFilteredCLData_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 100)
	s.SetReviewSystemTemplates(map[string]string{
		dks.GerritCRS:         "http://example.com/public/%s",
		dks.GerritInternalCRS: "http://example.com/internal/%s",
	})
	require.NoError(t, s.StartCacheProcess(ctx, time.Minute, 100))
	res, err := s.Search(ctx, &query.Search{
		IncludePositiveDigests:  true,
		IncludeNegativeDigests:  false,
		IncludeUntriagedDigests: true,
		Sort:                    query.SortDescending,
		IncludeIgnoredTraces:    false,
		TraceValues: paramtools.ParamSet{
			types.CorpusField:     []string{dks.CornersCorpus},
			types.PrimaryKeyField: []string{dks.SquareTest, dks.TriangleTest},
			dks.ColorModeKey:      []string{dks.RGBColorMode},
		},
		RGBAMinFilter:                  0,
		RGBAMaxFilter:                  255,
		ChangelistID:                   dks.ChangelistIDThatAttemptsToFixIOS,
		CodeReviewSystemID:             dks.GerritCRS,
		Patchsets:                      []int64{3}, // The first data we have for is PS order 3.
		IncludeDigestsProducedOnMaster: true,
	})
	require.NoError(t, err)
	var clCommits []frontend.Commit
	clCommits = append(clCommits, kitchenSinkCommits...)
	clCommits = append(clCommits, frontend.Commit{
		// This is the last time we ingested data for this CL.
		CommitTime:    time.Date(2020, time.December, 10, 4, 5, 6, 0, time.UTC).Unix(),
		Hash:          dks.ChangelistIDThatAttemptsToFixIOS,
		Author:        dks.UserOne,
		Subject:       "Fix iOS",
		ChangelistURL: "http://example.com/public/CL_fix_ios",
	})
	assert.Equal(t, &frontend.SearchResponse{
		Results: []*frontend.SearchResult{{
			Digest: dks.DigestB01Pos,
			Test:   dks.TriangleTest,
			// For this CL, the image has incorrectly had its triage status set to untriaged.
			Status: expectations.Untriaged,
			ParamSet: paramtools.ParamSet{
				dks.ColorModeKey:      []string{dks.RGBColorMode},
				types.CorpusField:     []string{dks.CornersCorpus},
				dks.DeviceKey:         []string{dks.IPadDevice, dks.IPhoneDevice, dks.TaimenDevice},
				dks.OSKey:             []string{dks.AndroidOS, dks.IOS},
				types.PrimaryKeyField: []string{dks.TriangleTest},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
					ID:            "1a16cbc8805378f0a6ef654a035d86c4",
					DigestIndices: []int{-1, -1, -1, -1, -1, -1, 0, 0, 0, 0, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.TaimenDevice,
						dks.OSKey:             dks.AndroidOS,
						types.PrimaryKeyField: dks.TriangleTest,
						"ext":                 "png",
					},
				}, {
					ID:            "760c2db998331eafd3023f4b6d135b06",
					DigestIndices: []int{1, -1, 2, -1, 2, -1, 1, -1, 0, -1, 0},
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
					DigestIndices: []int{1, 1, 2, 1, 1, 2, 1, 0, 0, 0, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.IPadDevice,
						dks.OSKey:             dks.IOS,
						types.PrimaryKeyField: dks.TriangleTest,
						"ext":                 "png",
					},
				}},
				Digests: []frontend.DigestStatus{
					// A user has incorrectly triaged B01Pos as untriaged on this CL.
					{Digest: dks.DigestB01Pos, Status: expectations.Untriaged},
					{Digest: dks.DigestB03Neg, Status: expectations.Negative},
					{Digest: dks.DigestBlank, Status: expectations.Untriaged},
				},
				TotalDigests: 3,
			},
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
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
				frontend.NegativeRef: {
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
			ClosestRef: frontend.PositiveRef,
		}, {
			Digest: dks.DigestA01Pos,
			Test:   dks.SquareTest,
			Status: expectations.Positive,
			ParamSet: paramtools.ParamSet{
				dks.ColorModeKey:      []string{dks.RGBColorMode},
				types.CorpusField:     []string{dks.CornersCorpus},
				dks.DeviceKey:         []string{dks.IPadDevice, dks.IPhoneDevice},
				dks.OSKey:             []string{dks.IOS},
				types.PrimaryKeyField: []string{dks.SquareTest},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
					ID:            "a95ccd579ee7c4771019a3374753db36",
					DigestIndices: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.IPadDevice,
						dks.OSKey:             dks.IOS,
						types.PrimaryKeyField: dks.SquareTest,
						"ext":                 "png",
					},
				}, {
					ID:            "ea0999cdbdb83a632327e9a1d65a565a",
					DigestIndices: []int{0, -1, 0, -1, 0, -1, 0, -1, 0, -1, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.CornersCorpus,
						dks.DeviceKey:         dks.IPhoneDevice,
						dks.OSKey:             dks.IOS,
						types.PrimaryKeyField: dks.SquareTest,
						"ext":                 "png",
					},
				}},
				Digests: []frontend.DigestStatus{
					{Digest: dks.DigestA01Pos, Status: expectations.Positive},
				},
				TotalDigests: 1,
			},
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
					CombinedMetric: 0.15655607, QueryMetric: 0.15655607, PixelDiffPercent: 3.125, NumDiffPixels: 2,
					MaxRGBADiffs: [4]int{4, 0, 0, 0},
					DimDiffer:    false,
					Digest:       dks.DigestA08Pos,
					Status:       expectations.Positive,
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
				},
				frontend.NegativeRef: {
					CombinedMetric: 10, QueryMetric: 10, PixelDiffPercent: 100, NumDiffPixels: 64,
					MaxRGBADiffs: [4]int{255, 255, 255, 255},
					DimDiffer:    false,
					Digest:       dks.DigestA09Neg,
					Status:       expectations.Negative,
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
			ClosestRef: frontend.PositiveRef,
		}},
		Offset:  0,
		Size:    2,
		Commits: clCommits,
		BulkTriageData: frontend.TriageRequestData{
			dks.SquareTest: {
				dks.DigestA01Pos: expectations.Positive,
			},
			dks.TriangleTest: {
				dks.DigestB01Pos: expectations.Positive,
			},
		},
	}, res)
}

func TestSearch_ResultHasNoReferenceDiffsNorExistingTraces_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 100)
	s.SetReviewSystemTemplates(map[string]string{
		dks.GerritCRS:         "http://example.com/public/%s",
		dks.GerritInternalCRS: "http://example.com/internal/%s",
	})
	require.NoError(t, s.StartCacheProcess(ctx, time.Minute, 100))
	res, err := s.Search(ctx, &query.Search{
		IncludePositiveDigests:  true,
		IncludeNegativeDigests:  true,
		IncludeUntriagedDigests: true,
		Sort:                    query.SortDescending,
		IncludeIgnoredTraces:    true,
		TraceValues: paramtools.ParamSet{
			types.CorpusField: []string{dks.TextCorpus},
		},
		RGBAMinFilter:                  0,
		RGBAMaxFilter:                  255,
		ChangelistID:                   dks.ChangelistIDThatAddsNewTests,
		CodeReviewSystemID:             dks.GerritInternalCRS,
		Patchsets:                      []int64{4}, // This is the second PS we have data for.
		IncludeDigestsProducedOnMaster: false,
	})
	require.NoError(t, err)
	var clCommits []frontend.Commit
	clCommits = append(clCommits, kitchenSinkCommits...)
	clCommits = append(clCommits, frontend.Commit{
		// This is the last time we ingested data for this CL.
		CommitTime:    time.Date(2020, time.December, 12, 9, 20, 33, 0, time.UTC).Unix(),
		Hash:          dks.ChangelistIDThatAddsNewTests,
		Author:        dks.UserTwo,
		Subject:       "Increase test coverage",
		ChangelistURL: "http://example.com/internal/CL_new_tests",
	})
	assert.Equal(t, &frontend.SearchResponse{
		Results: []*frontend.SearchResult{{
			Digest: dks.DigestD01Pos_CL,
			Test:   dks.SevenTest,
			Status: expectations.Positive,
			ParamSet: paramtools.ParamSet{
				dks.ColorModeKey:      []string{dks.GreyColorMode, dks.RGBColorMode},
				types.CorpusField:     []string{dks.TextCorpus},
				dks.DeviceKey:         []string{dks.QuadroDevice, dks.WalleyeDevice},
				dks.OSKey:             []string{dks.AndroidOS, dks.Windows10dot3OS},
				types.PrimaryKeyField: []string{dks.SevenTest},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
					ID:            "03fd07d9277767cd5461069ceb0a93ba",
					DigestIndices: []int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.GreyColorMode,
						types.CorpusField:     dks.TextCorpus,
						dks.DeviceKey:         dks.WalleyeDevice,
						dks.OSKey:             dks.AndroidOS,
						types.PrimaryKeyField: dks.SevenTest,
						"ext":                 "png",
					},
				}, {
					ID:            "0c3ffc9f53f2376f369ce73bc32f5ea9",
					DigestIndices: []int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.GreyColorMode,
						types.CorpusField:     dks.TextCorpus,
						dks.DeviceKey:         dks.QuadroDevice,
						dks.OSKey:             dks.Windows10dot3OS,
						types.PrimaryKeyField: dks.SevenTest,
						"ext":                 "png",
					},
				}, {
					ID:            "96e321dd10013b47edda21bf24029e0b",
					DigestIndices: []int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.TextCorpus,
						dks.DeviceKey:         dks.WalleyeDevice,
						dks.OSKey:             dks.AndroidOS,
						types.PrimaryKeyField: dks.SevenTest,
						"ext":                 "png",
					},
				}, {
					ID:            "c2a8d3f424ab2aee3c5bebb818c91557",
					DigestIndices: []int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 0},
					Params: paramtools.Params{
						dks.ColorModeKey:      dks.RGBColorMode,
						types.CorpusField:     dks.TextCorpus,
						dks.DeviceKey:         dks.QuadroDevice,
						dks.OSKey:             dks.Windows10dot3OS,
						types.PrimaryKeyField: dks.SevenTest,
						"ext":                 "png",
					},
				}},
				Digests: []frontend.DigestStatus{
					{Digest: dks.DigestD01Pos_CL, Status: expectations.Positive},
				},
				TotalDigests: 1,
			},
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: nil,
				frontend.NegativeRef: nil,
			},
			ClosestRef: frontend.NoRef,
		}},
		Offset:  0,
		Size:    1,
		Commits: clCommits,
		BulkTriageData: frontend.TriageRequestData{
			dks.SevenTest: {
				dks.DigestD01Pos_CL: "", // empty string means no closest reference
			},
		},
	}, res)
}

func TestGetPrimaryBranchParamset_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 100)
	ps, err := s.GetPrimaryBranchParamset(ctx)
	require.NoError(t, err)
	assert.Equal(t, paramtools.ReadOnlyParamSet{
		dks.ColorModeKey:             []string{dks.GreyColorMode, dks.RGBColorMode},
		dks.DeviceKey:                []string{dks.QuadroDevice, dks.IPadDevice, dks.IPhoneDevice, dks.TaimenDevice, dks.WalleyeDevice},
		types.PrimaryKeyField:        []string{dks.CircleTest, dks.SquareTest, dks.TriangleTest},
		dks.OSKey:                    []string{dks.AndroidOS, dks.Windows10dot2OS, dks.Windows10dot3OS, dks.IOS},
		types.CorpusField:            []string{dks.CornersCorpus, dks.RoundCorpus},
		"ext":                        []string{"png"},
		"fuzzy_max_different_pixels": []string{"2"},
		"image_matching_algorithm":   []string{"fuzzy"},
	}, ps)
}

func TestGetPrimaryBranchParamset_RespectsPublicParams_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	matcher, err := publicparams.MatcherFromRules(publicparams.MatchingRules{
		dks.RoundCorpus: {
			dks.DeviceKey: {dks.IPadDevice, dks.IPhoneDevice, dks.WalleyeDevice},
		},
		dks.CornersCorpus: {
			dks.DeviceKey: {dks.IPadDevice, dks.IPhoneDevice, dks.WalleyeDevice},
		},
	})
	require.NoError(t, err)

	s := New(db, 100)
	require.NoError(t, s.StartApplyingPublicParams(ctx, matcher, time.Minute))

	ps, err := s.GetPrimaryBranchParamset(ctx)
	require.NoError(t, err)
	assert.Equal(t, paramtools.ReadOnlyParamSet{
		dks.ColorModeKey:             []string{dks.GreyColorMode, dks.RGBColorMode},
		dks.DeviceKey:                []string{dks.IPadDevice, dks.IPhoneDevice, dks.WalleyeDevice},
		types.PrimaryKeyField:        []string{dks.CircleTest, dks.SquareTest, dks.TriangleTest},
		dks.OSKey:                    []string{dks.AndroidOS, dks.IOS},
		types.CorpusField:            []string{dks.CornersCorpus, dks.RoundCorpus},
		"ext":                        []string{"png"},
		"fuzzy_max_different_pixels": []string{"2"},
		"image_matching_algorithm":   []string{"fuzzy"},
	}, ps)
}

func TestGetChangelistParamset_ValidCLs_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 100)
	ps, err := s.GetChangelistParamset(ctx, dks.GerritCRS, dks.ChangelistIDThatAttemptsToFixIOS)
	require.NoError(t, err)
	assert.Equal(t, paramtools.ReadOnlyParamSet{
		dks.ColorModeKey:      []string{dks.GreyColorMode, dks.RGBColorMode},
		dks.DeviceKey:         []string{dks.IPadDevice, dks.IPhoneDevice, dks.TaimenDevice},
		types.PrimaryKeyField: []string{dks.CircleTest, dks.SquareTest, dks.TriangleTest},
		dks.OSKey:             []string{dks.AndroidOS, dks.IOS},
		types.CorpusField:     []string{dks.CornersCorpus, dks.RoundCorpus},
		"ext":                 []string{"png"},
	}, ps)

	ps, err = s.GetChangelistParamset(ctx, dks.GerritInternalCRS, dks.ChangelistIDThatAddsNewTests)
	require.NoError(t, err)
	assert.Equal(t, paramtools.ReadOnlyParamSet{
		dks.ColorModeKey:             []string{dks.GreyColorMode, dks.RGBColorMode},
		dks.DeviceKey:                []string{dks.QuadroDevice, dks.WalleyeDevice},
		types.PrimaryKeyField:        []string{dks.CircleTest, dks.RoundRectTest, dks.SevenTest, dks.SquareTest, dks.TriangleTest},
		dks.OSKey:                    []string{dks.AndroidOS, dks.Windows10dot3OS},
		types.CorpusField:            []string{dks.CornersCorpus, dks.RoundCorpus, dks.TextCorpus},
		"ext":                        []string{"png"},
		"fuzzy_max_different_pixels": []string{"2"},
		"image_matching_algorithm":   []string{"fuzzy"},
	}, ps)
}

func TestGetChangelistParamset_InvalidCL_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	s := New(db, 100)
	_, err := s.GetChangelistParamset(ctx, "does not", "exist")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Could not find")
}

func TestGetChangelistParamset_RespectsPublicView_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	matcher, err := publicparams.MatcherFromRules(publicparams.MatchingRules{
		dks.RoundCorpus: {
			dks.DeviceKey: {dks.IPadDevice, dks.IPhoneDevice, dks.WalleyeDevice},
		},
		dks.CornersCorpus: {
			dks.DeviceKey: {dks.IPadDevice, dks.IPhoneDevice, dks.WalleyeDevice},
		},
		dks.TextCorpus: {
			dks.DeviceKey: {dks.IPadDevice, dks.IPhoneDevice, dks.WalleyeDevice},
		},
	})
	require.NoError(t, err)

	s := New(db, 100)
	require.NoError(t, s.StartApplyingPublicParams(ctx, matcher, time.Minute))
	ps, err := s.GetChangelistParamset(ctx, dks.GerritCRS, dks.ChangelistIDThatAttemptsToFixIOS)
	require.NoError(t, err)
	assert.Equal(t, paramtools.ReadOnlyParamSet{
		dks.ColorModeKey:      []string{dks.GreyColorMode, dks.RGBColorMode},
		dks.DeviceKey:         []string{dks.IPadDevice, dks.IPhoneDevice},
		types.PrimaryKeyField: []string{dks.CircleTest, dks.SquareTest, dks.TriangleTest},
		dks.OSKey:             []string{dks.IOS},
		types.CorpusField:     []string{dks.CornersCorpus, dks.RoundCorpus},
		"ext":                 []string{"png"},
	}, ps)

	ps, err = s.GetChangelistParamset(ctx, dks.GerritInternalCRS, dks.ChangelistIDThatAddsNewTests)
	require.NoError(t, err)
	assert.Equal(t, paramtools.ReadOnlyParamSet{
		dks.ColorModeKey:             []string{dks.GreyColorMode, dks.RGBColorMode},
		dks.DeviceKey:                []string{dks.WalleyeDevice},
		types.PrimaryKeyField:        []string{dks.CircleTest, dks.RoundRectTest, dks.SevenTest, dks.SquareTest, dks.TriangleTest},
		dks.OSKey:                    []string{dks.AndroidOS},
		types.CorpusField:            []string{dks.CornersCorpus, dks.RoundCorpus, dks.TextCorpus},
		"ext":                        []string{"png"},
		"fuzzy_max_different_pixels": []string{"2"},
		"image_matching_algorithm":   []string{"fuzzy"},
	}, ps)
}

func TestGetBlamesForUntriagedDigests_UntriagedDigestsAtHeadInCorpus_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	s := New(db, 100)

	blames, err := s.GetBlamesForUntriagedDigests(ctx, dks.RoundCorpus)
	require.NoError(t, err)
	assertByBlameResponse(t, blames)
}

func TestGetBlamesForUntriagedDigests_UntriagedDigestsAtHeadInCorpus_WithMaterializedViews(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	s := New(db, 9)
	require.NoError(t, s.StartMaterializedViews(ctx, []string{dks.CornersCorpus, dks.RoundCorpus}, time.Minute))

	blames, err := s.GetBlamesForUntriagedDigests(ctx, dks.RoundCorpus)
	require.NoError(t, err)
	assertByBlameResponse(t, blames)
}

func assertByBlameResponse(t *testing.T, blames BlameSummaryV1) {
	assert.Equal(t, BlameSummaryV1{
		Ranges: []BlameEntry{
			{
				CommitRange:           dks.WindowsDriverUpdateCommitID,
				TotalUntriagedDigests: 2,
				AffectedGroupings: []*AffectedGrouping{
					{
						Grouping: paramtools.Params{
							types.CorpusField:     dks.RoundCorpus,
							types.PrimaryKeyField: dks.CircleTest,
						},
						UntriagedDigests: 2,
						SampleDigest:     dks.DigestC03Unt,
					},
				},
				Commits: []frontend.Commit{kitchenSinkCommits[3]},
			},
			{
				CommitRange:           dks.IOSFixTriangleTestsBreakCircleTestsCommitID,
				TotalUntriagedDigests: 1,
				AffectedGroupings: []*AffectedGrouping{
					{
						Grouping: paramtools.Params{
							types.CorpusField:     dks.RoundCorpus,
							types.PrimaryKeyField: dks.CircleTest,
						},
						UntriagedDigests: 1,
						SampleDigest:     dks.DigestC05Unt,
					},
				},
				Commits: []frontend.Commit{kitchenSinkCommits[7]},
			},
		},
	}, blames)
}

func TestSearch_IncludesBlameCommit_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 100)

	res, err := s.Search(ctx, &query.Search{
		BlameGroupID: dks.WindowsDriverUpdateCommitID,
		Sort:         query.SortDescending,
		TraceValues: paramtools.ParamSet{
			types.CorpusField: []string{dks.RoundCorpus},
		},
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
	})
	require.NoError(t, err)
	assertSearchBlameCommitResponse(t, res)
}

func TestSearch_IncludesBlameCommit_WithMaterializedViews(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	s := New(db, 10)
	require.NoError(t, s.StartMaterializedViews(ctx, []string{dks.CornersCorpus, dks.RoundCorpus}, time.Minute))

	res, err := s.Search(ctx, &query.Search{
		BlameGroupID: dks.WindowsDriverUpdateCommitID,
		Sort:         query.SortDescending,
		TraceValues: paramtools.ParamSet{
			types.CorpusField: []string{dks.RoundCorpus},
		},
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
	})
	require.NoError(t, err)
	assertSearchBlameCommitResponse(t, res)
}

func assertSearchBlameCommitResponse(t *testing.T, res *frontend.SearchResponse) {
	// This contains only the trace which still have an untriaged digest at head and had
	// an untriaged digest at the fourth commit (e.g. when the Windows OS was updated),
	// but not in a previous commit (if any).
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
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
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
				frontend.NegativeRef: nil,
			},
			ClosestRef: frontend.PositiveRef,
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
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
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
				frontend.NegativeRef: nil,
			},
			ClosestRef: frontend.PositiveRef,
		}},
		Offset:  0,
		Size:    2,
		Commits: kitchenSinkCommits,
		BulkTriageData: frontend.TriageRequestData{
			dks.CircleTest: {
				dks.DigestC03Unt: expectations.Positive,
				dks.DigestC04Unt: expectations.Positive,
			},
		},
	}, res)
}

func TestSearch_IncludesBlameRange_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 100)

	res, err := s.Search(ctx, &query.Search{
		// This isn't quite the same blame range that would have been returned, but it should
		// be equivalent.
		BlameGroupID: "0000000106:0000000108",
		Sort:         query.SortDescending,
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
				dks.OSKey:             []string{dks.IOS},
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
					// This trace is especially important because there was the untriaged data
					// wasn't seen in the range, but just after. Our algorithm should identify that
					// the the change in behavior that produced the untriaged digest could have
					// occurred in the range (because the data was simply missing in that time span)
					// and return it in the results.
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
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
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
				frontend.NegativeRef: nil,
			},
			ClosestRef: frontend.PositiveRef,
		}},
		Offset:  0,
		Size:    1,
		Commits: kitchenSinkCommits,
		BulkTriageData: frontend.TriageRequestData{
			dks.CircleTest: {
				dks.DigestC05Unt: expectations.Positive,
			},
		},
	}, res)
}

func TestSearch_BlameRespectsPublicParams_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	matcher, err := publicparams.MatcherFromRules(publicparams.MatchingRules{
		dks.RoundCorpus: {
			dks.DeviceKey: {dks.IPadDevice},
		},
	})
	require.NoError(t, err)

	s := New(db, 100)
	require.NoError(t, s.StartApplyingPublicParams(ctx, matcher, time.Minute))
	res, err := s.Search(ctx, &query.Search{
		BlameGroupID: "0000000106:0000000108",
		Sort:         query.SortDescending,
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
				dks.DeviceKey:         []string{dks.IPadDevice},
				dks.OSKey:             []string{dks.IOS},
				types.PrimaryKeyField: []string{dks.CircleTest},
				"ext":                 []string{"png"},
			},
			TraceGroup: frontend.TraceGroup{
				Traces: []frontend.Trace{{
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
			RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
				frontend.PositiveRef: {
					CombinedMetric: 4.9783297, QueryMetric: 4.9783297, PixelDiffPercent: 68.75, NumDiffPixels: 44,
					MaxRGBADiffs: [4]int{40, 149, 100, 0},
					DimDiffer:    false,
					Digest:       dks.DigestC01Pos,
					Status:       expectations.Positive,
					ParamSet: paramtools.ParamSet{
						dks.ColorModeKey:      []string{dks.RGBColorMode},
						types.CorpusField:     []string{dks.RoundCorpus},
						dks.DeviceKey:         []string{dks.IPadDevice},
						dks.OSKey:             []string{dks.IOS},
						types.PrimaryKeyField: []string{dks.CircleTest},
						"ext":                 []string{"png"},
					},
				},
				frontend.NegativeRef: nil,
			},
			ClosestRef: frontend.PositiveRef,
		}},
		Offset:  0,
		Size:    1,
		Commits: kitchenSinkCommits,
		BulkTriageData: frontend.TriageRequestData{
			dks.CircleTest: {
				dks.DigestC05Unt: expectations.Positive,
			},
		},
	}, res)
}

func TestSearch_BlameCommitInCorpusWithNoUntriaged_ReturnsEmptyResult(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db, 100)

	res, err := s.Search(ctx, &query.Search{
		BlameGroupID: dks.WindowsDriverUpdateCommitID,
		Sort:         query.SortDescending,
		TraceValues: paramtools.ParamSet{
			types.CorpusField: []string{dks.CornersCorpus},
		},
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
	})
	require.NoError(t, err)
	assert.Equal(t, &frontend.SearchResponse{
		Offset:  0,
		Size:    0,
		Commits: kitchenSinkCommits,
	}, res)
}

func TestGetBlamesForUntriagedDigests_RespectsPublicParams_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	matcher, err := publicparams.MatcherFromRules(publicparams.MatchingRules{
		dks.RoundCorpus: {
			dks.DeviceKey: {dks.QuadroDevice},
		},
	})
	require.NoError(t, err)

	s := New(db, 100)
	require.NoError(t, s.StartApplyingPublicParams(ctx, matcher, time.Minute))

	blames, err := s.GetBlamesForUntriagedDigests(ctx, dks.RoundCorpus)
	require.NoError(t, err)
	assert.Equal(t, BlameSummaryV1{
		Ranges: []BlameEntry{
			{
				CommitRange:           dks.WindowsDriverUpdateCommitID,
				TotalUntriagedDigests: 2,
				AffectedGroupings: []*AffectedGrouping{
					{
						Grouping: paramtools.Params{
							types.CorpusField:     dks.RoundCorpus,
							types.PrimaryKeyField: dks.CircleTest,
						},
						UntriagedDigests: 2,
						SampleDigest:     dks.DigestC03Unt,
					},
				},
				Commits: []frontend.Commit{kitchenSinkCommits[3]},
			},
		},
	}, blames)
}

func TestGetBlamesForUntriagedDigests_NoUntriagedDigestsAtHead_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	s := New(db, 100)

	// None of the traces for the corner tests have unignored, untriaged digests at head.
	// As a result, the blame returned should be empty.
	blames, err := s.GetBlamesForUntriagedDigests(ctx, dks.CornersCorpus)
	require.NoError(t, err)
	assert.Equal(t, BlameSummaryV1{}, blames)
}

func TestCombineIntoRanges_Success(t *testing.T) {
	unittest.SmallTest(t)

	alphaGrouping := paramtools.Params{types.PrimaryKeyField: "alpha", types.CorpusField: "the_corpus"}
	betaGrouping := paramtools.Params{types.PrimaryKeyField: "beta", types.CorpusField: "the_corpus"}
	groupings := map[schema.MD5Hash]paramtools.Params{
		mustHash(alphaGrouping): alphaGrouping,
		mustHash(betaGrouping):  betaGrouping,
	}
	simpleCommits := []frontend.Commit{
		{ID: "commit01"},
		{ID: "commit02"},
		{ID: "commit03"},
		{ID: "commit04"},
		{ID: "commit05"},
		{ID: "commit06"},
		{ID: "commit07"},
		{ID: "commit08"},
		{ID: "commit09"},
		{ID: "commit10"},
	}
	ctx := context.Background()

	test := func(name, inputDrawing string, expectedOutput []BlameEntry) {
		t.Run(name, func(t *testing.T) {
			input, exp := fromDrawing(inputDrawing, groupings)
			actual := combineIntoRanges(ctx, input, groupings, simpleCommits, exp)
			assert.Equal(t, expectedOutput, actual)
		})
	}

	// The convention for these drawings is that the left-aligned text define a digest (and its
	// associated grouping after the colon) that was untriaged at head. The indented text are the
	// traces that are producing that digest at head and should be processed to produce the blame.
	// The convention is that positively triaged digests are denoted by uppercase letters,
	// untriaged digests by lowercase letters/numbers and missing data by a dash.
	test("Three traces for the same test change at the same commit", `
b:alpha
	AAAAbbb
	AAAAbb-
	AAAAb-b
`, []BlameEntry{{
		CommitRange:           "commit05",
		TotalUntriagedDigests: 1,
		AffectedGroupings: []*AffectedGrouping{{
			Grouping:         alphaGrouping,
			UntriagedDigests: 1,
			SampleDigest:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}},
		Commits: []frontend.Commit{simpleCommits[4]},
	}})
	test("Three traces for the same test change to something different at the same commit", `
b:alpha
	AAAAbbb
c:alpha
	AAAAccc
d:alpha
	AAAAddd
`, []BlameEntry{{
		CommitRange:           "commit05",
		TotalUntriagedDigests: 3,
		AffectedGroupings: []*AffectedGrouping{{
			Grouping:         alphaGrouping,
			UntriagedDigests: 3,
			SampleDigest:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}},
		Commits: []frontend.Commit{simpleCommits[4]},
	}})
	test("Three sparse traces for the same test change in the same range", `
b:alpha
	AAA---b
	AA--bbb
	AA---bb
`, []BlameEntry{{
		CommitRange:           "commit04:commit05",
		TotalUntriagedDigests: 1,
		AffectedGroupings: []*AffectedGrouping{{
			Grouping:         alphaGrouping,
			UntriagedDigests: 1,
			SampleDigest:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}},
		Commits: []frontend.Commit{simpleCommits[3], simpleCommits[4]},
	}})
	test("Three sparse traces for different tests change in the same range", `
b:alpha
	AAA---b
	AA--bbb
d:beta
	CCC-ddd
`, []BlameEntry{{
		CommitRange:           "commit04:commit05",
		TotalUntriagedDigests: 2,
		AffectedGroupings: []*AffectedGrouping{{
			Grouping:         alphaGrouping,
			UntriagedDigests: 1,
			SampleDigest:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}, {
			Grouping:         betaGrouping,
			UntriagedDigests: 1,
			SampleDigest:     "dddddddddddddddddddddddddddddddd",
		}},
		Commits: []frontend.Commit{simpleCommits[3], simpleCommits[4]},
	}})
	test("Specific commit turned five traces flaky", `
d:alpha
	AAAAAcd
f:alpha
	AAAAAef
1:beta
	BBBBB01
2:beta
	BBBBB-2
4:beta
	BBBB-34
`, []BlameEntry{{
		CommitRange:           "commit06",
		TotalUntriagedDigests: 3,
		AffectedGroupings: []*AffectedGrouping{{
			Grouping:         alphaGrouping,
			UntriagedDigests: 2,
			SampleDigest:     "dddddddddddddddddddddddddddddddd",
		}, {
			Grouping:         betaGrouping,
			UntriagedDigests: 1,
			SampleDigest:     "11111111111111111111111111111111",
		}},
		Commits: []frontend.Commit{simpleCommits[5]},
	}, {
		CommitRange:           "commit05:commit06",
		TotalUntriagedDigests: 1,
		AffectedGroupings: []*AffectedGrouping{{
			Grouping:         betaGrouping,
			UntriagedDigests: 1,
			SampleDigest:     "44444444444444444444444444444444",
		}},
		Commits: []frontend.Commit{simpleCommits[4], simpleCommits[5]},
	}, {
		CommitRange:           "commit06:commit07",
		TotalUntriagedDigests: 1,
		AffectedGroupings: []*AffectedGrouping{{
			Grouping:         betaGrouping,
			UntriagedDigests: 1,
			SampleDigest:     "22222222222222222222222222222222",
		}},
		Commits: []frontend.Commit{simpleCommits[5], simpleCommits[6]},
	}})
	test("new traces appearing are blamed to first occurrence", `
b:alpha
	-------bbb
c:beta
	-------cc-
`, []BlameEntry{{
		CommitRange:           "commit08",
		TotalUntriagedDigests: 2,
		AffectedGroupings: []*AffectedGrouping{{
			Grouping:         alphaGrouping,
			UntriagedDigests: 1,
			SampleDigest:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}, {
			Grouping:         betaGrouping,
			UntriagedDigests: 1,
			SampleDigest:     "cccccccccccccccccccccccccccccccc",
		}},
		Commits: []frontend.Commit{simpleCommits[7]},
	}})
	// This might happen if a digest was triaged after we made our initial query.
	// If so, it shouldn't show up in any BlameEntries
	test("trace produces all triaged data", `
B:alpha
	BBBBBBBBBB
`, []BlameEntry{})
}

// fromDrawing turns a human readable drawing of test names and trace data into actual data
// that can be used to feed into combineIntoRanges.
func fromDrawing(drawing string, groupings map[schema.MD5Hash]paramtools.Params) ([]untriagedDigestAtHead, map[expectationKey]expectations.Label) {
	drawing = strings.TrimSpace(drawing)
	lines := strings.Split(drawing, "\n")
	findGroupingKey := func(testName string) schema.MD5Hash {
		for key, params := range groupings {
			if params[types.PrimaryKeyField] == testName {
				return key
			}
		}
		panic("unknown test name " + testName)
	}

	traceSize := 0

	var data []untriagedDigestAtHead
	exp := map[expectationKey]expectations.Label{}
	var currentData *untriagedDigestAtHead
	for i, line := range lines {
		if !strings.HasPrefix(line, "\t") {
			// Lines without a tab specify a new grouping:digest
			parts := strings.Split(line, ":")
			if len(parts) != 2 {
				panic("grouping lines should be like b:foo | line:" + strconv.Itoa(i))
			}
			if currentData != nil {
				data = append(data, *currentData)
			}
			currentData = &untriagedDigestAtHead{
				atHead: groupingDigestKey{
					groupingID: findGroupingKey(parts[1]),
					digest:     expandDigestToHash(parts[0]),
				},
			}
		} else {
			if currentData == nil {
				panic("need to specify grouping first")
			}
			line = strings.TrimPrefix(line, "\t")
			// Lines with a tab specify a trace's data, using one character per digest
			if traceSize != 0 && traceSize != len(line) {
				panic("traces must all have the same length | line:" + strconv.Itoa(i))
			} else if traceSize == 0 {
				traceSize = len(line)
			}
			traceData := make(traceData, traceSize)
			for i, c := range line {
				if c == '-' {
					continue
				}
				letter := string(c)
				digest := expandDigest(letter)
				traceData[i] = digest
				if letter == strings.ToLower(letter) {
					// lowercase and numbers are untriaged
					exp[expectationKey{
						groupingID: currentData.atHead.groupingID,
						digest:     digest,
					}] = expectations.Untriaged
				} else {
					exp[expectationKey{
						groupingID: currentData.atHead.groupingID,
						digest:     digest,
					}] = expectations.Positive
				}
			}
			currentData.traces = append(currentData.traces, traceData)
		}
	}
	data = append(data, *currentData)
	return data, exp
}

// expandDigest repeats the given letter 32 times to make a well-formatted digest.
func expandDigest(letter string) types.Digest {
	if len(letter) != 1 {
		panic("expected to expand a single letter into a digest: " + letter)
	}
	return types.Digest(strings.Repeat(letter, 2*md5.Size))
}

// expandDigestToHash creates a digest by expanding the given letter and returns it as a
// schema.MD5Hash
func expandDigestToHash(letter string) schema.MD5Hash {
	digest := expandDigest(letter)
	db, err := sql.DigestToBytes(digest)
	if err != nil {
		panic(err)
	}
	return sql.AsMD5Hash(db)
}

// mustHash returns the MD5hash of the serialized version of the map.
func mustHash(grouping paramtools.Params) schema.MD5Hash {
	_, b := sql.SerializeMap(grouping)
	return sql.AsMD5Hash(b)
}

func TestGetCluster_ShowAllDataFromPrimaryBranch_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()
	s := New(db, 100)
	res, err := s.GetCluster(ctx, ClusterOptions{
		Grouping: paramtools.Params{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
		},
		IncludePositiveDigests:  true,
		IncludeNegativeDigests:  true,
		IncludeUntriagedDigests: true,
	})
	require.NoError(t, err)
	assert.Equal(t, frontend.ClusterDiffResult{
		Test: dks.SquareTest,
		Nodes: []frontend.Node{
			{Digest: dks.DigestA01Pos, Status: expectations.Positive},
			{Digest: dks.DigestA02Pos, Status: expectations.Positive},
			{Digest: dks.DigestA03Pos, Status: expectations.Positive},
			{Digest: dks.DigestA08Pos, Status: expectations.Positive},
		},
		Links: []frontend.Link{
			{LeftIndex: 0, RightIndex: 1, Distance: 56.25},
			{LeftIndex: 0, RightIndex: 2, Distance: 56.25},
			{LeftIndex: 0, RightIndex: 3, Distance: 3.125},
			{LeftIndex: 1, RightIndex: 2, Distance: 1.5625},
			{LeftIndex: 1, RightIndex: 3, Distance: 56.25},
			{LeftIndex: 2, RightIndex: 3, Distance: 56.25},
		},
		ParamsetByDigest: map[types.Digest]paramtools.ParamSet{
			dks.DigestA01Pos: {
				types.CorpusField:     []string{dks.CornersCorpus},
				types.PrimaryKeyField: []string{dks.SquareTest},
				dks.ColorModeKey:      []string{dks.RGBColorMode},
				dks.DeviceKey:         []string{dks.QuadroDevice, dks.IPadDevice, dks.IPhoneDevice},
				dks.OSKey:             []string{dks.Windows10dot2OS, dks.Windows10dot3OS, dks.IOS},
				"ext":                 []string{"png"},
			},
			dks.DigestA02Pos: {
				types.CorpusField:     []string{dks.CornersCorpus},
				types.PrimaryKeyField: []string{dks.SquareTest},
				dks.ColorModeKey:      []string{dks.GreyColorMode},
				dks.DeviceKey:         []string{dks.QuadroDevice, dks.IPhoneDevice, dks.WalleyeDevice},
				dks.OSKey:             []string{dks.AndroidOS, dks.Windows10dot3OS, dks.IOS},
				"ext":                 []string{"png"},
			},
			dks.DigestA03Pos: {
				types.CorpusField:     []string{dks.CornersCorpus},
				types.PrimaryKeyField: []string{dks.SquareTest},
				dks.ColorModeKey:      []string{dks.GreyColorMode},
				dks.DeviceKey:         []string{dks.QuadroDevice, dks.IPadDevice},
				dks.OSKey:             []string{dks.Windows10dot2OS, dks.IOS},
				"ext":                 []string{"png"},
			},
			dks.DigestA08Pos: {
				types.CorpusField:            []string{dks.CornersCorpus},
				types.PrimaryKeyField:        []string{dks.SquareTest},
				dks.ColorModeKey:             []string{dks.RGBColorMode},
				dks.DeviceKey:                []string{dks.WalleyeDevice},
				dks.OSKey:                    []string{dks.AndroidOS},
				"ext":                        []string{"png"},
				"fuzzy_max_different_pixels": []string{"2"},
				"image_matching_algorithm":   []string{"fuzzy"},
			},
		},
		ParamsetsUnion: paramtools.ParamSet{
			types.CorpusField:            []string{dks.CornersCorpus},
			types.PrimaryKeyField:        []string{dks.SquareTest},
			dks.ColorModeKey:             []string{dks.GreyColorMode, dks.RGBColorMode},
			dks.DeviceKey:                []string{dks.QuadroDevice, dks.IPadDevice, dks.IPhoneDevice, dks.WalleyeDevice},
			dks.OSKey:                    []string{dks.AndroidOS, dks.Windows10dot2OS, dks.Windows10dot3OS, dks.IOS},
			"ext":                        []string{"png"},
			"fuzzy_max_different_pixels": []string{"2"},
			"image_matching_algorithm":   []string{"fuzzy"},
		},
	}, res)
}

func TestGetCluster_RespectsTriageStatuses_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()
	s := New(db, 100)
	res, err := s.GetCluster(ctx, ClusterOptions{
		Grouping: paramtools.Params{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.TriangleTest,
		},
		IncludePositiveDigests:  false,
		IncludeNegativeDigests:  true,
		IncludeUntriagedDigests: true,
	})
	require.NoError(t, err)
	// For this test, we expect there to be no results, so it should return an empty response.
	assert.Equal(t, frontend.ClusterDiffResult{
		Test: dks.TriangleTest,
	}, res)
}

func TestGetCluster_RespectsFilters_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()
	s := New(db, 100)
	res, err := s.GetCluster(ctx, ClusterOptions{
		Grouping: paramtools.Params{
			types.CorpusField:     dks.RoundCorpus,
			types.PrimaryKeyField: dks.CircleTest,
		},
		Filters: paramtools.ParamSet{
			dks.ColorModeKey: []string{dks.GreyColorMode},
			dks.OSKey:        []string{dks.AndroidOS, dks.IOS},
		},
		IncludePositiveDigests:  true,
		IncludeNegativeDigests:  true,
		IncludeUntriagedDigests: true,
	})
	require.NoError(t, err)
	assert.Equal(t, frontend.ClusterDiffResult{
		Test: dks.CircleTest,
		Nodes: []frontend.Node{
			{Digest: dks.DigestC02Pos, Status: expectations.Positive},
			{Digest: dks.DigestC05Unt, Status: expectations.Untriaged},
		},
		Links: []frontend.Link{
			{LeftIndex: 0, RightIndex: 1, Distance: 100},
		},
		ParamsetByDigest: map[types.Digest]paramtools.ParamSet{
			dks.DigestC02Pos: {
				types.CorpusField:     []string{dks.RoundCorpus},
				types.PrimaryKeyField: []string{dks.CircleTest},
				dks.ColorModeKey:      []string{dks.GreyColorMode},
				dks.DeviceKey:         []string{dks.WalleyeDevice},
				dks.OSKey:             []string{dks.AndroidOS},
				"ext":                 []string{"png"},
			},
			dks.DigestC05Unt: {
				types.CorpusField:     []string{dks.RoundCorpus},
				types.PrimaryKeyField: []string{dks.CircleTest},
				dks.ColorModeKey:      []string{dks.GreyColorMode},
				dks.DeviceKey:         []string{dks.IPadDevice, dks.IPhoneDevice},
				dks.OSKey:             []string{dks.IOS},
				"ext":                 []string{"png"},
			},
		},
		ParamsetsUnion: paramtools.ParamSet{
			types.CorpusField:     []string{dks.RoundCorpus},
			types.PrimaryKeyField: []string{dks.CircleTest},
			dks.ColorModeKey:      []string{dks.GreyColorMode},
			dks.DeviceKey:         []string{dks.IPadDevice, dks.IPhoneDevice, dks.WalleyeDevice},
			dks.OSKey:             []string{dks.AndroidOS, dks.IOS},
			"ext":                 []string{"png"},
		},
	}, res)
}

func TestGetCluster_RespectsPublicParams_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	matcher, err := publicparams.MatcherFromRules(publicparams.MatchingRules{
		dks.CornersCorpus: {
			dks.DeviceKey: {dks.QuadroDevice},
		},
	})
	require.NoError(t, err)
	s := New(db, 100)
	require.NoError(t, s.StartApplyingPublicParams(ctx, matcher, time.Minute))
	res, err := s.GetCluster(ctx, ClusterOptions{
		Grouping: paramtools.Params{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.TriangleTest,
		},
		IncludePositiveDigests:  true,
		IncludeNegativeDigests:  true,
		IncludeUntriagedDigests: true,
	})
	require.NoError(t, err)
	// For this test, we expect there to be no results, so it should return an empty response.
	assert.Equal(t, frontend.ClusterDiffResult{
		Test: dks.TriangleTest,
		Nodes: []frontend.Node{
			{Digest: dks.DigestB01Pos, Status: expectations.Positive},
			{Digest: dks.DigestB02Pos, Status: expectations.Positive},
		},
		Links: []frontend.Link{
			{LeftIndex: 0, RightIndex: 1, Distance: 43.75},
		},
		ParamsetByDigest: map[types.Digest]paramtools.ParamSet{
			dks.DigestB01Pos: {
				types.CorpusField:     []string{dks.CornersCorpus},
				types.PrimaryKeyField: []string{dks.TriangleTest},
				dks.ColorModeKey:      []string{dks.RGBColorMode},
				dks.DeviceKey:         []string{dks.QuadroDevice},
				dks.OSKey:             []string{dks.Windows10dot2OS, dks.Windows10dot3OS},
				"ext":                 []string{"png"},
			},
			dks.DigestB02Pos: {
				types.CorpusField:     []string{dks.CornersCorpus},
				types.PrimaryKeyField: []string{dks.TriangleTest},
				dks.ColorModeKey:      []string{dks.GreyColorMode},
				dks.DeviceKey:         []string{dks.QuadroDevice},
				dks.OSKey:             []string{dks.Windows10dot2OS, dks.Windows10dot3OS},
				"ext":                 []string{"png"},
			},
		},
		ParamsetsUnion: paramtools.ParamSet{
			types.CorpusField:     []string{dks.CornersCorpus},
			types.PrimaryKeyField: []string{dks.TriangleTest},
			dks.ColorModeKey:      []string{dks.GreyColorMode, dks.RGBColorMode},
			dks.DeviceKey:         []string{dks.QuadroDevice},
			dks.OSKey:             []string{dks.Windows10dot2OS, dks.Windows10dot3OS},
			"ext":                 []string{"png"},
		},
	}, res)
}

func TestClusterDataOfInterestStatement_Success(t *testing.T) {
	unittest.SmallTest(t)

	statement, err := clusterDataOfInterestStatement(ClusterOptions{
		Filters: paramtools.ParamSet{
			"key1": []string{"alpha", "beta"},
		},
	})
	require.NoError(t, err)
	expectedCondition := `
U0 AS (
	SELECT trace_id FROM Traces WHERE keys -> 'key1' = '"alpha"'
	UNION
	SELECT trace_id FROM Traces WHERE keys -> 'key1' = '"beta"'
),
TracesOfInterest AS (
	SELECT trace_id FROM U0
	INTERSECT
	SELECT trace_id FROM Traces WHERE grouping_id = $1 AND matches_any_ignore_rule = FALSE
),
DataOfInterest AS (
	SELECT ValuesAtHead.trace_id, options_id, digest FROM ValuesAtHead
	JOIN TracesOfInterest ON ValuesAtHead.trace_id = TracesOfInterest.trace_id
	WHERE most_recent_commit_id >= $2
)`
	assert.Equal(t, expectedCondition, statement)

	statement, err = clusterDataOfInterestStatement(ClusterOptions{
		Filters: paramtools.ParamSet{
			"key1": []string{"alpha", "beta"},
			"key2": []string{"gamma"},
			"key3": []string{"delta", "epsilon", "zeta"},
		},
	})
	require.NoError(t, err)
	expectedCondition = `
U0 AS (
	SELECT trace_id FROM Traces WHERE keys -> 'key1' = '"alpha"'
	UNION
	SELECT trace_id FROM Traces WHERE keys -> 'key1' = '"beta"'
),
U1 AS (
	SELECT trace_id FROM Traces WHERE keys -> 'key2' = '"gamma"'
),
U2 AS (
	SELECT trace_id FROM Traces WHERE keys -> 'key3' = '"delta"'
	UNION
	SELECT trace_id FROM Traces WHERE keys -> 'key3' = '"epsilon"'
	UNION
	SELECT trace_id FROM Traces WHERE keys -> 'key3' = '"zeta"'
),
TracesOfInterest AS (
	SELECT trace_id FROM U0
	INTERSECT
	SELECT trace_id FROM U1
	INTERSECT
	SELECT trace_id FROM U2
	INTERSECT
	SELECT trace_id FROM Traces WHERE grouping_id = $1 AND matches_any_ignore_rule = FALSE
),
DataOfInterest AS (
	SELECT ValuesAtHead.trace_id, options_id, digest FROM ValuesAtHead
	JOIN TracesOfInterest ON ValuesAtHead.trace_id = TracesOfInterest.trace_id
	WHERE most_recent_commit_id >= $2
)`
	assert.Equal(t, expectedCondition, statement)
}

func TestClusterDataOfInterestStatement_InvalidInput_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	_, err := clusterDataOfInterestStatement(ClusterOptions{
		Filters: paramtools.ParamSet{
			"key1": []string{"alpha", `beta"' OR 1=1`},
		},
	})
	require.Error(t, err)

	_, err = clusterDataOfInterestStatement(ClusterOptions{
		Filters: paramtools.ParamSet{
			`key1'='""' OR 1=1`: []string{"alpha"},
		},
	})
	require.Error(t, err)
}

var kitchenSinkCommits = makeKitchenSinkCommits()

func makeKitchenSinkCommits() []frontend.Commit {
	data := dks.Build()
	convert := func(row schema.GitCommitRow) frontend.Commit {
		return frontend.Commit{
			CommitTime: row.CommitTime.Unix(),
			ID:         string(row.CommitID),
			Hash:       row.GitHash,
			Author:     row.AuthorEmail,
			Subject:    row.Subject,
		}
	}
	return []frontend.Commit{
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
