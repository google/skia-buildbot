package bigtable

import (
	"context"
	"testing"

	assert "github.com/stretchr/testify/require"
	bt_testutil "go.skia.org/infra/go/bt/testutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_driver/go/db"
)

func setup(t *testing.T) (db.DB, func()) {
	unittest.LargeTest(t)
	project, instance, cleanup := bt_testutil.SetupBigTable(t, BT_TABLE, BT_COLUMN_FAMILY)

	d, err := NewBigTableDB(context.Background(), project, instance, nil)
	assert.NoError(t, err)
	return d, func() {
		testutils.AssertCloses(t, d)
		cleanup()
	}
}

func TestBigTableDB(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestDB(t, d)
}

func TestBigTableDBMessageOrdering(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestMessageOrdering(t, d)
}
