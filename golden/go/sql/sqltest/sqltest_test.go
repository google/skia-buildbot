package sqltest_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

func TestBulkInsertDataTables_ValidData_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(ctx, t)
	_, err := db.Exec(ctx, testSchema)
	require.NoError(t, err)

	err = sqltest.BulkInsertDataTables(ctx, db, testTables{
		TableOne: []tableOneRow{
			{ColumnOne: "apple", ColumnTwo: "banana"},
			{ColumnOne: "cherry", ColumnTwo: "durian"},
			{ColumnOne: "elderberry", ColumnTwo: "fig"},
		},
		TableTwo: []tableTwoRow{
			// The ComputedColumn is intentionally omitted here to make sure the appropriate
			// value is inserted at the SQL level.
			{SpecialColumnOne: []byte("arugula"), ForeignColumnTwo: "apple"},
			{SpecialColumnOne: []byte("beet"), ForeignColumnTwo: "elderberry"},
		},
	})
	require.NoError(t, err)

	// Spotcheck data
	row := db.QueryRow(ctx, `SELECT column_two FROM TableOne WHERE column_one = $1`, "cherry")
	var col string
	require.NoError(t, row.Scan(&col))
	assert.Equal(t, "durian", col)

	row = db.QueryRow(ctx, `SELECT count(*) FROM TableOne`)
	var count int
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 3, count)

	row = db.QueryRow(ctx, `SELECT * FROM TableTwo LIMIT 1`)
	var actual tableTwoRow
	require.NoError(t, row.Scan(&actual.SpecialColumnOne, &actual.ForeignColumnTwo, &actual.ComputedColumnThree))
	assert.Equal(t, tableTwoRow{
		SpecialColumnOne:    []byte("arugula"),
		ForeignColumnTwo:    "apple",
		ComputedColumnThree: 5,
	}, actual)
}

func TestBulkInsertDataTables_InvalidForeignKeys_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(ctx, t)
	_, err := db.Exec(ctx, testSchema)
	require.NoError(t, err)

	err = sqltest.BulkInsertDataTables(ctx, db, testTables{
		TableOne: []tableOneRow{
			{ColumnOne: "apple", ColumnTwo: "banana"},
		},
		TableTwo: []tableTwoRow{
			{SpecialColumnOne: []byte("beet"), ForeignColumnTwo: "elderberry"},
		},
	})
	require.Error(t, err)
}

const testSchema = `CREATE TABLE IF NOT EXISTS TableOne (
  column_one STRING PRIMARY KEY,
  column_two STRING NOT NULL
);
CREATE TABLE IF NOT EXISTS TableTwo (
  special_column_one BYTES PRIMARY KEY,
  foreign_column_two STRING REFERENCES TableOne (column_one),
  computed_column_three INT8 AS (char_length(foreign_column_two)) STORED NOT NULL
);`

type testTables struct {
	TableOne []tableOneRow
	TableTwo []tableTwoRow
}

type tableOneRow struct {
	ColumnOne string
	ColumnTwo string
}

func (r tableOneRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"column_one", "column_two"}, []interface{}{
		r.ColumnOne, r.ColumnTwo,
	}
}

type tableTwoRow struct {
	SpecialColumnOne    []byte
	ForeignColumnTwo    string
	ComputedColumnThree int
}

func (r tableTwoRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"special_column_one", "foreign_column_two"}, []interface{}{
		r.SpecialColumnOne, r.ForeignColumnTwo,
	}
}
