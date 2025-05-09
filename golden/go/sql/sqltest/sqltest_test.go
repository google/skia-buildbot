package sqltest_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

func TestBulkInsertDataTables_ValidData_Success_cdb(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(ctx, t)
	_, err := db.Exec(ctx, testSchema_cdb)
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
	rows, err := db.Query(ctx, `SELECT * FROM TableOne`)
	require.NoError(t, err)
	defer rows.Close()
	var actualRows []tableOneRow
	for rows.Next() {
		var r tableOneRow
		assert.NoError(t, rows.Scan(&r.ColumnOne, &r.ColumnTwo))
		actualRows = append(actualRows, r)
	}
	assert.Equal(t, []tableOneRow{{
		ColumnOne: "apple", ColumnTwo: "banana",
	}, {
		ColumnOne: "cherry", ColumnTwo: "durian",
	}, {
		ColumnOne: "elderberry", ColumnTwo: "fig",
	}}, actualRows)

	rows, err = db.Query(ctx, `SELECT * FROM TableTwo`)
	require.NoError(t, err)
	defer rows.Close()
	var actualRows2 []tableTwoRow
	for rows.Next() {
		var r tableTwoRow
		assert.NoError(t, rows.Scan(&r.SpecialColumnOne, &r.ForeignColumnTwo, &r.ComputedColumnThree))
		actualRows2 = append(actualRows2, r)
	}
	assert.Equal(t, []tableTwoRow{{
		SpecialColumnOne:    []byte("arugula"),
		ForeignColumnTwo:    "apple",
		ComputedColumnThree: 5,
	}, {
		SpecialColumnOne:    []byte("beet"),
		ForeignColumnTwo:    "elderberry",
		ComputedColumnThree: 10,
	}}, actualRows2)
}

func TestBulkInsertDataTables_InvalidForeignKeys_ReturnsError_cdb(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(ctx, t)
	_, err := db.Exec(ctx, testSchema_cdb)
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

func TestGetAllRows_RowsOrderByDefined_ReturnsInOrder_cdb(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(ctx, t)
	_, err := db.Exec(ctx, testSchema_cdb)
	require.NoError(t, err)

	err = sqltest.BulkInsertDataTables(ctx, db, testTables{
		TableThree: []tableThreeRow{
			{ColumnOne: "apricots", ColumnBool: schema.NBNull, ColumnTS: ts("2021-02-01T00:00:00Z")},
			{ColumnOne: "chorizo", ColumnBool: schema.NBFalse, ColumnTS: ts("2021-04-01T00:00:00Z")},
			{ColumnOne: "extra cheese", ColumnBool: schema.NBTrue, ColumnTS: ts("2021-03-01T00:00:00Z")},
		},
	})
	require.NoError(t, err)

	actualRows := sqltest.GetAllRows(ctx, t, db, "TableThree", &tableThreeRow{}).([]tableThreeRow)
	// The order matters because tableThreeRow has RowsOrderBy defined, which should return
	// the rows in descending order by column.
	assert.Equal(t, []tableThreeRow{
		{ColumnOne: "chorizo", ColumnBool: schema.NBFalse, ColumnTS: ts("2021-04-01T00:00:00Z")},
		{ColumnOne: "extra cheese", ColumnBool: schema.NBTrue, ColumnTS: ts("2021-03-01T00:00:00Z")},
		{ColumnOne: "apricots", ColumnBool: schema.NBNull, ColumnTS: ts("2021-02-01T00:00:00Z")},
	}, actualRows)
}

func TestGetAllRows_RowsOrderNotDefined_ReturnsInAnyOrder_cdb(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(ctx, t)
	_, err := db.Exec(ctx, testSchema_cdb)
	require.NoError(t, err)

	err = sqltest.BulkInsertDataTables(ctx, db, testTables{
		TableOne: []tableOneRow{
			{ColumnOne: "apple", ColumnTwo: "banana"},
			{ColumnOne: "cherry", ColumnTwo: "durian"},
			{ColumnOne: "elderberry", ColumnTwo: "fig"},
		},
	})
	require.NoError(t, err)

	actualRows := sqltest.GetAllRows(ctx, t, db, "TableOne", &tableOneRow{}).([]tableOneRow)
	assert.Equal(t, []tableOneRow{
		{ColumnOne: "apple", ColumnTwo: "banana"},
		{ColumnOne: "cherry", ColumnTwo: "durian"},
		{ColumnOne: "elderberry", ColumnTwo: "fig"},
	}, actualRows)
}

func TestGetRowChanges_RowsOrderDefined_ReturnsInOrder_cdb(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(ctx, t)
	_, err := db.Exec(ctx, testSchema_cdb)
	require.NoError(t, err)

	err = sqltest.BulkInsertDataTables(ctx, db, testTables{
		TableThree: []tableThreeRow{
			{ColumnOne: "apricots", ColumnBool: schema.NBNull, ColumnTS: ts("2021-02-01T00:00:00Z")},
			{ColumnOne: "chorizo", ColumnBool: schema.NBFalse, ColumnTS: ts("2021-04-01T00:00:00Z")},
			{ColumnOne: "extra cheese", ColumnBool: schema.NBTrue, ColumnTS: ts("2021-03-01T00:00:00Z")},
		},
	})
	require.NoError(t, err)

	t0 := time.Now()
	_, err = db.Exec(ctx, "INSERT INTO TableThree VALUES ('onions', true, '2021-05-01T00:00:00Z')")
	require.NoError(t, err)
	_, err = db.Exec(ctx, "UPDATE TableThree SET column_bool = true WHERE column_one = 'chorizo'")
	require.NoError(t, err)
	_, err = db.Exec(ctx, "DELETE FROM TableThree WHERE column_one = 'apricots'")
	require.NoError(t, err)

	missingRows, newRows := sqltest.GetRowChanges[tableThreeRow](ctx, t, db, "TableThree", t0)
	assert.Equal(t, []tableThreeRow{
		{ColumnOne: "chorizo", ColumnBool: schema.NBFalse, ColumnTS: ts("2021-04-01T00:00:00Z")},
		{ColumnOne: "apricots", ColumnBool: schema.NBNull, ColumnTS: ts("2021-02-01T00:00:00Z")},
	}, missingRows)
	assert.Equal(t, []tableThreeRow{
		{ColumnOne: "onions", ColumnBool: schema.NBTrue, ColumnTS: ts("2021-05-01T00:00:00Z")},
		{ColumnOne: "chorizo", ColumnBool: schema.NBTrue, ColumnTS: ts("2021-04-01T00:00:00Z")},
	}, newRows)
}

func TestGetRowChanges_RowsOrderNotDefined_ReturnsInAnyOrder_cdb(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(ctx, t)
	_, err := db.Exec(ctx, testSchema_cdb)
	require.NoError(t, err)

	err = sqltest.BulkInsertDataTables(ctx, db, testTables{
		TableOne: []tableOneRow{
			{ColumnOne: "apple", ColumnTwo: "banana"},
			{ColumnOne: "cherry", ColumnTwo: "durian"},
			{ColumnOne: "elderberry", ColumnTwo: "fig"},
		},
	})
	require.NoError(t, err)

	t0 := time.Now()
	_, err = db.Exec(ctx, "INSERT INTO TableOne VALUES ('grape', 'hackberry')")
	require.NoError(t, err)
	_, err = db.Exec(ctx, "UPDATE TableOne SET column_one = 'apricot' WHERE column_one = 'apple'")
	require.NoError(t, err)
	_, err = db.Exec(ctx, "DELETE FROM TableOne WHERE column_one = 'cherry'")
	require.NoError(t, err)

	missingRows, newRows := sqltest.GetRowChanges[tableOneRow](ctx, t, db, "TableOne", t0)
	assert.ElementsMatch(t, []tableOneRow{
		{ColumnOne: "apple", ColumnTwo: "banana"},
		{ColumnOne: "cherry", ColumnTwo: "durian"},
	}, missingRows)
	assert.ElementsMatch(t, []tableOneRow{
		{ColumnOne: "apricot", ColumnTwo: "banana"},
		{ColumnOne: "grape", ColumnTwo: "hackberry"},
	}, newRows)
}

func TestBulkInsertDataTables_ValidData_Success(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewSpannerDBForTests(ctx, t)
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
	rows, err := db.Query(ctx, `SELECT * FROM TableOne`)
	require.NoError(t, err)
	defer rows.Close()
	var actualRows []tableOneRow
	for rows.Next() {
		var r tableOneRow
		assert.NoError(t, rows.Scan(&r.ColumnOne, &r.ColumnTwo))
		actualRows = append(actualRows, r)
	}
	assert.ElementsMatch(t, []tableOneRow{{
		ColumnOne: "apple", ColumnTwo: "banana",
	}, {
		ColumnOne: "cherry", ColumnTwo: "durian",
	}, {
		ColumnOne: "elderberry", ColumnTwo: "fig",
	}}, actualRows)

	rows, err = db.Query(ctx, `SELECT * FROM TableTwo`)
	require.NoError(t, err)
	defer rows.Close()
	var actualRows2 []tableTwoRow
	for rows.Next() {
		var r tableTwoRow
		assert.NoError(t, rows.Scan(&r.SpecialColumnOne, &r.ForeignColumnTwo, &r.ComputedColumnThree))
		actualRows2 = append(actualRows2, r)
	}
	assert.ElementsMatch(t, []tableTwoRow{{
		SpecialColumnOne:    []byte("arugula"),
		ForeignColumnTwo:    "apple",
		ComputedColumnThree: 5,
	}, {
		SpecialColumnOne:    []byte("beet"),
		ForeignColumnTwo:    "elderberry",
		ComputedColumnThree: 10,
	}}, actualRows2)
}

func TestBulkInsertDataTables_InvalidForeignKeys_ReturnsError(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewSpannerDBForTests(ctx, t)
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

func TestGetAllRows_RowsOrderByDefined_ReturnsInOrder(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewSpannerDBForTests(ctx, t)
	_, err := db.Exec(ctx, testSchema)
	require.NoError(t, err)

	err = sqltest.BulkInsertDataTables(ctx, db, testTables{
		TableThree: []tableThreeRow{
			{ColumnOne: "apricots", ColumnBool: schema.NBNull, ColumnTS: ts("2021-02-01T00:00:00Z")},
			{ColumnOne: "chorizo", ColumnBool: schema.NBFalse, ColumnTS: ts("2021-04-01T00:00:00Z")},
			{ColumnOne: "extra cheese", ColumnBool: schema.NBTrue, ColumnTS: ts("2021-03-01T00:00:00Z")},
		},
	})
	require.NoError(t, err)

	actualRows := sqltest.GetAllRows(ctx, t, db, "TableThree", &tableThreeRow{}).([]tableThreeRow)
	// The order matters because tableThreeRow has RowsOrderBy defined, which should return
	// the rows in descending order by column.
	assert.Equal(t, []tableThreeRow{
		{ColumnOne: "chorizo", ColumnBool: schema.NBFalse, ColumnTS: ts("2021-04-01T00:00:00Z")},
		{ColumnOne: "extra cheese", ColumnBool: schema.NBTrue, ColumnTS: ts("2021-03-01T00:00:00Z")},
		{ColumnOne: "apricots", ColumnBool: schema.NBNull, ColumnTS: ts("2021-02-01T00:00:00Z")},
	}, actualRows)
}

func TestGetAllRows_RowsOrderNotDefined_ReturnsInAnyOrder(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewSpannerDBForTests(ctx, t)
	_, err := db.Exec(ctx, testSchema)
	require.NoError(t, err)

	err = sqltest.BulkInsertDataTables(ctx, db, testTables{
		TableOne: []tableOneRow{
			{ColumnOne: "apple", ColumnTwo: "banana"},
			{ColumnOne: "cherry", ColumnTwo: "durian"},
			{ColumnOne: "elderberry", ColumnTwo: "fig"},
		},
	})
	require.NoError(t, err)

	actualRows := sqltest.GetAllRows(ctx, t, db, "TableOne", &tableOneRow{}).([]tableOneRow)
	assert.ElementsMatch(t, []tableOneRow{
		{ColumnOne: "apple", ColumnTwo: "banana"},
		{ColumnOne: "cherry", ColumnTwo: "durian"},
		{ColumnOne: "elderberry", ColumnTwo: "fig"},
	}, actualRows)
}

func ts(s string) time.Time {
	ct, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return ct.UTC()
}

const testSchema_cdb = `CREATE TABLE IF NOT EXISTS TableOne (
  column_one STRING PRIMARY KEY,
  column_two STRING NOT NULL
);
CREATE TABLE IF NOT EXISTS TableTwo (
  special_column_one BYTES PRIMARY KEY,
  foreign_column_two STRING REFERENCES TableOne (column_one),
  computed_column_three INT8 NOT NULL
);
CREATE TABLE IF NOT EXISTS TableThree (
  column_one STRING PRIMARY KEY,
  column_bool BOOL,
  column_ts TIMESTAMP WITH TIME ZONE NOT NULL
);
`

const testSchema = `CREATE TABLE TableOne (
  column_one TEXT PRIMARY KEY,
  column_two TEXT NOT NULL
);

CREATE TABLE TableTwo (
  special_column_one BYTEA PRIMARY KEY,
  foreign_column_two TEXT,
  computed_column_three INT8 NOT NULL,
  CONSTRAINT FK_TableTwo_TableOne FOREIGN KEY (foreign_column_two) REFERENCES TableOne (column_one)
);

CREATE TABLE TableThree (
  column_one TEXT PRIMARY KEY,
  column_bool BOOL,
  column_ts TIMESTAMP WITH TIME ZONE NOT NULL
);
  `

type testTables struct {
	TableOne   []tableOneRow
	TableTwo   []tableTwoRow
	TableThree []tableThreeRow
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

func (r tableOneRow) GetPrimaryKeyCols() []string {
	return []string{"column_one"}
}

func (r *tableOneRow) ScanFrom(scan func(...interface{}) error) error {
	return scan(&r.ColumnOne, &r.ColumnTwo)
}

type tableTwoRow struct {
	SpecialColumnOne    []byte
	ForeignColumnTwo    string
	ComputedColumnThree int
}

func (r tableTwoRow) ToSQLRow() (colNames []string, colData []interface{}) {
	computedLength := len(r.ForeignColumnTwo)
	return []string{"special_column_one", "foreign_column_two", "computed_column_three"}, []interface{}{
		r.SpecialColumnOne, r.ForeignColumnTwo, computedLength,
	}
}

func (r tableTwoRow) GetPrimaryKeyCols() []string {
	return []string{"special_column_one"}
}

type tableThreeRow struct {
	ColumnOne  string
	ColumnBool schema.NullableBool
	ColumnTS   time.Time
}

func (r tableThreeRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"column_one", "column_bool", "column_ts"}, []interface{}{
		r.ColumnOne, r.ColumnBool.ToSQL(), r.ColumnTS,
	}
}

func (r tableThreeRow) GetPrimaryKeyCols() []string {
	return []string{"column_one"}
}

func (r *tableThreeRow) ScanFrom(scan func(...interface{}) error) error {
	var nb pgtype.Bool
	err := scan(&r.ColumnOne, &nb, &r.ColumnTS)
	if err != nil {
		return skerr.Wrap(err)
	}
	r.ColumnTS = r.ColumnTS.UTC()
	if nb.Status == pgtype.Present {
		if nb.Bool {
			r.ColumnBool = schema.NBTrue
		} else {
			r.ColumnBool = schema.NBFalse
		}
	} else {
		r.ColumnBool = schema.NBNull
	}
	return nil
}

func (r tableThreeRow) RowsOrderBy() string {
	return "ORDER BY column_ts DESC"
}
