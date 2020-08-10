package main

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/testutils/data_kitchen_sink"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

func TestImportantQueries(t *testing.T) {
	unittest.LargeTest(t)

	dbURL, deleteDB := sqltest.MakeLocalCockroachDBForTesting(t)
	defer deleteDB() // uncomment this to explore the results more easily

	// Note that none of these queries mutate the data, so we should be safe to write the data once
	// and then run all of our read-only subtests.
	db := loadSchemaAndData(t, dbURL)

	t.Run("ListTracesThatMatchKeys_Success", func(t *testing.T) {
		subTest_ListTracesThatMatchKeys_Success(t, db)
	})
	t.Run("ListDataForTrace_Success", func(t *testing.T) {
		subTest_ListDataForTrace_Success(t, db)
	})
	t.Run("ListNegativeExpectations_Success", func(t *testing.T) {
		subTest_ListNegativeExpectations_Success(t, db)
	})
	t.Run("ListUntriagedExpectations_Success", func(t *testing.T) {
		subTest_ListUntriagedExpectations_Success(t, db)
	})
	t.Run("ListIgnoredTraces_Success", func(t *testing.T) {
		subTest_ListIgnoredTraces_Success(t, db)
	})
	t.Run("SearchUntriagedDigestsAndTraces_Success", func(t *testing.T) {
		subTest_SearchUntriagedDigestsAndTraces_Success(t, db)
	})
	t.Run("SearchNegativedDigestsAndTraces_Success", func(t *testing.T) {
		subTest_SearchNegativeDigestsAndTraces_Success(t, db)
	})
	t.Run("FindDenseTile_Success", func(t *testing.T) {
		subTest_FindDenseTile_Success(t, db)
	})
	t.Run("CreateEntireParamsets_Success", func(t *testing.T) {
		subTest_CreateEntireParamset_Success(t, db)
	})
	t.Run("CreateLatestParamsets_Success", func(t *testing.T) {
		subTest_CreateLatestParamset_Success(t, db)
	})
	t.Run("FindTracesWithNameBeingOneOfMultipleValues_Success", func(t *testing.T) {
		subTest_FindTracesWithNameBeingOneOfMultipleValues_Success(t, db)
	})
	t.Run("SearchUntriagedDigestsAndTracesAtHEAD_Success", func(t *testing.T) {
		subTest_SearchUntriagedDigestsAndTracesAtHEAD_Success(t, db)
	})
}

func subTest_ListTracesThatMatchKeys_Success(t *testing.T, db *pgx.Conn) {
	const statement = `
SELECT encode(trace_id, 'hex'), keys FROM Traces 
WHERE keys @> '{"color mode": "GREY", "name": "triangle"}' ORDER BY 1;`
	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	var traceIDs []string
	var jsonKeys []string
	for rows.Next() {
		id := ""
		keys := ""
		err := rows.Scan(&id, &keys)
		require.NoError(t, err)
		traceIDs = append(traceIDs, id)
		jsonKeys = append(jsonKeys, keys)
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, []string{
		"47109b059f45e4f9d5ab61dd0199e2c9",
		"9a42e1337f848e4dbfa9688dda60fe7b",
		"b9c96f249f2551a5d33f264afdb23a46",
		"c0f2834eb3408acdc799dc5190e3533e",
		"c5b4010e73321614f9049ad1985324c2",
	}, traceIDs)
	assert.Equal(t, []string{
		`{"color mode": "GREY", "device": "iPad6,3", "name": "triangle", "os": "iOS", "source_type": "corners"}`,
		`{"color mode": "GREY", "device": "walleye", "name": "triangle", "os": "Android", "source_type": "corners"}`,
		`{"color mode": "GREY", "device": "QuadroP400", "name": "triangle", "os": "Windows10.3", "source_type": "corners"}`,
		`{"color mode": "GREY", "device": "QuadroP400", "name": "triangle", "os": "Windows10.2", "source_type": "corners"}`,
		`{"color mode": "GREY", "device": "iPhone12,1", "name": "triangle", "os": "iOS", "source_type": "corners"}`,
	}, jsonKeys)
}

func subTest_ListDataForTrace_Success(t *testing.T, db *pgx.Conn) {
	// This trace is iPad + Triangle + Grey
	const statement = `
SELECT commit_id, encode(digest, 'hex') FROM TraceValues
WHERE trace_id = x'47109b059f45e4f9d5ab61dd0199e2c9';`
	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	digests := make([]types.Digest, data_kitchen_sink.NumCommits)
	for rows.Next() {
		commitNum := 0
		digest := types.Digest("")
		err := rows.Scan(&commitNum, &digest)
		require.NoError(t, err)
		// subtract 1 to account for the fact that commit_id starts at 1
		digests[commitNum-1] = digest
	}
	expected := getTraceByID(",color mode=GREY,device=iPad6_3,name=triangle,os=iOS,source_type=corners,")
	assert.Equal(t, expected.Trace.Digests, digests)
}

func subTest_ListNegativeExpectations_Success(t *testing.T, db *pgx.Conn) {
	const statement = `
SELECT encode(digest, 'hex'), Groupings.keys FROM 
  (SELECT digest, grouping_id FROM Expectations 
   WHERE label = 2) AS Expectations -- 2 means negative
JOIN
  Groupings
ON Expectations.grouping_id = Groupings.grouping_id;`
	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	var digestsAndGrouping []digestKeysRow
	for rows.Next() {
		r := digestKeysRow{}
		err := rows.Scan(&r.Digest, &r.KeysJSON)
		require.NoError(t, err)
		digestsAndGrouping = append(digestsAndGrouping, r)
	}
	assert.ElementsMatch(t, []digestKeysRow{
		{
			Digest:   data_kitchen_sink.DigestBlank,
			KeysJSON: `{"name": "circle", "source_type": "round"}`,
		},
		{
			Digest:   data_kitchen_sink.DigestA09Neg,
			KeysJSON: `{"name": "square", "source_type": "corners"}`,
		},
		{
			Digest:   data_kitchen_sink.DigestB03Neg,
			KeysJSON: `{"name": "triangle", "source_type": "corners"}`,
		},
		{
			Digest:   data_kitchen_sink.DigestB04Neg,
			KeysJSON: `{"name": "triangle", "source_type": "corners"}`,
		},
	}, digestsAndGrouping)
}

func subTest_ListUntriagedExpectations_Success(t *testing.T, db *pgx.Conn) {
	const statement = `
SELECT encode(digest, 'hex'), Groupings.keys FROM 
  (SELECT digest, grouping_id FROM Expectations 
-- 0 means untriaged. When ingesting data, we are sure to write a 0 to this table if there
-- is not already an entry. That way, we can index on label and not have to deal with label
-- being 0 or NULL (leading to more compact and efficient queries).
   WHERE label = 0) AS Expectations 
JOIN
  Groupings
ON Expectations.grouping_id = Groupings.grouping_id;`
	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	var digestsAndGrouping []digestKeysRow
	for rows.Next() {
		r := digestKeysRow{}
		err := rows.Scan(&r.Digest, &r.KeysJSON)
		require.NoError(t, err)
		digestsAndGrouping = append(digestsAndGrouping, r)
	}
	assert.ElementsMatch(t, []digestKeysRow{
		{
			Digest:   data_kitchen_sink.DigestBlank,
			KeysJSON: `{"name": "triangle", "source_type": "corners"}`,
		},
		{
			Digest:   data_kitchen_sink.DigestA04Unt,
			KeysJSON: `{"name": "square", "source_type": "corners"}`,
		},
		{
			Digest:   data_kitchen_sink.DigestA05Unt,
			KeysJSON: `{"name": "square", "source_type": "corners"}`,
		},
		{
			Digest:   data_kitchen_sink.DigestA06Unt,
			KeysJSON: `{"name": "square", "source_type": "corners"}`,
		},
		{
			Digest:   data_kitchen_sink.DigestC03Unt,
			KeysJSON: `{"name": "circle", "source_type": "round"}`,
		},
		{
			Digest:   data_kitchen_sink.DigestC04Unt,
			KeysJSON: `{"name": "circle", "source_type": "round"}`,
		},
		{
			Digest:   data_kitchen_sink.DigestC05Unt,
			KeysJSON: `{"name": "circle", "source_type": "round"}`,
		},
	}, digestsAndGrouping)
}

func subTest_ListIgnoredTraces_Success(t *testing.T, db *pgx.Conn) {
	const statement = `SELECT Traces.keys FROM Traces WHERE Traces.matches_any_ignore_rule = true;`
	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	var jsonKeys []string
	for rows.Next() {
		keys := ""
		err := rows.Scan(&keys)
		require.NoError(t, err)
		jsonKeys = append(jsonKeys, keys)
	}

	assert.ElementsMatch(t, []string{
		`{"color mode": "RGB", "device": "taimen", "name": "square", "os": "Android", "source_type": "corners"}`,
		`{"color mode": "RGB", "device": "taimen", "name": "circle", "os": "Android", "source_type": "round"}`,
	}, jsonKeys)

	const countStatement = `SELECT count(Traces.trace_id) WHERE Traces.matches_any_ignore_rule IS NULL;`
	row := db.QueryRow(context.Background(), countStatement)
	count := 0
	row.Scan(&row)
	assert.Equal(t, 0, count, "All traces should be marked as ignored or not")
}

// This test searches for traces matching source_type:round and color mode: RGB that do not match
// any ignore rules and contain one or more untriaged digest. The query returns the traces and
// the untriaged digests; it is the first part to a general search query.
func subTest_SearchUntriagedDigestsAndTraces_Success(t *testing.T, db *pgx.Conn) {
	const statement = `
SELECT DISTINCT encode(TraceValues.digest, 'hex') AS digest, Traces.keys FROM
  (SELECT trace_id, keys FROM Traces 
   WHERE Traces.keys @> '{"source_type": "round", "color mode": "RGB"}' 
     AND Traces.matches_any_ignore_rule = false) AS Traces
JOIN
  (SELECT trace_id, digest, grouping_id FROM TraceValues 
   WHERE commit_id >= 1) AS TraceValues -- This range is just to show it possible
ON Traces.trace_id = TraceValues.trace_id
JOIN
  (SELECT grouping_id, digest FROM Expectations 
   WHERE label = 0) AS Expectations -- 0 means untriaged
ON TraceValues.grouping_id = Expectations.grouping_id
  AND TraceValues.digest = Expectations.digest;`
	// Of note, could add in the following clause when we support ranges
	// AND TraceValues.commit_id >= Expectations.start_index
	// AND TraceValues.commit_id < Expectations.end_index

	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	var digestsAndTraces []digestKeysRow
	for rows.Next() {
		r := digestKeysRow{}
		err := rows.Scan(&r.Digest, &r.KeysJSON)
		require.NoError(t, err)
		digestsAndTraces = append(digestsAndTraces, r)
	}

	assert.ElementsMatch(t, []digestKeysRow{
		{
			Digest:   data_kitchen_sink.DigestC03Unt,
			KeysJSON: `{"color mode": "RGB", "device": "QuadroP400", "name": "circle", "os": "Windows10.3", "source_type": "round"}`,
		},
		{
			Digest:   data_kitchen_sink.DigestC05Unt,
			KeysJSON: `{"color mode": "RGB", "device": "iPad6,3", "name": "circle", "os": "iOS", "source_type": "round"}`,
		},
		{
			Digest:   data_kitchen_sink.DigestC05Unt,
			KeysJSON: `{"color mode": "RGB", "device": "iPhone12,1", "name": "circle", "os": "iOS", "source_type": "round"}`,
		},
	}, digestsAndTraces)
}

func subTest_SearchNegativeDigestsAndTraces_Success(t *testing.T, db *pgx.Conn) {
	const statement = `
SELECT DISTINCT encode(TraceValues.digest, 'hex') AS digest, Traces.keys FROM
  (SELECT trace_id, keys FROM Traces 
   WHERE Traces.matches_any_ignore_rule = false) AS Traces
JOIN
  (SELECT trace_id, digest, grouping_id FROM TraceValues 
   WHERE commit_id >= 0) AS TraceValues -- This range is just to show it possible
ON Traces.trace_id = TraceValues.trace_id
JOIN
  (SELECT grouping_id, digest FROM Expectations 
   WHERE label = 2) AS Expectations -- 2 means negative
ON TraceValues.grouping_id = Expectations.grouping_id
  AND TraceValues.digest = Expectations.digest;`

	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	var digestsAndTraces []digestKeysRow
	for rows.Next() {
		r := digestKeysRow{}
		err := rows.Scan(&r.Digest, &r.KeysJSON)
		require.NoError(t, err)
		digestsAndTraces = append(digestsAndTraces, r)
	}

	assert.ElementsMatch(t, []digestKeysRow{
		{
			Digest:   data_kitchen_sink.DigestB03Neg,
			KeysJSON: `{"color mode": "RGB", "device": "iPhone12,1", "name": "triangle", "os": "iOS", "source_type": "corners"}`,
		},
		{
			Digest:   data_kitchen_sink.DigestB03Neg,
			KeysJSON: `{"color mode": "RGB", "device": "iPad6,3", "name": "triangle", "os": "iOS", "source_type": "corners"}`,
		},
		{
			Digest:   data_kitchen_sink.DigestB04Neg,
			KeysJSON: `{"color mode": "GREY", "device": "iPhone12,1", "name": "triangle", "os": "iOS", "source_type": "corners"}`,
		},
		{
			Digest:   data_kitchen_sink.DigestB04Neg,
			KeysJSON: `{"color mode": "GREY", "device": "iPad6,3", "name": "triangle", "os": "iOS", "source_type": "corners"}`,
		},
	}, digestsAndTraces)
}

// This gets the last 512 commit numbers where we have data. (i.e. get our Dense tile).
// TODO(kjlubick) actually make the data sparse.
func subTest_FindDenseTile_Success(t *testing.T, db *pgx.Conn) {
	const statement = ` 
SELECT commit_id FROM Commits
  WHERE has_data = true
  ORDER BY commit_id DESC LIMIT 512;`
	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	var commits []int
	for rows.Next() {
		c := 0
		err := rows.Scan(&c)
		require.NoError(t, err)
		commits = append(commits, c)
	}

	assert.Equal(t, []int{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}, commits)
}

func subTest_CreateEntireParamset_Success(t *testing.T, db *pgx.Conn) {
	const keysStatement = `
SELECT DISTINCT key, value FROM PrimaryBranchParams
WHERE commit_id > 0;`

	rows, err := db.Query(context.Background(), keysStatement)
	require.NoError(t, err)
	defer rows.Close()

	ps := paramtools.ParamSet{}
	for rows.Next() {
		key := ""
		value := ""
		err := rows.Scan(&key, &value)
		require.NoError(t, err)
		values := ps[key]
		if !util.In(value, values) {
			ps[key] = append(values, value)
		}
	}

	ps.Normalize()
	assert.Equal(t, data_kitchen_sink.MakeParamSet(), ps)
}

func subTest_CreateLatestParamset_Success(t *testing.T, db *pgx.Conn) {
	// The Windows 10.2 should not be in os, since it was phased out on commit 4.
	const keysStatement = `
SELECT DISTINCT key, value FROM PrimaryBranchParams
WHERE commit_id > 5;`

	rows, err := db.Query(context.Background(), keysStatement)
	require.NoError(t, err)
	defer rows.Close()

	ps := paramtools.ParamSet{}
	for rows.Next() {
		key := ""
		value := ""
		err := rows.Scan(&key, &value)
		require.NoError(t, err)
		values := ps[key]
		if !util.In(value, values) {
			ps[key] = append(values, value)
		}
	}

	ps.Normalize()

	// ParamSet should be exactly the full PS, except with the OSes trimmed down to these 3.
	expectedPS := data_kitchen_sink.MakeParamSet()
	expectedPS[data_kitchen_sink.OSKey] = []string{
		data_kitchen_sink.AndroidOS, data_kitchen_sink.Windows10dot3OS, data_kitchen_sink.ApplePhoneOS,
	}
	assert.Equal(t, expectedPS, ps)
}

func subTest_FindTracesWithNameBeingOneOfMultipleValues_Success(t *testing.T, db *pgx.Conn) {
	const statement = `
SELECT DISTINCT Traces.keys FROM 
  (SELECT trace_id, keys FROM Traces
   WHERE keys -> 'name' IN ('"triangle"'::JSONB, '"square"'::JSONB)
-- Note: The following condition could be written with an IN clause, but it appears the cost
-- optimizer does not know that's the same as an equality and the query plan is different. This
-- Way appears more efficient, as it performs the above filter on a smaller set of data.
     AND keys -> 'device' = '"iPad6,3"'::JSONB
     AND keys -> 'source_type' = '"corners"'::JSONB) AS Traces
JOIN
  TraceValues
ON Traces.trace_id = TraceValues.trace_id;`

	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	var keysJSON []string
	for rows.Next() {
		keys := ""
		err := rows.Scan(&keys)
		require.NoError(t, err)
		keysJSON = append(keysJSON, keys)
	}
	assert.ElementsMatch(t, []string{
		`{"color mode": "GREY", "device": "iPad6,3", "name": "triangle", "os": "iOS", "source_type": "corners"}`,
		`{"color mode": "GREY", "device": "iPad6,3", "name": "square", "os": "iOS", "source_type": "corners"}`,
		`{"color mode": "RGB", "device": "iPad6,3", "name": "triangle", "os": "iOS", "source_type": "corners"}`,
		`{"color mode": "RGB", "device": "iPad6,3", "name": "square", "os": "iOS", "source_type": "corners"}`,
	}, keysJSON)
}

func subTest_SearchUntriagedDigestsAndTracesAtHEAD_Success(t *testing.T, db *pgx.Conn) {
	// To get data at TOT, we use a self join on TraceValues using an aggregator
	const statement = `
SELECT encode(TraceValues.digest, 'hex'), Traces.keys FROM
  (SELECT max(commit_id) as commit_id, trace_id from TraceValues
  WHERE commit_id > 0
  GROUP BY trace_id) as HEAD
INNER LOOKUP JOIN
  TraceValues
ON TraceValues.trace_id = HEAD.trace_id AND TraceValues.commit_id = HEAD.commit_id
JOIN
  (SELECT grouping_id, digest FROM Expectations 
     WHERE label = 0) AS Expectations
  ON TraceValues.grouping_id = Expectations.grouping_id AND TraceValues.digest = Expectations.digest
JOIN
  (SELECT trace_id, keys FROM Traces
   WHERE matches_any_ignore_rule = false) AS Traces
ON TraceValues.trace_id = Traces.trace_id;`
	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	var digestsAndTraces []digestKeysRow
	for rows.Next() {
		r := digestKeysRow{}
		err := rows.Scan(&r.Digest, &r.KeysJSON)
		require.NoError(t, err)
		digestsAndTraces = append(digestsAndTraces, r)
	}

	assert.ElementsMatch(t, []digestKeysRow{
		{
			Digest:   data_kitchen_sink.DigestC03Unt,
			KeysJSON: `{"color mode": "RGB", "device": "QuadroP400", "name": "circle", "os": "Windows10.3", "source_type": "round"}`,
		},
		{
			Digest:   data_kitchen_sink.DigestC04Unt,
			KeysJSON: `{"color mode": "GREY", "device": "QuadroP400", "name": "circle", "os": "Windows10.3", "source_type": "round"}`,
		},
		{
			Digest:   data_kitchen_sink.DigestC05Unt,
			KeysJSON: `{"color mode": "RGB", "device": "iPad6,3", "name": "circle", "os": "iOS", "source_type": "round"}`,
		},
		{
			Digest:   data_kitchen_sink.DigestC05Unt,
			KeysJSON: `{"color mode": "GREY", "device": "iPad6,3", "name": "circle", "os": "iOS", "source_type": "round"}`,
		},
		{
			Digest:   data_kitchen_sink.DigestC05Unt,
			KeysJSON: `{"color mode": "RGB", "device": "iPhone12,1", "name": "circle", "os": "iOS", "source_type": "round"}`,
		},
		{
			Digest:   data_kitchen_sink.DigestC05Unt,
			KeysJSON: `{"color mode": "GREY", "device": "iPhone12,1", "name": "circle", "os": "iOS", "source_type": "round"}`,
		},
	}, digestsAndTraces)
}

func loadSchemaAndData(t *testing.T, url string) *pgx.Conn {
	ctx := context.Background()
	conf, err := pgx.ParseConfig(url)
	require.NoError(t, err)
	db, err := pgx.ConnectConfig(ctx, conf)
	require.NoError(t, err)

	_, err = db.Exec(ctx, sql.CockroachDBSchema)
	require.NoError(t, err)

	require.NoError(t, writeCommits(ctx, db))
	require.NoError(t, writePrimaryBranchTraceData(ctx, db))
	require.NoError(t, writeCLData(ctx, db))
	require.NoError(t, writePrimaryBranchExpectations(ctx, db))
	require.NoError(t, writeDiffMetrics(ctx, db))
	require.NoError(t, writeIgnoreRules(ctx, db))
	return db
}

func getTraceByID(id tiling.TraceID) tiling.TracePair {
	traces := data_kitchen_sink.MakeTraces()
	for _, tp := range traces {
		if tp.ID == id {
			return tp
		}
	}
	panic("Invalid id " + id)
}

// digestKeysRow is a helper type for several queries
type digestKeysRow struct {
	Digest   types.Digest
	KeysJSON string
}
