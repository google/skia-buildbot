package dsutil

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/datastore"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/ds"
	ds_testutil "go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
)

const TEST_ENTITY ds.Kind = "TestEntityNeverWritten"

var (
	nowMs = util.TimeStamp(time.Millisecond)

	k1 = TimeSortableKey(TEST_ENTITY, nowMs-100)
	k2 = TimeSortableKey(TEST_ENTITY, nowMs-80)
	k3 = TimeSortableKey(TEST_ENTITY, nowMs-60)
	k4 = TimeSortableKey(TEST_ENTITY, nowMs-40)
)

func TestRecently(t *testing.T) {
	unittest.MediumTest(t)

	evConsistentDeltaMs := int64(DefaultConsistencyDelta / time.Millisecond)

	container := &Recently{}
	container.update(k3, evConsistentDeltaMs, false)
	container.update(k2, evConsistentDeltaMs, false)
	container.update(k4, evConsistentDeltaMs, false)
	container.update(k1, evConsistentDeltaMs, false)

	expContainer := &Recently{
		Added: []*datastore.Key{k4, k3, k2, k1},
	}
	deepequal.AssertDeepEqual(t, expContainer, container)

	// Remove entries.
	container.update(k1, evConsistentDeltaMs, true)
	container.update(k2, evConsistentDeltaMs, true)
	expContainer.Added = []*datastore.Key{k4, k3}
	expContainer.Deleted = []*datastore.Key{k2, k1}
	deepequal.AssertDeepEqual(t, expContainer, container)

	// Re-add them.
	container.update(k2, evConsistentDeltaMs, false)
	container.update(k1, evConsistentDeltaMs, false)
	expContainer.Added = []*datastore.Key{k4, k3, k2, k1}
	expContainer.Deleted = []*datastore.Key{}
	deepequal.AssertDeepEqual(t, expContainer, container)

	evConsistentDeltaMs = 1000
	time.Sleep(1100 * time.Millisecond)
	k5 := TimeSortableKey(TEST_ENTITY, 0)
	container.update(k5, evConsistentDeltaMs, false)
	expContainer.Added = []*datastore.Key{k5}
	deepequal.AssertDeepEqual(t, expContainer, container)

	evConsistentDeltaMs = int64(DefaultConsistencyDelta / time.Millisecond)
	someRecentKeys := &Recently{}
	assert.True(t, someRecentKeys.update(k1, evConsistentDeltaMs, false))
	assert.True(t, someRecentKeys.update(k3, evConsistentDeltaMs, false))
	assert.True(t, someRecentKeys.update(k4, evConsistentDeltaMs, false))
	assert.False(t, someRecentKeys.update(k1, evConsistentDeltaMs, false))
	assert.False(t, someRecentKeys.update(k1, evConsistentDeltaMs, false))
	assert.False(t, someRecentKeys.update(k1, evConsistentDeltaMs, false))
	someQueryResult := []*datastore.Key{k2, k5, k3, k4}

	// Make sure the union of recent keys and query result is correct
	expCombined := []*datastore.Key{k5, k4, k3, k2, k1}
	deepequal.AssertDeepEqual(t, expCombined, someRecentKeys.Combine(someQueryResult))

	// Make sure we get the recent key changes if the query result is empty
	expCombined = []*datastore.Key{k4, k3, k1}
	deepequal.AssertDeepEqual(t, expCombined, someRecentKeys.Combine(nil))

	// Make sure we get the query result when the recent keys are empty
	expCombined = []*datastore.Key{k5, k4, k3, k2}
	deepequal.AssertDeepEqual(t, expCombined, (&Recently{}).Combine(someQueryResult))

	// Delete keys from recent keys but not from the query result.
	assert.True(t, someRecentKeys.update(k3, evConsistentDeltaMs, true))
	assert.True(t, someRecentKeys.update(k5, evConsistentDeltaMs, true))
	assert.True(t, someRecentKeys.update(k2, evConsistentDeltaMs, true))

	// Make sure recently deleted keys are filtered out from the query result.
	expCombined = []*datastore.Key{k4, k1}
	deepequal.AssertDeepEqual(t, expCombined, someRecentKeys.Combine(someQueryResult))
}

func TestRecentKeysList(t *testing.T) {
	unittest.LargeTest(t)

	// Run to the locally running emulator.
	cleanup := ds_testutil.InitDatastore(t,
		ds.HELPER_RECENT_KEYS)
	defer cleanup()

	client := ds.DS
	containerKey := ds.NewKey(ds.HELPER_RECENT_KEYS)
	containerKey.Name = "test-list-helper"
	recentKeysList := NewRecentKeysList(client, containerKey, DefaultConsistencyDelta)

	// Make sure we get an empty slice if we never call the helper.
	recentChanges, err := recentKeysList.GetRecent()
	assert.NoError(t, err)
	assert.NotNil(t, recentChanges)
	assert.Equal(t, 0, len(recentChanges.Added))

	_, err = client.RunInTransaction(context.Background(), func(tx *datastore.Transaction) error {
		assert.NoError(t, recentKeysList.Add(tx, k1))
		return nil
	})
	assert.NoError(t, err)

	recentChanges, err = recentKeysList.GetRecent()
	assert.NoError(t, err)
	assert.Equal(t, []*datastore.Key{k1}, recentChanges.Combine(nil))

	nonEmptyQuery := []*datastore.Key{k2, k3}
	assert.Equal(t, []*datastore.Key{k3, k2, k1}, recentChanges.Combine(nonEmptyQuery))

	// Delete k2 from the recent keys, but not from the query and make sure it's gone
	// from the result. This simulates eventual consistency when a key is removed.
	_, err = client.RunInTransaction(context.Background(), func(tx *datastore.Transaction) error {
		assert.NoError(t, recentKeysList.Delete(tx, k2))
		return nil
	})
	assert.NoError(t, err)

	recentChanges, err = recentKeysList.GetRecent()
	assert.NoError(t, err)
	assert.Equal(t, []*datastore.Key{k3, k1}, recentChanges.Combine(nonEmptyQuery))
}

func TestTimeBasedKeyID(t *testing.T) {
	unittest.SmallTest(t)

	ts := util.TimeStamp(time.Millisecond)
	keyID := getSortableTimeID(ts)
	assert.Equal(t, ts, GetTimeFromID(keyID))

	ts2 := ts + 2
	keyID2 := getSortableTimeID(ts2)
	assert.True(t, keyID2 < keyID)
	epochMs := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC).UnixNano() / int64(time.Millisecond)
	epochID := getSortableTimeID(epochMs)
	assert.Equal(t, int64(0), GetTimeFromID(epochID))
}
