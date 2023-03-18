package positive_if_only_image

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/golden/go/image/text"
)

func TestMatcher_NoExistingPostitiveImage_ReturnsTrue(t *testing.T) {
	matcher := Matcher{}
	assert.True(t, matcher.Match(nil, text.MustToNRGBA(image3x3White)))
}

func TestMatcher_IdenticalImages_ReturnsTrue(t *testing.T) {
	matcher := Matcher{}
	assert.True(t, matcher.Match(text.MustToNRGBA(image3x3White), text.MustToNRGBA(image3x3White)))
}

func TestMatcher_DifferentImages_ReturnsFalse(t *testing.T) {
	matcher := Matcher{}
	assert.False(t, matcher.Match(text.MustToNRGBA(image3x3White), text.MustToNRGBA(image3x3WhiteWithOnePixelBlack)))
	assert.Equal(t, &image.Point{X: 1, Y: 1}, matcher.LastDifferentPixelFound())

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
