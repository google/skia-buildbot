package expstorage

import (
	"testing"
)

import (
	// Using 'require' which is like using 'assert' but causes tests to fail.
	assert "github.com/stretchr/testify/require"
	"skia.googlesource.com/buildbot.git/perf/go/types"
)

func TestExpectationStores(t *testing.T) {
	memStore := NewMemExpectationsStore()
	testExpectationStore(t, memStore)
}

// Test against the expectation store interface.
func testExpectationStore(t *testing.T, store ExpectationsStore) {
	TEST_1, TEST_2 := "test1", "test2"

	// digests
	DIGEST_11, DIGEST_12 := "d11", "d12"
	DIGEST_21, DIGEST_22 := "d21", "d22"

	newExps := NewExpectations(true)
	newExps.AddDigests(map[string]types.TestClassification{
		TEST_1: types.TestClassification{
			DIGEST_11: types.POSITIVE,
			DIGEST_12: types.NEGATIVE,
		},
		TEST_2: types.TestClassification{
			DIGEST_21: types.POSITIVE,
			DIGEST_22: types.NEGATIVE,
		},
	})
	err := store.Put(newExps, "user-0")
	assert.Nil(t, err)

	foundExps, err := store.Get(false)
	assert.Nil(t, err)

	assert.Equal(t, newExps.Tests, foundExps.Tests)
	assert.NotEqual(t, &newExps, &foundExps)

	// Get modifiable expectations and change them
	changeExps, err := store.Get(true)
	assert.Nil(t, err)
	assert.NotEqual(t, &foundExps, &changeExps)

	changeExps.RemoveDigests([]string{DIGEST_11})
	changeExps.RemoveDigests([]string{DIGEST_11, DIGEST_22})
	err = store.Put(changeExps, "user-1")
	assert.Nil(t, err)

	foundExps, err = store.Get(false)
	assert.Nil(t, err)

	assert.Equal(t, types.TestClassification(map[string]types.Label{DIGEST_12: types.NEGATIVE}), foundExps.Tests[TEST_1])
	assert.Equal(t, types.TestClassification(map[string]types.Label{DIGEST_21: types.POSITIVE}), foundExps.Tests[TEST_2])

	changeExps.RemoveDigests([]string{DIGEST_12})
	err = store.Put(changeExps, "user-3")
	assert.Nil(t, err)

	foundExps, err = store.Get(false)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(foundExps.Tests))
}
