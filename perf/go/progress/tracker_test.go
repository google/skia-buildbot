package progress

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/now"
)

var testDate = time.Date(2020, 04, 01, 0, 0, 0, 0, time.UTC)

// setup creates and returns a new Tracker with a new Progress added to it.
func setup(t *testing.T) (*tracker, *progress) {
	tr, err := NewTracker("/foo/")
	require.NoError(t, err)
	p := New()
	tr.Add(p)

	return tr, p
}

func TestTracker_NewWithInvalidPath_ReturnsError(t *testing.T) {

	_, err := NewTracker("/does/not/end/with/slash")
	require.Error(t, err)
}

func TestTracker_Add_ProgressAppearsInCacheAndMetrics(t *testing.T) {

	tr, _ := setup(t)
	assert.Equal(t, 1, tr.cache.Len())
	assert.Equal(t, int64(0), tr.numEntriesInCache.Get())

	tr.singleStep(context.Background())
	assert.Equal(t, int64(1), tr.numEntriesInCache.Get())
}

func TestTracker_ProgressIsFinished_ProgressStillAppearsInCacheAndMetrics(t *testing.T) {

	tr, p := setup(t)
	p.Finished()

	tr.singleStep(context.Background())

	// Still there because it hasn't passed the expiration date.
	assert.Equal(t, 1, tr.cache.Len())
	assert.Equal(t, int64(1), tr.numEntriesInCache.Get())
}

func TestTracker_TimeAdvancesPastExpirationOfFinishedProgress_ProgressNoLongerAppearsInCacheAndMetrics(t *testing.T) {

	tr, p := setup(t)
	p.Finished()

	ctx := context.Background()
	ctx = context.WithValue(ctx, now.ContextKey, testDate)
	tr.singleStep(ctx)

	// This pass will evict the Progress from the cache.
	ctx = context.WithValue(ctx, now.ContextKey, testDate.Add(2*cacheDuration))
	tr.singleStep(ctx)

	assert.Equal(t, 0, tr.cache.Len())
	assert.Equal(t, int64(0), tr.numEntriesInCache.Get())
}

func TestTracker_RequestForNonExistentProgress_HandlerReturns404(t *testing.T) {

	tr, err := NewTracker("/foo/")
	require.NoError(t, err)

	r := httptest.NewRequest("GET", "/foo/123", nil)
	w := httptest.NewRecorder()
	tr.Handler(w, r)
	assert.Equal(t, 404, w.Result().StatusCode)
}

func TestTracker_RequestForProgress_ReturnsSerializedProgress(t *testing.T) {

	tr, err := NewTracker("/foo/")
	require.NoError(t, err)
	p := New()
	tr.Add(p)

	r := httptest.NewRequest("GET", p.state.URL, nil)
	w := httptest.NewRecorder()
	tr.Handler(w, r)
	assert.Equal(t, 200, w.Result().StatusCode)
	assert.Equal(t, "application/json", w.Result().Header.Get("Content-Type"))
	var expectedBody bytes.Buffer
	require.NoError(t, p.JSON(&expectedBody))
	actualBody, err := ioutil.ReadAll(w.Result().Body)
	require.NoError(t, err)
	assert.Equal(t, expectedBody.String(), string(actualBody))
}

func TestTracker_RequestForProgressWithUnSerializableResult_ReturnsError(t *testing.T) {

	tr, err := NewTracker("/foo/")
	require.NoError(t, err)
	p := New()
	p.Results(make(chan int)) // Not JSON serializable.
	tr.Add(p)

	r := httptest.NewRequest("GET", p.state.URL, nil)
	w := httptest.NewRecorder()
	tr.Handler(w, r)
	assert.Equal(t, 500, w.Result().StatusCode)
	actualBody, err := ioutil.ReadAll(w.Result().Body)
	assert.Contains(t, string(actualBody), "Failed to serialize JSON")
}
