package db

import (
	"testing"

	"skia.googlesource.com/buildbot.git/go/database/testutil"
)

func TestSQLiteVersioning(t *testing.T) {
	testutil.SQLiteVersioningTests(t, migrationSteps)
}

func TestMySQLVersioning(t *testing.T) {
	testutil.MySQLVersioningTests(t, "skia", migrationSteps)
}
