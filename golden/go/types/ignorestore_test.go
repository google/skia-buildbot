package types

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
)

func TestTestMemIgnoreStore(t *testing.T) {
	memStore := NewMemIgnoreStore()
	testIgnoreStore(t, memStore)
}

func testIgnoreStore(t *testing.T, store IgnoreStore) {
	// Add a few instances.
	r1 := NewIgnoreRule("jon@example.com", time.Now().Add(time.Hour), "config=gpu", "reason")
	r2 := NewIgnoreRule("jim@example.com", time.Now().Add(time.Minute*10), "config=8888", "No good reason.")
	r3 := NewIgnoreRule("jon@example.com", time.Now().Add(time.Minute*50), "extra=123&extra=abc", "Ignore multiple.")
	r4 := NewIgnoreRule("jon@example.com", time.Now().Add(time.Minute*100), "extra=123&extra=abc&config=8888", "Ignore multiple.")
	r5 := NewIgnoreRule("jon@example.com", time.Now().Add(time.Millisecond*5), "extra=123&extra=abc&config=8888", "Ignore multiple.")
	assert.Nil(t, store.Create(r1))
	assert.Nil(t, store.Create(r2))
	assert.Nil(t, store.Create(r3))
	assert.Nil(t, store.Create(r4))
	assert.Nil(t, store.Create(r5))

	// Wait long enough for r5 to expire
	time.Sleep(time.Millisecond * 10)

	allRules, err := store.List()
	assert.Nil(t, err)
	assert.Equal(t, 4, len(allRules))

	// Test the rule matcher
	matcher, err := store.BuildRuleMatcher()
	assert.Nil(t, err)
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

	// Remove the third and fourth rue
	delCount, err := store.Delete(r3.ID, "jon@example.com")
	assert.Nil(t, err)
	assert.Equal(t, 1, delCount)
	delCount, err = store.Delete(r4.ID, "jon@example.com")
	assert.Nil(t, err)
	assert.Equal(t, 1, delCount)
	allRules, err = store.List()
	assert.Nil(t, err)
	assert.Equal(t, 2, len(allRules))

	for _, oneRule := range allRules {
		assert.True(t, (oneRule.ID == r1.ID) || (oneRule.ID == r2.ID))
	}

	delCount, err = store.Delete(r1.ID, "jane@example.com")
	assert.Nil(t, err)
	allRules, err = store.List()
	assert.Equal(t, 1, len(allRules))
	assert.Equal(t, r2.ID, allRules[0].ID)

	delCount, err = store.Delete(r2.ID, "jon@example.com")
	assert.Nil(t, err)
	allRules, err = store.List()
	assert.Nil(t, err)
	assert.Equal(t, 0, len(allRules))

	delCount, err = store.Delete(1000000, "someuser@example.com")
	assert.Nil(t, err)
	assert.Equal(t, delCount, 0)
	allRules, err = store.List()
	assert.Nil(t, err)
	assert.Equal(t, 0, len(allRules))

}
