package exact

import (
	"fmt"
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/golden/go/image/text"
)

func TestMatcher_IdenticalImages_ReturnsTrue(t *testing.T) {
	testIdenticalImages(t, "3x3 white image", image3x3White)
	testIdenticalImages(t, "3x3 white image with one pixel black", image3x3WhiteWithOnePixelBlack)
	testIdenticalImages(t, "3x4 white image", image3x4White)
	testIdenticalImages(t, "4x3 white image", image4x3White)
}

func testIdenticalImages(t *testing.T, name, inputImage string) {
	t.Run(name, func(t *testing.T) {
		img := text.MustToNRGBA(inputImage)
		matcher := Matcher{}
		assert.True(t, matcher.Match(img, img))
		assert.Nil(t, matcher.LastDifferentPixelFound())
	})
}

func TestMatcher_DifferentImages_ReturnsFalse(t *testing.T) {
	testDifferentImages(t, "3x3 vs 3x4 image", image3x3White, image3x4White, nil)
	testDifferentImages(t, "3x3 vs 4x3 image", image3x3White, image4x3White, nil)
	testDifferentImages(t, "one pixel different", image3x3White, image3x3WhiteWithOnePixelBlack, &image.Point{X: 1, Y: 1})
}

func testDifferentImages(t *testing.T, name, inputImage1, inputImage2 string, lastDifferentPixelFound *image.Point) {
	img1 := text.MustToNRGBA(inputImage1)
	img2 := text.MustToNRGBA(inputImage2)

	t.Run(fmt.Sprintf("%s, inputImage1 vs inputImage 2", name), func(t *testing.T) {
		matcher := Matcher{}
		assert.False(t, matcher.Match(img1, img2))
		assert.Equal(t, lastDifferentPixelFound, matcher.LastDifferentPixelFound())
	})

	t.Run(fmt.Sprintf("%s, inputImage2 vs inputImage 1", name), func(t *testing.T) {
		matcher := Matcher{}
		assert.False(t, matcher.Match(img2, img1))
		assert.Equal(t, lastDifferentPixelFound, matcher.LastDifferentPixelFound())
	})
}

const image3x3White = `! SKTEXTSIMPLE
3 3
0xFF 0xFF 0xFF
0xFF 0xFF 0xFF
0xFF 0xFF 0xFF`

const image3x3WhiteWithOnePixelBlack = `! SKTEXTSIMPLE
3 3
0xFF 0xFF 0xFF
0xFF 0x00 0xFF
0xFF 0xFF 0xFF`

const image3x4White = `! SKTEXTSIMPLE
3 4
0xFF 0xFF 0xFF
0xFF 0xFF 0xFF
0xFF 0xFF 0xFF
0xFF 0xFF 0xFF`

const image4x3White = `! SKTEXTSIMPLE
4 3
0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF`
