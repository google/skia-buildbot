package db

import (
	"testing"

	"go.skia.org/infra/go/database/testutil"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestMySQLVersioning(t *testing.T) {
	unittest.LargeTest(t)
	testutil.MySQLVersioningTests(t, "correctness", migrationSteps)
}
