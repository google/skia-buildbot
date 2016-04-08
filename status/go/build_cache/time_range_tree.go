package build_cache

import (
	"time"

	"github.com/petar/GoLLRB/llrb"
)

// treeNode is a struct used as a node in a LLRB tree. It implements the
// llrb.Item interface.
type treeNode struct {
	key    int64
	values []string
}

func (n *treeNode) Less(b llrb.Item) bool {
	return n.key < b.(*treeNode).key
}

// TimeRangeTree is a struct used for indexing builds by timestamp. It stores
// BuildIDs in a tree ordered by time and supports querying for builds within
// a time range.
type TimeRangeTree struct {
	tree *llrb.LLRB
}

// NewTimeRangeTree returns a TimeRangeTree instance.
func NewTimeRangeTree() *TimeRangeTree {
	return &TimeRangeTree{
		tree: llrb.New(),
	}

}

// Insert inserts the given BuildID into the TimeRangeTree.
func (t *TimeRangeTree) Insert(k time.Time, v string) {
	ts := k.UnixNano()
	newItem := &treeNode{
		key:    ts,
		values: []string{v},
	}
	item := t.tree.Get(newItem)
	if item != nil {
		node := item.(*treeNode)
		node.values = append(node.values, v)
	} else {
		item = newItem
	}
	t.tree.ReplaceOrInsert(item)
}

// Delete removes the given BuildID from the TimeRangeTree.
func (t *TimeRangeTree) Delete(k time.Time, v string) string {
	rv := ""
	ts := k.UnixNano()
	item := t.tree.Get(&treeNode{
		key: ts,
	})
	if item != nil {
		node := item.(*treeNode)
		i := -1
		for idx, val := range node.values {
			if v == val {
				i = idx
				break
			}
		}
		if i == -1 {
			return ""
		}
		rv = node.values[i]
		node.values = append(node.values[:i], node.values[i+1:]...)
		if len(node.values) == 0 {
			t.tree.Delete(node)
		} else {
			t.tree.ReplaceOrInsert(node)
		}
	}
	return rv
}

// GetRange returns BuildIDs within the given time range.
func (t *TimeRangeTree) GetRange(from, to time.Time) []string {
	rv := []string{}
	a := &treeNode{
		key: from.UnixNano(),
	}
	b := &treeNode{
		key: to.UnixNano(),
	}
	t.tree.AscendRange(a, b, func(v llrb.Item) bool {
		rv = append(rv, v.(*treeNode).values...)
		return true
	})
	return rv
}
