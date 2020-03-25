package text

import (
	"bytes"
	"image"
	"testing"

	"go.skia.org/infra/go/testutils/unittest"
)

const IMAGE = `! SKTEXTSIMPLE
2 2
0x112233ff 0xffffffff
0xddeeff00 0xffffff88`

func TestDecode_ValidImage_Success(t *testing.T) {
	unittest.SmallTest(t)
	buf := bytes.NewBufferString(IMAGE)
	img, err := Decode(buf)
	if err != nil {
		t.Fatalf("Failed to decode a valid image: %s", err)
	}

	if got, want := img.Bounds().Dx(), 2; got != want {
		t.Errorf("Wrong x dim: Got %v Want %v", got, want)
	}
	if got, want := img.Bounds().Dy(), 2; got != want {
		t.Errorf("Wrong y dim: Got %v Want %v", got, want)
	}
	nrgba := img.(*image.NRGBA)

	testCases := []struct {
		x, y       int
		r, g, b, a uint8
	}{
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
	for _, tc := range testCases {
		c := nrgba.NRGBAAt(tc.x, tc.y)
		if got, want := c.R, uint8(tc.r); got != want {
			t.Errorf("Wrong r channel value: Got %x Want %x", got, want)
		}
		if got, want := c.G, uint8(tc.g); got != want {
			t.Errorf("Wrong g channel value: Got %x Want %x", got, want)
		}
		if got, want := c.B, uint8(tc.b); got != want {
			t.Errorf("Wrong b channel value: Got %x Want %x", got, want)
		}
		if got, want := c.A, uint8(tc.a); got != want {
			t.Errorf("Wrong a channel value: Got %x Want %x", got, want)
		}
	}
}

const ZERO_IMAGE = `! SKTEXTSIMPLE
0 0
`

func TestDecode_ZeroImage_Success(t *testing.T) {
	unittest.SmallTest(t)
	buf := bytes.NewBufferString(ZERO_IMAGE)
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

const BAD_IMAGE_1 = ``

const BAD_IMAGE_2 = `! SKTEXTBAD
0 0`

const BAD_IMAGE_3 = `! SKTEXTSIMPLE
1 1
0x112233ff 0xffffffff
0xddeeff00 0xffffff88`

const BAD_IMAGE_4 = `! SKTEXTSIMPLE
2 2
0x11       0xffffffff
0xddeeff00 0xffffff88`

const BAD_IMAGE_5 = `! SKTEXTSIMPLE
2 2
  112233ff 0xffffffff
0xddeeff00 0xffffff88`

func TestDecode_InvalidImage_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	for _, tc := range []string{BAD_IMAGE_1, BAD_IMAGE_2, BAD_IMAGE_3, BAD_IMAGE_4, BAD_IMAGE_5} {
		buf := bytes.NewBufferString(tc)
		_, err := Decode(buf)
		if err == nil {
			t.Fatalf("Decodede an invalid image: %s", tc)
		}
	}
}

const NON_SQUARE_IMAGE = `! SKTEXTSIMPLE
2 3
0x112233ff 0xffffffff
0xddeeff00 0xffffff88
0x001100ff 0x11001188`

const NON_SQUARE_IMAGE_2 = `! SKTEXTSIMPLE
1 3
0x112233ff
0xddeeff00
0x001100ff`

func TestDecodeThenEncode_ReturnsTheSameImage(t *testing.T) {
	unittest.SmallTest(t)
	for _, tc := range []string{ZERO_IMAGE, IMAGE, NON_SQUARE_IMAGE, NON_SQUARE_IMAGE_2} {
		buf := bytes.NewBufferString(tc)
		img, err := Decode(buf)
		if err != nil {
			t.Fatalf("Failed to decode a valid image: %s", tc)
		}
		wbuf := &bytes.Buffer{}
		if err := Encode(wbuf, img.(*image.NRGBA)); err != nil {
			t.Fatalf("Decodede an encode a valid image: %s", tc)
		}
		if got, want := wbuf.String(), tc; got != want {
			t.Errorf("Roundtrip mismatch: Got %q Want %q", got, want)
		}
	}
}
