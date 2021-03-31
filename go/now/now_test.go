package now

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNow_ConstValue_Success(t *testing.T) {
	unittest.SmallTest(t)

	var mockTime = time.Unix(12, 11).UTC()
	backgroundCtx := context.Background()
	ctx := context.WithValue(backgroundCtx, ContextKey, mockTime)

	require.NotEqual(t, mockTime, Now(backgroundCtx))
	require.Equal(t, mockTime, Now(ctx))
}

func TestNow_NowProvider_Success(t *testing.T) {
	unittest.SmallTest(t)

	var monotonicTime int64 = 0
	var mockTimeProvider = func() time.Time {
		monotonicTime += 1
		return time.Unix(monotonicTime, 0).UTC()
	}
	backgroundCtx := context.Background()
	ctx := context.WithValue(backgroundCtx, ContextKey, NowProvider(mockTimeProvider))

	// Calling with ctx makes repeated calls to mocktimeProvider.
	require.Equal(t, int64(1), Now(ctx).Unix())
	require.Equal(t, int64(2), Now(ctx).Unix())
	require.Equal(t, int64(2), monotonicTime)

	// Calling with backgroundCtx returns the real time.
	require.NotEqual(t, int64(2), Now(backgroundCtx))

	// Assert that mockTimeProvider was not called.
	require.Equal(t, int64(2), monotonicTime)

}

func TestNow_InvalidValue_Panics(t *testing.T) {
	unittest.SmallTest(t)

	backgroundCtx := context.Background()
	ctx := context.WithValue(backgroundCtx, ContextKey, "strings are not valid types for ContextKey")

	require.Panics(t, func() {
		Now(ctx)
	})
}
