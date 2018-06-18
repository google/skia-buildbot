package expstorage

import (
	"testing"

	"cloud.google.com/go/datastore"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/types"
)

var (
	TEST_1, TEST_2       = "test1", "test2"
	DIGEST_11, DIGEST_12 = "d11", "d12"
	DIGEST_21, DIGEST_22 = "d21", "d22"

	expChange_1 = map[string]types.TestClassification{
		TEST_1: {
			DIGEST_11: types.POSITIVE,
			DIGEST_12: types.NEGATIVE,
		},
		TEST_2: {
			DIGEST_21: types.POSITIVE,
			DIGEST_22: types.NEGATIVE,
		},
	}
)

type testEntity struct {
	Payload  string
	Children []*datastore.Key `datastore:",noindex"`
}

// TODO(stephana): Add tests for some of the special cases, e.g.:
//  - buildTDESlice where input is larger than nDigestsPerRec and/or TDESlice.add when full
//  - TDESlice.update for non-empty TDESlice
//  - TDESlice.update when pre-existing untriaged
//  - TDESlice.update when new untriaged

// func TestNameDigestLabels(t *testing.T) {
// 	testutils.SmallTest(t)

// 	TEST_1, TEST_2 := "test1", "test2"
// 	DIGEST_11, DIGEST_12 := "d11", "d12"
// 	DIGEST_21, DIGEST_22 := "d21", "d22"

// 	expChange_1 := map[string]types.TestClassification{
// 		TEST_1: {
// 			DIGEST_11: types.POSITIVE,
// 			DIGEST_12: types.NEGATIVE,
// 		},
// 		TEST_2: {
// 			DIGEST_21: types.POSITIVE,
// 			DIGEST_22: types.NEGATIVE,
// 		},
// 	}

// 	expColl := buildTDESlice(expChange_1)
// 	assert.Equal(t, expChange_1, expColl.toExpectations(false).Tests)

// 	var emptyColl TDESlice
// 	emptyColl.update(expChange_1)
// 	assert.Equal(t, expChange_1, emptyColl.toExpectations(false).Tests)
// }

func TestExpsBlobs(t *testing.T) {
	testutils.LargeTest(t)

	// os.Setenv("DATASTORE_EMULATOR_HOST", "localhost:8891")

	// mixinParentKind := ds.Kind("test-mixin-test-kind")
	// blobParentKind := ds.Kind("test-blob-parent-kind")
	// blobKind := ds.Kind("test-blob-kind")
	// cleanup := initDS(t, mixinParentKind, blobParentKind, blobKind)
	// defer cleanup()
	// client := ds.DS
	// changes := getRandomChange(100, 5000)
	// changes_2 := getRandomChange(10, 5000)
	// blobLoader := BlobLoader{client: client}

	// // Test the mixin with an existing entity
	// foundChanges := map[string]types.TestClassification{}
	// children, err := blobLoader.WriteJsonBlobParts(blobKind, changes)
	// assert.NoError(t, err)

	// mixinParentKey := ds.NewKey(mixinParentKind)
	// mixinParent := &testEntity{Payload: "some string", Children: children}
	// mixinParentKey, err = client.Put(context.TODO(), mixinParentKey, mixinParent)
	// assert.NoError(t, err)

	// blobParent, err := blobLoader.LoadJsonBlob(nil, mixinParentKey, &foundChanges)
	// assert.NoError(t, err)
	// assert.Equal(t, changes, foundChanges)

	// _, err = blobLoader.UpdateJsonBlob(nil, mixinParentKey, blobParent, blobKind, changes_2)
	// assert.NoError(t, err)

	// foundChanges = nil
	// blobParent, err = blobLoader.LoadJsonBlob(nil, mixinParentKey, &foundChanges)
	// assert.NoError(t, err)
	// assert.Equal(t, changes_2, foundChanges)

	// foundMixinParent := &testEntity{}
	// assert.NoError(t, client.Get(context.TODO(), mixinParentKey, foundMixinParent))
	// mixinParent.Children = blobParent.Children
	// assert.Equal(t, mixinParent, foundMixinParent)

	// // Test when the blob parent is a stand-alone entity
	// testParentKey := ds.NewKey(blobParentKind)
	// testParentKey.Name = "test-parent-key-id"

	// // Make sure we get the right error if the blob doesn't exist.
	// foundChanges = nil
	// blobParent, err = blobLoader.LoadJsonBlob(nil, testParentKey, foundChanges)
	// assert.Nil(t, blobParent)

	// testParentKey, err = blobLoader.UpdateJsonBlob(nil, testParentKey, nil, blobParentKind, changes)
	// assert.NoError(t, err)

	// foundChanges = nil
	// _, err = blobLoader.LoadJsonBlob(nil, testParentKey, &foundChanges)
	// assert.NoError(t, err)
	// assert.NotNil(t, changes)
	// assert.Equal(t, changes, foundChanges)
}
