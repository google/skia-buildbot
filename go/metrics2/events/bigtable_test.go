package events

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	bt_testutil "go.skia.org/infra/go/bt/testutil"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestInsertRetrieveBT(t *testing.T) {
	unittest.LargeTest(t)

	project, instance, cleanup := bt_testutil.SetupBigTable(t, BT_TABLE, BT_COLUMN_FAMILY)
	defer cleanup()
	d, err := NewBTEventDB(context.Background(), project, instance, nil)
	require.NoError(t, err)
	testInsertRetrieve(t, d)
}
