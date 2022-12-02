package bigtable

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	bt_testutil "go.skia.org/infra/go/bt/testutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_driver/go/db"
	"go.skia.org/infra/task_driver/go/db/shared_tests"
)

func setup(t *testing.T) (db.DB, func()) {
	project, instance, cleanup := bt_testutil.SetupBigTable(t, btTable, btColumnFamily)

	d, err := NewBigTableDB(context.Background(), project, instance, nil)
	require.NoError(t, err)
	return d, func() {
		testutils.AssertCloses(t, d)
		cleanup()
	}
}

func TestBigTableDB(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	shared_tests.TestDB(t, d)
}

func TestBigTableDBMessageOrdering(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	shared_tests.TestMessageOrdering(t, d)
}
