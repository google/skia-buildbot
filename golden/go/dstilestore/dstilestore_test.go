package dstilestore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrace(t *testing.T) {
	trace := NewTrace()
	assert.Equal(t, []string{}, trace.Digests)
	assert.Equal(t, []int{0, 0, 0, 0}, trace.Trace[:4])

	trace.Add("123", 0)
	assert.Equal(t, []string{"123"}, trace.Digests)
	assert.Equal(t, []int{1, 0, 0, 0}, trace.Trace[:4])

	trace.Add("123", 2)
	assert.Equal(t, []string{"123"}, trace.Digests)
	assert.Equal(t, []int{1, 0, 1, 0}, trace.Trace[:4])

	trace.Add("456", 1)
	assert.Equal(t, []string{"123", "456"}, trace.Digests)
	assert.Equal(t, []int{1, 2, 1, 0}, trace.Trace[:4])

	trace.Add("789", 53)
	assert.Equal(t, []string{"123", "456", "789"}, trace.Digests)
	assert.Equal(t, []int{1, 2, 1, 3}, trace.Trace[:4])

	gt := trace.AsGoldenTrace(",arch=x86,config=8888,")
	assert.Equal(t, map[string]string{"arch": "x86", "config": "8888"}, gt.Params_)
	assert.Equal(t, []string{"123", "456", "123", "789", ""}, gt.Values[:5])
}
