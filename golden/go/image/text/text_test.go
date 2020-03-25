package text

import (
	"bytes"
	"image"
	"testing"

	"go.skia.org/infra/go/testutils/unittest"
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
	unittest.SmallTest(t)
	buf := bytes.NewBufferString(validImage)
	img, err := Decode(buf)
	if err != nil {
		t.Fatalf("Failed to decode a valid image: %s", err)
	}
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
	unittest.SmallTest(t)
	buf := bytes.NewBufferString(grayscaleNotationImage)
	img, err := Decode(buf)
	if err != nil {
		t.Fatalf("Failed to decode a valid image: %s", err)
	}
	assertImageEqualsExpectedPixels(t, img.(*image.NRGBA), 2, 2, grayscaleNotationImageExpectedPixels)
}

const zeroImage = `! SKTEXTSIMPLE
0 0
`

func TestDecode_ZeroImage_Success(t *testing.T) {
	unittest.SmallTest(t)
	buf := bytes.NewBufferString(zeroImage)
	img, err := Decode(buf)
	if err != nil {
		t.Fatalf("Failed to decode a valid image: %s", err)
	}
	if got, want := img.Bounds().Dx(), 0; got != want {
		t.Errorf("Wrong x dim: Got %v Want %v", got, want)
	}
	if got, want := img.Bounds().Dy(), 0; got != want {
		t.Errorf("Wrong y dim: Got %v Want %v", got, want)
	}
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
	unittest.SmallTest(t)
	for _, tc := range []string{badImage1, badImage2, badImage3, badImage4, badImage5} {
		buf := bytes.NewBufferString(tc)
		_, err := Decode(buf)
		if err == nil {
			t.Fatalf("Decoded an invalid image: %s", tc)
		}
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
	unittest.SmallTest(t)
	for _, tc := range []string{zeroImage, validImage, nonSquareImage, nonSquareImage2} {
		buf := bytes.NewBufferString(tc)
		img, err := Decode(buf)
		if err != nil {
			t.Fatalf("Failed to decode a valid image: %s", tc)
		}
		wbuf := &bytes.Buffer{}
		if err := Encode(wbuf, img.(*image.NRGBA)); err != nil {
			t.Fatalf("Decoded an encode a valid image: %s", tc)
		}
		if got, want := wbuf.String(), tc; got != want {
			t.Errorf("Roundtrip mismatch: Got %q Want %q", got, want)
		}
	}
}

func assertImageEqualsExpectedPixels(t *testing.T, nrgba *image.NRGBA, expectedWidth, expectedHeight int, expectedPixels []expectedPixel) {
	if got, want := nrgba.Bounds().Dx(), expectedWidth; got != want {
		t.Errorf("Wrong x dim: Got %v Want %v", got, want)
	}
	if got, want := nrgba.Bounds().Dy(), expectedHeight; got != want {
		t.Errorf("Wrong y dim: Got %v Want %v", got, want)
	}

	for _, p := range expectedPixels {
		c := nrgba.NRGBAAt(p.x, p.y)
		if got, want := c.R, uint8(p.r); got != want {
			t.Errorf("Wrong r channel value: Got %x Want %x", got, want)
		}
		if got, want := c.G, uint8(p.g); got != want {
			t.Errorf("Wrong g channel value: Got %x Want %x", got, want)
		}
		if got, want := c.B, uint8(p.b); got != want {
			t.Errorf("Wrong b channel value: Got %x Want %x", got, want)
		}
		if got, want := c.A, uint8(p.a); got != want {
			t.Errorf("Wrong a channel value: Got %x Want %x", got, want)
		}
	}
}
