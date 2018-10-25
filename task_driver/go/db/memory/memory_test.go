package memory

import (
	"path"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_driver/go/db"
)

func setup(t *testing.T) (db.DB, func()) {
	testutils.SmallTest(t)
	wd, cleanup := testutils.TempDir(t)
	d, err := NewInMemoryDB(path.Join(wd, "db.gob"))
	assert.NoError(t, err)
	return d, cleanup
}

func TestMemoryDB(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestDB(t, d)
}

func TestMemoryDBMessageOrdering(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestMessageOrdering(t, d)
}
