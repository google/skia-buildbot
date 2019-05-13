package unthrottle

import (
	"context"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestUnthrottle(t *testing.T) {
	unittest.LargeTest(t)

	r := "fake-roller"
	ctx := context.Background()
	testutil.InitDatastore(t, ds.KIND_AUTOROLL_UNTHROTTLE)

	check := func(expect bool) {
		actual, err := Get(ctx, r)
		assert.NoError(t, err)
		assert.Equal(t, expect, actual)
	}

	// No entry exists; ensure that we return false and no error.
	check(false)

	// Unthrottle the roller.
	assert.NoError(t, Unthrottle(ctx, r))
	check(true)

	// Reset.
	assert.NoError(t, Reset(ctx, r))
	check(false)
}
