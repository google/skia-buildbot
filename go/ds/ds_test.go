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
	Random   int64
	Sortable int64
}

func TestDeleteAll(t *testing.T) {
	testutils.LargeTest(t)

	assert.NoError(t, InitForTesting("test-project", "test-namespace"))
	client := DS

	nEntries := 1200
	maxID := int64(nEntries * 10)
	addRandEntities(t, client, nEntries, maxID)
	_, err := DeleteAll(client, TEST_KIND, true)
	assert.NoError(t, err)

	count, err := client.Count(context.TODO(), NewQuery(TEST_KIND))
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func addRandEntities(t *testing.T, client *datastore.Client, nEntries int, maxID int64) {
	_, err := DeleteAll(client, TEST_KIND, true)
	assert.NoError(t, err)

	// Create a test type and fill it with random values
	exp := make([]*testEntity, 0, nEntries)
	ctx := context.TODO()
	for i := 0; i < nEntries; i++ {
		newEntry := &testEntity{
			Random:   rand.Int63(),
			Sortable: (rand.Int63() % maxID) + 1,
		}
		exp = append(exp, newEntry)
		_, err := client.Put(ctx, NewKey(TEST_KIND), newEntry)
		assert.NoError(t, err)
	}

	sort.Slice(exp, func(i, j int) bool {
		return ((exp[i].Sortable < exp[j].Sortable) ||
			(exp[i].Sortable == exp[j].Sortable) && (exp[i].Random < exp[j].Random))
	})

	wait(t, client, TEST_KIND, nEntries)
	assert.Equal(t, nEntries, len(exp))
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
