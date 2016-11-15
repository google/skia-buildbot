package db

import (
	"testing"

	"go.skia.org/infra/go/database/testutil"
	"go.skia.org/infra/go/testutils"
)

func TestMySQLVersioning(t *testing.T) {
	testutils.LargeTest(t)
	testutil.MySQLVersioningTests(t, "correctness", migrationSteps)
}
