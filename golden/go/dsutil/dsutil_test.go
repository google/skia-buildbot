package dsutil

import (
	"context"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/datastore"
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/ds"
	ds_testutil "go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils"
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

func TestKeyContainer(t *testing.T) {
	evConsistentDeltaMs := int64(DefaultConsistencyDelta / time.Millisecond)

	container := emptyKeyContainer()
	container.update(k3, evConsistentDeltaMs, false)
	container.update(k2, evConsistentDeltaMs, false)
	container.update(k4, evConsistentDeltaMs, false)
	container.update(k1, evConsistentDeltaMs, false)

	expContainer := &keyContainer{
		RecentChanges: []*datastore.Key{k4, k3, k2, k1},
	}
	assert.Equal(t, expContainer, container)

	// Remove entries.
	container.update(k1, evConsistentDeltaMs, true)
	container.update(k2, evConsistentDeltaMs, true)
	expContainer.RecentChanges = []*datastore.Key{k4, k3}
	assert.Equal(t, expContainer, container)

	// Re-add them.
	container.update(k2, evConsistentDeltaMs, false)
	container.update(k1, evConsistentDeltaMs, false)
	expContainer.RecentChanges = []*datastore.Key{k4, k3, k2, k1}
	assert.Equal(t, expContainer, container)

	evConsistentDeltaMs = 1000
	time.Sleep(1100 * time.Millisecond)
	k5 := TimeSortableKey(TEST_ENTITY, 0)
	container.update(k5, evConsistentDeltaMs, false)
	expContainer.RecentChanges = []*datastore.Key{k5}
	assert.Equal(t, expContainer, container)

	randomRecentKeys := []*datastore.Key{k1, k2, k5, k3, k4, k1, k1, k1}
	randomQueryResult := []*datastore.Key{k2, k5, k3, k4, k4}
	merged := KeySlice(randomRecentKeys).Merge(randomQueryResult)
	expMerged := []*datastore.Key{k5, k4, k3, k2, k1}
	assert.Equal(t, expMerged, merged)

	assert.Equal(t, expMerged, KeySlice(randomRecentKeys).Merge(nil))
	expMerged = []*datastore.Key{k5, k4, k3, k2}
	assert.Equal(t, expMerged, KeySlice(randomQueryResult).Merge(nil))

}

func TestListHelper(t *testing.T) {
	testutils.LargeTest(t)

	os.Setenv("DATASTORE_DATASET", "skia-infra")
	os.Setenv("DATASTORE_EMULATOR_HOST", "localhost:8888")
	os.Setenv("DATASTORE_EMULATOR_HOST_PATH", "localhost:8888/datastore")
	os.Setenv("DATASTORE_HOST", "http://localhost:8888")
	os.Setenv("DATASTORE_PROJECT_ID", "skia-infra")

	// Run to the locally running emulator.
	cleanup := ds_testutil.InitDatastore(t,
		ds.RECENT_KEYS)
	defer cleanup()
	client := ds.DS

	listHelper := NewListHelper(client, ds.RECENT_KEYS, "test-list-helper", DefaultConsistencyDelta)

	client.RunInTransaction(context.Background(), func(tx *datastore.Transaction) error {
		assert.NoError(t, listHelper.Add(tx, k1))
		return nil
	})
	foundKeySlice, err := listHelper.GetRecent()
	assert.NoError(t, err)
	foundKeys := foundKeySlice.Merge(nil)
	assert.Equal(t, []*datastore.Key{k1}, foundKeys)
}

func TestTimeBasedKeyID(t *testing.T) {
	ts := util.TimeStamp(time.Millisecond)
	keyID := getSortableTimeID(ts)
	assert.Equal(t, ts, GetTimeFromID(keyID))
}
