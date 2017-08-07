package dynamicdiff

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
)

func TestIsDynamicContentPixel(t *testing.T) {
	testutils.SmallTest(t)
	assert.True(t, isDynamicContentPixel(0, 255, 255))
	assert.False(t, isDynamicContentPixel(128, 128, 128))
}

func TestDeltaOffset(t *testing.T) {
	testutils.SmallTest(t)
	assert.Equal(t, 6, deltaOffset(765))
	assert.Equal(t, 2, deltaOffset(256))
	assert.Equal(t, 0, deltaOffset(1))
}

func TestDynamicContentDiff(t *testing.T) {
	testutils.SmallTest(t)

	left := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	left.SetNRGBA(0, 0, color.NRGBA{0, 255, 255, 255})
	left.SetNRGBA(0, 1, color.NRGBA{7, 7, 7, 255})

	right := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	right.SetNRGBA(0, 1, color.NRGBA{7, 7, 7, 255})
	right.SetNRGBA(1, 0, color.NRGBA{128, 128, 128, 255})
	right.SetNRGBA(1, 1, color.NRGBA{0, 255, 255, 255})

	// Calculate the diff. Only two pixels are not cyan and out of those, only one
	// is different.
	diffMetrics, diffImg := DynamicContentDiff(left, right)

	// Verify the diff image is correct.
	expectedImg := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	expectedImg.SetNRGBA(0, 0, color.NRGBA{0, 255, 255, 255})
	expectedImg.SetNRGBA(1, 0, color.NRGBA{241, 105, 19, 255})
	expectedImg.SetNRGBA(1, 1, color.NRGBA{0, 255, 255, 255})
	assert.Equal(t, expectedImg, diffImg)

	// Verify the diff metrics are correct.
	expectedDiffMetrics := &DynamicDiffMetrics{
		NumDiffPixels:    1,
		PixelDiffPercent: 50,
		MaxRGBDiffs:      []int{128, 128, 128},
		NumStaticPixels:  2,
		NumDynamicPixels: 2,
	}
	assert.Equal(t, expectedDiffMetrics, diffMetrics)
}
