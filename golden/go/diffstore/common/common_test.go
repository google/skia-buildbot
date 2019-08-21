package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Arbitrary MD5 digests
	imgOne = types.Digest("098f6bcd4621d373cade4e832627b4f6")
	imgTwo = types.Digest("1660f0783f4076284bc18c5f4bdc9608")

	exampleDiffID = "098f6bcd4621d373cade4e832627b4f6-1660f0783f4076284bc18c5f4bdc9608"
)

func TestDiffID(t *testing.T) {
	unittest.SmallTest(t)

	diOne := DiffID(imgOne, imgTwo)
	diTwo := DiffID(imgTwo, imgOne)
	assert.Equal(t, diOne, diTwo)
	assert.Equal(t, exampleDiffID, diOne)
}

func TestSplitDiffID(t *testing.T) {
	unittest.SmallTest(t)

	actualDiffID := DiffID(imgOne, imgTwo)
	actualLeft, actualRight := SplitDiffID(exampleDiffID)
	assert.Equal(t, exampleDiffID, actualDiffID)
	assert.Equal(t, imgOne, actualLeft)
	assert.Equal(t, imgTwo, actualRight)
}
