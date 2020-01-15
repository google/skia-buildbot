package fs_ignorestore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/ignore"
)

func TestCreateListIgnoreRule(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	ctx := context.Background()
	f := newEmptyStore(ctx, t, c)

	xir := makeIgnoreRules()
	// Add them in a not-sorted order to make sure List sorts them.
	require.NoError(t, f.Create(ctx, xir[2]))
	require.NoError(t, f.Create(ctx, xir[0]))
	require.NoError(t, f.Create(ctx, xir[3]))
	require.NoError(t, f.Create(ctx, xir[1]))

	requireCurrentListMatchesExpected(t, ctx, f)
}

func TestCreateDelete(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	ctx := context.Background()
	f := newEmptyStore(ctx, t, c)

	xir := makeIgnoreRules()
	// Add 0, 1, 2, 2, 2, 2, 3 (there are 3 extra of index 2)
	require.NoError(t, f.Create(ctx, xir[0]))
	require.NoError(t, f.Create(ctx, xir[1]))
	require.NoError(t, f.Create(ctx, xir[2]))
	require.NoError(t, f.Create(ctx, xir[2]))
	require.NoError(t, f.Create(ctx, xir[2]))
	require.NoError(t, f.Create(ctx, xir[2]))
	require.NoError(t, f.Create(ctx, xir[3]))
	// Wait until those 7 rules are in the list
	require.Eventually(t, func() bool {
		actualRules, _ := f.List(ctx)
		return len(actualRules) == 7
	}, 5*time.Second, 200*time.Millisecond)
	// Re-query the rules to make sure none got dropped or added unexpectedly.
	actualRules, err := f.List(ctx)
	require.NoError(t, err)
	require.Len(t, actualRules, 7) // should still have 7 elements in the list

	ok, err := f.Delete(ctx, actualRules[3].ID)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = f.Delete(ctx, actualRules[4].ID)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = f.Delete(ctx, actualRules[5].ID)
	require.NoError(t, err)
	assert.True(t, ok)

	requireCurrentListMatchesExpected(t, ctx, f)
}

func TestDeleteNonExistentRule(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	ctx := context.Background()
	f := New(ctx, c)
	ok, err := f.Delete(ctx, "Not in there")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestDeleteEmptyRule(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	ctx := context.Background()
	f := New(ctx, c)
	_, err := f.Delete(ctx, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestCreateUpdate(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	ctx := context.Background()
	f := newEmptyStore(ctx, t, c)

	xir := makeIgnoreRules()
	require.NoError(t, f.Create(ctx, xir[1]))
	// Wait until that rule is in the list
	require.Eventually(t, func() bool {
		actualRules, _ := f.List(ctx)
		return len(actualRules) == 1
	}, 5*time.Second, 200*time.Millisecond)

	actualRules, err := f.List(ctx)
	require.NoError(t, err)
	toUpdateID := actualRules[0].ID
	newContent := xir[0]
	newContent.ID = toUpdateID

	require.NoError(t, f.Update(ctx, newContent))
	require.Eventually(t, func() bool {
		rules, err := f.List(ctx)
		assert.NoError(t, err)
		return compareIgnoreRulesIgnoringIDs(rules, []ignore.Rule{
			{
				// Notice the Name is the same as the original rule, but everything else is changed.
				CreatedBy: "beta@example.com",
				UpdatedBy: "alpha@example.com",
				Expires:   now.Add(-time.Hour),
				Query:     "config=gpu",
				Note:      "expired",
			},
		})
	}, 5*time.Second, 200*time.Millisecond)
}

func TestUpdateNonExistentRule(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	ctx := context.Background()
	f := New(ctx, c)
	ir := makeIgnoreRules()[0]
	ir.ID = "whoops"
	err := f.Update(ctx, ir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "before updating")
}

func TestUpdateEmptyRule(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	ctx := context.Background()
	f := New(ctx, c)

	err := f.Update(ctx, ignore.Rule{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

var now = time.Date(2020, time.January, 13, 14, 15, 16, 0, time.UTC)

func newEmptyStore(ctx context.Context, t *testing.T, c *firestore.Client) *StoreImpl {
	f := New(ctx, c)
	empty, err := f.List(ctx)
	require.NoError(t, err)
	require.Empty(t, empty)
	return f
}

func makeIgnoreRules() []ignore.Rule {
	return []ignore.Rule{
		ignore.NewRule("alpha@example.com", now.Add(-time.Hour), "config=gpu", "expired"),
		ignore.NewRule("beta@example.com", now.Add(time.Minute*10), "config=8888", "No good reason."),
		ignore.NewRule("beta@example.com", now.Add(time.Minute*50), "extra=123&extra=abc", "Ignore multiple."),
		ignore.NewRule("alpha@example.com", now.Add(time.Minute*100), "extra=123&extra=abc&config=8888", "Ignore multiple 2."),
	}
}

// compareIgnoreRulesIgnoringIDs returns true if the two lists of rules match (disregarding the ID).
// ID is ignored because it is nondeterministic.
func compareIgnoreRulesIgnoringIDs(first []ignore.Rule, second []ignore.Rule) bool {
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
// matches makeIgnoreRules() or it panics because it does not match.
func requireCurrentListMatchesExpected(t *testing.T, ctx context.Context, f *StoreImpl) {
	// List uses a query snapshot, which is not synchronous, so we might have to query a few times
	// before everything syncs up.
	require.Eventually(t, func() bool {
		actualRules, err := f.List(ctx)
		assert.NoError(t, err)
		return compareIgnoreRulesIgnoringIDs(actualRules, makeIgnoreRules())
	}, 5*time.Second, 200*time.Millisecond)
}
