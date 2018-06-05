package dsutil

import (
	"context"
	"math/rand"
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

// var (
// 	TEST_1, TEST_2       = "test1", "test2"
// 	DIGEST_11, DIGEST_12 = "d11", "d12"
// 	DIGEST_21, DIGEST_22 = "d21", "d22"

// 	expChange_1 = map[string]types.TestClassification{
// 		TEST_1: {
// 			DIGEST_11: types.POSITIVE,
// 			DIGEST_12: types.NEGATIVE,
// 		},
// 		TEST_2: {
// 			DIGEST_21: types.POSITIVE,
// 			DIGEST_22: types.NEGATIVE,
// 		},
// 	}
// )

type testEntity struct {
	Payload  string
	Children []*datastore.Key `datastore:",noindex"`
}

func TestBlobStore(t *testing.T) {
	testutils.LargeTest(t)

	os.Setenv("DATASTORE_EMULATOR_HOST", "localhost:8891")

	blobKind := ds.Kind("test-blob-parent-kind")
	blobFragKind := ds.Kind("test-blob-kind")
	ctx := context.TODO()

	// Run to the locally running emulator.
	cleanup := ds_testutil.InitDatastore(t,
		blobKind,
		blobFragKind)
	defer cleanup()
	client := ds.DS
	changes_2 := getRandomMap(10000, 50)
	blobStore := NewBlobStore(client, blobKind, blobFragKind)

	// Test when the blob parent is a stand-alone entity
	testBlobKey := ds.NewKey(blobKind)
	testBlobKey.Name = "test-parent-key-id"

	// Make sure we get the right error if the blob doesn't exist.
	foundChanges := map[string]map[string]int{}
	assert.Equal(t, datastore.ErrNoSuchEntity, blobStore.Load(testBlobKey, &foundChanges))

	// Write a blob to the store and fetch it back.
	changes := getRandomMap(100, 5000)
	testBlobKey, err := blobStore.Save(changes)
	assert.NoError(t, err)

	time.Sleep(3 * time.Second)

	// Get the number of fragments that should have been written to the db
	fragments, err := jsonEncodeBlobParts(changes)
	assert.NoError(t, err)
	change1Frags := len(fragments)

	nBlobs, err := client.Count(ctx, ds.NewQuery(blobKind))
	assert.NoError(t, err)
	assert.Equal(t, 1, nBlobs)

	foundChanges = nil
	assert.NoError(t, blobStore.Load(testBlobKey, &foundChanges))
	assert.Equal(t, changes, foundChanges)

	key2, err := blobStore.Save(changes_2)
	assert.NoError(t, err)
	foundChanges = nil
	assert.NoError(t, blobStore.Load(key2, &foundChanges))
	assert.Equal(t, changes_2, foundChanges)

	time.Sleep(3 * time.Second)

	nBlobs, err = client.Count(ctx, ds.NewQuery(blobKind))
	assert.NoError(t, err)
	assert.Equal(t, 2, nBlobs)

	assert.NoError(t, blobStore.Delete(key2))
	time.Sleep(3 * time.Second)

	nBlobs, err = client.Count(ctx, ds.NewQuery(blobKind))
	assert.NoError(t, err)
	assert.Equal(t, 1, nBlobs)

	nFrags, err := client.Count(ctx, ds.NewQuery(blobFragKind))
	assert.NoError(t, err)
	assert.Equal(t, change1Frags, nFrags)
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
