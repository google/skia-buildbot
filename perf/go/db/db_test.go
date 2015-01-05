package db

import (
	"testing"

	"skia.googlesource.com/buildbot.git/go/database/testutil"
)

func TestMySQLVersioning(t *testing.T) {
	testutil.MySQLVersioningTests(t, "skia", migrationSteps)
}
