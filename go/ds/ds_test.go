package ds

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestDeleteAll(t *testing.T) {
	unittest.LargeTest(t)

	require.NoError(t, InitForTesting("test-project", "test-namespace"))
	client := DS

	nEntries := 1200
	maxID := int64(nEntries * 10)

	// Ignore the cleanup-call returned by addRandEntities since we are
	// calling DeleteAll in this function anyway.
	_, _ = addRandEntities(t, client, nEntries, maxID)
	_, err := DeleteAll(client, testKind, true)
	require.NoError(t, err)

	count, err := client.Count(context.TODO(), NewQuery(testKind))
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestIterKeys(t *testing.T) {
	unittest.LargeTest(t)

	nEntries := 1200
	maxID := int64(nEntries / 2)
	require.NoError(t, InitForTesting("test-project", "test-namespace"))
	client := DS
	exp, cleanup := addRandEntities(t, client, nEntries, maxID)
	defer cleanup()

	// Iterate over the type and collect the instances
	iterCh, err := IterKeys(client, testKind, 10)
	require.NoError(t, err)
	ctx := context.TODO()
	var found []*testEntity

	for item := range iterCh {
		require.NoError(t, item.Err)
		keySlice := item.Keys
		target := make([]*testEntity, len(keySlice))
		require.NoError(t, client.GetMulti(ctx, keySlice, target))
		found = append(found, target...)
	}
	require.Equal(t, exp, found)
}
