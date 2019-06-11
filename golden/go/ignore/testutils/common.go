package testutils

import (
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/golden/go/ignore"
)

func IgnoreStoreAll(t sktest.TestingT, store ignore.IgnoreStore) {
	// Add a few instances.
	r1 := ignore.NewIgnoreRule("jon@example.com", time.Now().Add(time.Hour), "config=gpu", "reason")
	r2 := ignore.NewIgnoreRule("jim@example.com", time.Now().Add(time.Minute*10), "config=8888", "No good reason.")
	r3 := ignore.NewIgnoreRule("jon@example.com", time.Now().Add(time.Minute*50), "extra=123&extra=abc", "Ignore multiple.")
	r4 := ignore.NewIgnoreRule("jon@example.com", time.Now().Add(time.Minute*100), "extra=123&extra=abc&config=8888", "Ignore multiple.")
	assert.Equal(t, int64(0), store.Revision())
	assert.NoError(t, store.Create(r1))
	assert.NoError(t, store.Create(r2))
	assert.NoError(t, store.Create(r3))
	assert.NoError(t, store.Create(r4))
	assert.Equal(t, int64(4), store.Revision())

	allRules, err := store.List()
	assert.NoError(t, err)
	assert.Equal(t, 4, len(allRules))
	assert.Equal(t, int64(4), store.Revision())

	// Test the rule matcher
	matcher, err := store.BuildRuleMatcher()
	assert.NoError(t, err)
	found, ok := matcher(map[string]string{"config": "565"})
	assert.False(t, ok)
	assert.Equal(t, []*ignore.IgnoreRule{}, found)
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
	allRules, err = store.List()
	assert.NoError(t, err)
	assert.Equal(t, 3, len(allRules))

	delCount, err = store.Delete(r4.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, delCount)
	allRules, err = store.List()
	assert.NoError(t, err)
	assert.Equal(t, 2, len(allRules))
	assert.Equal(t, int64(6), store.Revision())

	for _, oneRule := range allRules {
		assert.True(t, (oneRule.ID == r1.ID) || (oneRule.ID == r2.ID))
	}

	delCount, err = store.Delete(r1.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, delCount)
	allRules, err = store.List()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(allRules))
	assert.Equal(t, r2.ID, allRules[0].ID)
	assert.Equal(t, int64(7), store.Revision())

	// Update a rule.
	updatedRule := *allRules[0]
	updatedRule.Note = "an updated rule"
	err = store.Update(updatedRule.ID, &updatedRule)
	assert.NoError(t, err, "Update should succeed.")
	allRules, err = store.List()
	assert.NoError(t, err)
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
	assert.Equal(t, 1, delCount)

	allRules, err = store.List()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(allRules))
	assert.Equal(t, int64(9), store.Revision())

	// This id doesn't exist, so we shouldn't be able to delete it.
	delCount, err = store.Delete(1000000)
	assert.NoError(t, err)
	assert.Equal(t, delCount, 0)
	allRules, err = store.List()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(allRules))
	assert.Equal(t, int64(9), store.Revision())
}
