package memory

import (
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_driver/go/db"
	"go.skia.org/infra/task_driver/go/db/shared_tests"
)

func setup(t *testing.T) db.DB {
	// Medium because we use the disk, and the test downloads from GCS.
	unittest.LargeTest(t)
	wd := t.TempDir()
	d, err := NewInMemoryDB(path.Join(wd, "db.gob"))
	require.NoError(t, err)
	return d
}

func TestMemoryDB(t *testing.T) {
	d := setup(t)
	shared_tests.TestDB(t, d)
}

func TestMemoryDBMessageOrdering(t *testing.T) {
	d := setup(t)
	shared_tests.TestMessageOrdering(t, d)
}
