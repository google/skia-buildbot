package datakitchensink_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/golden/go/sql"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
)

func TestBuild_DataIsValidAndMatchesSchema(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	data := dks.Build()
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, data))

	// Spot check the data.
	row := db.QueryRow(ctx, "SELECT count(*) from TraceValues")
	count := 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 166, count)

	row = db.QueryRow(ctx, "SELECT count(*) from Traces WHERE corpus = $1", "round")
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 15, count)

	row = db.QueryRow(ctx, "SELECT count(*) from Traces WHERE matches_any_ignore_rule = $1", true)
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 2, count)
	row = db.QueryRow(ctx, "SELECT count(*) from Traces WHERE matches_any_ignore_rule = $1", false)
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 39, count)
	row = db.QueryRow(ctx, "SELECT count(*) from Traces WHERE matches_any_ignore_rule IS NULL")
	count = -1
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 0, count)

	row = db.QueryRow(ctx, "SELECT count(*) from Expectations WHERE label = $1", string(schema.LabelPositive))
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 9, count)
	row = db.QueryRow(ctx, "SELECT count(*) from Expectations WHERE label = $1", string(schema.LabelNegative))
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 4, count)
	row = db.QueryRow(ctx, "SELECT count(*) from Expectations WHERE label = $1", string(schema.LabelUntriaged))
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 7, count)

	row = db.QueryRow(ctx, "SELECT count(*) from Changelists")
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 5, count)
	row = db.QueryRow(ctx, "SELECT count(*) from Patchsets")
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 6, count)
	row = db.QueryRow(ctx, "SELECT count(*) from Tryjobs")
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 13, count)
	row = db.QueryRow(ctx, "SELECT count(*) from SecondaryBranchValues")
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 47, count)

	row = db.QueryRow(ctx, "SELECT count(*) from GitCommits")
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 14, count)

	row = db.QueryRow(ctx, "SELECT count(*) from CommitsWithData")
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 10, count)
}

func TestGroupingIDs_HardCodedValuesMatchComputedValuesAndAreInComputedData(t *testing.T) {
	data := dks.Build()
	test := func(name, hardCodedHex string, hardCodedID []byte, groupingKeys paramtools.Params) {
		t.Run(name, func(t *testing.T) {
			_, idBytes := sql.SerializeMap(groupingKeys)
			assert.Equal(t, idBytes, hardCodedID)
			expectedID := hex.EncodeToString(idBytes)
			assert.Equal(t, expectedID, hardCodedHex)

			// Make sure grouping exists in the data tables and matches the keys we specified.
			found := false
			for _, g := range data.Groupings {
				if bytes.Equal(g.GroupingID, idBytes) {
					found = true
					assert.Equal(t, g.Keys, groupingKeys)
				}
			}
			assert.True(t, found, "grouping was not in sample data tables")
		})
	}

	test("CircleGroupingIDHex", dks.CircleGroupingIDHex, dks.CircleGroupingID, paramtools.Params{
		types.CorpusField:     dks.RoundCorpus,
		types.PrimaryKeyField: dks.CircleTest,
	})
	test("TriangleGroupingIDHex", dks.TriangleGroupingIDHex, dks.TriangleGroupingID, paramtools.Params{
		types.CorpusField:     dks.CornersCorpus,
		types.PrimaryKeyField: dks.TriangleTest,
	})
	test("SquareGroupingIDHex", dks.SquareGroupingIDHex, dks.SquareGroupingID, paramtools.Params{
		types.CorpusField:     dks.CornersCorpus,
		types.PrimaryKeyField: dks.SquareTest,
	})
	test("RoundRectGroupingIDHex", dks.RoundRectGroupingIDHex, dks.RoundRectGroupingID, paramtools.Params{
		types.CorpusField:     dks.RoundCorpus,
		types.PrimaryKeyField: dks.RoundRectTest,
	})
	test("TextSevenGroupingIDHex", dks.TextSevenGroupingIDHex, dks.TextSevenGroupingID, paramtools.Params{
		types.CorpusField:     dks.TextCorpus,
		types.PrimaryKeyField: dks.SevenTest,
	})
}
