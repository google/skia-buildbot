package trie

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/deepequal/assertdeep"
)

func TestTrie(t *testing.T) {
	trie := New()
	trie.Insert([]string{"a", "b", "c"}, "1")
	trie.Insert([]string{"a", "b"}, "2")
	trie.Insert([]string{"c", "b", "a"}, "3")
	trie.Insert([]string{}, "4")
	trie.Insert([]string{"c", "d"}, "5")
	assert.Equal(t, 5, trie.Len())
	searchSubset := func(search []string) []string {
		got := trie.SearchSubset(search)
		rv := make([]string, 0, len(got))
		for _, s := range got {
			rv = append(rv, s.(string))
		}
		sort.Strings(rv)
		return rv
	}
	searchExact := func(search []string) []string {
		got := trie.Search(search)
		rv := make([]string, 0, len(got))
		for _, s := range got {
			rv = append(rv, s.(string))
		}
		sort.Strings(rv)
		return rv
	}
	assertdeep.Equal(t, []string{"4"}, searchSubset([]string{}))
	assertdeep.Equal(t, []string{"4"}, searchExact([]string{}))
	assertdeep.Equal(t, []string{"4"}, searchSubset([]string{"a"}))
	assertdeep.Equal(t, []string{}, searchExact([]string{"a"}))
	assertdeep.Equal(t, []string{"2", "4"}, searchSubset([]string{"a", "b"}))
	assertdeep.Equal(t, []string{"2"}, searchExact([]string{"a", "b"}))
	assertdeep.Equal(t, []string{"1", "2", "3", "4"}, searchSubset([]string{"a", "b", "c"}))
	assertdeep.Equal(t, []string{"1", "3"}, searchExact([]string{"a", "b", "c"}))
	assertdeep.Equal(t, []string{"1", "2", "3", "4", "5"}, searchSubset([]string{"d", "b", "a", "c"}))
	assertdeep.Equal(t, []string{}, searchExact([]string{"d", "b", "a", "c"}))

	trie.Delete([]string{"c", "b", "a"}, "3")
	assert.Equal(t, 4, trie.Len())

	assertdeep.Equal(t, []string{"1", "2", "4"}, searchSubset([]string{"a", "b", "c"}))
	assertdeep.Equal(t, []string{"1", "2", "4", "5"}, searchSubset([]string{"d", "b", "a", "c"}))
}

func TestString(t *testing.T) {
	trie := New()
	assert.Equal(t, "Trie(Node([], {}))", trie.String())
	trie.Insert([]string{"a"}, "1")
	assert.Equal(t, `Trie(Node([], {
  "a": Node([1], {}),
}))`, trie.String())
	trie.Insert([]string{"b", "c"}, "2")
	assert.Equal(t, `Trie(Node([], {
  "a": Node([1], {}),
  "b": Node([], {
    "c": Node([2], {}),
  }),
}))`, trie.String())
}

func TestLen(t *testing.T) {
	trie := New()
	assert.Equal(t, 0, trie.Len())
	trie.Insert([]string{"a"}, "1")
	assert.Equal(t, 1, trie.Len())
	trie.Insert([]string{"b", "c"}, "2")
	assert.Equal(t, 2, trie.Len())
	trie.Delete([]string{"a"}, "1")
	trie.Delete([]string{"b", "c"}, "2")
	assert.Equal(t, 0, trie.Len())
}
