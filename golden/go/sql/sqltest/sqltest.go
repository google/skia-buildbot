package sqltest

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/bazel/external/cockroachdb"
	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
)

// NewCockroachDBForTests creates a randomly named database on a test CockroachDB instance (aka the
// CockroachDB emulator). The returned pool will automatically be closed after the test finishes.
func NewCockroachDBForTests(ctx context.Context, t testing.TB) *pgxpool.Pool {
	unittest.RequiresCockroachDB(t)

	cockroach := "cockroach"
	if bazel.InBazelTest() {
		var err error
		cockroach, err = cockroachdb.FindCockroach()
		require.NoError(t, err)
	}

	out, err := exec.Command(cockroach, "version").CombinedOutput()
	require.NoError(t, err, "Do you have 'cockroach' on your path? %s", out)

	n, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	require.NoError(t, err)
	dbName := "for_tests" + n.String()
	host := emulators.GetEmulatorHostEnvVar(emulators.CockroachDB)

	out, err = exec.Command(cockroach, "sql", "--insecure", "--host="+host,
		"--execute=CREATE DATABASE IF NOT EXISTS "+dbName).CombinedOutput()
	require.NoError(t, err, `creating test database: %s
If running locally, make sure you set the env var COCKROACHDB_EMULATOR_STORE_DIR and ran:
./scripts/run_emulators/run_emulators start
and now currently have %s set. Even though we call it an "emulator",
this sets up a real version of cockroachdb.
`, out, emulators.GetEmulatorHostEnvVarName(emulators.CockroachDB))

	connectionString := fmt.Sprintf("postgresql://root@%s/%s?sslmode=disable", host, dbName)
	conf, err := pgxpool.ParseConfig(connectionString)
	require.NoError(t, err)
	conf.MaxConns = 4
	conn, err := pgxpool.ConnectConfig(ctx, conf)
	require.NoError(t, err)
	t.Cleanup(func() {
		conn.Close()
	})
	return conn
}

// NewCockroachDBForTestsWithProductionSchema returns a SQL database with the production
// schema. It will be aimed at a randomly named database.
func NewCockroachDBForTestsWithProductionSchema(ctx context.Context, t testing.TB) *pgxpool.Pool {
	db := NewCockroachDBForTests(ctx, t)
	_, err := db.Exec(ctx, schema.Schema)
	require.NoError(t, err)
	return db
}

// SQLExporter is an abstraction around a type that can be written as a single row in a SQL table.
type SQLExporter interface {
	// ToSQLRow returns the column names and the column data that should be written for this row.
	ToSQLRow() (colNames []string, colData []interface{})
}

// SQLScanner is an abstraction around reading a single row from an SQL table.
type SQLScanner interface {
	// ScanFrom takes in a function that takes in any number of pointers and will fill in the data.
	// The arguments passed into scan should be the row fields in the order they appear in the
	// table schema, as they will be filled in via a SELECT * FROM Table.
	ScanFrom(scan func(...interface{}) error) error
}

// RowsOrder is an option that rows in a table can implement to specify the ordering of the returned
// data (to make for easier to debug tests).
type RowsOrder interface {
	// RowsOrderBy returns a SQL fragment like "ORDER BY my_field DESC".
	RowsOrderBy() string
}

// BulkInsertDataTables adds all the data from tables to the provided database. tables is expected
// to be a struct that contains fields which are slices of SQLExporter. The tables will be inserted
// in the same order that the fields are in the struct - if there are foreign key relationships,
// be sure to order them correctly. This method panics if the passed in tables parameter is of
// the wrong type.
func BulkInsertDataTables(ctx context.Context, db *pgxpool.Pool, tables interface{}) error {
	// It's tempting to insert these in parallel, but that could make foreign keys flaky.
	v := reflect.ValueOf(tables)
	for i := 0; i < v.NumField(); i++ {
		tableName := v.Type().Field(i).Name
		table := v.Field(i) // Fields of the outer type are expected to be tables.
		if table.Kind() != reflect.Slice {
			panic(`Expected table should be a slice: ` + tableName)
		}

		if err := writeToTable(ctx, db, tableName, table); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

func writeToTable(ctx context.Context, db *pgxpool.Pool, name string, table reflect.Value) error {
	var arguments []interface{}
	var colNames []string
	numRows := table.Len()
	// Go through each element of the table slice, cast it to ToSQLRow and then call that
	// function on it to get the arguments needed for that row.
	for j := 0; j < numRows; j++ {
		r := table.Index(j)
		row, ok := r.Interface().(SQLExporter)
		if !ok {
			panic(`Expected table should be a slice of types that implement ToSQLRow: ` + name)
		}
		cn, args := row.ToSQLRow()
		if len(colNames) == 0 {
			colNames = cn
		}
		if len(colNames) != len(args) {
			panic(`Expected length of colNames and arguments to match for ` + name)
		}
		arguments = append(arguments, args...)
	}
	if len(arguments) == 0 {
		return nil
	}
	numCols := len(colNames)
	// a chunkSize of 3000 means we can stay under the 64k number limit as long as there are
	// fewer than 22 columns, which should be realistic for all our tables.
	err := util.ChunkIter(numRows, 3000, func(startIdx int, endIdx int) error {
		argBatch := arguments[startIdx*numCols : endIdx*numCols]
		vp := sql.ValuesPlaceholders(numCols, endIdx-startIdx)
		insert := fmt.Sprintf(`INSERT INTO %s (%s) VALUES %s`, name, strings.Join(colNames, ","), vp)

		_, err := db.Exec(ctx, insert, argBatch...)
		return skerr.Wrapf(err, "batch: %d-%d (%d-%d)", startIdx, endIdx, startIdx*numCols, endIdx*numCols)
	})
	return skerr.Wrapf(err, "Inserting %d rows into table %s", numRows, name)
}

// GetAllRows returns all rows for a given table. The passed in row param must be a pointer type
// that implements the sqltest.Scanner interface. The passed in row may optionally implement the
// sqltest.RowsOrder interface to specify an ordering to return the rows in (this can make for
// easier to debug tests).
// The returned value is a slice of the provided row type (without a pointer) and can be converted
// to a normal slice via a type assertion. If anything goes wrong, the function will panic.
func GetAllRows(ctx context.Context, t *testing.T, db *pgxpool.Pool, table string, row interface{}, whereClauses ...string) interface{} {
	if _, ok := row.(SQLScanner); !ok {
		require.Fail(t, "Row does not implement SQLScanner. Need pointer type.", "%#v", row)
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = whereClauses[0]
	}
	statement := `SELECT * FROM ` + table + " " + whereClause
	if ro, ok := row.(RowsOrder); ok {
		statement += " " + ro.RowsOrderBy()
	}
	rows, err := db.Query(ctx, statement)
	require.NoError(t, err)
	defer rows.Close()

	// typ now refers to the non-pointer type of row (aka RowType).
	typ := reflect.TypeOf(row).Elem()
	// Make an empty slice of RowType.
	rv := reflect.MakeSlice(reflect.SliceOf(typ), 0, 5)
	for rows.Next() {
		// Create a new *RowType
		newVal := reflect.New(typ)
		// Convert it to SQLScanner. This should always succeed because of the type
		// assertion earlier.
		s := newVal.Interface().(SQLScanner)
		// Have the type extract its values from the rows object.
		require.NoError(t, s.ScanFrom(rows.Scan))
		// Add the dereferenced (non-pointer) type to the slice
		rv = reflect.Append(rv, newVal.Elem())
	}
	// Return the slice as type interface{}; It can be type asserted to []RowType by the caller.
	return rv.Interface()
}

// GetExplain returns the query plan for a given statement and arguments.
func GetExplain(t *testing.T, db *pgxpool.Pool, statement string, args ...interface{}) string {
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
	return strings.Join(explainRows, "\n")
}
