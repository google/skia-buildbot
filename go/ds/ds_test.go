package ds

import (
	"context"
	"math/rand"
	"sort"
	"testing"
	"time"

	"cloud.google.com/go/datastore"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

const TEST_KIND = Kind("DS_TEST_KIND")

type testEntity struct {
	Key      *datastore.Key `datastore:"__key__"`
	Random   int64
	Sortable int64
}

func TestDeleteAll(t *testing.T) {
	testutils.LargeTest(t)

	assert.NoError(t, InitForTesting("test-project", "test-namespace"))
	client := DS

	nEntries := 1200
	maxID := int64(nEntries * 10)

	// Ignore the cleanup-call returned by addRandEntities since we are
	// calling DeleteAll in this function anyway.
	_, _ = addRandEntities(t, client, nEntries, maxID)
	_, err := DeleteAll(client, TEST_KIND, true)
	assert.NoError(t, err)

	count, err := client.Count(context.TODO(), NewQuery(TEST_KIND))
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestIterKeys(t *testing.T) {
	testutils.LargeTest(t)

	nEntries := 1200
	maxID := int64(nEntries / 2)
	assert.NoError(t, InitForTesting("test-project", "test-namespace"))
	client := DS
	exp, cleanup := addRandEntities(t, client, nEntries, maxID)
	defer cleanup()

	// Iterate over the type and collect the instances
	iterCh, err := IterKeys(client, TEST_KIND, 10)
	assert.NoError(t, err)
	ctx := context.TODO()
	var found []*testEntity

	for item := range iterCh {
		assert.NoError(t, item.Err)
		keySlice := item.Keys
		target := make([]*testEntity, len(keySlice))
		assert.NoError(t, client.GetMulti(ctx, keySlice, target))
		found = append(found, target...)
	}
	assert.Equal(t, exp, found)
}

func addRandEntities(t *testing.T, client *datastore.Client, nEntries int, maxID int64) ([]*testEntity, func()) {
	_, err := DeleteAll(client, TEST_KIND, true)
	assert.NoError(t, err)

	cleanup := func() {
		_, err := DeleteAll(client, TEST_KIND, true)
		assert.NoError(t, err)
	}

	// Create a test type and fill it with random values
	exp := make([]*testEntity, 0, nEntries)
	ctx := context.TODO()
	for i := 0; i < nEntries; i++ {
		newEntry := &testEntity{
			Random:   rand.Int63(),
			Sortable: (rand.Int63() % maxID) + 1,
		}
		newEntry.Key, err = client.Put(ctx, NewKey(TEST_KIND), newEntry)
		assert.NoError(t, err)
		exp = append(exp, newEntry)
	}

	sort.Slice(exp, func(i, j int) bool { return exp[i].Key.ID < exp[j].Key.ID })
	wait(t, client, TEST_KIND, nEntries)
	assert.Equal(t, nEntries, len(exp))
	return exp, cleanup
}

func wait(t *testing.T, client *datastore.Client, kind Kind, expectedCount int) {
	for {
		count, err := client.Count(context.TODO(), NewQuery(kind))
		assert.NoError(t, err)
		if count == expectedCount {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}
