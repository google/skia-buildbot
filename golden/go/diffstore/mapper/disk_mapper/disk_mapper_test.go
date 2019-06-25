package disk_mapper

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diffstore/testutils"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Arbitrary MD5 digests
	imgOne = types.Digest("098f6bcd4621d373cade4e832627b4f6")
	imgTwo = types.Digest("1660f0783f4076284bc18c5f4bdc9608")

	exampleDiffId = "098f6bcd4621d373cade4e832627b4f6-1660f0783f4076284bc18c5f4bdc9608"

	// PNG extension.
	png = ".png"
)

func TestDiffPath(t *testing.T) {
	unittest.SmallTest(t)

	dm := New(&testutils.DummyDiffMetrics{})

	actualDiffPath := dm.DiffPath(imgOne, imgTwo)
	expectedPath := filepath.Join("09", "8f", exampleDiffId+png)
	assert.Equal(t, expectedPath, actualDiffPath)
}

func TestImagePaths(t *testing.T) {
	unittest.SmallTest(t)

	dm := New(&testutils.DummyDiffMetrics{})

	expectedLocalPath := filepath.Join("09", "8f", string(imgOne)+png)
	expectedGSPath := string(imgOne + png)
	localPath, gsPath := dm.ImagePaths(imgOne)
	assert.Equal(t, expectedLocalPath, localPath)
	assert.Equal(t, expectedGSPath, gsPath)
}
