package main

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
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
SELECT commit_number, encode(digest, 'hex') FROM TraceValues
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
		digests[commitNum] = digest
	}
	expected := getTraceByID(",color mode=GREY,device=iPad6_3,name=triangle,os=iOS,source_type=corners,")
	assert.Equal(t, expected.Trace.Digests, digests)
}

func subTest_ListNegativeExpectations_Success(t *testing.T, db *pgx.Conn) {
	const statement = `
SELECT Groupings.keys, encode(digest, 'hex') FROM 
  (SELECT digest, grouping_id FROM Expectations 
   WHERE label = 2) AS Expectations -- 2 means negative
JOIN
  Groupings
ON Expectations.grouping_id = Groupings.grouping_id
ORDER BY 2;`
	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	var jsonGrouping []string
	var digests []types.Digest
	for rows.Next() {
		grouping := ""
		digest := types.Digest("")
		err := rows.Scan(&grouping, &digest)
		require.NoError(t, err)
		jsonGrouping = append(jsonGrouping, grouping)
		digests = append(digests, digest)
	}
	assert.Equal(t, []string{
		`{"name": "circle", "source_type": "round"}`,
		`{"name": "square", "source_type": "corners"}`,
		`{"name": "triangle", "source_type": "corners"}`,
		`{"name": "triangle", "source_type": "corners"}`,
	}, jsonGrouping)
	assert.Equal(t, []types.Digest{
		data_kitchen_sink.DigestBlank,
		data_kitchen_sink.DigestA09Neg,
		data_kitchen_sink.DigestB03Neg,
		data_kitchen_sink.DigestB04Neg,
	}, digests)
}

func subTest_ListUntriagedExpectations_Success(t *testing.T, db *pgx.Conn) {
	const statement = `
SELECT Groupings.keys, encode(digest, 'hex') FROM 
  (SELECT digest, grouping_id FROM Expectations 
-- 0 means untriaged. When ingesting data, we are sure to write a 0 to this table if there
-- is not already an entry. That way, we can index on label and not have to deal with label
-- being 0 or NULL (leading to more compact and efficient queries).
   WHERE label = 0) AS Expectations 
JOIN
  Groupings
ON Expectations.grouping_id = Groupings.grouping_id
ORDER BY 2;`
	rows, err := db.Query(context.Background(), statement)
	require.NoError(t, err)
	defer rows.Close()

	var jsonGrouping []string
	var digests []types.Digest
	for rows.Next() {
		grouping := ""
		digest := types.Digest("")
		err := rows.Scan(&grouping, &digest)
		require.NoError(t, err)
		jsonGrouping = append(jsonGrouping, grouping)
		digests = append(digests, digest)
	}
	assert.Equal(t, []string{
		`{"name": "triangle", "source_type": "corners"}`,
		`{"name": "square", "source_type": "corners"}`,
		`{"name": "square", "source_type": "corners"}`,
		`{"name": "square", "source_type": "corners"}`,
		`{"name": "circle", "source_type": "round"}`,
		`{"name": "circle", "source_type": "round"}`,
		`{"name": "circle", "source_type": "round"}`,
	}, jsonGrouping)
	assert.Equal(t, []types.Digest{
		data_kitchen_sink.DigestBlank,
		data_kitchen_sink.DigestA04Unt,
		data_kitchen_sink.DigestA05Unt,
		data_kitchen_sink.DigestA06Unt,
		data_kitchen_sink.DigestC03Unt,
		data_kitchen_sink.DigestC04Unt,
		data_kitchen_sink.DigestC05Unt,
	}, digests)
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
