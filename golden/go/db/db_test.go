package db

import (
	"testing"

	"go.skia.org/infra/go/database/testutil"
)

func TestMySQLVersioning(t *testing.T) {
	testutil.MySQLVersioningTests(t, "correctness", migrationSteps)
}
