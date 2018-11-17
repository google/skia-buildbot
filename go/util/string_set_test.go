package util

import (
	"sort"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

func TestStringSets(t *testing.T) {
	testutils.SmallTest(t)
	ret := NewStringSet([]string{"abc", "abc"}, []string{"efg", "abc"}).Keys()
	sort.Strings(ret)
	assert.Equal(t, []string{"abc", "efg"}, ret)

	assert.Empty(t, NewStringSet().Keys())
	assert.Equal(t, []string{"abc"}, NewStringSet([]string{"abc"}).Keys())
	assert.Equal(t, []string{"abc"}, NewStringSet([]string{"abc", "abc", "abc"}).Keys())
}

func TestStringSetCopy(t *testing.T) {
	testutils.SmallTest(t)
	someKeys := []string{"gamma", "beta", "alpha"}
	orig := NewStringSet(someKeys)
	copy := orig.Copy()

	delete(orig, "alpha")
	orig["mu"] = true

	assert.True(t, copy["alpha"])
	assert.True(t, copy["beta"])
	assert.True(t, copy["gamma"])
	assert.False(t, copy["mu"])

	delete(copy, "beta")
	copy["nu"] = true

	assert.False(t, orig["alpha"])
	assert.True(t, orig["beta"])
	assert.True(t, orig["gamma"])
	assert.True(t, orig["mu"])
	assert.False(t, orig["nu"])

	assert.Nil(t, (StringSet(nil)).Copy())
}

func TestStringSetKeys(t *testing.T) {
	testutils.SmallTest(t)
	expectedKeys := []string{"gamma", "beta", "alpha"}
	s := NewStringSet(append(expectedKeys, expectedKeys...))
	keys := s.Keys()
	assert.Equal(t, 3, len(keys))
	assert.True(t, In("alpha", keys))
	assert.True(t, In("beta", keys))
	assert.True(t, In("gamma", keys))

	s = nil
	keys = s.Keys()
	assert.Empty(t, keys)
}

func TestStringSetIntersect(t *testing.T) {
	testutils.SmallTest(t)
	someKeys := []string{"gamma", "beta", "alpha"}
	otherKeys := []string{"mu", "nu", "omicron"}
	a := NewStringSet(append(someKeys, otherKeys...))
	b := NewStringSet(someKeys)
	c := a.Intersect(b)

	keys := c.Keys()
	assert.Equal(t, 3, len(keys))
	assert.True(t, In("alpha", keys))
	assert.True(t, In("beta", keys))
	assert.True(t, In("gamma", keys))

	d := b.Intersect(a)
	keys = d.Keys()
	assert.Equal(t, 3, len(keys))
	assert.True(t, In("alpha", keys))
	assert.True(t, In("beta", keys))
	assert.True(t, In("gamma", keys))
}

func TestStringSetComplement(t *testing.T) {
	testutils.SmallTest(t)
	someKeys := []string{"gamma", "beta", "alpha"}
	otherKeys := []string{"mu", "nu", "omicron"}
	a := NewStringSet(append(someKeys, otherKeys...))
	b := NewStringSet(someKeys)
	c := a.Complement(b)

	keys := c.Keys()
	assert.Equal(t, 3, len(keys))
	assert.True(t, In("mu", keys))
	assert.True(t, In("nu", keys))
	assert.True(t, In("omicron", keys))

	d := b.Complement(a)
	assert.Empty(t, d.Keys())
}

func TestStringSetUnion(t *testing.T) {
	testutils.SmallTest(t)
	someKeys := []string{"gamma", "beta", "alpha", "zeta"}
	otherKeys := []string{"mu", "nu", "omicron", "zeta"}
	a := NewStringSet(otherKeys)
	b := NewStringSet(someKeys)
	c := a.Union(b)

	keys := c.Keys()
	assert.Equal(t, 7, len(keys))
	assert.True(t, In("alpha", keys))
	assert.True(t, In("beta", keys))
	assert.True(t, In("gamma", keys))
	assert.True(t, In("zeta", keys))
	assert.True(t, In("mu", keys))
	assert.True(t, In("nu", keys))
	assert.True(t, In("omicron", keys))

	d := b.Union(a)
	keys = d.Keys()
	assert.Equal(t, 7, len(keys))
	assert.True(t, In("alpha", keys))
	assert.True(t, In("beta", keys))
	assert.True(t, In("gamma", keys))
	assert.True(t, In("zeta", keys))
	assert.True(t, In("mu", keys))
	assert.True(t, In("nu", keys))
	assert.True(t, In("omicron", keys))
}

func TestStringSetEqual(t *testing.T) {
	testutils.SmallTest(t)
	assert.True(t, StringSet(nil).Equals(nil))
	assert.True(t, NewStringSet(nil).Equals(nil))
	assert.True(t, NewStringSet(nil).Equals(NewStringSet(nil)))
	assert.True(t, NewStringSet([]string{}).Equals(nil))
	someKeys := []string{"gamma", "beta", "alpha", "zeta"}
	assert.True(t, NewStringSet(someKeys).Equals(NewStringSet(someKeys)))
	assert.False(t, NewStringSet(someKeys).Equals(NewStringSet(someKeys[:3])))
	assert.False(t, NewStringSet(someKeys[:3]).Equals(NewStringSet(someKeys)))
	assert.True(t, NewStringSet(someKeys[:1]).Equals(NewStringSet(someKeys[:1])))
	assert.False(t, NewStringSet(someKeys[:1]).Equals(NewStringSet(someKeys[1:2])))
	assert.False(t, NewStringSet(someKeys[0:1]).Equals(NewStringSet(someKeys[2:3])))
}
