package ds

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestMigrateData(t *testing.T) {
	// disabled: https://bugs.chromium.org/p/skia/issues/detail?id=9061
	unittest.ManualTest(t)
	unittest.RequiresDatastoreEmulator(t)

	ctx := context.TODO()

	// Create the source project.
	require.NoError(t, InitForTesting("src-project", "test-namespace"))
	srcClient := DS

	// Populate the source.
	nEntries := 1200
	maxID := int64(nEntries * 10)
	entitiesSrc, cleanupSrc := addRandEntities(t, srcClient, nEntries, maxID)
	defer cleanupSrc()
	srcCount, err := srcClient.Count(ctx, NewQuery(TEST_KIND))
	require.NoError(t, err)
	require.Equal(t, nEntries, srcCount)

	// Create the destination project.
	require.NoError(t, InitForTesting("dest-project", "test-namespace"))
	destClient := DS
	defer func() {
		_, err := DeleteAll(destClient, TEST_KIND, true)
		require.NoError(t, err)
	}()
	destCountEmpty, err := destClient.Count(ctx, NewQuery(TEST_KIND))
	require.NoError(t, err)
	require.Equal(t, 0, destCountEmpty)

	// Migrate from source to destination reusing the old keys.
	require.NoError(t, MigrateData(ctx, srcClient, destClient, TEST_KIND, false /* createNewKey */))
	wait(t, destClient, TEST_KIND, nEntries)
	destCountPopulated, err := destClient.Count(ctx, NewQuery(TEST_KIND))
	require.NoError(t, err)
	require.Equal(t, nEntries, destCountPopulated)
	// Spot check to make sure source and destination data match.
	for _, entitySrc := range []*testEntity{entitiesSrc[0], entitiesSrc[10], entitiesSrc[nEntries-1]} {
		entityDest := &testEntity{}
		require.NoError(t, destClient.Get(ctx, entitySrc.Key, entityDest))
		require.Equal(t, entitySrc.Key, entityDest.Key)
		require.Equal(t, entitySrc.Random, entityDest.Random)
		require.Equal(t, entitySrc.Sortable, entityDest.Sortable)
	}
	// Cleanup.
	_, err = DeleteAll(destClient, TEST_KIND, true)
	require.NoError(t, err)

	// Migrate from source to destination by creating new keys.
	require.NoError(t, MigrateData(ctx, srcClient, destClient, TEST_KIND, true /* createNewKey */))
	wait(t, destClient, TEST_KIND, nEntries)
	destCountPopulated, err = destClient.Count(ctx, NewQuery(TEST_KIND))
	require.NoError(t, err)
	require.Equal(t, nEntries, destCountPopulated)
}
