package expstorage

import "testing"

import (
	// Using 'require' which is like using 'assert' but causes tests to fail.
	assert "github.com/stretchr/testify/require"

	"skia.googlesource.com/buildbot.git/go/database"
	"skia.googlesource.com/buildbot.git/go/database/testutil"
	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/golden/go/db"
	"skia.googlesource.com/buildbot.git/golden/go/types"
)

func TestChanges(t *testing.T) {
	// Create the starting point of expectations.
	m := NewMemExpectationsStore()
	e, err := m.Get(true)
	if err != nil {
		t.Fatalf("Failed to get expectations: %s", err)
	}
	tc := map[string]types.TestClassification{
		"test1": map[string]types.Label{
			"aaa": types.POSITIVE,
		},
		"test3": map[string]types.Label{
			"ddd": types.UNTRIAGED,
		},
	}
	e.AddDigests(tc)
	m.Put(e, "")

	// Test the degenrate case of a Put with no actual changes.
	e, err = m.Get(true)
	if err != nil {
		t.Fatalf("Failed to get expectations: %s", err)
	}
	ch := m.Changes()
	ch2 := m.Changes()
	m.Put(e, "")
	tests := <-ch
	_ = <-ch2 // Verify channels are stuffed in go routines.
	if got, want := tests, []string{}; !util.SSliceEqual(got, want) {
		t.Errorf("Changes: Got %v Want %v", got, want)
	}

	// Now change some expectations.
	tc = map[string]types.TestClassification{
		"test1": map[string]types.Label{
			"aaa": types.POSITIVE,
			"bbb": types.NEGATIVE,
		},
		"test2": map[string]types.Label{
			"ccc": types.UNTRIAGED,
		},
	}
	e.AddDigests(tc)
	m.Put(e, "")
	tests = <-ch
	_ = <-ch2
	if got, want := tests, []string{"test1", "test2"}; !util.SSliceEqual(got, want) {
		t.Errorf("Changes: Got %v Want %v", got, want)
	}
}

func TestMySQLExpectationsStore(t *testing.T) {
	// Set up the test database.
	testDb := testutil.SetupMySQLTestDatabase(t, db.MigrationSteps())
	defer testDb.Close()

	conf := testutil.LocalTestDatabaseConfig(db.MigrationSteps())
	vdb := database.NewVersionedDB(conf)

	// Test the MySQL backed store
	sqlStore := NewSQLExpectationStore(vdb)
	testExpectationStore(t, sqlStore)

	// Test the caching version of the MySQL store.
	cachingStore := NewCachingExpectationStore(sqlStore)
	testExpectationStore(t, cachingStore)
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
	assert.False(t, &newExps == &foundExps)

	// Get modifiable expectations and change them
	changeExps, err := store.Get(true)
	assert.Nil(t, err)
	assert.False(t, &foundExps == &changeExps)

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
