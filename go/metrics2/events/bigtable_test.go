package events

import (
	"context"
	"testing"

	assert "github.com/stretchr/testify/require"
	bt_testutil "go.skia.org/infra/go/bt/testutil"
	"go.skia.org/infra/go/testutils"
)

func TestInsertRetrieveBT(t *testing.T) {
	testutils.LargeTest(t)

	project, instance, cleanup := bt_testutil.SetupBigTable(t, BT_TABLE, BT_COLUMN_FAMILY)
	defer cleanup()
	d, err := NewBTEventDB(context.Background(), project, instance, nil)
	assert.NoError(t, err)
	testInsertRetrieve(t, d)
}
