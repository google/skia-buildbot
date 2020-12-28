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
)

// MakeLocalCockroachDBForTesting starts up an instance of cockroachdb. It returns the postgresql
// connection url and a function to run to delete all
// data from the db and stop the cockroach instance.
func MakeLocalCockroachDBForTesting(t *testing.T, cleanup bool) string {
	unittest.LinuxOnlyTest(t)
	_, err := startLocalCockroachDB("26257", "8080")
	require.NoError(t, err)

	out, err := exec.Command("cockroach", "sql", "--insecure", "--host=localhost:26257",
		`--execute=CREATE DATABASE IF NOT EXISTS db_for_tests;`).CombinedOutput()
	require.NoError(t, err, "creating test database: %s", out)

	if cleanup {
		t.Cleanup(func() {
			_ = exec.Command("killall", "-9", "cockroach").Run()
		})
	}
	return "postgresql://root@localhost:26257/db_for_tests?sslmode=disable"
}

// startLocalCockroachDB kills any currently running cockroach instances and then starts a new
// one with the default ports. It will use a temp directory as a data store, which is returned as
// the first return value.
func startLocalCockroachDB(sqlPort, httpPort string) (string, error) {
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
		"start-single-node", "--insecure", "--listen-addr=localhost:"+sqlPort,
		"--http-addr=localhost:"+httpPort, "--background",
		"--store="+tmpDir,
	).Start()

	if err != nil {
		return "", skerr.Wrapf(err, "starting local cockroach")
	}

	// Wait for DB to come up.
	time.Sleep(3 * time.Second)
	return tmpDir, nil
}
