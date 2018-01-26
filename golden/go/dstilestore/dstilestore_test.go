package dstilestore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/dsconst"
)

func TestTrace(t *testing.T) {
	testutils.SmallTest(t)

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

func TestAdd(t *testing.T) {
	testutils.LargeTest(t)

	cleanup := testutil.InitDatastore(t,
		dsconst.TILE,
		dsconst.TEST_NAME,
		dsconst.TRACE)
	defer cleanup()

}
