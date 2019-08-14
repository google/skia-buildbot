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
	// Arbitrary MD5 digest.
	imgOne = types.Digest("098f6bcd4621d373cade4e832627b4f6")

	// PNG extension.
	png = ".png"
)

func TestImagePaths(t *testing.T) {
	unittest.SmallTest(t)

	dm := New(&testutils.DummyDiffMetrics{})

	expectedLocalPath := filepath.Join("09", "8f", string(imgOne)+png)
	expectedGSPath := string(imgOne + png)
	localPath, gsPath := dm.ImagePaths(imgOne)
	assert.Equal(t, expectedLocalPath, localPath)
	assert.Equal(t, expectedGSPath, gsPath)
}
