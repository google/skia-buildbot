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
	f := New(ctx, c)

	xir := makeIgnoreRules()
	// Add them in a not-sorted order to make sure List sorts them.
	require.NoError(t, f.Create(ctx, xir[2]))
	require.NoError(t, f.Create(ctx, xir[0]))
	require.NoError(t, f.Create(ctx, xir[3]))
	require.NoError(t, f.Create(ctx, xir[1]))

	// List uses a query snapshot, which is not synchronous, so we might have to query a few times
	// before everything syncs up.
	assert.Eventually(t, func() bool {
		actualRules, err := f.List(ctx)
		assert.NoError(t, err)
		return compareIgnoreRulesIgnoringIDs(actualRules, makeIgnoreRules())
	}, 5*time.Second, 200*time.Millisecond)
}

var now = time.Date(2020, time.January, 13, 14, 15, 16, 0, time.UTC)

func makeIgnoreRules() []ignore.Rule {
	return []ignore.Rule{
		ignore.NewRule("alpha@example.com", now.Add(-time.Hour), "config=gpu", "expired"),
		ignore.NewRule("beta@example.com", now.Add(time.Minute*10), "config=8888", "No good reason."),
		ignore.NewRule("beta@example.com", now.Add(time.Minute*50), "extra=123&extra=abc", "Ignore multiple."),
		ignore.NewRule("alpha@example.com", now.Add(time.Minute*100), "extra=123&extra=abc&config=8888", "Ignore multiple 2."),
	}
}

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
