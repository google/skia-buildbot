package fscommentstore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/comment/trace"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
)

func TestCreateListTraceComment(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := newEmptyStore(ctx, t, c)

	xtc := makeTraceComments()
	// Add them in a not-sorted order to make sure ListComments sorts them.
	createAndRequireNoError(ctx, t, f, xtc[2])
	createAndRequireNoError(ctx, t, f, xtc[0])
	createAndRequireNoError(ctx, t, f, xtc[3])
	createAndRequireNoError(ctx, t, f, xtc[1])

	requireCurrentListMatchesExpected(t, ctx, f)
}

func TestCreateDeleteTraceComment(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := newEmptyStore(ctx, t, c)

	xtc := makeTraceComments()
	// Add 0, 1, 2, 2, 2, 2, 3 (there are 3 extra of index 2)
	createAndRequireNoError(ctx, t, f, xtc[0])
	createAndRequireNoError(ctx, t, f, xtc[1])
	createAndRequireNoError(ctx, t, f, xtc[2])
	createAndRequireNoError(ctx, t, f, xtc[2])
	createAndRequireNoError(ctx, t, f, xtc[2])
	createAndRequireNoError(ctx, t, f, xtc[2])
	createAndRequireNoError(ctx, t, f, xtc[3])
	// Wait until those 7 comments are in the list
	require.Eventually(t, func() bool {
		actualComments, _ := f.ListComments(ctx)
		return len(actualComments) == 7
	}, 5*time.Second, 200*time.Millisecond)
	// Re-query the comments to make sure none got dropped or added unexpectedly.
	actualComments, err := f.ListComments(ctx)
	require.NoError(t, err)
	require.Len(t, actualComments, 7) // should still have 7 elements in the list

	err = f.DeleteComment(ctx, actualComments[3].ID)
	require.NoError(t, err)
	err = f.DeleteComment(ctx, actualComments[4].ID)
	require.NoError(t, err)
	err = f.DeleteComment(ctx, actualComments[5].ID)
	require.NoError(t, err)

	requireCurrentListMatchesExpected(t, ctx, f)
}

func TestDeleteNonExistentComment(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := New(ctx, c)
	err := f.DeleteComment(ctx, "Not in there")
	require.NoError(t, err)
}

func TestDeleteEmptyComment(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := New(ctx, c)
	err := f.DeleteComment(ctx, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestCreateUpdate(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := newEmptyStore(ctx, t, c)

	xtc := makeTraceComments()
	createAndRequireNoError(ctx, t, f, xtc[0])
	// Wait until that comment is in the list
	require.Eventually(t, func() bool {
		actualComments, _ := f.ListComments(ctx)
		return len(actualComments) == 1
	}, 5*time.Second, 200*time.Millisecond)

	actualComments, err := f.ListComments(ctx)
	require.NoError(t, err)
	toUpdateID := actualComments[0].ID
	newContent := xtc[3]
	newContent.ID = toUpdateID

	require.NoError(t, f.UpdateComment(ctx, newContent))
	require.Eventually(t, func() bool {
		comments, err := f.ListComments(ctx)
		assert.NoError(t, err)
		return compareTraceCommentsIgnoringIDs(comments, []trace.Comment{
			{
				// These fields are from the original xtc[0]
				CreatedBy: "zulu@example.com",
				CreatedTS: time.Date(2020, time.February, 19, 18, 17, 16, 0, time.UTC),
				// These fields were overwritten from xtc[3]
				UpdatedBy: "uniform@example.com",
				UpdatedTS: time.Date(2020, time.February, 26, 26, 26, 26, 0, time.UTC),
				Comment:   "On Wednesdays, this device draws pink",
				QueryToMatch: paramtools.ParamSet{
					"device": []string{"This device does not exist"},
				},
			},
		})
	}, 5*time.Second, 200*time.Millisecond)
}

func TestUpdateNonExistentTraceComment(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := New(ctx, c)
	tc := makeTraceComments()[0]
	tc.ID = "whoops"
	err := f.UpdateComment(ctx, tc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "before updating")
}

func TestUpdateEmptyTraceComment(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := New(ctx, c)

	err := f.UpdateComment(ctx, trace.Comment{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func createAndRequireNoError(ctx context.Context, t *testing.T, f *StoreImpl, comment trace.Comment) {
	_, err := f.CreateComment(ctx, comment)
	require.NoError(t, err)
}

func newEmptyStore(ctx context.Context, t *testing.T, c *firestore.Client) *StoreImpl {
	f := New(ctx, c)
	empty, err := f.ListComments(ctx)
	require.NoError(t, err)
	require.Empty(t, empty)
	return f
}

// compareTraceCommentsIgnoringIDs returns true if the two lists of comments match (disregarding the
// ID). ID is ignored because it is nondeterministic.
func compareTraceCommentsIgnoringIDs(first []trace.Comment, second []trace.Comment) bool {
	if len(first) != len(second) {
		return false
	}
	for i := range first {
		r1, r2 := first[i], second[i]
		r1.ID = ""
		r2.ID = ""
		if !deepequal.DeepEqual(r1, r2) {
			return false
		}
	}
	return true
}

// requireCurrentListMatchesExpected either returns because the content in the given store
// matches makeTraceComments() or it panics because it does not match.
func requireCurrentListMatchesExpected(t *testing.T, ctx context.Context, f *StoreImpl) {
	// List uses a query snapshot, which is not synchronous, so we might have to query a few times
	// before everything syncs up.
	require.Eventually(t, func() bool {
		actualComments, err := f.ListComments(ctx)
		assert.NoError(t, err)
		return compareTraceCommentsIgnoringIDs(actualComments, makeTraceComments())
	}, 5*time.Second, 200*time.Millisecond)
}

// makeTraceComments returns 4 traces with arbitrary, but valid data. The QueryToMatch fields are
// normalized (i.e. the values are sorted) and the slice is sorted low to high by UpdatedTS.
func makeTraceComments() []trace.Comment {
	return []trace.Comment{
		{
			CreatedBy: "zulu@example.com",
			UpdatedBy: "zulu@example.com",
			CreatedTS: time.Date(2020, time.February, 19, 18, 17, 16, 0, time.UTC),
			UpdatedTS: time.Date(2020, time.February, 19, 18, 17, 16, 0, time.UTC),
			Comment:   "All bullhead devices draw upside down",
			QueryToMatch: paramtools.ParamSet{
				"device": []string{data.BullheadDevice},
			},
		},
		{
			CreatedBy: "yankee@example.com",
			UpdatedBy: "xray@example.com",
			CreatedTS: time.Date(2020, time.February, 2, 18, 17, 16, 0, time.UTC),
			UpdatedTS: time.Date(2020, time.February, 20, 18, 17, 16, 0, time.UTC),
			Comment:   "Watch pixel 0,4 to make sure it's not purple",
			QueryToMatch: paramtools.ParamSet{
				types.PRIMARY_KEY_FIELD: []string{string(data.AlphaTest)},
			},
		},
		{
			CreatedBy: "victor@example.com",
			UpdatedBy: "victor@example.com",
			CreatedTS: time.Date(2020, time.February, 22, 18, 17, 16, 0, time.UTC),
			UpdatedTS: time.Date(2020, time.February, 22, 18, 17, 16, 0, time.UTC),
			Comment:   "This test should be ABGR instead of RGBA on angler and bullhead due to hysterical raisins",
			QueryToMatch: paramtools.ParamSet{
				"device":                []string{data.AnglerDevice, data.BullheadDevice},
				types.PRIMARY_KEY_FIELD: []string{string(data.BetaTest)},
			},
		},
		{
			CreatedBy: "uniform@example.com",
			UpdatedBy: "uniform@example.com",
			CreatedTS: time.Date(2020, time.February, 26, 26, 26, 26, 0, time.UTC),
			UpdatedTS: time.Date(2020, time.February, 26, 26, 26, 26, 0, time.UTC),
			Comment:   "On Wednesdays, this device draws pink",
			QueryToMatch: paramtools.ParamSet{
				"device": []string{"This device does not exist"},
			},
		},
	}
}
