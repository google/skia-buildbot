package expstorage

import "testing"

import (
	// Using 'require' which is like using 'assert' but causes tests to fail.
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/database/testutil"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/db"
	"go.skia.org/infra/golden/go/types"
)

func TestChanges(t *testing.T) {
	// Create the starting point of expectations.
	m := NewMemExpectationsStore()
	_, err := m.Get()
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
	m.AddChange(tc, "")

	// Test the degenrate case of a Put with no actual changes.
	_, err = m.Get()
	if err != nil {
		t.Fatalf("Failed to get expectations: %s", err)
	}
	ch := m.Changes()
	ch2 := m.Changes()
	m.AddChange(map[string]types.TestClassification{}, "")
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
	m.AddChange(tc, "")
	tests = <-ch
	_ = <-ch2
	if got, want := tests, []string{"test1", "test2"}; !util.SSliceEqual(got, want) {
		t.Errorf("Changes: Got %v Want %v", got, want)
	}
}

func TestMySQLExpectationsStore(t *testing.T) {
	// Set up the test database.
	testDb := testutil.SetupMySQLTestDatabase(t, db.MigrationSteps())
	defer testDb.Close(t)

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

	newExps := map[string]types.TestClassification{
		TEST_1: types.TestClassification{
			DIGEST_11: types.POSITIVE,
			DIGEST_12: types.NEGATIVE,
		},
		TEST_2: types.TestClassification{
			DIGEST_21: types.POSITIVE,
			DIGEST_22: types.NEGATIVE,
		},
	}
	err := store.AddChange(newExps, "user-0")
	assert.Nil(t, err)

	foundExps, err := store.Get()
	assert.Nil(t, err)

	assert.Equal(t, newExps, foundExps.Tests)
	assert.False(t, &newExps == &foundExps.Tests)
}
