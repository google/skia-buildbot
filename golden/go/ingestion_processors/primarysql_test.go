package ingestion_processors

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
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
	s := PrimaryBranchSQL(ctx, src, map[string]string{sqlTileWidthConfig: "5"}, db)

	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := s.Process(ctx, dks.WindowsFile3)
	require.NoError(t, err)

	actualSourceFiles := getAllSourceFiles(ctx, t, db)
	assert.Equal(t, []schema.SourceFileRow{{
		SourceFileID: h(dks.WindowsFile3),
		SourceFile:   dks.WindowsFile3,
		LastIngested: fakeIngestionTime,
	}}, actualSourceFiles)

	actualGroupings := getAllGroupings(ctx, t, db)
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

	actualOptions := getAllOptions(ctx, t, db)
	assert.ElementsMatch(t, []schema.OptionsRow{{
		OptionsID: h(pngOptions),
		Keys: map[string]string{
			"ext": "png",
		},
	}}, actualOptions)

	actualTraces := getAllTraces(ctx, t, db)
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

	actualCommitsWithData := getAllCommitsWithData(ctx, t, db)
	assert.Equal(t, []schema.CommitWithDataRow{{
		CommitID: expectedCommitID,
		TileID:   0,
	}}, actualCommitsWithData)

	actualTraceValues := getAllTraceValues(ctx, t, db)
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

	actualValuesAtHead := getAllValuesAtHead(ctx, t, db)
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
		Label:                schema.LabelUntriaged,
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
		Label:                schema.LabelUntriaged,
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
		Label:                schema.LabelUntriaged,
		MatchesAnyIgnoreRule: schema.NBNull,
	}}, actualValuesAtHead)

	actualExpectations := getAllExpectations(ctx, t, db)
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

	actualPrimaryBranchParams := getAllPrimaryBranchParams(ctx, t, db)
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

	actualTiledTraceDigests := getAllTiledTraceDigests(ctx, t, db)
	assert.Equal(t, []schema.TiledTraceDigestRow{
		{TraceID: h(squareTraceKeys), Digest: d(dks.DigestA01Pos), TileID: 0},
		{TraceID: h(triangleTraceKeys), Digest: d(dks.DigestB01Pos), TileID: 0},
		{TraceID: h(circleTraceKeys), Digest: d(dks.DigestC01Pos), TileID: 0},
	}, actualTiledTraceDigests)
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

func getAllSourceFiles(ctx context.Context, t *testing.T, db *pgxpool.Pool) []schema.SourceFileRow {
	rows, err := db.Query(ctx, `SELECT * FROM SourceFiles ORDER BY last_ingested ASC`)
	require.NoError(t, err)
	defer rows.Close()
	var rv []schema.SourceFileRow
	for rows.Next() {
		var sfr schema.SourceFileRow
		require.NoError(t, rows.Scan(&sfr.SourceFileID, &sfr.SourceFile, &sfr.LastIngested))
		sfr.LastIngested = sfr.LastIngested.UTC()
		rv = append(rv, sfr)
	}
	return rv
}

func getAllGroupings(ctx context.Context, t *testing.T, db *pgxpool.Pool) []schema.GroupingRow {
	rows, err := db.Query(ctx, `SELECT * FROM Groupings`)
	require.NoError(t, err)
	defer rows.Close()
	var rv []schema.GroupingRow
	for rows.Next() {
		var gr schema.GroupingRow
		require.NoError(t, rows.Scan(&gr.GroupingID, &gr.Keys))
		rv = append(rv, gr)
	}
	return rv
}

func getAllOptions(ctx context.Context, t *testing.T, db *pgxpool.Pool) []schema.OptionsRow {
	rows, err := db.Query(ctx, `SELECT * FROM Options`)
	require.NoError(t, err)
	defer rows.Close()
	var rv []schema.OptionsRow
	for rows.Next() {
		var or schema.OptionsRow
		require.NoError(t, rows.Scan(&or.OptionsID, &or.Keys))
		rv = append(rv, or)
	}
	return rv
}

func getAllTraces(ctx context.Context, t *testing.T, db *pgxpool.Pool) []schema.TraceRow {
	rows, err := db.Query(ctx, `SELECT * FROM Traces ORDER BY keys->>'name'`)
	require.NoError(t, err)
	defer rows.Close()
	var rv []schema.TraceRow
	for rows.Next() {
		var tr schema.TraceRow
		var matches pgtype.Bool
		require.NoError(t, rows.Scan(&tr.TraceID, &tr.Corpus, &tr.GroupingID, &tr.Keys, &matches))
		tr.MatchesAnyIgnoreRule = toNullableBool(matches)
		rv = append(rv, tr)
	}
	return rv
}

func toNullableBool(matches pgtype.Bool) schema.NullableBool {
	if matches.Status == pgtype.Present {
		if matches.Bool {
			return schema.NBTrue
		} else {
			return schema.NBFalse
		}
	}
	return schema.NBNull
}

func getAllCommitsWithData(ctx context.Context, t *testing.T, db *pgxpool.Pool) []schema.CommitWithDataRow {
	rows, err := db.Query(ctx, `SELECT * FROM CommitsWithData ORDER BY commit_id ASC`)
	require.NoError(t, err)
	defer rows.Close()
	var rv []schema.CommitWithDataRow
	for rows.Next() {
		var cr schema.CommitWithDataRow
		require.NoError(t, rows.Scan(&cr.CommitID, &cr.TileID))
		rv = append(rv, cr)
	}
	return rv
}

func getAllTraceValues(ctx context.Context, t *testing.T, db *pgxpool.Pool) []schema.TraceValueRow {
	rows, err := db.Query(ctx, `SELECT * FROM TraceValues`)
	require.NoError(t, err)
	defer rows.Close()
	var rv []schema.TraceValueRow
	for rows.Next() {
		var tr schema.TraceValueRow
		require.NoError(t, rows.Scan(&tr.Shard, &tr.TraceID, &tr.CommitID, &tr.Digest, &tr.GroupingID,
			&tr.OptionsID, &tr.SourceFileID))
		rv = append(rv, tr)
	}
	return rv
}

func getAllValuesAtHead(ctx context.Context, t *testing.T, db *pgxpool.Pool) []schema.ValueAtHeadRow {
	rows, err := db.Query(ctx, `SELECT * FROM ValuesAtHead`)
	require.NoError(t, err)
	defer rows.Close()
	var rv []schema.ValueAtHeadRow
	for rows.Next() {
		var vr schema.ValueAtHeadRow
		var matches pgtype.Bool
		require.NoError(t, rows.Scan(&vr.TraceID, &vr.MostRecentCommitID, &vr.Digest, &vr.OptionsID,
			&vr.GroupingID, &vr.Corpus, &vr.Keys, &vr.Label, &vr.ExpectationRecordID, &matches))
		vr.MatchesAnyIgnoreRule = toNullableBool(matches)
		rv = append(rv, vr)
	}
	return rv
}

func getAllExpectations(ctx context.Context, t *testing.T, db *pgxpool.Pool) []schema.ExpectationRow {
	rows, err := db.Query(ctx, `SELECT * FROM Expectations ORDER BY digest, grouping_id ASC`)
	require.NoError(t, err)
	defer rows.Close()
	var rv []schema.ExpectationRow
	for rows.Next() {
		var er schema.ExpectationRow
		require.NoError(t, rows.Scan(&er.GroupingID, &er.Digest, &er.Label, &er.ExpectationRecordID))
		rv = append(rv, er)
	}
	return rv
}

func getAllPrimaryBranchParams(ctx context.Context, t *testing.T, db *pgxpool.Pool) []schema.PrimaryBranchParamRow {
	rows, err := db.Query(ctx, `SELECT * FROM PrimaryBranchParams ORDER BY tile_id, key ASC`)
	require.NoError(t, err)
	defer rows.Close()
	var rv []schema.PrimaryBranchParamRow
	for rows.Next() {
		var pr schema.PrimaryBranchParamRow
		require.NoError(t, rows.Scan(&pr.TileID, &pr.Key, &pr.Value))
		rv = append(rv, pr)
	}
	return rv
}

func getAllTiledTraceDigests(ctx context.Context, t *testing.T, db *pgxpool.Pool) []schema.TiledTraceDigestRow {
	rows, err := db.Query(ctx, `SELECT * FROM TiledTraceDigests ORDER BY tile_id, digest`)
	require.NoError(t, err)
	defer rows.Close()
	var rv []schema.TiledTraceDigestRow
	for rows.Next() {
		var tr schema.TiledTraceDigestRow
		require.NoError(t, rows.Scan(&tr.TraceID, &tr.TileID, &tr.Digest))
		rv = append(rv, tr)
	}
	return rv
}
