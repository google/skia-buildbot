package tracefilter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSingleChildSingleParent(t *testing.T) {
	tf := NewTraceFilter()
	tf.AddPath([]string{"root", "p1", "p2", "p3", "t1"}, "key1")
	tf.AddPath([]string{"root", "p1", "p2"}, "key2")

	res := tf.GetLeafNodeTraceKeys()
	assert.Equal(t, 1, len(res))
	assert.Equal(t, "key1", res[0])
}

func TestMultipleChildSingleParent(t *testing.T) {
	tf := NewTraceFilter()
	tf.AddPath([]string{"root", "p1", "p2", "p3"}, "parentKey")
	tf.AddPath([]string{"root", "p1", "p2", "p3", "t1"}, "key1")
	tf.AddPath([]string{"root", "p1", "p2", "p4", "t2"}, "key2")

	res := tf.GetLeafNodeTraceKeys()
	assert.Equal(t, 2, len(res))
	assert.True(t, res[0] == "key1" || res[1] == "key1")
	assert.True(t, res[0] == "key2" || res[1] == "key2")
}

func TestNoParent(t *testing.T) {
	tf := NewTraceFilter()
	tf.AddPath([]string{"root", "p1", "p2", "p3", "t1"}, "key1")

	res := tf.GetLeafNodeTraceKeys()
	assert.Equal(t, 1, len(res))
	assert.Equal(t, "key1", res[0])
}
