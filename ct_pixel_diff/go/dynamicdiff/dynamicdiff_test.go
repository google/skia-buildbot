package dynamicdiff

import (
	"image"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/diff"
)

func TestDeltaOffset(t *testing.T) {
	testutils.SmallTest(t)
	assert.Equal(t, 6, deltaOffset(765))
	assert.Equal(t, 2, deltaOffset(256))
	assert.Equal(t, 0, deltaOffset(1))
}

func TestDynamicContentDiff(t *testing.T) {
	testutils.MediumTest(t)

	// Get the test images.
	left := decodeImage(t, "testdata/http___www_amazon_com.png")
	right := decodeImage(t, "testdata/http___amazon_com.png")

	// Calculate the diff.
	diffMetrics, diffImg := DynamicContentDiff(left, right)

	// Verify that the diff image and diff metrics are correct.
	expectedImg := decodeImage(t, "testdata/diff.png")
	assert.Equal(t, expectedImg, diffImg)
	expectedDiffMetrics := &diff.DiffMetrics{
		NumDiffPixels:    538919,
		PixelDiffPercent: 70.462585,
		MaxRGBADiffs:     []int{255, 255, 255},
	}
	assert.Equal(t, expectedDiffMetrics, diffMetrics)
}

func decodeImage(t *testing.T, path string) image.Image {
	reader, err := os.Open(path)
	assert.NoError(t, err)
	defer reader.Close()

	image, _, err := image.Decode(reader)
	assert.NoError(t, err)
	return image
}
