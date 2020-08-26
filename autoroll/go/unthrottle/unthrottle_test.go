package unthrottle

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestUnthrottle(t *testing.T) {
	unittest.LargeTest(t)

	r := "fake-roller"
	ctx := context.Background()
	testutil.InitDatastore(t, ds.KIND_AUTOROLL_UNTHROTTLE)

	db := NewDatastore(ctx)

	check := func(expect bool) {
		actual, err := db.Get(ctx, r)
		require.NoError(t, err)
		require.Equal(t, expect, actual)
	}

	// No entry exists; ensure that we return false and no error.
	check(false)

	// Unthrottle the roller.
	require.NoError(t, db.Unthrottle(ctx, r))
	check(true)

	// Reset.
	require.NoError(t, db.Reset(ctx, r))
	check(false)
}
