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
	t.Run("SearchPositiveDigestsTracesAndUsers_Success", func(t *testing.T) {
		subTest_SearchPositiveDigestsTracesAndUsers_Success(t, db)
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
		subTest_FindClosestDigestsAndUsers_Success(t, db)
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

	t.Run("ChangelistDataIsSeparateFromPrimaryBranch_Success", func(t *testing.T) {
		subTest_ChangelistDataIsSeparateFromPrimaryBranch_Success(t, db)
	})
	t.Run("SearchCLForUntriagedUnignoredDigestsAndTracesFor_Success", func(t *testing.T) {
		subTest_SearchCLForUntriagedUnignoredDigestsAndTraces_Success(t, db)
	})
	t.Run("SearchCLForExclusiveUntriagedUnignoredDigestsAndTracesFor_Success", func(t *testing.T) {
		subTest_SearchCLForExclusiveUntriagedUnignoredDigestsAndTraces_Success(t, db)
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

func subTest_SearchPositiveDigestsTracesAndUsers_Success(t *testing.T, db *pgx.Conn) {
	const statement = `
SELECT DISTINCT encode(TraceValues.digest, 'hex') AS digest, Traces.keys, ExpectationRecords.user_name FROM
 (SELECT grouping_id, digest, expectation_record_id FROM Expectations@label_idx 
   WHERE label = 1) AS Expectations 
INNER LOOKUP JOIN
  (SELECT trace_id, digest, grouping_id FROM TraceValues@expectations_idx 
   WHERE commit_id > 0) AS TraceValues -- This range is just to show it possible
ON TraceValues.grouping_id = Expectations.grouping_id
  AND TraceValues.digest = Expectations.digest
INNER LOOKUP JOIN 
 (SELECT trace_id, keys FROM Traces@ignored_idx 
  WHERE Traces.matches_any_ignore_rule = false 
  AND Traces.keys @> '{"color mode":"RGB","device":"walleye"}') AS Traces
ON Traces.trace_id = TraceValues.trace_id
LEFT LOOKUP JOIN
  ExpectationRecords
ON ExpectationRecords.expectation_record_id = Expectations.expectation_record_id;`
	assertNoFullTableScans(t, db, statement)
	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	type digestKeysUser struct {
		Digest   types.Digest
		KeysJSON string
		User     string
	}

	var results []digestKeysUser
	for rows.Next() {
		r := digestKeysUser{}
		err := rows.Scan(&r.Digest, &r.KeysJSON, &r.User)
		require.NoError(t, err)
		results = append(results, r)
	}

	assert.ElementsMatch(t, []digestKeysUser{
		{
			Digest:   data_kitchen_sink.DigestA01Pos,
			KeysJSON: `{"color mode": "RGB", "device": "walleye", "name": "square", "os": "Android", "source_type": "corners"}`,
			User:     data_kitchen_sink.UserOne,
		},
		{
			Digest:   data_kitchen_sink.DigestA07Pos,
			KeysJSON: `{"color mode": "RGB", "device": "walleye", "name": "square", "os": "Android", "source_type": "corners"}`,
			User:     "fuzzy",
		},
		{
			Digest:   data_kitchen_sink.DigestA08Pos,
			KeysJSON: `{"color mode": "RGB", "device": "walleye", "name": "square", "os": "Android", "source_type": "corners"}`,
			User:     "fuzzy",
		},
		{
			Digest:   data_kitchen_sink.DigestB01Pos,
			KeysJSON: `{"color mode": "RGB", "device": "walleye", "name": "triangle", "os": "Android", "source_type": "corners"}`,
			User:     data_kitchen_sink.UserOne,
		},
		{
			Digest:   data_kitchen_sink.DigestC01Pos,
			KeysJSON: `{"color mode": "RGB", "device": "walleye", "name": "circle", "os": "Android", "source_type": "round"}`,
			User:     data_kitchen_sink.UserOne,
		},
	}, results)
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

func subTest_FindClosestDigestsAndUsers_Success(t *testing.T, db *pgx.Conn) {
	// Get closest digests (positive and negative) to digest 000... and b02... searching in the
	// entire grouping with hash aa8d3... (this is {"name": "triangle", "source_type": "corners"})
	// See https://stackoverflow.com/a/7630564 for more on DISTINCT ON
	const statement = `
SELECT DISTINCT ON (left_digest, label)
  label, encode(left_digest, 'hex') as left_digest, encode(right_digest, 'hex') as right_digest,
  num_diff_pixels, max_rgba_diff, dimensions_differ, ExpectationRecords.user_name
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
  (SELECT digest, label, expectation_record_id FROM Expectations@by_group_idx
   WHERE label > 0 
   AND Expectations.grouping_id = x'aa8d3c14238a4f717b9a99f7fe3735a7') AS Expectations
ON Expectations.digest = DiffMetrics.right_digest
INNER LOOKUP JOIN
  ExpectationRecords
ON ExpectationRecords.expectation_record_id = Expectations.expectation_record_id
ORDER BY left_digest, label, num_diff_pixels ASC, max_channel_diff ASC, right_digest ASC;`
	assertNoFullTableScans(t, db, statement)
	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	var closestResults []diffRow
	for rows.Next() {
		row := diffRow{}
		diffs := [4]int{}
		diffSlice := diffs[:]
		err := rows.Scan(&row.Label, &row.Left, &row.Right, &row.NumDiffPixels, &diffSlice, &row.DimDiffer, &row.UserWhoTriagedRight)
		require.NoError(t, err)
		copy(row.MaxRGBADiffs[:], diffSlice)
		closestResults = append(closestResults, row)
	}

	metrics000vsB01 := getDiffMetricsFor(data_kitchen_sink.DigestBlank, data_kitchen_sink.DigestB01Pos)
	metrics000vsB04 := getDiffMetricsFor(data_kitchen_sink.DigestBlank, data_kitchen_sink.DigestB04Neg)
	metricsB02vsB01 := getDiffMetricsFor(data_kitchen_sink.DigestB02Pos, data_kitchen_sink.DigestB01Pos)
	metricsB02vsB03 := getDiffMetricsFor(data_kitchen_sink.DigestB02Pos, data_kitchen_sink.DigestB03Neg)

	assert.ElementsMatch(t, []diffRow{
		{
			Label:               sql.LabelPositive,
			Left:                data_kitchen_sink.DigestBlank,
			Right:               data_kitchen_sink.DigestB01Pos,
			NumDiffPixels:       metrics000vsB01.NumDiffPixels,
			MaxRGBADiffs:        metrics000vsB01.MaxRGBADiffs,
			DimDiffer:           metrics000vsB01.DimDiffer,
			UserWhoTriagedRight: data_kitchen_sink.UserOne,
		},
		{
			Label:               sql.LabelNegative,
			Left:                data_kitchen_sink.DigestBlank,
			Right:               data_kitchen_sink.DigestB04Neg,
			NumDiffPixels:       metrics000vsB04.NumDiffPixels,
			MaxRGBADiffs:        metrics000vsB04.MaxRGBADiffs,
			DimDiffer:           metrics000vsB04.DimDiffer,
			UserWhoTriagedRight: data_kitchen_sink.UserOne,
		},
		{
			Label:               sql.LabelPositive,
			Left:                data_kitchen_sink.DigestB02Pos,
			Right:               data_kitchen_sink.DigestB01Pos,
			NumDiffPixels:       metricsB02vsB01.NumDiffPixels,
			MaxRGBADiffs:        metricsB02vsB01.MaxRGBADiffs,
			DimDiffer:           metricsB02vsB01.DimDiffer,
			UserWhoTriagedRight: data_kitchen_sink.UserOne,
		},
		{
			Label:               sql.LabelNegative,
			Left:                data_kitchen_sink.DigestB02Pos,
			Right:               data_kitchen_sink.DigestB03Neg,
			NumDiffPixels:       metricsB02vsB03.NumDiffPixels,
			MaxRGBADiffs:        metricsB02vsB03.MaxRGBADiffs,
			DimDiffer:           metricsB02vsB03.DimDiffer,
			UserWhoTriagedRight: data_kitchen_sink.UserOne,
		},
	}, closestResults)
}

func subTest_FindClosestDigestsRestrictRightSide_Success(t *testing.T, db *pgx.Conn) {
	// This is similar to the previous test except it restricts the right side to be
	// "color mode": "GREY"
	const statement = `
SELECT DISTINCT ON (left_digest, label)
  label, encode(left_digest, 'hex') as left_digest, encode(right_digest, 'hex') as right_digest,
  num_diff_pixels, max_rgba_diff, dimensions_differ, ExpectationRecords.user_name
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
  (SELECT digest, label, expectation_record_id FROM Expectations@by_group_idx
   WHERE label > 0 
   AND Expectations.grouping_id = x'aa8d3c14238a4f717b9a99f7fe3735a7') AS Expectations
ON Expectations.digest = DiffMetrics.right_digest
INNER LOOKUP JOIN
  ExpectationRecords
ON ExpectationRecords.expectation_record_id = Expectations.expectation_record_id
ORDER BY left_digest, label, num_diff_pixels ASC, max_channel_diff ASC, right_digest ASC;`
	assertNoFullTableScans(t, db, statement)
	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	var closestResults []diffRow
	for rows.Next() {
		row := diffRow{}
		diffs := [4]int{}
		diffSlice := diffs[:]
		err := rows.Scan(&row.Label, &row.Left, &row.Right, &row.NumDiffPixels, &diffSlice, &row.DimDiffer, &row.UserWhoTriagedRight)
		require.NoError(t, err)
		copy(row.MaxRGBADiffs[:], diffSlice)
		closestResults = append(closestResults, row)
	}

	metrics000vsB02 := getDiffMetricsFor(data_kitchen_sink.DigestBlank, data_kitchen_sink.DigestB02Pos)
	metrics000vsB04 := getDiffMetricsFor(data_kitchen_sink.DigestBlank, data_kitchen_sink.DigestB04Neg)
	metricsB02vsB04 := getDiffMetricsFor(data_kitchen_sink.DigestB02Pos, data_kitchen_sink.DigestB04Neg)

	assert.ElementsMatch(t, []diffRow{
		{
			Label:               sql.LabelPositive,
			Left:                data_kitchen_sink.DigestBlank,
			Right:               data_kitchen_sink.DigestB02Pos,
			NumDiffPixels:       metrics000vsB02.NumDiffPixels,
			MaxRGBADiffs:        metrics000vsB02.MaxRGBADiffs,
			DimDiffer:           metrics000vsB02.DimDiffer,
			UserWhoTriagedRight: data_kitchen_sink.UserOne,
		},
		{
			Label:               sql.LabelNegative,
			Left:                data_kitchen_sink.DigestBlank,
			Right:               data_kitchen_sink.DigestB04Neg,
			NumDiffPixels:       metrics000vsB04.NumDiffPixels,
			MaxRGBADiffs:        metrics000vsB04.MaxRGBADiffs,
			DimDiffer:           metrics000vsB04.DimDiffer,
			UserWhoTriagedRight: data_kitchen_sink.UserOne,
		},
		{
			Label:               sql.LabelNegative,
			Left:                data_kitchen_sink.DigestB02Pos,
			Right:               data_kitchen_sink.DigestB04Neg,
			NumDiffPixels:       metricsB02vsB04.NumDiffPixels,
			MaxRGBADiffs:        metricsB02vsB04.MaxRGBADiffs,
			DimDiffer:           metricsB02vsB04.DimDiffer,
			UserWhoTriagedRight: data_kitchen_sink.UserOne,
		},
		// In these results there is no closest positive to DigestB02Pos
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

func subTest_ChangelistDataIsSeparateFromPrimaryBranch_Success(t *testing.T, db *pgx.Conn) {
	const primaryStatement = `SELECT DISTINCT keys ->> 'name' FROM Traces;`
	const clStatement = `SELECT DISTINCT keys ->> 'name' FROM ChangelistTraces;`

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

	// Changelist data should at least have these two test names (which were added in CLs)
	clNames := getTestNames(clStatement)
	assert.Contains(t, clNames, data_kitchen_sink.SevenTest)
	assert.Contains(t, clNames, data_kitchen_sink.RoundRectTest)
}

func subTest_CreateParamsetsForCL_Success(t *testing.T, db *pgx.Conn) {
	const statement = `
SELECT DISTINCT key, value FROM ChangelistParams
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

func subTest_SearchCLForUntriagedUnignoredDigestsAndTraces_Success(t *testing.T, db *pgx.Conn) {
	const statement = `
WITH 
ProbableMatchesFromCL AS (
  SELECT ChangelistExpectations.grouping_id, ChangelistExpectations.digest,
  CASE WHEN ChangelistExpectations.label = -1 -- -1 is a value meaning "fallthrough"
    THEN
      COALESCE(Expectations.label, 0) -- report untriaged
    ELSE
      ChangelistExpectations.label
  END AS label
   FROM
   (SELECT grouping_id, digest, label FROM ChangelistExpectations@changelist_idx
    WHERE changelist_id = $1 AND (label = $3 OR label = -1)) AS ChangelistExpectations
  LEFT LOOKUP JOIN
    Expectations
  ON Expectations.grouping_id = ChangelistExpectations.grouping_id AND Expectations.digest = ChangelistExpectations.digest
),
ExpectationsFromPrimaryBranch AS (
  SELECT Expectations.grouping_id, Expectations.digest FROM 
    (SELECT grouping_id, digest FROM Expectations@label_idx
     WHERE label = $3) AS Expectations
  LEFT JOIN
    (SELECT grouping_id, digest FROM ChangelistExpectations@changelist_idx
     WHERE changelist_id = $1) AS OnChangelist
  ON Expectations.grouping_id = OnChangelist.grouping_id 
    AND Expectations.digest = OnChangelist.digest
  WHERE OnChangelist.digest IS NULL -- removes rows accounted for by ProbableMatchesFromCL
),
MatchingExpectations AS (
  SELECT grouping_id, digest FROM ProbableMatchesFromCL WHERE label = $3
  UNION DISTINCT
  SELECT grouping_id, digest FROM ExpectationsFromPrimaryBranch
)
SELECT DISTINCT encode(ChangelistValues.digest, 'hex') AS digest, ChangelistTraces.keys FROM
  MatchingExpectations
JOIN
  (SELECT digest, grouping_id, changelist_trace_id FROM ChangelistValues
   WHERE changelist_id = $1 AND patchset_id = $2) AS ChangelistValues
ON ChangelistValues.grouping_id = MatchingExpectations.grouping_id
  AND ChangelistValues.digest = MatchingExpectations.digest
JOIN
  (SELECT keys, changelist_trace_id FROM ChangelistTraces
   WHERE matches_any_ignore_rule = false) AS ChangelistTraces
ON ChangelistValues.changelist_trace_id = ChangelistTraces.changelist_trace_id;`
	searchCLPS := func(cl, ps string) []digestKeysRow {
		args := []interface{}{cl, ps, sql.LabelUntriaged}
		assertNoFullTableScans(t, db, statement, args...)
		rows, err := db.Query(context.Background(), statement, args...)
		require.NoError(t, err)
		defer rows.Close()

		var digestsAndKeys []digestKeysRow
		for rows.Next() {
			r := digestKeysRow{}
			err := rows.Scan(&r.Digest, &r.KeysJSON)
			require.NoError(t, err)
			digestsAndKeys = append(digestsAndKeys, r)
		}
		return digestsAndKeys
	}

	iPadCL := qualifyIDWithSystem(data_kitchen_sink.GerritCRS, data_kitchen_sink.ChangeListIDThatAttemptsToFixIOS)
	iPadPS := qualifyIDWithSystem(data_kitchen_sink.GerritCRS, data_kitchen_sink.PatchSetIDFixesIPadButNotIPhone)
	assert.ElementsMatch(t, []digestKeysRow{
		{Digest: data_kitchen_sink.DigestA04Unt,
			KeysJSON: `{"color mode": "GREY", "device": "iPad6,3", "name": "square", "os": "iOS", "source_type": "corners"}`},
		{Digest: data_kitchen_sink.DigestC07Unt_CL,
			KeysJSON: `{"color mode": "RGB", "device": "iPhone12,1", "name": "circle", "os": "iOS", "source_type": "round"}`},
		// Remember, this CL has an incorrect triage rule for digest B01, so we expect it to show up
		// in the search for untriaged digests.
		{Digest: data_kitchen_sink.DigestB01Pos,
			KeysJSON: `{"color mode": "RGB", "device": "iPad6,3", "name": "triangle", "os": "iOS", "source_type": "corners"}`},
	}, searchCLPS(iPadCL, iPadPS))

	newTestsCL := qualifyIDWithSystem(data_kitchen_sink.GerritInternalCRS, data_kitchen_sink.ChangeListIDThatAddsNewTests)
	newTestsPS1 := qualifyIDWithSystem(data_kitchen_sink.GerritInternalCRS, data_kitchen_sink.PatchsetIDAddsNewCorpus)
	newTestsPS2 := qualifyIDWithSystem(data_kitchen_sink.GerritInternalCRS, data_kitchen_sink.PatchsetIDAddsNewCorpusAndTest)
	assert.ElementsMatch(t, []digestKeysRow{
		{Digest: data_kitchen_sink.DigestBlank,
			KeysJSON: `{"color mode": "GREY", "device": "QuadroP400", "name": "seven", "os": "Windows10.3", "source_type": "text"}`},
		{Digest: data_kitchen_sink.DigestBlank,
			KeysJSON: `{"color mode": "RGB", "device": "QuadroP400", "name": "seven", "os": "Windows10.3", "source_type": "text"}`},
	}, searchCLPS(newTestsCL, newTestsPS1))

	assert.ElementsMatch(t, []digestKeysRow{
		{Digest: data_kitchen_sink.DigestE03Unt_CL,
			KeysJSON: `{"color mode": "RGB", "device": "walleye", "name": "round rect", "os": "Android", "source_type": "round"}`},
	}, searchCLPS(newTestsCL, newTestsPS2))
}

func subTest_SearchCLForExclusiveUntriagedUnignoredDigestsAndTraces_Success(t *testing.T, db *pgx.Conn) {
	const statement = `
WITH 
ExclusiveCLExpectations AS (
  SELECT ChangelistExpectations.grouping_id, ChangelistExpectations.digest,
  CASE WHEN ChangelistExpectations.label = -1 -- -1 is a value meaning "fallthrough"
    THEN
      0 -- report untriaged since we know from the join this is not on the primary branch.
    ELSE
      ChangelistExpectations.label
  END AS label
   FROM
   (SELECT grouping_id, digest, label FROM ChangelistExpectations@changelist_idx
    WHERE changelist_id = $1 AND (label = $3 OR label = -1)) AS ChangelistExpectations
  LEFT LOOKUP JOIN
    Expectations
  ON Expectations.grouping_id = ChangelistExpectations.grouping_id AND Expectations.digest = ChangelistExpectations.digest
  WHERE Expectations.digest IS NULL -- remove expectations that are on primary branch
)
SELECT DISTINCT encode(ChangelistValues.digest, 'hex') AS digest, ChangelistTraces.keys FROM
  ExclusiveCLExpectations
JOIN
  (SELECT digest, grouping_id, changelist_trace_id FROM ChangelistValues
   WHERE changelist_id = $1 AND patchset_id = $2) AS ChangelistValues
ON ChangelistValues.grouping_id = ExclusiveCLExpectations.grouping_id
  AND ChangelistValues.digest = ExclusiveCLExpectations.digest
JOIN
  (SELECT keys, changelist_trace_id FROM ChangelistTraces
   WHERE matches_any_ignore_rule = false) AS ChangelistTraces
ON ChangelistValues.changelist_trace_id = ChangelistTraces.changelist_trace_id;`
	searchCLPS := func(cl, ps string) []digestKeysRow {
		args := []interface{}{cl, ps, sql.LabelUntriaged}
		assertNoFullTableScans(t, db, statement, args...)
		rows, err := db.Query(context.Background(), statement, args...)
		require.NoError(t, err)
		defer rows.Close()

		var digestsAndKeys []digestKeysRow
		for rows.Next() {
			r := digestKeysRow{}
			err := rows.Scan(&r.Digest, &r.KeysJSON)
			require.NoError(t, err)
			digestsAndKeys = append(digestsAndKeys, r)
		}
		return digestsAndKeys
	}

	iPadCL := qualifyIDWithSystem(data_kitchen_sink.GerritCRS, data_kitchen_sink.ChangeListIDThatAttemptsToFixIOS)
	iPadPS := qualifyIDWithSystem(data_kitchen_sink.GerritCRS, data_kitchen_sink.PatchSetIDFixesIPadButNotIPhone)
	assert.ElementsMatch(t, []digestKeysRow{
		{Digest: data_kitchen_sink.DigestC07Unt_CL,
			KeysJSON: `{"color mode": "RGB", "device": "iPhone12,1", "name": "circle", "os": "iOS", "source_type": "round"}`},
	}, searchCLPS(iPadCL, iPadPS))

	newTestsCL := qualifyIDWithSystem(data_kitchen_sink.GerritInternalCRS, data_kitchen_sink.ChangeListIDThatAddsNewTests)
	newTestsPS1 := qualifyIDWithSystem(data_kitchen_sink.GerritInternalCRS, data_kitchen_sink.PatchsetIDAddsNewCorpus)
	newTestsPS2 := qualifyIDWithSystem(data_kitchen_sink.GerritInternalCRS, data_kitchen_sink.PatchsetIDAddsNewCorpusAndTest)
	// Even though DigestBlank has been seen on the primary branch before, it has not been seen
	// for this grouping, so we want it to show up in these search results.
	assert.ElementsMatch(t, []digestKeysRow{
		{Digest: data_kitchen_sink.DigestBlank,
			KeysJSON: `{"color mode": "GREY", "device": "QuadroP400", "name": "seven", "os": "Windows10.3", "source_type": "text"}`},
		{Digest: data_kitchen_sink.DigestBlank,
			KeysJSON: `{"color mode": "RGB", "device": "QuadroP400", "name": "seven", "os": "Windows10.3", "source_type": "text"}`},
	}, searchCLPS(newTestsCL, newTestsPS1))

	assert.ElementsMatch(t, []digestKeysRow{
		{Digest: data_kitchen_sink.DigestE03Unt_CL,
			KeysJSON: `{"color mode": "RGB", "device": "walleye", "name": "round rect", "os": "Android", "source_type": "round"}`},
	}, searchCLPS(newTestsCL, newTestsPS2))
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
		// Remember that DigestB01Pos was incorrectly triaged as untriaged on this CL, so it shouldn't
		// show up as "positive".
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
	// This query (the meat of which is in the JoinedExpectations Subquery) is a bit of a doozy.
	//
	// Precondition: The table ChangelistExpectations is full of expectations users explicitly
	// added for various CLs. Additionally, it has rows where label equals a fallthrough value (-1)
	// for digests that were uploaded to a given CL. As an optimization to keep ChangelistExpectations
	// small, the fallthrough rows are not added when there already is a matching entry in
	// Expectations. (Note: It is ok if there are grouping+digests that are in both tables, the logic
	// below accounts for that).
	//
	// Starting with ProbableMatchesFromCL, we query all rows from ChangelistExpectations where the
	// label matches what we are searching for or the fallthrough value (-1). We do a left join of
	// these rows on the primary branch's Expectations. With this join, we look at the label applied
	// in the CL. If the label is the fallthrough value, we use the label from Expectations.label or
	// 0 (untriaged) if Expectations.label is NULL (see COALESCE). Otherwise, we use the label from
	// ChangelistExpectations.
	//
	// At this point ProbableMatchesFromCL contains any grouping+digests which were 1) explicitly
	// explicitly triaged to the requested label by a user (overriding the primary branch's label;
	// 2) Any grouping+digests that were seen on this CL, but not the primary branch (as untriaged);
	// 3) Any grouping+digests that are in both tables, using the primary branch's label in
	// the case of fallthrough logic. This extra logic ensures that we *correctly* have all "newly
	// seen on this CL" digests marked as untriaged (for the untriaged case), but can lead to having
	// a few extra rows of the wrong label due to the above edge cases.
	//
	// We select just the rows from ProbableMatchesFromCL that match the target label and UNION the
	// results with all primary branch expectations that don't have an entry in Changelist
	// expectations (RemainingExpectations). It's important to UNION against RemainingExpectations
	// and not Expectations directly since if a CL overwrote the triage status of the primary
	// branch, we don't want to include the overwritten values (which would be incorrect).
	const statement = `
WITH 
  ProbableMatchesFromCL AS (
    SELECT ChangelistExpectations.grouping_id, ChangelistExpectations.digest,
    CASE WHEN ChangelistExpectations.label = -1 -- -1 is a value meaning "fallthrough"
      THEN
        COALESCE(Expectations.label, 0) -- report untriaged
      ELSE
        ChangelistExpectations.label
    END AS label
     FROM
     (SELECT grouping_id, digest, label FROM ChangelistExpectations@changelist_idx
      WHERE changelist_id = $1 AND (label = $2 OR label = -1)) AS ChangelistExpectations
    LEFT LOOKUP JOIN
      Expectations
    ON Expectations.grouping_id = ChangelistExpectations.grouping_id AND Expectations.digest = ChangelistExpectations.digest
  ),
  RemainingExpectations AS (
    SELECT Expectations.grouping_id, Expectations.digest FROM 
      (SELECT grouping_id, digest FROM Expectations@label_idx
       WHERE label = $2) AS Expectations
    LEFT JOIN
      (SELECT grouping_id, digest FROM ChangelistExpectations@changelist_idx
       WHERE changelist_id = $1) AS OnChangelist
    ON Expectations.grouping_id = OnChangelist.grouping_id 
      AND Expectations.digest = OnChangelist.digest
    WHERE OnChangelist.digest IS NULL -- removes rows accounted for by ProbableMatchesFromCL
  ),
  MatchingExpectations AS (
    SELECT grouping_id, digest FROM ProbableMatchesFromCL WHERE label = $2
    UNION DISTINCT
    SELECT grouping_id, digest FROM RemainingExpectations
  )
SELECT Groupings.keys, encode(MatchingExpectations.digest, 'hex') FROM
  MatchingExpectations
INNER LOOKUP JOIN
  Groupings
ON MatchingExpectations.grouping_id = Groupings.grouping_id;`

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
	require.NoError(t, writeChangelistData(ctx, db))
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
	Label               sql.ExpectationsLabel
	Left                types.Digest
	Right               types.Digest
	NumDiffPixels       int
	MaxRGBADiffs        [4]int
	DimDiffer           bool
	UserWhoTriagedRight string
}

const (
	circleGrouping    = `{"name": "circle", "source_type": "round"}`
	squareGrouping    = `{"name": "square", "source_type": "corners"}`
	triangleGrouping  = `{"name": "triangle", "source_type": "corners"}`
	textGrouping      = `{"name": "seven", "source_type": "text"}`
	roundRectGrouping = `{"name": "round rect", "source_type": "round"}`
)
