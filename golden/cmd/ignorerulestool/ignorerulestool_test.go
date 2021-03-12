package main

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"testing"
	"time"

	"go.skia.org/infra/golden/go/types"

	"github.com/google/uuid"

	"go.skia.org/infra/go/paramtools"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

func TestUpdateIgnoredTraces_StartsNull_SetToCorrectValue(t *testing.T) {
	unittest.LargeTest(t)
	existingData := dks.Build()
	// Force these all to be null
	for i := range existingData.Traces {
		existingData.Traces[i].MatchesAnyIgnoreRule = schema.NBNull
	}
	for i := range existingData.ValuesAtHead {
		existingData.ValuesAtHead[i].MatchesAnyIgnoreRule = schema.NBNull
	}
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	// Verify things are null
	row := db.QueryRow(ctx, `SELECT count(*) FROM Traces WHERE matches_any_ignore_rule IS NULL`)
	var count int
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 41, count)
	assert.Equal(t, 41, len(existingData.Traces))
	row = db.QueryRow(ctx, `SELECT count(*) FROM ValuesAtHead WHERE matches_any_ignore_rule IS NULL`)
	count = 0
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 33, count)
	assert.Equal(t, 33, len(existingData.ValuesAtHead))

	require.NoError(t, updateIgnoredTraces(ctx, db))
	// Verify things are not null
	row = db.QueryRow(ctx, `SELECT count(*) FROM Traces WHERE matches_any_ignore_rule IS NULL`)
	count = -1
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 0, count)
	row = db.QueryRow(ctx, `SELECT count(*) FROM ValuesAtHead WHERE matches_any_ignore_rule IS NULL`)
	count = -1
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 0, count)

	expectedData := dks.Build()
	actualTraces := sqltest.GetAllRows(ctx, t, db, "Traces", &schema.TraceRow{}).([]schema.TraceRow)
	assert.ElementsMatch(t, expectedData.Traces, actualTraces)

	actualValuesAtHead := sqltest.GetAllRows(ctx, t, db, "ValuesAtHead", &schema.ValueAtHeadRow{}).([]schema.ValueAtHeadRow)
	assert.ElementsMatch(t, expectedData.ValuesAtHead, actualValuesAtHead)
}

func TestUpdateIgnoredTraces_StartsNotNull_NotChanged(t *testing.T) {
	unittest.LargeTest(t)
	existingData := dks.Build()
	// Force these all to be a sentinel value
	for i := range existingData.Traces {
		existingData.Traces[i].MatchesAnyIgnoreRule = schema.NBFalse
	}
	for i := range existingData.ValuesAtHead {
		existingData.ValuesAtHead[i].MatchesAnyIgnoreRule = schema.NBTrue
	}
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	// Verify things are that sentinel value
	row := db.QueryRow(ctx, `SELECT count(*) FROM Traces WHERE matches_any_ignore_rule = FALSE`)
	var count int
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 41, count)
	assert.Equal(t, 41, len(existingData.Traces))
	row = db.QueryRow(ctx, `SELECT count(*) FROM ValuesAtHead WHERE matches_any_ignore_rule = TRUE`)
	count = 0
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 33, count)
	assert.Equal(t, 33, len(existingData.ValuesAtHead))

	require.NoError(t, updateIgnoredTraces(ctx, db))

	// Verify things were unchanged
	row = db.QueryRow(ctx, `SELECT count(*) FROM Traces WHERE matches_any_ignore_rule = FALSE`)
	count = 0
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 41, count)
	row = db.QueryRow(ctx, `SELECT count(*) FROM ValuesAtHead WHERE matches_any_ignore_rule = TRUE`)
	count = 0
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 33, count)
}

func TestUpdateIgnoredTraces_MultipleBatches_Success(t *testing.T) {
	unittest.LargeTest(t)

	const numTraces = 6060 // This is a number bigger than the batch size.
	traceIDs := makeRandomTraceIDs(numTraces)
	g := md5.Sum([]byte("whatever grouping"))
	arbitraryBytes := g[:]
	traces := make([]schema.TraceRow, 0, numTraces)
	atHead := make([]schema.ValueAtHeadRow, 0, numTraces)
	i := 0
	for ; i < numTraces/3; i++ {
		traces = append(traces, schema.TraceRow{
			TraceID:    traceIDs[i],
			GroupingID: arbitraryBytes,
			Keys: paramtools.Params{
				types.CorpusField:   "corpus",
				"should_be_ignored": "true",
			},
			MatchesAnyIgnoreRule: schema.NBNull,
		})
		atHead = append(atHead, schema.ValueAtHeadRow{
			TraceID:            traceIDs[i],
			MostRecentCommitID: "does not matter",
			Digest:             arbitraryBytes,
			OptionsID:          arbitraryBytes,
			GroupingID:         arbitraryBytes,
			Keys: paramtools.Params{
				types.CorpusField:   "corpus",
				"should_be_ignored": "true",
			},
			MatchesAnyIgnoreRule: schema.NBNull,
		})
	}
	for ; i < numTraces; i++ {
		traces = append(traces, schema.TraceRow{
			TraceID:    traceIDs[i],
			GroupingID: arbitraryBytes,
			Keys: paramtools.Params{
				types.CorpusField:   "corpus",
				"should_be_ignored": "false",
			},
			MatchesAnyIgnoreRule: schema.NBNull,
		})
		atHead = append(atHead, schema.ValueAtHeadRow{
			TraceID:            traceIDs[i],
			MostRecentCommitID: "does not matter",
			Digest:             arbitraryBytes,
			OptionsID:          arbitraryBytes,
			GroupingID:         arbitraryBytes,
			Keys: paramtools.Params{
				types.CorpusField:   "corpus",
				"should_be_ignored": "false",
			},
			MatchesAnyIgnoreRule: schema.NBNull,
		})
	}

	existingData := schema.Tables{
		Traces: traces,
		IgnoreRules: []schema.IgnoreRuleRow{{
			IgnoreRuleID: uuid.New(),
			CreatorEmail: "whomever",
			UpdatedEmail: "whomever",
			Expires:      time.Now(), // arbitrary
			Note:         "arbitrary",
			Query: paramtools.ReadOnlyParamSet{
				"should_be_ignored": []string{"true"},
			},
		}},
		ValuesAtHead: atHead,
	}
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	// Verify things are null
	row := db.QueryRow(ctx, `SELECT count(*) FROM Traces WHERE matches_any_ignore_rule IS NULL`)
	var count int
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, numTraces, count)
	row = db.QueryRow(ctx, `SELECT count(*) FROM ValuesAtHead WHERE matches_any_ignore_rule IS NULL`)
	count = 0
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, numTraces, count)

	require.NoError(t, updateIgnoredTraces(ctx, db))

	// Verify things are the right value
	row = db.QueryRow(ctx, `SELECT count(*) FROM Traces WHERE matches_any_ignore_rule IS NULL`)
	count = -1
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 0, count)
	row = db.QueryRow(ctx, `SELECT count(*) FROM ValuesAtHead WHERE matches_any_ignore_rule IS NULL`)
	count = -1
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 0, count)

	row = db.QueryRow(ctx, `SELECT count(*) FROM Traces WHERE matches_any_ignore_rule = TRUE`)
	count = 0
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, numTraces/3, count)
	row = db.QueryRow(ctx, `SELECT count(*) FROM ValuesAtHead WHERE matches_any_ignore_rule = TRUE`)
	count = 0
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, numTraces/3, count)
}

func makeRandomTraceIDs(n int) []schema.TraceID {
	rv := make([]schema.TraceID, 0, n)
	for i := 0; i < n; i++ {
		t := make(schema.TraceID, md5.Size)
		_, err := rand.Read(t)
		if err != nil {
			panic(err)
		}
		rv = append(rv, t)
	}
	return rv
}
