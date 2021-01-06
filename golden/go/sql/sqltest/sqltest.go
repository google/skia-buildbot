package sqltest

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/sql"
)

// cockroachDBEmulatorHostEnvVar is the name of the environment variable
// that points to a test instance of CockroachDB.
const cockroachDBEmulatorHostEnvVar = "COCKROACHDB_EMULATOR_HOST"

// NewCockroachDBForTests creates a randomly named database on the presumed to be running
// cockroachDB instance as configured by the COCKROACHDB_EMULATOR_HOST environment variable.
// The returned pool will automatically be closed after the test finishes.
func NewCockroachDBForTests(ctx context.Context, t *testing.T) *pgxpool.Pool {
	unittest.RequiresCockroachDB(t)
	out, err := exec.Command("cockroach", "version").CombinedOutput()
	require.NoError(t, err, "Do you have 'cockroach' on your path? %s", out)

	n, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	require.NoError(t, err)
	dbName := "for_tests" + n.String()
	port := os.Getenv(cockroachDBEmulatorHostEnvVar)

	out, err = exec.Command("cockroach", "sql", "--insecure", "--host="+port,
		"--execute=CREATE DATABASE IF NOT EXISTS "+dbName).CombinedOutput()
	require.NoError(t, err, `creating test database: %s
If running locally, make sure you set the env var TMPDIR and ran:
./scripts/run_emulators/run_emulators start
and now currently have COCKROACHDB_EMULATOR_HOST set. Even though we call it an "emulator",
this sets up a real version of cockroachdb.
`, out)

	connectionString := fmt.Sprintf("postgresql://root@%s/%s?sslmode=disable", port, dbName)
	conn, err := pgxpool.Connect(ctx, connectionString)
	require.NoError(t, err)
	t.Cleanup(func() {
		conn.Close()
	})
	return conn
}

// SQLExporter is an abstraction around a type that can be written as a single row in a SQL table.
type SQLExporter interface {
	// ToSQLRow returns the column names and the column data that should be written for this row.
	ToSQLRow() (colNames []string, colData []interface{})
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
	// Go through each element of the table slice, cast it to ToSQLRow and then call that
	// function on it to get the arguments needed for that row.
	for j := 0; j < table.Len(); j++ {
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

	vp := sql.ValuesPlaceholders(len(colNames), table.Len())
	insert := fmt.Sprintf(`INSERT INTO %s (%s) VALUES %s`, name, strings.Join(colNames, ","), vp)

	_, err := db.Exec(ctx, insert, arguments...)
	return skerr.Wrapf(err, "Inserting %d rows into table %s", table.Len(), name)
}
