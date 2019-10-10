package ds_ignorestore

import (
	"testing"
	"time"

	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/golden/go/ignore"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ds"
	ds_testutil "go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestCloudIgnoreStore(t *testing.T) {
	unittest.LargeTest(t)

	// Run against the locally running emulator.
	cleanup := ds_testutil.InitDatastore(t, ds.IGNORE_RULE, ds.HELPER_RECENT_KEYS)
	defer cleanup()

	store, err := New(ds.DS)
	require.NoError(t, err)
	ignoreStoreAll(t, store)
}

func ignoreStoreAll(t sktest.TestingT, store ignore.Store) {
	// Add a few instances.
	r1 := ignore.NewRule("jon@example.com", time.Now().Add(time.Hour), "config=gpu", "reason")
	r2 := ignore.NewRule("jim@example.com", time.Now().Add(time.Minute*10), "config=8888", "No good reason.")
	r3 := ignore.NewRule("jon@example.com", time.Now().Add(time.Minute*50), "extra=123&extra=abc", "Ignore multiple.")
	r4 := ignore.NewRule("jon@example.com", time.Now().Add(time.Minute*100), "extra=123&extra=abc&config=8888", "Ignore multiple.")
	require.Equal(t, int64(0), store.Revision())
	require.NoError(t, store.Create(r1))
	require.NoError(t, store.Create(r2))
	require.NoError(t, store.Create(r3))
	require.NoError(t, store.Create(r4))
	require.Equal(t, int64(4), store.Revision())

	allRules, err := store.List()
	require.NoError(t, err)
	require.Equal(t, 4, len(allRules))
	require.Equal(t, int64(4), store.Revision())

	// Test the rule matcher
	matcher, err := store.BuildRuleMatcher()
	require.NoError(t, err)
	found, ok := matcher(map[string]string{"config": "565"})
	require.False(t, ok)
	require.Empty(t, found)
	found, ok = matcher(map[string]string{"config": "8888"})
	require.True(t, ok)
	require.Equal(t, 1, len(found))
	found, ok = matcher(map[string]string{"extra": "123"})
	require.True(t, ok)
	require.Equal(t, 1, len(found))
	found, ok = matcher(map[string]string{"extra": "abc"})
	require.True(t, ok)
	require.Equal(t, 1, len(found))
	found, ok = matcher(map[string]string{"extra": "abc", "config": "8888"})
	require.True(t, ok)
	require.Equal(t, 3, len(found))
	found, ok = matcher(map[string]string{"extra": "abc", "config": "gpu"})
	require.True(t, ok)
	require.Equal(t, 2, len(found))
	require.Equal(t, int64(4), store.Revision())

	// Remove the third and fourth rule
	delCount, err := store.Delete(r3.ID)
	require.NoError(t, err)
	require.Equal(t, 1, delCount)
	allRules, err = store.List()
	require.NoError(t, err)
	require.Equal(t, 3, len(allRules))

	delCount, err = store.Delete(r4.ID)
	require.NoError(t, err)
	require.Equal(t, 1, delCount)
	allRules, err = store.List()
	require.NoError(t, err)
	require.Equal(t, 2, len(allRules))
	require.Equal(t, int64(6), store.Revision())

	for _, oneRule := range allRules {
		require.True(t, (oneRule.ID == r1.ID) || (oneRule.ID == r2.ID))
	}

	delCount, err = store.Delete(r1.ID)
	require.NoError(t, err)
	require.Equal(t, 1, delCount)
	allRules, err = store.List()
	require.NoError(t, err)
	require.Equal(t, 1, len(allRules))
	require.Equal(t, r2.ID, allRules[0].ID)
	require.Equal(t, int64(7), store.Revision())

	// Update a rule.
	updatedRule := *allRules[0]
	updatedRule.Note = "an updated rule"
	err = store.Update(updatedRule.ID, &updatedRule)
	require.NoError(t, err, "Update should succeed.")
	allRules, err = store.List()
	require.NoError(t, err)
	require.Equal(t, 1, len(allRules))
	require.Equal(t, r2.ID, allRules[0].ID)
	require.Equal(t, "an updated rule", allRules[0].Note)
	require.Equal(t, int64(8), store.Revision())

	// Try to update a non-existent rule.
	updatedRule = *allRules[0]
	err = store.Update(100001, &updatedRule)
	require.Error(t, err, "Update should fail for a bad id.")
	require.Equal(t, int64(8), store.Revision())

	delCount, err = store.Delete(r2.ID)
	require.NoError(t, err)
	require.Equal(t, 1, delCount)

	allRules, err = store.List()
	require.NoError(t, err)
	require.Equal(t, 0, len(allRules))
	require.Equal(t, int64(9), store.Revision())

	// This id doesn't exist, so we shouldn't be able to delete it.
	delCount, err = store.Delete(1000000)
	require.NoError(t, err)
	require.Equal(t, delCount, 0)
	allRules, err = store.List()
	require.NoError(t, err)
	require.Equal(t, 0, len(allRules))
	require.Equal(t, int64(9), store.Revision())
}
