package util

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringSets(t *testing.T) {
	ret := NewStringSet([]string{"abc", "abc"}, []string{"efg", "abc"}).Keys()
	sort.Strings(ret)
	require.Equal(t, []string{"abc", "efg"}, ret)

	require.Empty(t, NewStringSet().Keys())
	require.Equal(t, []string{"abc"}, NewStringSet([]string{"abc"}).Keys())
	require.Equal(t, []string{"abc"}, NewStringSet([]string{"abc", "abc", "abc"}).Keys())
}

func TestStringSetCopy(t *testing.T) {
	someKeys := []string{"gamma", "beta", "alpha"}
	orig := NewStringSet(someKeys)
	copy := orig.Copy()

	delete(orig, "alpha")
	orig["mu"] = true

	require.True(t, copy["alpha"])
	require.True(t, copy["beta"])
	require.True(t, copy["gamma"])
	require.False(t, copy["mu"])

	delete(copy, "beta")
	copy["nu"] = true

	require.False(t, orig["alpha"])
	require.True(t, orig["beta"])
	require.True(t, orig["gamma"])
	require.True(t, orig["mu"])
	require.False(t, orig["nu"])

	require.Nil(t, (StringSet(nil)).Copy())
}

func TestStringSetKeys(t *testing.T) {
	expectedKeys := []string{"gamma", "beta", "alpha"}
	s := NewStringSet(append(expectedKeys, expectedKeys...))
	keys := s.Keys()
	require.Equal(t, 3, len(keys))
	require.True(t, In("alpha", keys))
	require.True(t, In("beta", keys))
	require.True(t, In("gamma", keys))

	s = nil
	keys = s.Keys()
	require.Empty(t, keys)
}

func TestStringSetIntersect(t *testing.T) {
	someKeys := []string{"gamma", "beta", "alpha"}
	otherKeys := []string{"mu", "nu", "omicron"}
	a := NewStringSet(append(someKeys, otherKeys...))
	b := NewStringSet(someKeys)
	c := a.Intersect(b)

	keys := c.Keys()
	require.Equal(t, 3, len(keys))
	require.True(t, In("alpha", keys))
	require.True(t, In("beta", keys))
	require.True(t, In("gamma", keys))

	d := b.Intersect(a)
	keys = d.Keys()
	require.Equal(t, 3, len(keys))
	require.True(t, In("alpha", keys))
	require.True(t, In("beta", keys))
	require.True(t, In("gamma", keys))
}

func TestStringSetComplement(t *testing.T) {
	someKeys := []string{"gamma", "beta", "alpha"}
	otherKeys := []string{"mu", "nu", "omicron"}
	a := NewStringSet(append(someKeys, otherKeys...))
	b := NewStringSet(someKeys)
	c := a.Complement(b)

	keys := c.Keys()
	require.Equal(t, 3, len(keys))
	require.True(t, In("mu", keys))
	require.True(t, In("nu", keys))
	require.True(t, In("omicron", keys))

	d := b.Complement(a)
	require.Empty(t, d.Keys())
}

func TestStringSetUnion(t *testing.T) {
	someKeys := []string{"gamma", "beta", "alpha", "zeta"}
	otherKeys := []string{"mu", "nu", "omicron", "zeta"}
	a := NewStringSet(otherKeys)
	b := NewStringSet(someKeys)
	c := a.Union(b)

	keys := c.Keys()
	require.Equal(t, 7, len(keys))
	require.True(t, In("alpha", keys))
	require.True(t, In("beta", keys))
	require.True(t, In("gamma", keys))
	require.True(t, In("zeta", keys))
	require.True(t, In("mu", keys))
	require.True(t, In("nu", keys))
	require.True(t, In("omicron", keys))

	d := b.Union(a)
	keys = d.Keys()
	require.Equal(t, 7, len(keys))
	require.True(t, In("alpha", keys))
	require.True(t, In("beta", keys))
	require.True(t, In("gamma", keys))
	require.True(t, In("zeta", keys))
	require.True(t, In("mu", keys))
	require.True(t, In("nu", keys))
	require.True(t, In("omicron", keys))
}

func TestStringSetEqual(t *testing.T) {
	require.True(t, StringSet(nil).Equals(nil))
	require.True(t, NewStringSet(nil).Equals(nil))
	require.True(t, NewStringSet(nil).Equals(NewStringSet(nil)))
	require.True(t, NewStringSet([]string{}).Equals(nil))
	someKeys := []string{"gamma", "beta", "alpha", "zeta"}
	require.True(t, NewStringSet(someKeys).Equals(NewStringSet(someKeys)))
	require.False(t, NewStringSet(someKeys).Equals(NewStringSet(someKeys[:3])))
	require.False(t, NewStringSet(someKeys[:3]).Equals(NewStringSet(someKeys)))
	require.True(t, NewStringSet(someKeys[:1]).Equals(NewStringSet(someKeys[:1])))
	require.False(t, NewStringSet(someKeys[:1]).Equals(NewStringSet(someKeys[1:2])))
	require.False(t, NewStringSet(someKeys[0:1]).Equals(NewStringSet(someKeys[2:3])))
}
