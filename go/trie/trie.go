package trie

import (
	"fmt"
	"sort"

	"go.skia.org/infra/go/util"
)

// Trie is a struct used for efficient searching on sets of strings.
type Trie struct {
	root *trieNode
}

// New returns a Trie instance.
func New() *Trie {
	return &Trie{
		root: newTrieNode(),
	}
}

func sorted(s []string) []string {
	cpy := make([]string, len(s))
	copy(cpy, s)
	sort.Strings(cpy)
	return cpy
}

// Insert inserts the data into the trie with the given string keys.
func (t *Trie) Insert(strs []string, data interface{}) {
	t.root.Insert(sorted(strs), data)
}

// Delete removes the data from the trie.
func (t *Trie) Delete(strs []string, data interface{}) {
	t.root.Delete(sorted(strs), data)
}

// Search returns all inserted data which exactly matches the given string keys.
func (t *Trie) Search(strs []string) []interface{} {
	return t.root.Search(sorted(strs))
}

type searchContext struct {
	count int
	data  [][]interface{}
}

// SearchSubset returns all inserted data which matches a subset of the given
// string keys.
func (t *Trie) SearchSubset(strs []string) []interface{} {
	keys := make(map[string]bool, len(strs))
	for _, k := range strs {
		keys[k] = true
	}
	ctx := searchContext{
		count: 0,
		data:  [][]interface{}{},
	}
	t.root.SearchSubset(keys, &ctx)
	rv := make([]interface{}, ctx.count)
	idx := 0
	for _, d := range ctx.data {
		copy(rv[idx:idx+len(d)], d)
		idx += len(d)
	}
	return rv
}

// String returns a string representation of the Trie.
func (t *Trie) String() string {
	return fmt.Sprintf("Trie(%s)", t.root.String(0))
}

func (t *Trie) Len() int {
	return t.root.Len()
}

type trieNode struct {
	children map[string]*trieNode
	data     []interface{}
}

func newTrieNode() *trieNode {
	return &trieNode{
		children: map[string]*trieNode{},
		data:     []interface{}{},
	}
}

func (n *trieNode) Insert(strs []string, data interface{}) {
	if len(strs) == 0 {
		n.data = append(n.data, data)
	} else {
		child, ok := n.children[strs[0]]
		if !ok {
			child = newTrieNode()
			n.children[strs[0]] = child
		}
		child.Insert(strs[1:], data)
	}
}

func (n *trieNode) Delete(strs []string, data interface{}) {
	if len(strs) == 0 {
		idx := -1
		for i, v := range n.data {
			if v == data {
				idx = i
			}
		}
		if idx == -1 {
			return
		}
		n.data = append(n.data[:idx], n.data[idx+1:]...)
	} else if child, ok := n.children[strs[0]]; ok {
		child.Delete(strs[1:], data)
	}
}

func (n *trieNode) Search(strs []string) []interface{} {
	if len(strs) == 0 {
		return n.data
	} else {
		if child, ok := n.children[strs[0]]; ok {
			return child.Search(strs[1:])
		} else {
			return []interface{}{}
		}
	}
}

func (n *trieNode) SearchSubset(strs map[string]bool, ctx *searchContext) {
	ctx.count += len(n.data)
	ctx.data = append(ctx.data, n.data)
	for k, c := range n.children {
		if strs[k] {
			c.SearchSubset(strs, ctx)
		}
	}
}

func (n *trieNode) String(indent int) string {
	rv := fmt.Sprintf("Node(%v, {", n.data)
	if len(n.children) == 0 {
		return rv + "})"
	}
	rv += "\n"
	childKeys := make([]string, 0, len(n.children))
	for k := range n.children {
		childKeys = append(childKeys, k)
	}
	sort.Strings(childKeys)
	for _, k := range childKeys {
		rv += fmt.Sprintf("%s\"%s\": %s,\n", util.RepeatJoin("  ", "", indent+1), k, n.children[k].String(indent+1))
	}
	rv += fmt.Sprintf("%s})", util.RepeatJoin("  ", "", indent))
	return rv
}

func (n *trieNode) Len() int {
	rv := len(n.data)
	for _, child := range n.children {
		rv += child.Len()
	}
	return rv
}
