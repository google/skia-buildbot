package sqltest

import (
	"io/ioutil"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/sql"
)

// MakeLocalCockroachDBForTesting starts up an instance of cockroachdb. It returns the postgresql
// connection url and a function to run to delete all
// data from the db and stop the cockroach instance.
func MakeLocalCockroachDBForTesting(t *testing.T) (string, func()) {
	unittest.LinuxOnlyTest(t)
	_, err := StartLocalCockroachDB()
	require.NoError(t, err)

	out, err := exec.Command("cockroach", "sql", "--insecure", "--host=localhost:26257",
		`--execute=CREATE DATABASE IF NOT EXISTS db_for_tests;`).CombinedOutput()
	require.NoError(t, err, "creating test database: %s", out)

	return "postgresql://root@localhost:26257/db_for_tests?sslmode=disable", func() {
		_ = exec.Command("killall", "-9", "cockroach").Run()
	}
}

// StartLocalCockroachDB kills any currently running cockroach instances and then starts a new
// one with the default ports. It will use a temp directory as a data store, which is returned as
// the first return value.
func StartLocalCockroachDB() (string, error) {
	err := exec.Command("killall", "-9", "cockroach").Run()
	if err != nil {
		sklog.Warning("Attempted to stop previous cockroach instances failed. Probably were none.")
	}

	out, err := exec.Command("cockroach", "version").CombinedOutput()
	if err != nil {
		return "", skerr.Wrapf(err, "Do you have 'cockroach' on your path? %s", out)
	}

	tmpDir, err := ioutil.TempDir("", "cockroach-db")
	if err != nil {
		return "", skerr.Wrapf(err, "making tempdir")
	}

	err = exec.Command("cockroach",
		"start-single-node", "--insecure", "--listen-addr=localhost:26257",
		"--http-addr=localhost:8080", "--background",
		"--store="+tmpDir,
	).Start()

	if err != nil {
		return "", skerr.Wrapf(err, "starting local cockroach")
	}

	// Wait for DB to come up.
	time.Sleep(3 * time.Second)
	return tmpDir, nil
}

// RemoveOldDataAndResetSchema resets the singleton test database to a clean state, deleting all
// data that was there before.
func RemoveOldDataAndResetSchema(t *testing.T) {
	// Dropping database each time doesn't work, as sometimes something locks up occasionally
	// Instead we just delete all the rows from each table.

	out, err := exec.Command("cockroach", "sql", "--insecure", "--host=localhost:26257",
		"--database=db_for_tests", // Connect to db_for_tests that we just made
		"--execute="+sql.CockroachDBSchema,
	).CombinedOutput()
	require.NoError(t, err, "creating tables in test database: %s", out)

	out, err = exec.Command("cockroach", "sql", "--insecure", "--host=localhost:26257",
		"--database=db_for_tests", // Connect to db_for_tests that we just made
		`--execute=DELETE FROM TraceValues RETURNING NOTHING;
DELETE FROM TryJobValues RETURNING NOTHING;
DELETE FROM Commits RETURNING NOTHING;
DELETE FROM KeyValueMaps RETURNING NOTHING;
DELETE FROM SourceFiles RETURNING NOTHING;
DELETE FROM Expectations RETURNING NOTHING;
DELETE FROM CLExpectations RETURNING NOTHING;
DELETE FROM ExpectationsDeltas RETURNING NOTHING;
DELETE FROM ExpectationsRecords RETURNING NOTHING;
DELETE FROM DiffMetrics RETURNING NOTHING;
`).CombinedOutput()
	require.NoError(t, err, "creating test database: %s", out)
}
