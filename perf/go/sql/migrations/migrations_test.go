package migrations

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

const cockroachMigrations = "../../../migrations/cockroachdb"

const cockroachDBTest = "cockroach://root@localhost:26257?sslmode=disable"

func TestUpDownCockroachDB(t *testing.T) {
	unittest.MediumTest(t)

	err := Up(cockroachMigrations, cockroachDBTest)
	assert.NoError(t, err)
	err = Down(cockroachMigrations, cockroachDBTest)
	assert.NoError(t, err)
}
