package dsutil

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"cloud.google.com/go/datastore"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ds"
	ds_testutil "go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

func TestBlobStore(t *testing.T) {
	testutils.LargeTest(t)

	blobKind := ds.Kind("test-blob-parent-kind")
	blobFragKind := ds.Kind("test-blob-kind")
	ctx := context.TODO()

	// Run to the locally running emulator.
	cleanup := ds_testutil.InitDatastore(t,
		blobKind,
		blobFragKind)
	defer cleanup()
	client := ds.DS

	_, err := ds.DeleteAll(client, blobKind, true)
	assert.NoError(t, err)
	_, err = ds.DeleteAll(client, blobFragKind, true)
	assert.NoError(t, err)

	// Create key for the blob
	blobStore := NewBlobStore(client, blobKind, blobFragKind)
	testBlobKey := ds.NewKey(blobKind)
	testBlobKey.Name = "test-parent-key-id"

	// Make sure we get the right error if the blob doesn't exist.
	foundChanges := map[string]map[string]int{}
	assert.Equal(t, datastore.ErrNoSuchEntity, blobStore.Load(testBlobKey, &foundChanges))

	// Create a map that is similar to the expectations in Gold. But this works
	// for any type that can be serialized to JSON.
	changes := getRandomMap(100, 5000)

	// Write a blob to the store.
	testBlobKey, err = blobStore.Save(changes)
	assert.NoError(t, err)

	// Get the number of fragments that should have been written to the db and
	// make sure we are testing with more than one fragment.
	fragments, err := jsonEncodeBlobParts(changes)
	assert.NoError(t, err)
	nChangeFrags := len(fragments)
	assert.True(t, nChangeFrags > 1)

	// Make sure we get the right number of fragments
	assert.NoError(t, testutils.EventuallyConsistent(10*time.Second, func() error {
		nFragments, err := client.Count(ctx, ds.NewQuery(blobFragKind))
		assert.NoError(t, err)
		assert.True(t, nFragments <= nChangeFrags)

		// Make sure only have one blob in total in the store
		nBlobs, err := client.Count(ctx, ds.NewQuery(blobKind))
		assert.NoError(t, err)

		if (nFragments < nChangeFrags) && (nBlobs != 1) {
			return testutils.TryAgainErr
		}
		return nil
	}))

	// Read the blob back and make sure it matches
	foundChanges = nil
	assert.NoError(t, blobStore.Load(testBlobKey, &foundChanges))
	assert.Equal(t, changes, foundChanges)
}

func getRandomMap(nTests, nDigests int) map[string]map[string]int {
	intVals := []int{1, 2, 3, 4, 5}
	ret := make(map[string]map[string]int, nTests)
	for i := 0; i < nTests; i++ {
		digests := make(map[string]int, nDigests)
		for j := 0; j < nDigests; j++ {
			digests[util.RandomName()] = intVals[rand.Intn(len(intVals))]
		}
		ret[util.RandomName()] = digests
	}
	return ret
}
