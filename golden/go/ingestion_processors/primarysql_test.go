package ingestion_processors

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/sql/databuilder"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
)

// This tests the first ingestion of data, with no data filled in except the GitCommits table, which
// will be read from during ingestion.
func TestPrimarySQL_Process_AllNewData_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	validCommits := dks.Build().GitCommits
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, schema.Tables{
		GitCommits: validCommits,
	}))

	// This file has data from 3 traces across 2 corpora. The data is from the third commit.
	const squareTraceKeys = `{"color mode":"RGB","device":"QuadroP400","name":"square","os":"Windows10.2","source_type":"corners"}`
	const triangleTraceKeys = `{"color mode":"RGB","device":"QuadroP400","name":"triangle","os":"Windows10.2","source_type":"corners"}`
	const circleTraceKeys = `{"color mode":"RGB","device":"QuadroP400","name":"circle","os":"Windows10.2","source_type":"round"}`
	const expectedCommitID = "0000000100"
	src := fakeGCSSourceFromFile(t, "primary1.json")
	s := PrimaryBranchSQL(src, map[string]string{sqlTileWidthConfig: "5"}, db)
	totalMetricBefore := s.filesProcessed.Get()
	successMetricBefore := s.filesSuccess.Get()
	resultsMetricBefore := s.resultsIngested.Get()

	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := s.Process(ctx, dks.WindowsFile3)
	require.NoError(t, err)

	actualSourceFiles := sqltest.GetAllRows(ctx, t, db, "SourceFiles", &schema.SourceFileRow{}).([]schema.SourceFileRow)
	assert.Equal(t, []schema.SourceFileRow{{
		SourceFileID: h(dks.WindowsFile3),
		SourceFile:   dks.WindowsFile3,
		LastIngested: fakeIngestionTime,
	}}, actualSourceFiles)

	actualGroupings := sqltest.GetAllRows(ctx, t, db, "Groupings", &schema.GroupingRow{}).([]schema.GroupingRow)
	assert.ElementsMatch(t, []schema.GroupingRow{{
		GroupingID: h(circleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.RoundCorpus,
			types.PrimaryKeyField: dks.CircleTest,
		},
	}, {
		GroupingID: h(squareGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
		},
	}, {
		GroupingID: h(triangleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.TriangleTest,
		},
	}}, actualGroupings)

	actualOptions := sqltest.GetAllRows(ctx, t, db, "Options", &schema.OptionsRow{}).([]schema.OptionsRow)
	assert.ElementsMatch(t, []schema.OptionsRow{{
		OptionsID: h(pngOptions),
		Keys: map[string]string{
			"ext": "png",
		},
	}}, actualOptions)

	actualTraces := sqltest.GetAllRows(ctx, t, db, "Traces", &schema.TraceRow{}).([]schema.TraceRow)
	assert.Equal(t, []schema.TraceRow{{
		TraceID:    h(circleTraceKeys),
		Corpus:     dks.RoundCorpus,
		GroupingID: h(circleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.RoundCorpus,
			types.PrimaryKeyField: dks.CircleTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.Windows10dot2OS,
			dks.DeviceKey:         dks.QuadroDevice,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}, {
		TraceID:    h(squareTraceKeys),
		Corpus:     dks.CornersCorpus,
		GroupingID: h(squareGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.Windows10dot2OS,
			dks.DeviceKey:         dks.QuadroDevice,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}, {
		TraceID:    h(triangleTraceKeys),
		Corpus:     dks.CornersCorpus,
		GroupingID: h(triangleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.TriangleTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.Windows10dot2OS,
			dks.DeviceKey:         dks.QuadroDevice,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}}, actualTraces)

	actualCommitsWithData := sqltest.GetAllRows(ctx, t, db, "CommitsWithData", &schema.CommitWithDataRow{}).([]schema.CommitWithDataRow)
	assert.Equal(t, []schema.CommitWithDataRow{{
		CommitID: expectedCommitID,
		TileID:   0,
	}}, actualCommitsWithData)

	actualTraceValues := sqltest.GetAllRows(ctx, t, db, "TraceValues", &schema.TraceValueRow{}).([]schema.TraceValueRow)
	assert.ElementsMatch(t, []schema.TraceValueRow{{
		Shard:        0x4,
		TraceID:      h(squareTraceKeys),
		CommitID:     expectedCommitID,
		Digest:       d(dks.DigestA01Pos),
		GroupingID:   h(squareGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.WindowsFile3),
	}, {
		Shard:        0x3,
		TraceID:      h(triangleTraceKeys),
		CommitID:     expectedCommitID,
		Digest:       d(dks.DigestB01Pos),
		GroupingID:   h(triangleGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.WindowsFile3),
	}, {
		Shard:        0x7,
		TraceID:      h(circleTraceKeys),
		CommitID:     expectedCommitID,
		Digest:       d(dks.DigestC01Pos),
		GroupingID:   h(circleGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.WindowsFile3),
	}}, actualTraceValues)

	actualValuesAtHead := sqltest.GetAllRows(ctx, t, db, "ValuesAtHead", &schema.ValueAtHeadRow{}).([]schema.ValueAtHeadRow)
	assert.ElementsMatch(t, []schema.ValueAtHeadRow{{
		TraceID:            h(squareTraceKeys),
		MostRecentCommitID: expectedCommitID,
		Digest:             d(dks.DigestA01Pos),
		OptionsID:          h(pngOptions),
		GroupingID:         h(squareGrouping),
		Corpus:             dks.CornersCorpus,
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.Windows10dot2OS,
			dks.DeviceKey:         dks.QuadroDevice,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}, {
		TraceID:            h(triangleTraceKeys),
		MostRecentCommitID: expectedCommitID,
		Digest:             d(dks.DigestB01Pos),
		OptionsID:          h(pngOptions),
		GroupingID:         h(triangleGrouping),
		Corpus:             dks.CornersCorpus,
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.TriangleTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.Windows10dot2OS,
			dks.DeviceKey:         dks.QuadroDevice,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}, {
		TraceID:            h(circleTraceKeys),
		MostRecentCommitID: expectedCommitID,
		Digest:             d(dks.DigestC01Pos),
		OptionsID:          h(pngOptions),
		GroupingID:         h(circleGrouping),
		Corpus:             dks.RoundCorpus,
		Keys: map[string]string{
			types.CorpusField:     dks.RoundCorpus,
			types.PrimaryKeyField: dks.CircleTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.Windows10dot2OS,
			dks.DeviceKey:         dks.QuadroDevice,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}}, actualValuesAtHead)

	actualExpectations := sqltest.GetAllRows(ctx, t, db, "Expectations", &schema.ExpectationRow{}).([]schema.ExpectationRow)
	assert.Equal(t, []schema.ExpectationRow{{
		GroupingID: h(squareGrouping),
		Digest:     d(dks.DigestA01Pos),
		Label:      schema.LabelUntriaged,
	}, {
		GroupingID: h(triangleGrouping),
		Digest:     d(dks.DigestB01Pos),
		Label:      schema.LabelUntriaged,
	}, {
		GroupingID: h(circleGrouping),
		Digest:     d(dks.DigestC01Pos),
		Label:      schema.LabelUntriaged,
	}}, actualExpectations)

	actualPrimaryBranchParams := sqltest.GetAllRows(ctx, t, db, "PrimaryBranchParams", &schema.PrimaryBranchParamRow{}).([]schema.PrimaryBranchParamRow)
	assert.Equal(t, []schema.PrimaryBranchParamRow{
		{Key: dks.ColorModeKey, Value: dks.RGBColorMode, TileID: 0},
		{Key: dks.DeviceKey, Value: dks.QuadroDevice, TileID: 0},
		{Key: "ext", Value: "png", TileID: 0},
		{Key: types.PrimaryKeyField, Value: dks.CircleTest, TileID: 0},
		{Key: types.PrimaryKeyField, Value: dks.SquareTest, TileID: 0},
		{Key: types.PrimaryKeyField, Value: dks.TriangleTest, TileID: 0},
		{Key: dks.OSKey, Value: dks.Windows10dot2OS, TileID: 0},
		{Key: types.CorpusField, Value: dks.CornersCorpus, TileID: 0},
		{Key: types.CorpusField, Value: dks.RoundCorpus, TileID: 0},
	}, actualPrimaryBranchParams)

	actualTiledTraceDigests := sqltest.GetAllRows(ctx, t, db, "TiledTraceDigests", &schema.TiledTraceDigestRow{}).([]schema.TiledTraceDigestRow)
	assert.Equal(t, []schema.TiledTraceDigestRow{
		{TraceID: h(squareTraceKeys), Digest: d(dks.DigestA01Pos), TileID: 0},
		{TraceID: h(triangleTraceKeys), Digest: d(dks.DigestB01Pos), TileID: 0},
		{TraceID: h(circleTraceKeys), Digest: d(dks.DigestC01Pos), TileID: 0},
	}, actualTiledTraceDigests)

	assert.Equal(t, totalMetricBefore+1, s.filesProcessed.Get())
	assert.Equal(t, successMetricBefore+1, s.filesSuccess.Get())
	assert.Equal(t, resultsMetricBefore+3, s.resultsIngested.Get())
}

func TestPrimarySQL_Process_TileAlreadyComputed_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	const androidTraceKeys = `{"name":"square","os":"Android","source_type":"corners"}`
	const windowsTraceKeys = `{"name":"square","os":"Windows10.3","source_type":"corners"}`
	const fuzzyOptions = `{"ext":"png","fuzzy_ignored_border_thickness":"0","fuzzy_max_different_pixels":"10","fuzzy_pixel_delta_threshold":"20","image_matching_algorithm":"fuzzy"}`

	existingData := makeDataForTileTests()

	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	src := fakeGCSSourceFromFile(t, "use_existing_commit_in_tile1.json")
	s := PrimaryBranchSQL(src, map[string]string{sqlTileWidthConfig: "4"}, db)
	require.NoError(t, s.Process(ctx, "use_existing_commit_in_tile1.json"))

	// Check that all tiled data is calculated correctly
	actualCommitsWithData := sqltest.GetAllRows(ctx, t, db, "CommitsWithData", &schema.CommitWithDataRow{}).([]schema.CommitWithDataRow)
	assert.Equal(t, []schema.CommitWithDataRow{
		{CommitID: "0000000098", TileID: 0},
		{CommitID: "0000000099", TileID: 0},
		{CommitID: "0000000100", TileID: 0},
		{CommitID: "0000000101", TileID: 0},
		{CommitID: "0000000103", TileID: 1},
		{CommitID: "0000000106", TileID: 1},
		{CommitID: "0000000107", TileID: 1},
		{CommitID: "0000000108", TileID: 1},
	}, actualCommitsWithData)

	actualTiledTraces := sqltest.GetAllRows(ctx, t, db, "TiledTraceDigests", &schema.TiledTraceDigestRow{}).([]schema.TiledTraceDigestRow)
	assert.ElementsMatch(t, []schema.TiledTraceDigestRow{
		{TraceID: h(androidTraceKeys), Digest: d(dks.DigestA02Pos), TileID: 0},
		{TraceID: h(windowsTraceKeys), Digest: d(dks.DigestA03Pos), TileID: 0},
		{TraceID: h(androidTraceKeys), Digest: d(dks.DigestA02Pos), TileID: 1},
		{TraceID: h(androidTraceKeys), Digest: d(dks.DigestA03Pos), TileID: 1},
		{TraceID: h(windowsTraceKeys), Digest: d(dks.DigestA02Pos), TileID: 1},
		{TraceID: h(windowsTraceKeys), Digest: d(dks.DigestA03Pos), TileID: 1},
	}, actualTiledTraces)

	actualPrimaryBranchParams := sqltest.GetAllRows(ctx, t, db, "PrimaryBranchParams", &schema.PrimaryBranchParamRow{}).([]schema.PrimaryBranchParamRow)
	assert.ElementsMatch(t, []schema.PrimaryBranchParamRow{
		{Key: "ext", Value: "png", TileID: 0},
		{Key: "ext", Value: "png", TileID: 1},
		{Key: "fuzzy_ignored_border_thickness", Value: "0", TileID: 1},
		{Key: "fuzzy_max_different_pixels", Value: "10", TileID: 1},
		{Key: "fuzzy_pixel_delta_threshold", Value: "20", TileID: 1},
		{Key: "image_matching_algorithm", Value: "fuzzy", TileID: 1},
		{Key: types.PrimaryKeyField, Value: dks.SquareTest, TileID: 0},
		{Key: types.PrimaryKeyField, Value: dks.SquareTest, TileID: 1},
		{Key: dks.OSKey, Value: dks.AndroidOS, TileID: 0},
		{Key: dks.OSKey, Value: dks.AndroidOS, TileID: 1},
		{Key: dks.OSKey, Value: dks.Windows10dot3OS, TileID: 0},
		{Key: dks.OSKey, Value: dks.Windows10dot3OS, TileID: 1},
		{Key: types.CorpusField, Value: dks.CornersCorpus, TileID: 0},
		{Key: types.CorpusField, Value: dks.CornersCorpus, TileID: 1},
	}, actualPrimaryBranchParams)

	// Spot check some other interesting data.
	actualOptions := sqltest.GetAllRows(ctx, t, db, "Options", &schema.OptionsRow{}).([]schema.OptionsRow)
	assert.ElementsMatch(t, []schema.OptionsRow{{
		OptionsID: h(pngOptions),
		Keys: map[string]string{
			"ext": "png",
		},
	}, {
		OptionsID: h(fuzzyOptions),
		Keys: map[string]string{
			"ext":                            "png",
			"image_matching_algorithm":       "fuzzy",
			"fuzzy_max_different_pixels":     "10",
			"fuzzy_pixel_delta_threshold":    "20",
			"fuzzy_ignored_border_thickness": "0",
		},
	}}, actualOptions)

	actualValuesAtHead := sqltest.GetAllRows(ctx, t, db, "ValuesAtHead", &schema.ValueAtHeadRow{}).([]schema.ValueAtHeadRow)
	assert.Equal(t, []schema.ValueAtHeadRow{{
		TraceID:            h(windowsTraceKeys),
		MostRecentCommitID: "0000000108",
		Digest:             d(dks.DigestA03Pos),
		OptionsID:          h(pngOptions),
		GroupingID:         h(squareGrouping),
		Corpus:             dks.CornersCorpus,
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
			dks.OSKey:             dks.Windows10dot3OS,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}, {
		TraceID:            h(androidTraceKeys),
		MostRecentCommitID: "0000000108",
		Digest:             d(dks.DigestA02Pos),
		OptionsID:          h(pngOptions),
		GroupingID:         h(squareGrouping),
		Corpus:             dks.CornersCorpus,
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
			dks.OSKey:             dks.AndroidOS,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}}, actualValuesAtHead)
}

func TestPrimarySQL_Process_PreviousTilesAreFull_NewTileCreated(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	const androidTraceKeys = `{"name":"square","os":"Android","source_type":"corners"}`
	const windowsTraceKeys = `{"name":"square","os":"Windows10.3","source_type":"corners"}`
	const fuzzyOptions = `{"ext":"png","fuzzy_ignored_border_thickness":"0","fuzzy_max_different_pixels":"10","fuzzy_pixel_delta_threshold":"20","image_matching_algorithm":"fuzzy"}`
	const latestCommitID = "0000000109"

	existingData := makeDataForTileTests()
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	src := fakeGCSSourceFromFile(t, "should_start_tile_2.json")
	s := PrimaryBranchSQL(src, map[string]string{sqlTileWidthConfig: "4"}, db)
	require.NoError(t, s.Process(ctx, "should_start_tile_2.json"))

	// Check that all tiled data is calculated correctly
	actualCommitsWithData := sqltest.GetAllRows(ctx, t, db, "CommitsWithData", &schema.CommitWithDataRow{}).([]schema.CommitWithDataRow)
	assert.Equal(t, []schema.CommitWithDataRow{
		{CommitID: "0000000098", TileID: 0},
		{CommitID: "0000000099", TileID: 0},
		{CommitID: "0000000100", TileID: 0},
		{CommitID: "0000000101", TileID: 0},
		{CommitID: "0000000103", TileID: 1},
		{CommitID: "0000000106", TileID: 1},
		{CommitID: "0000000107", TileID: 1},
		{CommitID: "0000000108", TileID: 1},
		{CommitID: latestCommitID, TileID: 2}, // newly created
	}, actualCommitsWithData)

	actualTiledTraces := sqltest.GetAllRows(ctx, t, db, "TiledTraceDigests", &schema.TiledTraceDigestRow{}).([]schema.TiledTraceDigestRow)
	assert.ElementsMatch(t, []schema.TiledTraceDigestRow{
		{TraceID: h(androidTraceKeys), Digest: d(dks.DigestA02Pos), TileID: 0},
		{TraceID: h(windowsTraceKeys), Digest: d(dks.DigestA03Pos), TileID: 0},
		{TraceID: h(androidTraceKeys), Digest: d(dks.DigestA02Pos), TileID: 1},
		{TraceID: h(androidTraceKeys), Digest: d(dks.DigestA03Pos), TileID: 1},
		{TraceID: h(windowsTraceKeys), Digest: d(dks.DigestA03Pos), TileID: 1},
		{TraceID: h(androidTraceKeys), Digest: d(dks.DigestA02Pos), TileID: 2},
		{TraceID: h(windowsTraceKeys), Digest: d(dks.DigestA02Pos), TileID: 2},
	}, actualTiledTraces)

	actualPrimaryBranchParams := sqltest.GetAllRows(ctx, t, db, "PrimaryBranchParams", &schema.PrimaryBranchParamRow{}).([]schema.PrimaryBranchParamRow)
	assert.ElementsMatch(t, []schema.PrimaryBranchParamRow{
		{Key: "ext", Value: "png", TileID: 0},
		{Key: "ext", Value: "png", TileID: 1},
		{Key: "ext", Value: "png", TileID: 2},
		{Key: "fuzzy_ignored_border_thickness", Value: "0", TileID: 2},
		{Key: "fuzzy_max_different_pixels", Value: "10", TileID: 2},
		{Key: "fuzzy_pixel_delta_threshold", Value: "20", TileID: 2},
		{Key: "image_matching_algorithm", Value: "fuzzy", TileID: 2},
		{Key: types.PrimaryKeyField, Value: dks.SquareTest, TileID: 0},
		{Key: types.PrimaryKeyField, Value: dks.SquareTest, TileID: 1},
		{Key: types.PrimaryKeyField, Value: dks.SquareTest, TileID: 2},
		{Key: dks.OSKey, Value: dks.AndroidOS, TileID: 0},
		{Key: dks.OSKey, Value: dks.AndroidOS, TileID: 1},
		{Key: dks.OSKey, Value: dks.AndroidOS, TileID: 2},
		{Key: dks.OSKey, Value: dks.Windows10dot3OS, TileID: 0},
		{Key: dks.OSKey, Value: dks.Windows10dot3OS, TileID: 1},
		{Key: dks.OSKey, Value: dks.Windows10dot3OS, TileID: 2},
		{Key: types.CorpusField, Value: dks.CornersCorpus, TileID: 0},
		{Key: types.CorpusField, Value: dks.CornersCorpus, TileID: 1},
		{Key: types.CorpusField, Value: dks.CornersCorpus, TileID: 2},
	}, actualPrimaryBranchParams)

	// Spot check some other interesting data.
	actualOptions := sqltest.GetAllRows(ctx, t, db, "Options", &schema.OptionsRow{}).([]schema.OptionsRow)
	assert.ElementsMatch(t, []schema.OptionsRow{{
		OptionsID: h(pngOptions),
		Keys: map[string]string{
			"ext": "png",
		},
	}, {
		OptionsID: h(fuzzyOptions),
		Keys: map[string]string{
			"ext":                            "png",
			"image_matching_algorithm":       "fuzzy",
			"fuzzy_max_different_pixels":     "10",
			"fuzzy_pixel_delta_threshold":    "20",
			"fuzzy_ignored_border_thickness": "0",
		},
	}}, actualOptions)

	actualValuesAtHead := sqltest.GetAllRows(ctx, t, db, "ValuesAtHead", &schema.ValueAtHeadRow{}).([]schema.ValueAtHeadRow)
	assert.Equal(t, []schema.ValueAtHeadRow{{
		TraceID:            h(windowsTraceKeys),
		MostRecentCommitID: latestCommitID,
		Digest:             d(dks.DigestA02Pos),
		OptionsID:          h(pngOptions),
		GroupingID:         h(squareGrouping),
		Corpus:             dks.CornersCorpus,
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
			dks.OSKey:             dks.Windows10dot3OS,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}, {
		TraceID:            h(androidTraceKeys),
		MostRecentCommitID: latestCommitID,
		Digest:             d(dks.DigestA02Pos),
		OptionsID:          h(fuzzyOptions),
		GroupingID:         h(squareGrouping),
		Corpus:             dks.CornersCorpus,
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
			dks.OSKey:             dks.AndroidOS,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}}, actualValuesAtHead)
}

func TestPrimarySQL_Process_BetweenTwoTiles_UseHigherTile(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	const androidTraceKeys = `{"name":"square","os":"Android","source_type":"corners"}`
	const windowsTraceKeys = `{"name":"square","os":"Windows10.3","source_type":"corners"}`

	existingData := makeDataForTileTests()
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	src := fakeGCSSourceFromFile(t, "between_tile_1_and_2.json")
	s := PrimaryBranchSQL(src, map[string]string{sqlTileWidthConfig: "4"}, db)
	require.NoError(t, s.Process(ctx, "between_tile_1_and_2.json"))

	// Check that all tiled data is calculated correctly
	actualCommitsWithData := sqltest.GetAllRows(ctx, t, db, "CommitsWithData", &schema.CommitWithDataRow{}).([]schema.CommitWithDataRow)
	assert.Equal(t, []schema.CommitWithDataRow{
		{CommitID: "0000000098", TileID: 0},
		{CommitID: "0000000099", TileID: 0},
		{CommitID: "0000000100", TileID: 0},
		{CommitID: "0000000101", TileID: 0},
		{CommitID: "0000000102", TileID: 1}, // newly created
		{CommitID: "0000000103", TileID: 1},
		{CommitID: "0000000106", TileID: 1},
		{CommitID: "0000000107", TileID: 1},
		{CommitID: "0000000108", TileID: 1},
	}, actualCommitsWithData)

	actualTiledTraces := sqltest.GetAllRows(ctx, t, db, "TiledTraceDigests", &schema.TiledTraceDigestRow{}).([]schema.TiledTraceDigestRow)
	assert.ElementsMatch(t, []schema.TiledTraceDigestRow{
		{TraceID: h(androidTraceKeys), Digest: d(dks.DigestA02Pos), TileID: 0},
		{TraceID: h(windowsTraceKeys), Digest: d(dks.DigestA03Pos), TileID: 0},
		{TraceID: h(androidTraceKeys), Digest: d(dks.DigestA02Pos), TileID: 1},
		{TraceID: h(androidTraceKeys), Digest: d(dks.DigestA03Pos), TileID: 1},
		{TraceID: h(windowsTraceKeys), Digest: d(dks.DigestA02Pos), TileID: 1},
		{TraceID: h(windowsTraceKeys), Digest: d(dks.DigestA03Pos), TileID: 1},
	}, actualTiledTraces)

	actualPrimaryBranchParams := sqltest.GetAllRows(ctx, t, db, "PrimaryBranchParams", &schema.PrimaryBranchParamRow{}).([]schema.PrimaryBranchParamRow)
	assert.ElementsMatch(t, []schema.PrimaryBranchParamRow{
		{Key: "ext", Value: "png", TileID: 0},
		{Key: "ext", Value: "png", TileID: 1},
		{Key: types.PrimaryKeyField, Value: dks.SquareTest, TileID: 0},
		{Key: types.PrimaryKeyField, Value: dks.SquareTest, TileID: 1},
		{Key: dks.OSKey, Value: dks.AndroidOS, TileID: 0},
		{Key: dks.OSKey, Value: dks.AndroidOS, TileID: 1},
		{Key: dks.OSKey, Value: dks.Windows10dot3OS, TileID: 0},
		{Key: dks.OSKey, Value: dks.Windows10dot3OS, TileID: 1},
		{Key: types.CorpusField, Value: dks.CornersCorpus, TileID: 0},
		{Key: types.CorpusField, Value: dks.CornersCorpus, TileID: 1},
	}, actualPrimaryBranchParams)
}

func TestPrimarySQL_Process_SurroundingCommitsHaveSameTile_UseThatTile(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	const androidTraceKeys = `{"name":"square","os":"Android","source_type":"corners"}`
	const windowsTraceKeys = `{"name":"square","os":"Windows10.3","source_type":"corners"}`

	existingData := makeDataForTileTests()
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	src := fakeGCSSourceFromFile(t, "should_create_in_tile_1.json")
	s := PrimaryBranchSQL(src, map[string]string{sqlTileWidthConfig: "4"}, db)
	require.NoError(t, s.Process(ctx, "should_create_in_tile_1.json"))

	// Check that all tiled data is calculated correctly
	actualCommitsWithData := sqltest.GetAllRows(ctx, t, db, "CommitsWithData", &schema.CommitWithDataRow{}).([]schema.CommitWithDataRow)
	assert.Equal(t, []schema.CommitWithDataRow{
		{CommitID: "0000000098", TileID: 0},
		{CommitID: "0000000099", TileID: 0},
		{CommitID: "0000000100", TileID: 0},
		{CommitID: "0000000101", TileID: 0},
		{CommitID: "0000000103", TileID: 1},
		{CommitID: "0000000105", TileID: 1}, // newly created
		{CommitID: "0000000106", TileID: 1},
		{CommitID: "0000000107", TileID: 1},
		{CommitID: "0000000108", TileID: 1},
	}, actualCommitsWithData)

	actualTiledTraces := sqltest.GetAllRows(ctx, t, db, "TiledTraceDigests", &schema.TiledTraceDigestRow{}).([]schema.TiledTraceDigestRow)
	assert.ElementsMatch(t, []schema.TiledTraceDigestRow{
		{TraceID: h(androidTraceKeys), Digest: d(dks.DigestA02Pos), TileID: 0},
		{TraceID: h(windowsTraceKeys), Digest: d(dks.DigestA03Pos), TileID: 0},
		{TraceID: h(androidTraceKeys), Digest: d(dks.DigestA02Pos), TileID: 1},
		{TraceID: h(androidTraceKeys), Digest: d(dks.DigestA03Pos), TileID: 1},
		{TraceID: h(windowsTraceKeys), Digest: d(dks.DigestA02Pos), TileID: 1},
		{TraceID: h(windowsTraceKeys), Digest: d(dks.DigestA03Pos), TileID: 1},
	}, actualTiledTraces)

	actualPrimaryBranchParams := sqltest.GetAllRows(ctx, t, db, "PrimaryBranchParams", &schema.PrimaryBranchParamRow{}).([]schema.PrimaryBranchParamRow)
	assert.ElementsMatch(t, []schema.PrimaryBranchParamRow{
		{Key: "ext", Value: "png", TileID: 0},
		{Key: "ext", Value: "png", TileID: 1},
		{Key: types.PrimaryKeyField, Value: dks.SquareTest, TileID: 0},
		{Key: types.PrimaryKeyField, Value: dks.SquareTest, TileID: 1},
		{Key: dks.OSKey, Value: dks.AndroidOS, TileID: 0},
		{Key: dks.OSKey, Value: dks.AndroidOS, TileID: 1},
		{Key: dks.OSKey, Value: dks.Windows10dot3OS, TileID: 0},
		{Key: dks.OSKey, Value: dks.Windows10dot3OS, TileID: 1},
		{Key: types.CorpusField, Value: dks.CornersCorpus, TileID: 0},
		{Key: types.CorpusField, Value: dks.CornersCorpus, TileID: 1},
	}, actualPrimaryBranchParams)
}

func TestPrimarySQL_Process_AtEndTileNotFull_UseThatTile(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	const androidTraceKeys = `{"name":"square","os":"Android","source_type":"corners"}`
	const windowsTraceKeys = `{"name":"square","os":"Windows10.3","source_type":"corners"}`

	existingData := makeDataForTileTests()
	// Trim off last 3 commits with data
	existingData.CommitsWithData = existingData.CommitsWithData[:len(existingData.CommitsWithData)-3]
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	src := fakeGCSSourceFromFile(t, "should_create_in_tile_1.json")
	s := PrimaryBranchSQL(src, map[string]string{sqlTileWidthConfig: "4"}, db)
	require.NoError(t, s.Process(ctx, "should_create_in_tile_1.json"))

	// Check that all tiled data is calculated correctly
	actualCommitsWithData := sqltest.GetAllRows(ctx, t, db, "CommitsWithData", &schema.CommitWithDataRow{}).([]schema.CommitWithDataRow)
	assert.Equal(t, []schema.CommitWithDataRow{
		{CommitID: "0000000098", TileID: 0},
		{CommitID: "0000000099", TileID: 0},
		{CommitID: "0000000100", TileID: 0},
		{CommitID: "0000000101", TileID: 0},
		{CommitID: "0000000103", TileID: 1},
		{CommitID: "0000000105", TileID: 1}, // newly created
	}, actualCommitsWithData)

	actualTiledTraces := sqltest.GetAllRows(ctx, t, db, "TiledTraceDigests", &schema.TiledTraceDigestRow{}).([]schema.TiledTraceDigestRow)
	assert.ElementsMatch(t, []schema.TiledTraceDigestRow{
		{TraceID: h(androidTraceKeys), Digest: d(dks.DigestA02Pos), TileID: 0},
		{TraceID: h(windowsTraceKeys), Digest: d(dks.DigestA03Pos), TileID: 0},
		{TraceID: h(androidTraceKeys), Digest: d(dks.DigestA02Pos), TileID: 1},
		{TraceID: h(androidTraceKeys), Digest: d(dks.DigestA03Pos), TileID: 1},
		{TraceID: h(windowsTraceKeys), Digest: d(dks.DigestA02Pos), TileID: 1},
		{TraceID: h(windowsTraceKeys), Digest: d(dks.DigestA03Pos), TileID: 1},
	}, actualTiledTraces)

	actualPrimaryBranchParams := sqltest.GetAllRows(ctx, t, db, "PrimaryBranchParams", &schema.PrimaryBranchParamRow{}).([]schema.PrimaryBranchParamRow)
	assert.ElementsMatch(t, []schema.PrimaryBranchParamRow{
		{Key: "ext", Value: "png", TileID: 0},
		{Key: "ext", Value: "png", TileID: 1},
		{Key: types.PrimaryKeyField, Value: dks.SquareTest, TileID: 0},
		{Key: types.PrimaryKeyField, Value: dks.SquareTest, TileID: 1},
		{Key: dks.OSKey, Value: dks.AndroidOS, TileID: 0},
		{Key: dks.OSKey, Value: dks.AndroidOS, TileID: 1},
		{Key: dks.OSKey, Value: dks.Windows10dot3OS, TileID: 0},
		{Key: dks.OSKey, Value: dks.Windows10dot3OS, TileID: 1},
		{Key: types.CorpusField, Value: dks.CornersCorpus, TileID: 0},
		{Key: types.CorpusField, Value: dks.CornersCorpus, TileID: 1},
	}, actualPrimaryBranchParams)
}

func TestPrimarySQL_Process_SameFileMultipleTimesInParallel_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	totalMetricBefore := metrics2.GetCounter("gold_primarysqlingestion_files_processed").Get()
	successMetricBefore := metrics2.GetCounter("gold_primarysqlingestion_files_success").Get()
	resultsMetricBefore := metrics2.GetCounter("gold_primarysqlingestion_results_ingested").Get()

	wg := sync.WaitGroup{}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			src := fakeGCSSourceFromFile(t, "primary2.json")
			s := PrimaryBranchSQL(src, map[string]string{sqlTileWidthConfig: "5"}, db)

			for j := 0; j < 10; j++ {
				if err := ctx.Err(); err != nil {
					return
				}
				if err := s.Process(ctx, "primary2.json"); err != nil {
					assert.NoError(t, err)
					return
				}
			}
			return
		}()
	}
	wg.Wait()
	// spot check the data to make sure it was written
	actualValuesAtHead := sqltest.GetAllRows(ctx, t, db, "ValuesAtHead", &schema.ValueAtHeadRow{}).([]schema.ValueAtHeadRow)
	assert.Contains(t, actualValuesAtHead, schema.ValueAtHeadRow{
		TraceID:            h(`{"color mode":"RGB","device":"QuadroP400","name":"triangle","os":"Windows10.2","source_type":"corners"}`),
		MostRecentCommitID: "0000000105", // This was updated because of the ingested file
		Digest:             d(dks.DigestBlank),
		OptionsID:          h(pngOptions),
		GroupingID:         h(triangleGrouping),
		Corpus:             dks.CornersCorpus,
		Keys: paramtools.Params{
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.DeviceKey:         dks.QuadroDevice,
			dks.OSKey:             dks.Windows10dot2OS,
			types.PrimaryKeyField: dks.TriangleTest,
			types.CorpusField:     dks.CornersCorpus,
		},
		MatchesAnyIgnoreRule: schema.NBFalse,
	})
	actualExpectations := sqltest.GetAllRows(ctx, t, db, "Expectations", &schema.ExpectationRow{}).([]schema.ExpectationRow)
	assert.Contains(t, actualExpectations, schema.ExpectationRow{
		GroupingID: h(squareGrouping),
		Digest:     d(dks.DigestBlank),
		Label:      schema.LabelUntriaged,
	})
	assert.Contains(t, actualExpectations, schema.ExpectationRow{
		GroupingID: h(triangleGrouping),
		Digest:     d(dks.DigestBlank),
		Label:      schema.LabelUntriaged,
	})

	// We ingested a total of 40 files, each with 12 results
	assert.Equal(t, totalMetricBefore+40, metrics2.GetCounter("gold_primarysqlingestion_files_processed").Get())
	assert.Equal(t, successMetricBefore+40, metrics2.GetCounter("gold_primarysqlingestion_files_success").Get())
	assert.Equal(t, resultsMetricBefore+480, metrics2.GetCounter("gold_primarysqlingestion_results_ingested").Get())
}

func TestPrimarySQL_Process_UnknownGitHash_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	// GitCommits table is empty, meaning all commits are unknown.

	src := fakeGCSSourceFromFile(t, "primary1.json")
	s := PrimaryBranchSQL(src, map[string]string{sqlTileWidthConfig: "5"}, db)

	err := s.Process(ctx, "whatever")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "looking up git_hash")
}

func TestPrimarySQL_Process_MissingGitHash_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	src := fakeGCSSourceFromFile(t, "missing_git_hash.json")
	s := PrimaryBranchSQL(src, map[string]string{sqlTileWidthConfig: "5"}, db)

	err := s.Process(ctx, "whatever")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"gitHash" must be hexadecimal`)
}

func TestPrimarySQL_Process_NoResults_NoDataWritten(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	validCommits := dks.Build().GitCommits
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, schema.Tables{
		GitCommits: validCommits,
	}))

	src := fakeGCSSourceFromFile(t, "no_results.json")
	s := PrimaryBranchSQL(src, map[string]string{sqlTileWidthConfig: "5"}, db)
	totalMetricBefore := s.filesProcessed.Get()
	successMetricBefore := s.filesSuccess.Get()
	resultsMetricBefore := s.resultsIngested.Get()

	err := s.Process(ctx, "whatever")
	require.NoError(t, err)

	actualSourceFiles := sqltest.GetAllRows(ctx, t, db, "SourceFiles", &schema.SourceFileRow{}).([]schema.SourceFileRow)
	assert.Empty(t, actualSourceFiles)

	acutalCommitsWithData := sqltest.GetAllRows(ctx, t, db, "CommitsWithData", &schema.CommitWithDataRow{}).([]schema.CommitWithDataRow)
	assert.Empty(t, acutalCommitsWithData)

	assert.Equal(t, totalMetricBefore+1, s.filesProcessed.Get())
	assert.Equal(t, successMetricBefore, s.filesSuccess.Get())
	assert.Equal(t, resultsMetricBefore, s.resultsIngested.Get())
}

func repeat(s string, n int) []string {
	rv := make([]string, 0, n)
	for i := 0; i < n; i++ {
		rv = append(rv, s)
	}
	return rv
}

var fakeIngestionTime = time.Date(2021, time.March, 14, 15, 9, 26, 0, time.UTC)

const (
	circleGrouping   = `{"name":"circle","source_type":"round"}`
	squareGrouping   = `{"name":"square","source_type":"corners"}`
	triangleGrouping = `{"name":"triangle","source_type":"corners"}`

	pngOptions = `{"ext":"png"}`
)

// h returns the MD5 hash of the provided string.
func h(s string) []byte {
	hash := md5.Sum([]byte(s))
	return hash[:]
}

// d returns the bytes associated with the hex-encoded digest string.
func d(digest types.Digest) []byte {
	if len(digest) != 2*md5.Size {
		panic("digest wrong length " + string(digest))
	}
	b, err := hex.DecodeString(string(digest))
	if err != nil {
		panic(err)
	}
	return b
}

func overwriteNow(ctx context.Context, ts time.Time) context.Context {
	return context.WithValue(ctx, overwriteNowKey, ts)
}

// makeDataForTileTests returns a data set that has some gaps for new data to be inserted in
// various places to test that tileIDs are properly created and respected.
func makeDataForTileTests() schema.Tables {
	b := databuilder.TablesBuilder{TileWidth: 4}
	b.CommitsWithData().
		Insert("0000000098", "user", "commit 98, expected tile 0", "2020-12-01T00:00:00Z").
		Insert("0000000099", "user", "commit 99, expected tile 0", "2020-12-02T00:00:00Z").
		Insert("0000000100", "user", "commit 100, expected tile 0", "2020-12-03T00:00:00Z").
		Insert("0000000101", "user", "commit 101, expected tile 0", "2020-12-04T00:00:00Z").
		Insert("0000000103", "user", "commit 103, expected tile 1", "2020-12-05T00:00:00Z").
		Insert("0000000106", "user", "commit 106, expected tile 1", "2020-12-07T00:00:00Z").
		Insert("0000000107", "user", "commit 107, expected tile 1", "2020-12-08T00:00:00Z").
		Insert("0000000108", "user", "commit 108, expected tile 1", "2020-12-09T00:00:00Z")
	b.CommitsWithNoData().
		Insert("0000000102", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "user", "commit 102, expected tile 1", "2020-12-05T00:00:00Z").
		Insert("0000000105", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "user", "commit 105, expected tile 1", "2020-12-05T00:00:00Z").
		Insert("0000000109", "cccccccccccccccccccccccccccccccccccccccc", "user", "commit 109, expected tile 2", "2020-12-05T00:00:00Z")
	b.SetDigests(map[rune]types.Digest{
		'B': dks.DigestA02Pos,
		'C': dks.DigestA03Pos,
	})
	b.SetGroupingKeys(types.CorpusField, types.PrimaryKeyField)
	b.AddTracesWithCommonKeys(paramtools.Params{
		types.CorpusField:     dks.CornersCorpus,
		types.PrimaryKeyField: dks.SquareTest,
	}).History(
		"BBBBCBCB",
		"CCCCCCCC",
	).Keys([]paramtools.Params{
		{dks.OSKey: dks.AndroidOS},
		{dks.OSKey: dks.Windows10dot3OS},
	}).OptionsAll(paramtools.Params{"ext": "png"}).
		IngestedFrom(repeat("dontcare", 8), repeat("2020-12-01T00:42:00Z", 8))
	existingData := b.Build()
	return existingData
}
