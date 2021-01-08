package fs_ingestionstore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
)

// TestGetExpectations writes some changes and then reads back the
// aggregated results.
func TestSetContains(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	c, cleanup := firestore.NewClientForTesting(ctx, t)
	defer cleanup()

	f := New(c)

	b, err := f.WasIngested(ctx, "nope", "not here")
	require.NoError(t, err)
	require.False(t, b)

	notUsedTS := time.Now()

	err = f.SetIngested(ctx, "skia-gold-flutter/dm-json-v1/2019/foo.json", "version1", notUsedTS)
	require.NoError(t, err)
	err = f.SetIngested(ctx, "skia-gold-flutter/dm-json-v1/2019/foo.json", "version2", notUsedTS)
	require.NoError(t, err)
	err = f.SetIngested(ctx, "skia-gold-flutter/dm-json-v1/2020/bar.json", "versionA", notUsedTS)
	require.NoError(t, err)

	b, err = f.WasIngested(ctx, "skia-gold-flutter/dm-json-v1/2019/foo.json", "version2")
	require.NoError(t, err)
	require.True(t, b)

	b, err = f.WasIngested(ctx, "skia-gold-flutter/dm-json-v1/2019/foo.json", "version1")
	require.NoError(t, err)
	require.True(t, b)

	b, err = f.WasIngested(ctx, "nope", "version1")
	require.NoError(t, err)
	require.False(t, b)

	b, err = f.WasIngested(ctx, "skia-gold-flutter/dm-json-v1/2019/foo.json", "versionA")
	require.NoError(t, err)
	require.False(t, b)
}
