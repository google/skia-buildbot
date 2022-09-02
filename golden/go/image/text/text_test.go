package text

import (
	"bytes"
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type expectedPixel struct {
	x, y       int
	r, g, b, a uint8
}

const validImage = `! SKTEXTSIMPLE
2 2
0x112233ff 0xffffffff
0xddeeff00 0xffffff88`

var validImageExpectedPixels = []expectedPixel{
	{
		x: 0, y: 0,
		r: 0x11, g: 0x22, b: 0x33, a: 0xff,
	},
	{
		x: 1, y: 0,
		r: 0xff, g: 0xff, b: 0xff, a: 0xff,
	},
	{
		x: 0, y: 1,
		r: 0xdd, g: 0xee, b: 0xff, a: 0x00,
	},
	{
		x: 1, y: 1,
		r: 0xff, g: 0xff, b: 0xff, a: 0x88,
	},
}

func TestDecode_ValidImage_Success(t *testing.T) {
	buf := bytes.NewBufferString(validImage)
	img, err := Decode(buf)
	require.NoError(t, err)
	assertImageEqualsExpectedPixels(t, img.(*image.NRGBA), 2, 2, validImageExpectedPixels)
}

const grayscaleNotationImage = `! SKTEXTSIMPLE
2 2
0x12 0x34
0xab 0xcd`

var grayscaleNotationImageExpectedPixels = []expectedPixel{
	{
		x: 0, y: 0,
		r: 0x12, g: 0x12, b: 0x12, a: 0xff,
	},
	{
		x: 1, y: 0,
		r: 0x34, g: 0x34, b: 0x34, a: 0xff,
	},
	{
		x: 0, y: 1,
		r: 0xab, g: 0xab, b: 0xab, a: 0xff,
	},
	{
		x: 1, y: 1,
		r: 0xcd, g: 0xcd, b: 0xcd, a: 0xff,
	},
}

func TestDecode_ValidImageWithGrayscaleNotation_Success(t *testing.T) {
	buf := bytes.NewBufferString(grayscaleNotationImage)
	img, err := Decode(buf)
	require.NoError(t, err)
	assertImageEqualsExpectedPixels(t, img.(*image.NRGBA), 2, 2, grayscaleNotationImageExpectedPixels)
}

const zeroImage = `! SKTEXTSIMPLE
0 0
`

func TestDecode_ZeroImage_Success(t *testing.T) {
	buf := bytes.NewBufferString(zeroImage)
	img, err := Decode(buf)
	require.NoError(t, err)
	assert.Equal(t, 0, img.Bounds().Dx())
	assert.Equal(t, 0, img.Bounds().Dy())
}

const badImage1 = ``

const badImage2 = `! SKTEXTBAD
0 0`

const badImage3 = `! SKTEXTSIMPLE
1 1
0x112233ff 0xffffffff
0xddeeff00 0xffffff88`

const badImage4 = `! SKTEXTSIMPLE
2 2
0x11       0xffffffff
0xddeeff00 0xffffff88`

const badImage5 = `! SKTEXTSIMPLE
2 2
  112233ff 0xffffffff
0xddeeff00 0xffffff88`

func TestDecode_InvalidImage_ReturnsError(t *testing.T) {
	for _, tc := range []string{badImage1, badImage2, badImage3, badImage4, badImage5} {
		buf := bytes.NewBufferString(tc)
		_, err := Decode(buf)
		assert.Error(t, err)
	}
}

const nonSquareImage = `! SKTEXTSIMPLE
2 3
0x112233ff 0xffffffff
0xddeeff00 0xffffff88
0x001100ff 0x11001188`

const nonSquareImage2 = `! SKTEXTSIMPLE
1 3
0x112233ff
0xddeeff00
0x001100ff`

func TestDecodeThenEncode_ReturnsTheSameImage(t *testing.T) {
	for _, tc := range []string{zeroImage, validImage, nonSquareImage, nonSquareImage2} {
		// Decode image.
		buf := bytes.NewBufferString(tc)
		img, err := Decode(buf)
		require.NoError(t, err)

		// Encode it as SKTEXT.
		wbuf := &bytes.Buffer{}
		err = Encode(wbuf, img.(*image.NRGBA))
		require.NoError(t, err)

		assert.Equal(t, tc, wbuf.String())
	}
}

func TestMustToNRGBA_ValidImage_Success(t *testing.T) {
	img := MustToNRGBA(validImage)
	assertImageEqualsExpectedPixels(t, img, 2, 2, validImageExpectedPixels)
}

func TestMustToNRGBA_InvalidImage_Panics(t *testing.T) {
	assert.Panics(t, func() { MustToNRGBA(badImage1) })
}

func TestMustToGray_ValidImageWithGrayscaleNotation_Success(t *testing.T) {
	img := MustToGray(grayscaleNotationImage)

	assert.Equal(t, 2, img.Bounds().Dx())
	assert.Equal(t, 2, img.Bounds().Dy())

	for _, p := range grayscaleNotationImageExpectedPixels {
		y := img.GrayAt(p.x, p.y).Y
		assert.Equal(t, y, p.r, "(%v, %v)", p.x, p.y)
		assert.Equal(t, y, p.g, "(%v, %v)", p.x, p.y)
		assert.Equal(t, y, p.b, "(%v, %v)", p.x, p.y)
	}
}

func TestMustToGray_InvalidImage_Panics(t *testing.T) {
	assert.Panics(t, func() { MustToGray(badImage1) })
}

func assertImageEqualsExpectedPixels(t *testing.T, nrgba *image.NRGBA, expectedWidth, expectedHeight int, expectedPixels []expectedPixel) {
	assert.Equal(t, expectedWidth, nrgba.Bounds().Dx())
	assert.Equal(t, expectedHeight, nrgba.Bounds().Dy())

	for _, p := range expectedPixels {
		c := nrgba.NRGBAAt(p.x, p.y)
		assert.Equal(t, c.R, p.r, "(%v, %v)", p.x, p.y)
		assert.Equal(t, c.G, p.g, "(%v, %v)", p.x, p.y)
		assert.Equal(t, c.B, p.b, "(%v, %v)", p.x, p.y)
		assert.Equal(t, c.A, p.a, "(%v, %v)", p.x, p.y)
	}
}
