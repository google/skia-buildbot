package now

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNow_ConstValue_Success(t *testing.T) {

	var mockTime = time.Unix(12, 11).UTC()
	backgroundCtx := context.Background()
	ctx := context.WithValue(backgroundCtx, ContextKey, mockTime)

	require.NotEqual(t, mockTime, Now(backgroundCtx))
	require.Equal(t, mockTime, Now(ctx))
}

func TestNow_NowProvider_Success(t *testing.T) {

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

	backgroundCtx := context.Background()
	ctx := context.WithValue(backgroundCtx, ContextKey, "strings are not valid types for ContextKey")

	require.Panics(t, func() {
		Now(ctx)
	})
}

func TestTimeTravelingContext_SetTime_ChangesWhenNowIs(t *testing.T) {

	firstTime := time.Date(2021, time.September, 1, 10, 0, 0, 0, time.UTC)
	secondTime := time.Date(2021, time.September, 1, 10, 1, 0, 0, time.UTC)
	thirdTime := time.Date(2021, time.September, 1, 10, 1, 5, 0, time.UTC)

	ctx := TimeTravelingContext(firstTime)

	assert.Equal(t, firstTime, Now(ctx))
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, firstTime, Now(ctx)) // Not impacted by wall clock

	ctx.SetTime(secondTime)

	assert.Equal(t, secondTime, Now(ctx))
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, secondTime, Now(ctx)) // Not impacted by wall clock

	ctx.SetTime(thirdTime)

	assert.Equal(t, thirdTime, Now(ctx))
}

func TestTimeTravelingContext_WithContext_AllowsWrappingContext(t *testing.T) {

	firstTime := time.Date(2021, time.September, 1, 10, 0, 0, 0, time.UTC)
	secondTime := time.Date(2021, time.August, 20, 4, 0, 0, 0, time.UTC)

	baseCtx := context.WithValue(context.Background(), "foo", "bar")

	ctx := TimeTravelingContext(firstTime).WithContext(baseCtx)

	assert.Equal(t, firstTime, Now(ctx))
	ctx.SetTime(secondTime)
	assert.Equal(t, secondTime, Now(ctx))

	assert.Equal(t, "bar", ctx.Value("foo"))
}
