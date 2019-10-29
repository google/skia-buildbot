package ds_ignorestore

import (
	"context"
	"testing"
	"time"

	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/golden/go/ignore"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ds"
	ds_testutil "go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestDatastoreIgnoreStore(t *testing.T) {
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
	require.NoError(t, store.Create(context.Background(), r1))
	require.NoError(t, store.Create(context.Background(), r2))
	require.NoError(t, store.Create(context.Background(), r3))
	require.NoError(t, store.Create(context.Background(), r4))

	allRules, err := store.List(context.Background())
	require.NoError(t, err)
	require.Equal(t, 4, len(allRules))

	// Remove the third and fourth rule
	delCount, err := store.Delete(context.Background(), r3.ID)
	require.NoError(t, err)
	require.Equal(t, 1, delCount)
	allRules, err = store.List(context.Background())
	require.NoError(t, err)
	require.Equal(t, 3, len(allRules))

	delCount, err = store.Delete(context.Background(), r4.ID)
	require.NoError(t, err)
	require.Equal(t, 1, delCount)
	allRules, err = store.List(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, len(allRules))

	for _, oneRule := range allRules {
		require.True(t, (oneRule.ID == r1.ID) || (oneRule.ID == r2.ID))
	}

	delCount, err = store.Delete(context.Background(), r1.ID)
	require.NoError(t, err)
	require.Equal(t, 1, delCount)
	allRules, err = store.List(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, len(allRules))
	require.Equal(t, r2.ID, allRules[0].ID)

	// Update a rule.
	updatedRule := *allRules[0]
	updatedRule.Note = "an updated rule"
	err = store.Update(context.Background(), updatedRule.ID, &updatedRule)
	require.NoError(t, err, "Update should succeed.")
	allRules, err = store.List(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, len(allRules))
	require.Equal(t, r2.ID, allRules[0].ID)
	require.Equal(t, "an updated rule", allRules[0].Note)

	// Try to update a non-existent rule.
	updatedRule = *allRules[0]
	err = store.Update(context.Background(), "100001", &updatedRule)
	require.Error(t, err, "Update should fail for a bad id.")

	delCount, err = store.Delete(context.Background(), r2.ID)
	require.NoError(t, err)
	require.Equal(t, 1, delCount)

	allRules, err = store.List(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, len(allRules))

	// This id doesn't exist, so we shouldn't be able to delete it.
	delCount, err = store.Delete(context.Background(), "1000000")
	require.NoError(t, err)
	require.Equal(t, delCount, 0)
	allRules, err = store.List(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, len(allRules))
}
