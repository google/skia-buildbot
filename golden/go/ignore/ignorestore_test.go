package ignore

import (
	"net/url"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/database/testutil"
	"go.skia.org/infra/go/ds"
	ds_testutil "go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/db"
)

func TestTestMemIgnoreStore(t *testing.T) {
	unittest.SmallTest(t)
	memStore := NewMemIgnoreStore()
	testIgnoreStore(t, memStore)
}

func TestSQLIgnoreStore(t *testing.T) {
	unittest.LargeTest(t)
	// Set up the database. This also locks the db until this test is finished
	// causing similar tests to wait.
	migrationSteps := db.MigrationSteps()
	mysqlDB := testutil.SetupMySQLTestDatabase(t, migrationSteps)
	defer mysqlDB.Close(t)

	vdb, err := testutil.LocalTestDatabaseConfig(migrationSteps).NewVersionedDB()
	assert.NoError(t, err)
	defer testutils.AssertCloses(t, vdb)

	store := NewSQLIgnoreStore(vdb, nil, nil)
	testIgnoreStore(t, store)
}

func TestCloudIgnoreStore(t *testing.T) {
	unittest.LargeTest(t)

	// Run to the locally running emulator.
	cleanup := ds_testutil.InitDatastore(t,
		ds.IGNORE_RULE,
		ds.HELPER_RECENT_KEYS)
	defer cleanup()

	store, err := NewCloudIgnoreStore(ds.DS, nil, nil)
	assert.NoError(t, err)
	testIgnoreStore(t, store)
}

func testIgnoreStore(t *testing.T, store IgnoreStore) {
	// Add a few instances.
	r1 := NewIgnoreRule("jon@example.com", time.Now().Add(time.Hour), "config=gpu", "reason")
	r2 := NewIgnoreRule("jim@example.com", time.Now().Add(time.Minute*10), "config=8888", "No good reason.")
	r3 := NewIgnoreRule("jon@example.com", time.Now().Add(time.Minute*50), "extra=123&extra=abc", "Ignore multiple.")
	r4 := NewIgnoreRule("jon@example.com", time.Now().Add(time.Minute*100), "extra=123&extra=abc&config=8888", "Ignore multiple.")
	assert.Equal(t, int64(0), store.Revision())
	assert.NoError(t, store.Create(r1))
	assert.NoError(t, store.Create(r2))
	assert.NoError(t, store.Create(r3))
	assert.NoError(t, store.Create(r4))
	assert.Equal(t, int64(4), store.Revision())

	allRules, err := store.List(false)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(allRules))
	assert.Equal(t, int64(4), store.Revision())

	// Test the rule matcher
	matcher, err := store.BuildRuleMatcher()
	assert.NoError(t, err)
	found, ok := matcher(map[string]string{"config": "565"})
	assert.False(t, ok)
	assert.Equal(t, []*IgnoreRule{}, found)
	found, ok = matcher(map[string]string{"config": "8888"})
	assert.True(t, ok)
	assert.Equal(t, 1, len(found))
	found, ok = matcher(map[string]string{"extra": "123"})
	assert.True(t, ok)
	assert.Equal(t, 1, len(found))
	found, ok = matcher(map[string]string{"extra": "abc"})
	assert.True(t, ok)
	assert.Equal(t, 1, len(found))
	found, ok = matcher(map[string]string{"extra": "abc", "config": "8888"})
	assert.True(t, ok)
	assert.Equal(t, 3, len(found))
	found, ok = matcher(map[string]string{"extra": "abc", "config": "gpu"})
	assert.True(t, ok)
	assert.Equal(t, 2, len(found))
	assert.Equal(t, int64(4), store.Revision())

	// Remove the third and fourth rule
	delCount, err := store.Delete(r3.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, delCount)
	allRules, err = store.List(false)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(allRules))

	delCount, err = store.Delete(r4.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, delCount)
	allRules, err = store.List(false)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(allRules))
	assert.Equal(t, int64(6), store.Revision())

	for _, oneRule := range allRules {
		assert.True(t, (oneRule.ID == r1.ID) || (oneRule.ID == r2.ID))
	}

	delCount, err = store.Delete(r1.ID)
	assert.NoError(t, err)
	allRules, err = store.List(false)
	assert.Equal(t, 1, len(allRules))
	assert.Equal(t, r2.ID, allRules[0].ID)
	assert.Equal(t, int64(7), store.Revision())

	// Update a rule.
	updatedRule := *allRules[0]
	updatedRule.Note = "an updated rule"
	err = store.Update(updatedRule.ID, &updatedRule)
	assert.NoError(t, err, "Update should succeed.")
	allRules, err = store.List(false)
	assert.Equal(t, 1, len(allRules))
	assert.Equal(t, r2.ID, allRules[0].ID)
	assert.Equal(t, "an updated rule", allRules[0].Note)
	assert.Equal(t, int64(8), store.Revision())

	// Try to update a non-existent rule.
	updatedRule = *allRules[0]
	err = store.Update(100001, &updatedRule)
	assert.Error(t, err, "Update should fail for a bad id.")
	assert.Equal(t, int64(8), store.Revision())

	delCount, err = store.Delete(r2.ID)
	assert.NoError(t, err)
	allRules, err = store.List(false)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(allRules))
	assert.Equal(t, int64(9), store.Revision())

	delCount, err = store.Delete(1000000)
	assert.NoError(t, err)
	assert.Equal(t, delCount, 0)
	allRules, err = store.List(false)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(allRules))
	assert.Equal(t, int64(9), store.Revision())
}

func TestToQuery(t *testing.T) {
	unittest.SmallTest(t)
	queries, err := ToQuery([]*IgnoreRule{})
	assert.NoError(t, err)
	assert.Len(t, queries, 0)

	r1 := NewIgnoreRule("jon@example.com", time.Now().Add(time.Hour), "config=gpu", "reason")
	queries, err = ToQuery([]*IgnoreRule{r1})
	assert.NoError(t, err)
	assert.Equal(t, queries[0], url.Values{"config": []string{"gpu"}})

	r1 = NewIgnoreRule("jon@example.com", time.Now().Add(time.Hour), "bad=%", "reason")
	queries, err = ToQuery([]*IgnoreRule{r1})
	assert.NotNil(t, err)
}
