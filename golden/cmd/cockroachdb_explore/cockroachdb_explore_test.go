package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/testutils/data_kitchen_sink"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

// This tests some SQL queries that are representative of the queries that will be executed on the
// data in prod. This serves as a POC for the various systems that will be querying the data.
// Some indexes have been explicitly called out in the queries (e.g. Traces@head_idx); this is
// mostly for documentation purposes (e.g. "Why do we have this index?")
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
		subTest_SearchUntriagedUnignoredDigestsAndTracesAtHEAD_Success(t, db)
	})
	t.Run("ListHistoryFromTraces_Success", func(t *testing.T) {
		subTest_ListHistoryFromTraces_Success(t, db)
	})
	t.Run("FindClosestDigests_Success", func(t *testing.T) {
		subTest_FindClosestDigests_Success(t, db)
	})
	t.Run("FindClosestDigestsRestrictRightSide_Success", func(t *testing.T) {
		subTest_FindClosestDigestsRestrictRightSide_Success(t, db)
	})
	t.Run("SummarizeAllDigestsByTest_Success", func(t *testing.T) {
		subTest_SummarizeAllDigestsByTest_Success(t, db)
	})
	t.Run("SummarizeDigestsAtHeadByTest_Success", func(t *testing.T) {
		subTest_SummarizeDigestsAtHeadByTest_Success(t, db)
	})
	t.Run("SummarizeNonIgnoredDigestsByTest_Success", func(t *testing.T) {
		subTest_SummarizeNonIgnoredDigestsByTest_Success(t, db)
	})
	t.Run("SummarizeNonIgnoredDigestsAtHeadByTest_Success", func(t *testing.T) {
		subTest_SummarizeNonIgnoredDigestsAtHeadByTest_Success(t, db)
	})

	t.Run("TryjobDataIsSeparateFromPrimaryBranch_Success", func(t *testing.T) {
		subTest_TryjobDataIsSeparateFromPrimaryBranch_Success(t, db)
	})
	t.Run("SearchUntriagedUnignoredDigestsAndTracesForCL_Success", func(t *testing.T) {
		subTest_SearchUntriagedUnignoredDigestsAndTracesForCL_Success(t, db)
	})
	t.Run("CreateParamsetsForCL_Success", func(t *testing.T) {
		subTest_CreateParamsetsForCL_Success(t, db)
	})
	t.Run("ListUntriagedChangelistExpectations_Success", func(t *testing.T) {
		subTest_ListUntriagedChangelistExpectations_Success(t, db)
	})
	t.Run("ListPositiveChangelistExpectations_Success", func(t *testing.T) {
		subTest_ListPositiveChangelistExpectations_Success(t, db)
	})

}

func subTest_ListTracesThatMatchKeys_Success(t *testing.T, db *pgx.Conn) {
	const statement = `
SELECT encode(trace_id, 'hex'), keys FROM Traces 
WHERE keys @> '{"color mode": "GREY", "name": "triangle"}' ORDER BY 1;`
	assertNoFullTableScans(t, db, statement)
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
	assertNoFullTableScans(t, db, statement)
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
	assertNoFullTableScans(t, db, statement)
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
		{Digest: data_kitchen_sink.DigestBlank, KeysJSON: circleGrouping},
		{Digest: data_kitchen_sink.DigestA09Neg, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestB03Neg, KeysJSON: triangleGrouping},
		{Digest: data_kitchen_sink.DigestB04Neg, KeysJSON: triangleGrouping},
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
	assertNoFullTableScans(t, db, statement)
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
		{Digest: data_kitchen_sink.DigestBlank, KeysJSON: triangleGrouping},
		{Digest: data_kitchen_sink.DigestA04Unt, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestA05Unt, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestA06Unt, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestC03Unt, KeysJSON: circleGrouping},
		{Digest: data_kitchen_sink.DigestC04Unt, KeysJSON: circleGrouping},
		{Digest: data_kitchen_sink.DigestC05Unt, KeysJSON: circleGrouping},
	}, digestsAndGrouping)
}

func subTest_ListIgnoredTraces_Success(t *testing.T, db *pgx.Conn) {
	const statement = `SELECT keys FROM Traces@ignored_idx WHERE matches_any_ignore_rule;`
	assertNoFullTableScans(t, db, statement)
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
  (SELECT trace_id, digest, grouping_id FROM TraceValues@trace_and_commit_idx 
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
	assertNoFullTableScans(t, db, statement)
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
	assertNoFullTableScans(t, db, statement)
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
SELECT commit_id FROM Commits@has_data_idx
  WHERE has_data = true
  ORDER BY commit_id DESC LIMIT 512;`
	assertNoFullTableScans(t, db, statement)
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
	const statement = `
SELECT DISTINCT key, value FROM PrimaryBranchParams
WHERE commit_id > 0;`
	assertNoFullTableScans(t, db, statement)
	rows, err := db.Query(context.Background(), statement)
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
	const statement = `
SELECT DISTINCT key, value FROM PrimaryBranchParams
WHERE commit_id > 5;`
	assertNoFullTableScans(t, db, statement)
	rows, err := db.Query(context.Background(), statement)
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
	assertNoFullTableScans(t, db, statement)
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

func subTest_SearchUntriagedUnignoredDigestsAndTracesAtHEAD_Success(t *testing.T, db *pgx.Conn) {
	// This join starts with Expectations because the size of the Expectations table is much much
	// smaller than TraceValues or Traces.
	const statement = `
SELECT encode(TraceValues.digest, 'hex'), Traces.keys FROM
  (SELECT grouping_id, digest FROM Expectations@label_idx
   WHERE label = 0) AS Expectations
JOIN
  (SELECT digest, grouping_id, trace_id, commit_id FROM TraceValues@expectations_idx
   WHERE commit_id > 0) AS TraceValues
ON TraceValues.grouping_id = Expectations.grouping_id AND TraceValues.digest = Expectations.digest
JOIN
  (SELECT trace_id, keys, most_recent_commit_id FROM Traces@head_idx
   WHERE matches_any_ignore_rule = false AND most_recent_commit_id > 0) AS Traces
ON TraceValues.trace_id = Traces.trace_id AND TraceValues.commit_id = Traces.most_recent_commit_id;`
	assertNoFullTableScans(t, db, statement)
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

func subTest_ListHistoryFromTraces_Success(t *testing.T, db *pgx.Conn) {
	// Select data from 2 traces
	const statement = `
SELECT encode(trace_id, 'hex'), encode(digest, 'hex'), commit_id FROM TraceValues WHERE trace_id
IN (x'796f2cc3f33fa6a9a1f4bef3aa9c48c4', x'3b44c31afc832ef9d1a2d25a5b873152')
AND commit_id >= 0;`
	assertNoFullTableScans(t, db, statement)
	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	const greyIpadCircle = "3b44c31afc832ef9d1a2d25a5b873152"
	const greyIpadSquare = "796f2cc3f33fa6a9a1f4bef3aa9c48c4"

	traces := map[string][]types.Digest{
		greyIpadCircle: make([]types.Digest, data_kitchen_sink.NumCommits),
		greyIpadSquare: make([]types.Digest, data_kitchen_sink.NumCommits),
	}

	for rows.Next() {
		traceID := ""
		digest := types.Digest("")
		commitID := 0
		err := rows.Scan(&traceID, &digest, &commitID)
		require.NoError(t, err)
		// subtract 1 from commitID because it is 1 indexed
		traces[traceID][commitID-1] = digest
	}

	giCircle := getTraceByID(",color mode=GREY,device=iPad6_3,name=circle,os=iOS,source_type=round,")
	assert.Equal(t, giCircle.Trace.Digests, traces[greyIpadCircle])

	giSquare := getTraceByID(",color mode=GREY,device=iPad6_3,name=square,os=iOS,source_type=corners,")
	assert.Equal(t, giSquare.Trace.Digests, traces[greyIpadSquare])
}

func subTest_FindClosestDigests_Success(t *testing.T, db *pgx.Conn) {
	// Get closest digests (positive and negative) to digest 000... and b02... searching in the
	// entire grouping with hash aa8d3... (this is {"name": "triangle", "source_type": "corners"})
	const statement = `
SELECT DISTINCT label, encode(left_digest, 'hex') as left, encode(right_digest, 'hex') as right,
  num_diff_pixels, max_rgba_diff, dimensions_differ
FROM
  (SELECT digest FROM TraceValues 
   WHERE TraceValues.commit_id > 0 
     AND TraceValues.grouping_id = x'aa8d3c14238a4f717b9a99f7fe3735a7') AS TraceValues
JOIN
  (SELECT * FROM DiffMetrics
   WHERE DiffMetrics.left_digest IN (x'00000000000000000000000000000000', x'b02b02b02b02b02b02b02b02b02b02b0')
     AND max_channel_diff >= 0 AND max_channel_diff <= 255) AS DiffMetrics
ON DiffMetrics.right_digest = TraceValues.digest
JOIN
  (SELECT digest, label FROM Expectations
   WHERE label > 0 
   AND Expectations.grouping_id = x'aa8d3c14238a4f717b9a99f7fe3735a7') AS Expectations
ON Expectations.digest = DiffMetrics.right_digest
ORDER BY 2, label, num_diff_pixels, 3;`
	assertNoFullTableScans(t, db, statement)
	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	type diffCategory struct {
		Left  types.Digest
		Label sql.ExpectationsLabel
	}

	closestResults := map[diffCategory]diffRow{}

	for rows.Next() {
		row := diffRow{}
		diffs := [4]int{}
		diffSlice := diffs[:]
		err := rows.Scan(&row.Label, &row.Left, &row.Right, &row.NumDiffPixels, &diffSlice, &row.DimDiffer)
		require.NoError(t, err)
		copy(row.MaxRGBADiffs[:], diffSlice)

		// Store the first row we find for a given category
		cat := diffCategory{Label: row.Label, Left: row.Left}
		if _, ok := closestResults[cat]; !ok {
			closestResults[cat] = row
		}
	}

	closestPositiveTo000 := diffCategory{Label: sql.LabelPositive, Left: data_kitchen_sink.DigestBlank}
	closestNegativeTo000 := diffCategory{Label: sql.LabelNegative, Left: data_kitchen_sink.DigestBlank}
	closestPositiveToB02 := diffCategory{Label: sql.LabelPositive, Left: data_kitchen_sink.DigestB02Pos}
	closestNegativeToB02 := diffCategory{Label: sql.LabelNegative, Left: data_kitchen_sink.DigestB02Pos}

	metrics000vsB01 := getDiffMetricsFor(data_kitchen_sink.DigestBlank, data_kitchen_sink.DigestB01Pos)
	metrics000vsB04 := getDiffMetricsFor(data_kitchen_sink.DigestBlank, data_kitchen_sink.DigestB04Neg)
	metricsB02vsB01 := getDiffMetricsFor(data_kitchen_sink.DigestB02Pos, data_kitchen_sink.DigestB01Pos)
	metricsB02vsB03 := getDiffMetricsFor(data_kitchen_sink.DigestB02Pos, data_kitchen_sink.DigestB03Neg)

	assert.Equal(t, map[diffCategory]diffRow{
		closestPositiveTo000: {
			Label:         sql.LabelPositive,
			Left:          data_kitchen_sink.DigestBlank,
			Right:         data_kitchen_sink.DigestB01Pos,
			NumDiffPixels: metrics000vsB01.NumDiffPixels,
			MaxRGBADiffs:  metrics000vsB01.MaxRGBADiffs,
			DimDiffer:     metrics000vsB01.DimDiffer,
		},
		closestNegativeTo000: {
			Label:         sql.LabelNegative,
			Left:          data_kitchen_sink.DigestBlank,
			Right:         data_kitchen_sink.DigestB04Neg,
			NumDiffPixels: metrics000vsB04.NumDiffPixels,
			MaxRGBADiffs:  metrics000vsB04.MaxRGBADiffs,
			DimDiffer:     metrics000vsB04.DimDiffer,
		},
		closestPositiveToB02: {
			Label:         sql.LabelPositive,
			Left:          data_kitchen_sink.DigestB02Pos,
			Right:         data_kitchen_sink.DigestB01Pos,
			NumDiffPixels: metricsB02vsB01.NumDiffPixels,
			MaxRGBADiffs:  metricsB02vsB01.MaxRGBADiffs,
			DimDiffer:     metricsB02vsB01.DimDiffer,
		},
		closestNegativeToB02: {
			Label:         sql.LabelNegative,
			Left:          data_kitchen_sink.DigestB02Pos,
			Right:         data_kitchen_sink.DigestB03Neg,
			NumDiffPixels: metricsB02vsB03.NumDiffPixels,
			MaxRGBADiffs:  metricsB02vsB03.MaxRGBADiffs,
			DimDiffer:     metricsB02vsB03.DimDiffer,
		},
	}, closestResults)
}

func subTest_FindClosestDigestsRestrictRightSide_Success(t *testing.T, db *pgx.Conn) {
	// This is similar to the previous test except it restricts the right side to be
	// "color mode": "GREY"
	const statement = `
SELECT DISTINCT label, encode(left_digest, 'hex') as left, encode(right_digest, 'hex') as right,
  num_diff_pixels, max_rgba_diff, dimensions_differ
FROM
  (SELECT digest, trace_id FROM TraceValues 
   WHERE TraceValues.commit_id > 0 
     AND TraceValues.grouping_id = x'aa8d3c14238a4f717b9a99f7fe3735a7') AS TraceValues
JOIN
  (SELECT trace_id FROM Traces
   WHERE keys @> '{"name": "triangle", "source_type": "corners", "color mode": "GREY"}') AS Traces
ON TraceValues.trace_id = Traces.trace_id
JOIN
  (SELECT * FROM DiffMetrics
   WHERE DiffMetrics.left_digest IN (x'00000000000000000000000000000000', x'b02b02b02b02b02b02b02b02b02b02b0')
     AND max_channel_diff >= 0 AND max_channel_diff <= 255) AS DiffMetrics
ON DiffMetrics.right_digest = TraceValues.digest
JOIN
  (SELECT digest, label FROM Expectations
   WHERE label > 0 
   AND Expectations.grouping_id = x'aa8d3c14238a4f717b9a99f7fe3735a7') AS Expectations
ON Expectations.digest = DiffMetrics.right_digest
ORDER BY 2, label, num_diff_pixels, 3;`
	assertNoFullTableScans(t, db, statement)
	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	type diffCategory struct {
		Left  types.Digest
		Label sql.ExpectationsLabel
	}

	closestResults := map[diffCategory]diffRow{}

	for rows.Next() {
		row := diffRow{}
		diffs := [4]int{}
		diffSlice := diffs[:]
		err := rows.Scan(&row.Label, &row.Left, &row.Right, &row.NumDiffPixels, &diffSlice, &row.DimDiffer)
		require.NoError(t, err)
		copy(row.MaxRGBADiffs[:], diffSlice)

		// Store the first row we find for a given category
		cat := diffCategory{Label: row.Label, Left: row.Left}
		if _, ok := closestResults[cat]; !ok {
			closestResults[cat] = row
		}
	}

	closestPositiveTo000 := diffCategory{Label: sql.LabelPositive, Left: data_kitchen_sink.DigestBlank}
	closestNegativeTo000 := diffCategory{Label: sql.LabelNegative, Left: data_kitchen_sink.DigestBlank}
	// In these results there is no closest positive to DigestB01Pos
	closestNegativeToB02 := diffCategory{Label: sql.LabelNegative, Left: data_kitchen_sink.DigestB02Pos}

	metrics000vsB02 := getDiffMetricsFor(data_kitchen_sink.DigestBlank, data_kitchen_sink.DigestB02Pos)
	metrics000vsB04 := getDiffMetricsFor(data_kitchen_sink.DigestBlank, data_kitchen_sink.DigestB04Neg)
	metricsB02vsB04 := getDiffMetricsFor(data_kitchen_sink.DigestB02Pos, data_kitchen_sink.DigestB04Neg)

	assert.Equal(t, map[diffCategory]diffRow{
		closestPositiveTo000: {
			Label:         sql.LabelPositive,
			Left:          data_kitchen_sink.DigestBlank,
			Right:         data_kitchen_sink.DigestB02Pos,
			NumDiffPixels: metrics000vsB02.NumDiffPixels,
			MaxRGBADiffs:  metrics000vsB02.MaxRGBADiffs,
			DimDiffer:     metrics000vsB02.DimDiffer,
		},
		closestNegativeTo000: {
			Label:         sql.LabelNegative,
			Left:          data_kitchen_sink.DigestBlank,
			Right:         data_kitchen_sink.DigestB04Neg,
			NumDiffPixels: metrics000vsB04.NumDiffPixels,
			MaxRGBADiffs:  metrics000vsB04.MaxRGBADiffs,
			DimDiffer:     metrics000vsB04.DimDiffer,
		},
		closestNegativeToB02: {
			Label:         sql.LabelNegative,
			Left:          data_kitchen_sink.DigestB02Pos,
			Right:         data_kitchen_sink.DigestB04Neg,
			NumDiffPixels: metricsB02vsB04.NumDiffPixels,
			MaxRGBADiffs:  metricsB02vsB04.MaxRGBADiffs,
			DimDiffer:     metricsB02vsB04.DimDiffer,
		},
	}, closestResults)
}

type byTestRow struct {
	corpus string
	name   types.TestName
	label  sql.ExpectationsLabel
	count  int
}

func subTest_SummarizeAllDigestsByTest_Success(t *testing.T, db *pgx.Conn) {
	// This join starts with Expectations because the size of the Expectations table is much much
	// smaller than TraceValues or Traces. Even so, we can't avoid a full-table scan of Expectations
	// here (since we *are* summarizing all digests seen based on labels). To combat this potentially
	// slow query in production, just cache the result and serve the data a bit stale.
	const statement = `
SELECT corpus, name, label, count(label) FROM (
  SELECT DISTINCT Traces.keys ->> 'name' AS name, Traces.keys ->> 'source_type' AS corpus,
    encode(TraceValues.digest, 'hex') AS digest, Expectations.label
  FROM
   Expectations
  INNER LOOKUP JOIN
    (SELECT trace_id, grouping_id, digest FROM TraceValues@expectations_idx
     WHERE TraceValues.commit_id > 0) AS TraceValues
  ON Expectations.grouping_id = TraceValues.grouping_id AND Expectations.digest = TraceValues.digest
  INNER LOOKUP JOIN 
    Traces
  ON Traces.trace_id = TraceValues.trace_id
) GROUP BY corpus, name, label;`
	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	var results []byTestRow
	for rows.Next() {
		r := byTestRow{}
		err := rows.Scan(&r.corpus, &r.name, &r.label, &r.count)
		require.NoError(t, err)
		results = append(results, r)
	}

	assert.ElementsMatch(t, []byTestRow{
		{
			corpus: data_kitchen_sink.CornersCorpus,
			name:   data_kitchen_sink.SquareTest,
			label:  sql.LabelUntriaged,
			count:  3,
		},
		{
			corpus: data_kitchen_sink.CornersCorpus,
			name:   data_kitchen_sink.SquareTest,
			label:  sql.LabelPositive,
			count:  5,
		},
		{
			corpus: data_kitchen_sink.CornersCorpus,
			name:   data_kitchen_sink.SquareTest,
			label:  sql.LabelNegative,
			count:  1,
		},
		{
			corpus: data_kitchen_sink.CornersCorpus,
			name:   data_kitchen_sink.TriangleTest,
			label:  sql.LabelUntriaged,
			count:  1,
		},
		{
			corpus: data_kitchen_sink.CornersCorpus,
			name:   data_kitchen_sink.TriangleTest,
			label:  sql.LabelPositive,
			count:  2,
		},
		{
			corpus: data_kitchen_sink.CornersCorpus,
			name:   data_kitchen_sink.TriangleTest,
			label:  sql.LabelNegative,
			count:  2,
		},
		{
			corpus: data_kitchen_sink.RoundCorpus,
			name:   data_kitchen_sink.CircleTest,
			label:  sql.LabelUntriaged,
			count:  3,
		},
		{
			corpus: data_kitchen_sink.RoundCorpus,
			name:   data_kitchen_sink.CircleTest,
			label:  sql.LabelPositive,
			count:  2,
		},
		// No negatives from the circle test
	}, results)
}

func subTest_SummarizeNonIgnoredDigestsByTest_Success(t *testing.T, db *pgx.Conn) {
	// As above, no way to avoid a full table scan on Expectations.
	const statement = `
SELECT corpus, name, label, count(label) FROM (
  SELECT DISTINCT Traces.keys ->> 'name' AS name, Traces.keys ->> 'source_type' AS corpus,
    encode(TraceValues.digest, 'hex') AS digest, Expectations.label
  FROM
   Expectations
  INNER LOOKUP JOIN
    (SELECT trace_id, grouping_id, digest FROM TraceValues@expectations_idx
     WHERE TraceValues.commit_id > 0) AS TraceValues
  ON Expectations.grouping_id = TraceValues.grouping_id AND Expectations.digest = TraceValues.digest
  INNER LOOKUP JOIN 
    (SELECT trace_id, keys FROM Traces WHERE matches_any_ignore_rule = false) AS Traces
  ON Traces.trace_id = TraceValues.trace_id
) GROUP BY corpus, name, label;`
	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	var results []byTestRow
	for rows.Next() {
		r := byTestRow{}
		err := rows.Scan(&r.corpus, &r.name, &r.label, &r.count)
		require.NoError(t, err)
		results = append(results, r)
	}

	assert.ElementsMatch(t, []byTestRow{
		{
			corpus: data_kitchen_sink.CornersCorpus,
			name:   data_kitchen_sink.SquareTest,
			label:  sql.LabelUntriaged,
			count:  3,
		},
		{
			corpus: data_kitchen_sink.CornersCorpus,
			name:   data_kitchen_sink.SquareTest,
			label:  sql.LabelPositive,
			count:  5,
		},
		// DigestA09Neg was only on an ignored trace, so it is missing compared to counting all
		// the digests.
		{
			corpus: data_kitchen_sink.CornersCorpus,
			name:   data_kitchen_sink.TriangleTest,
			label:  sql.LabelUntriaged,
			count:  1,
		},
		{
			corpus: data_kitchen_sink.CornersCorpus,
			name:   data_kitchen_sink.TriangleTest,
			label:  sql.LabelPositive,
			count:  2,
		},
		{
			corpus: data_kitchen_sink.CornersCorpus,
			name:   data_kitchen_sink.TriangleTest,
			label:  sql.LabelNegative,
			count:  2,
		},
		{
			corpus: data_kitchen_sink.RoundCorpus,
			name:   data_kitchen_sink.CircleTest,
			label:  sql.LabelUntriaged,
			count:  3,
		},
		{
			corpus: data_kitchen_sink.RoundCorpus,
			name:   data_kitchen_sink.CircleTest,
			label:  sql.LabelPositive,
			count:  2,
		},
		// No negatives from the circle test
	}, results)
}

func subTest_SummarizeNonIgnoredDigestsAtHeadByTest_Success(t *testing.T, db *pgx.Conn) {
	// Even with indexes, this query will be expensive because it has to go over all traces at head.
	const statement = `
SELECT corpus, name, label, count(label) FROM (
  SELECT DISTINCT Traces.keys ->> 'name' AS name, Traces.keys ->> 'source_type' AS corpus,
    encode(TraceValues.digest, 'hex') AS digest, Expectations.label
  FROM
    (SELECT trace_id, keys, most_recent_commit_id FROM Traces@head_idx
     WHERE matches_any_ignore_rule = false AND most_recent_commit_id > 0) AS Traces
  INNER LOOKUP JOIN 
    TraceValues@trace_and_commit_idx
  ON TraceValues.trace_id = Traces.trace_id AND TraceValues.commit_id = Traces.most_recent_commit_id
  INNER LOOKUP JOIN
    Expectations
  ON Expectations.grouping_id = TraceValues.grouping_id AND Expectations.digest = TraceValues.digest
) GROUP BY corpus, name, label;`
	assertNoFullTableScans(t, db, statement)
	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	var results []byTestRow
	for rows.Next() {
		r := byTestRow{}
		err := rows.Scan(&r.corpus, &r.name, &r.label, &r.count)
		require.NoError(t, err)
		results = append(results, r)
	}

	assert.ElementsMatch(t, []byTestRow{
		{
			corpus: data_kitchen_sink.CornersCorpus,
			name:   data_kitchen_sink.SquareTest,
			label:  sql.LabelPositive,
			count:  4,
		},
		{
			corpus: data_kitchen_sink.CornersCorpus,
			name:   data_kitchen_sink.TriangleTest,
			label:  sql.LabelPositive,
			count:  2,
		},
		{
			corpus: data_kitchen_sink.RoundCorpus,
			name:   data_kitchen_sink.CircleTest,
			label:  sql.LabelUntriaged,
			count:  3,
		},
		{
			corpus: data_kitchen_sink.RoundCorpus,
			name:   data_kitchen_sink.CircleTest,
			label:  sql.LabelPositive,
			count:  2,
		},
	}, results)
}

func subTest_SummarizeDigestsAtHeadByTest_Success(t *testing.T, db *pgx.Conn) {
	const statement = `
SELECT corpus, name, label, count(label) FROM (
  SELECT DISTINCT Traces.keys ->> 'name' AS name, Traces.keys ->> 'source_type' AS corpus,
    encode(TraceValues.digest, 'hex') AS digest, Expectations.label
  FROM
    (SELECT trace_id, keys, most_recent_commit_id FROM Traces@head_idx
     WHERE most_recent_commit_id > 0) AS Traces
  INNER LOOKUP JOIN 
    TraceValues@trace_and_commit_idx
  ON TraceValues.trace_id = Traces.trace_id AND TraceValues.commit_id = Traces.most_recent_commit_id
  INNER LOOKUP JOIN
    Expectations
  ON Expectations.grouping_id = TraceValues.grouping_id AND Expectations.digest = TraceValues.digest
) GROUP BY corpus, name, label;`
	assertNoFullTableScans(t, db, statement)
	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	var results []byTestRow
	for rows.Next() {
		r := byTestRow{}
		err := rows.Scan(&r.corpus, &r.name, &r.label, &r.count)
		require.NoError(t, err)
		results = append(results, r)
	}

	assert.ElementsMatch(t, []byTestRow{
		{
			corpus: data_kitchen_sink.CornersCorpus,
			name:   data_kitchen_sink.SquareTest,
			label:  sql.LabelPositive,
			count:  4,
		},
		{
			corpus: data_kitchen_sink.CornersCorpus,
			name:   data_kitchen_sink.SquareTest,
			label:  sql.LabelNegative,
			count:  1,
		},
		{
			corpus: data_kitchen_sink.CornersCorpus,
			name:   data_kitchen_sink.TriangleTest,
			label:  sql.LabelPositive,
			count:  2,
		},
		{
			corpus: data_kitchen_sink.RoundCorpus,
			name:   data_kitchen_sink.CircleTest,
			label:  sql.LabelUntriaged,
			count:  3,
		},
		{
			corpus: data_kitchen_sink.RoundCorpus,
			name:   data_kitchen_sink.CircleTest,
			label:  sql.LabelPositive,
			count:  2,
		},
	}, results)
}

func subTest_TryjobDataIsSeparateFromPrimaryBranch_Success(t *testing.T, db *pgx.Conn) {
	const primaryStatement = `SELECT DISTINCT keys ->> 'name' FROM Traces;`
	const tryjobStatement = `SELECT DISTINCT keys ->> 'name' FROM TryjobTraces;`

	getTestNames := func(statement string) []types.TestName {
		rows, err := db.Query(context.Background(), statement)
		require.NoError(t, err)
		defer rows.Close()

		var names []types.TestName
		for rows.Next() {
			tn := types.TestName("")
			err := rows.Scan(&tn)
			require.NoError(t, err)
			names = append(names, tn)
		}
		return names
	}

	// Primary branch should have only these test names
	assert.ElementsMatch(t, []types.TestName{
		data_kitchen_sink.CircleTest, data_kitchen_sink.SquareTest, data_kitchen_sink.TriangleTest,
	}, getTestNames(primaryStatement))

	// Tryjob data should at least have these two test names (which were added in CLs)
	assert.Contains(t, getTestNames(tryjobStatement), data_kitchen_sink.SevenTest)
	assert.Contains(t, getTestNames(tryjobStatement), data_kitchen_sink.RoundRectTest)
}

func subTest_CreateParamsetsForCL_Success(t *testing.T, db *pgx.Conn) {
	const statement = `
SELECT DISTINCT key, value FROM TryjobParams
WHERE changelist_id = $1 AND patchset_id = $2;`
	crsCLID := qualifyIDWithSystem(data_kitchen_sink.GerritCRS, data_kitchen_sink.ChangeListIDThatAttemptsToFixIOS)
	crsPSID := qualifyIDWithSystem(data_kitchen_sink.GerritCRS, data_kitchen_sink.PatchSetIDFixesIPadButNotIPhone)

	args := []interface{}{crsCLID, crsPSID}
	assertNoFullTableScans(t, db, statement, args...)
	rows, err := db.Query(context.Background(), statement, args...)
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
	assert.Equal(t, paramtools.ParamSet{
		data_kitchen_sink.ColorModeKey:    []string{"GREY", "RGB"},
		data_kitchen_sink.DeviceKey:       []string{"iPad6,3", "iPhone12,1", "taimen"},
		data_kitchen_sink.ExtensionOption: []string{"png"},
		types.PrimaryKeyField:             []string{"circle", "square", "triangle"},
		data_kitchen_sink.OSKey:           []string{"Android", "iOS"},
		types.CorpusField:                 []string{"corners", "round"},
	}, ps)
}

func subTest_SearchUntriagedUnignoredDigestsAndTracesForCL_Success(t *testing.T, db *pgx.Conn) {
	t.Skip("Will do")
	assert.Fail(t, "needs impl")
}

func subTest_ListUntriagedChangelistExpectations_Success(t *testing.T, db *pgx.Conn) {
	// Just like with the primary branch Expectations table, we should be able to query for untriaged
	// digests that appeared in the data for a given changelist.
	iosCL := qualifyIDWithSystem(data_kitchen_sink.GerritCRS, data_kitchen_sink.ChangeListIDThatAttemptsToFixIOS)
	newTestsCL := qualifyIDWithSystem(data_kitchen_sink.GerritInternalCRS, data_kitchen_sink.ChangeListIDThatAddsNewTests)

	untriagedIOSExpectations := getCLExpectationsWithLabel(t, db, iosCL, expectations.Untriaged)
	assert.ElementsMatch(t, []digestKeysRow{
		// From Primary Branch
		{Digest: data_kitchen_sink.DigestBlank, KeysJSON: triangleGrouping},
		{Digest: data_kitchen_sink.DigestA04Unt, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestA05Unt, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestA06Unt, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestC03Unt, KeysJSON: circleGrouping},
		{Digest: data_kitchen_sink.DigestC04Unt, KeysJSON: circleGrouping},
		{Digest: data_kitchen_sink.DigestC05Unt, KeysJSON: circleGrouping},

		// This digest was newly seen on this CL
		{Digest: data_kitchen_sink.DigestC07Unt_CL, KeysJSON: circleGrouping},
		// This digest was erroneously triaged on this CL as untriaged (and thus it overrides the
		// primary branch).
		{Digest: data_kitchen_sink.DigestB01Pos, KeysJSON: triangleGrouping},
	}, untriagedIOSExpectations)

	untriagedNewTestsExpectations := getCLExpectationsWithLabel(t, db, newTestsCL, expectations.Untriaged)
	assert.ElementsMatch(t, []digestKeysRow{
		// From Primary Branch
		{Digest: data_kitchen_sink.DigestBlank, KeysJSON: triangleGrouping},
		{Digest: data_kitchen_sink.DigestA04Unt, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestA05Unt, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestA06Unt, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestC03Unt, KeysJSON: circleGrouping},
		{Digest: data_kitchen_sink.DigestC04Unt, KeysJSON: circleGrouping},
		{Digest: data_kitchen_sink.DigestC05Unt, KeysJSON: circleGrouping},

		// These digests were seen on this CL.
		{Digest: data_kitchen_sink.DigestBlank, KeysJSON: textGrouping},
		{Digest: data_kitchen_sink.DigestE03Unt_CL, KeysJSON: roundRectGrouping},
	}, untriagedNewTestsExpectations)
}

func subTest_ListPositiveChangelistExpectations_Success(t *testing.T, db *pgx.Conn) {
	// Just like with the primary branch Expectations table, we should be able to query for untriaged
	// digests that appeared in the data for a given changelist.
	iosCL := qualifyIDWithSystem(data_kitchen_sink.GerritCRS, data_kitchen_sink.ChangeListIDThatAttemptsToFixIOS)
	newTestsCL := qualifyIDWithSystem(data_kitchen_sink.GerritInternalCRS, data_kitchen_sink.ChangeListIDThatAddsNewTests)

	positiveIOSExpectations := getCLExpectationsWithLabel(t, db, iosCL, expectations.Positive)
	assert.ElementsMatch(t, []digestKeysRow{
		// From Primary Branch
		{Digest: data_kitchen_sink.DigestA01Pos, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestA02Pos, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestA03Pos, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestA07Pos, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestA08Pos, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestB01Pos, KeysJSON: triangleGrouping},
		{Digest: data_kitchen_sink.DigestB02Pos, KeysJSON: triangleGrouping},
		{Digest: data_kitchen_sink.DigestC01Pos, KeysJSON: circleGrouping},
		{Digest: data_kitchen_sink.DigestC02Pos, KeysJSON: circleGrouping},

		// This digest was newly seen on this CL
		{Digest: data_kitchen_sink.DigestC06Pos_CL, KeysJSON: circleGrouping},
	}, positiveIOSExpectations)

	positiveNewTestsExpectations := getCLExpectationsWithLabel(t, db, newTestsCL, expectations.Positive)
	assert.ElementsMatch(t, []digestKeysRow{
		// From Primary Branch
		{Digest: data_kitchen_sink.DigestA01Pos, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestA02Pos, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestA03Pos, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestA07Pos, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestA08Pos, KeysJSON: squareGrouping},
		{Digest: data_kitchen_sink.DigestB01Pos, KeysJSON: triangleGrouping},
		{Digest: data_kitchen_sink.DigestB02Pos, KeysJSON: triangleGrouping},
		{Digest: data_kitchen_sink.DigestC01Pos, KeysJSON: circleGrouping},
		{Digest: data_kitchen_sink.DigestC02Pos, KeysJSON: circleGrouping},

		// These digests were seen on this CL.
		{Digest: data_kitchen_sink.DigestE01Pos_CL, KeysJSON: roundRectGrouping},
		{Digest: data_kitchen_sink.DigestE02Pos_CL, KeysJSON: roundRectGrouping},
		{Digest: data_kitchen_sink.DigestD01Pos_CL, KeysJSON: textGrouping},
	}, positiveNewTestsExpectations)
}

func getCLExpectationsWithLabel(t *testing.T, db *pgx.Conn, changelistFK string, label expectations.Label) []digestKeysRow {
	// FIXME explain this query
	const statement = `
SELECT Groupings.keys, encode(JoinedExpectations.digest, 'hex') FROM
  (WITH 
    ProbableMatchesFromCL AS (
      SELECT ChangelistExpectations.grouping_id, ChangelistExpectations.digest,
      CASE WHEN ChangelistExpectations.label = -1
        THEN
          COALESCE(Expectations.label, 0)
        ELSE
          COALESCE(ChangelistExpectations.label, Expectations.label)
      END AS label
       FROM
       (SELECT grouping_id, digest, label FROM ChangelistExpectations@changelist_idx
        WHERE changelist_id = $1 AND (label = $2 OR label = -1)) AS ChangelistExpectations
      LEFT LOOKUP JOIN
        Expectations
      ON Expectations.grouping_id = ChangelistExpectations.grouping_id AND Expectations.digest = ChangelistExpectations.digest
    )
  SELECT grouping_id, digest FROM
    (SELECT grouping_id, digest FROM ProbableMatchesFromCL
     WHERE label = $2) AS ChangelistExpectations
  UNION
    SELECT grouping_id, digest FROM Expectations@label_idx WHERE label = $2
  ) AS JoinedExpectations
JOIN
  Groupings
ON JoinedExpectations.grouping_id = Groupings.grouping_id;`

	args := []interface{}{changelistFK, sql.ConvertLabelFromString(label)}
	assertNoFullTableScans(t, db, statement, args...)
	rows, err := db.Query(context.Background(), statement, args...)
	require.NoError(t, err)
	defer rows.Close()

	var rv []digestKeysRow
	for rows.Next() {
		var d digestKeysRow
		err := rows.Scan(&d.KeysJSON, &d.Digest)
		require.NoError(t, err)
		rv = append(rv, d)
	}
	return rv
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
	require.NoError(t, writeTryjobsChangelistsAndPatchsets(ctx, db))
	require.NoError(t, writeTryjobData(ctx, db))
	require.NoError(t, writeChangelistExpectations(ctx, db))
	require.NoError(t, writePrimaryBranchExpectations(ctx, db))
	require.NoError(t, writeDiffMetrics(ctx, db))
	require.NoError(t, writeIgnoreRules(ctx, db))
	return db
}

func assertNoFullTableScans(t *testing.T, db *pgx.Conn, statement string, args ...interface{}) {
	rows, err := db.Query(context.Background(), "EXPLAIN "+statement, args...)
	require.NoError(t, err)
	defer rows.Close()

	var explainRows []string
	for rows.Next() {
		var tree string
		var field string
		var desc string
		err := rows.Scan(&tree, &field, &desc)
		require.NoError(t, err)
		explainRows = append(explainRows, fmt.Sprintf("%s | %s | %s", tree, field, desc))
	}
	assert.NotContains(t, strings.Join(explainRows, "\n"), "FULL")
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

func getDiffMetricsFor(one, two types.Digest) diff.DiffMetrics {
	metrics := data_kitchen_sink.MakePixelDiffsForCorpusNameGrouping()
	for _, m := range metrics {
		if m.LeftDigest == one && m.RightDigest == two || m.LeftDigest == two && m.RightDigest == one {
			return m.Metrics
		}
	}
	panic("cannot find metrics for " + one + " " + two)
}

// digestKeysRow is a helper type for several queries
type digestKeysRow struct {
	Digest   types.Digest
	KeysJSON string
}

// diffRow is a helper type for queries around diffs
type diffRow struct {
	Label         sql.ExpectationsLabel
	Left          types.Digest
	Right         types.Digest
	NumDiffPixels int
	MaxRGBADiffs  [4]int
	DimDiffer     bool
}

const (
	circleGrouping    = `{"name": "circle", "source_type": "round"}`
	squareGrouping    = `{"name": "square", "source_type": "corners"}`
	triangleGrouping  = `{"name": "triangle", "source_type": "corners"}`
	textGrouping      = `{"name": "seven", "source_type": "text"}`
	roundRectGrouping = `{"name": "round rect", "source_type": "round"}`
)
