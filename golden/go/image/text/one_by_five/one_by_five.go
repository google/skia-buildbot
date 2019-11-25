// Package one_by_five contains some sample images that are in the text format and are (mostly) one
// pixel wide by five pixels tall. It should be used for testing only.
package one_by_five

import (
	"bytes"
	"fmt"
	"image"

	"go.skia.org/infra/golden/go/image/text"
)

const (
	// ImageOne is a simple 1x5 image of a perfectly transparent black pixel, then 4 almost
	// transparent black pixels, with one value in each of red, then green, then blue, then alpha.
	ImageOne = `! SKTEXTSIMPLE
	1 5
	0x00000000
	0x01000000
	0x00010000
	0x00000100
	0x00000001`

	// ImageTwo is different from ImageOne by one in each channel per pixel.
	ImageTwo = `! SKTEXTSIMPLE
	1 5
	0x01000000
	0x02000000
	0x00020000
	0x00000200
	0x00000002`

	// ImageThree is different in each pixel from ImageOne by 6 in each channel.
	ImageThree = `! SKTEXTSIMPLE
	1 5
	0x06000000
	0x07000000
	0x00070000
	0x00000700
	0x00000007`

	// ImageFour is a perfectly white opaque pixel.
	ImageFour = `! SKTEXTSIMPLE
	1 5
	0xffffffff
	0xffffffff
	0xffffffff
	0xffffffff
	0xffffffff`

	// ImageFive is ImageTwo, but flipped 90 degrees counter clockwise
	ImageFive = `! SKTEXTSIMPLE
	5 1
	0x01000000 0x02000000 0x00020000 0x00000200 0x00000002`

	// ImageSix is different from ImageOne in four of the five pixels by 1 or 2 values
	// per channel.
	ImageSix = `! SKTEXTSIMPLE
	1 5
	0x01000000
	0x03000000
	0x00010000
	0x00000200
	0x00000003`

	// DiffImageOneAndTwo is the computed diff between ImageOne and ImageTwo
	// It should have all the pixels as the pixel diff color with an
	// offset of 1, except the last pixel which is only different in the alpha by
	// an offset of 1.
	DiffImageOneAndTwo = `! SKTEXTSIMPLE
	1 5
	0xfdd0a2ff
	0xfdd0a2ff
	0xfdd0a2ff
	0xfdd0a2ff
	0xc6dbefff`

	// DiffImageOneAndThree is the computed diff between ImageOne and ImageThree.
	// It Should have all the pixels as the pixel diff color with an
	// offset of 6, except the last pixel which is only different in the alpha by
	// an offset of 6.
	DiffImageOneAndThree = `! SKTEXTSIMPLE
	1 5
	0xfd8d3cff
	0xfd8d3cff
	0xfd8d3cff
	0xfd8d3cff
	0x6baed6ff`

	// DiffImageOneAndFour is the computed diff between ImageOne and ImageFour.
	DiffImageOneAndFour = `! SKTEXTSIMPLE
	1 5
	0x7f2704ff
	0x7f2704ff
	0x7f2704ff
	0x7f2704ff
	0x7f2704ff`

	// DiffImageTwoAndFive is the computed image between ImageTwo and ImageFive (note they
	// are different)
	DiffImageTwoAndFive = `! SKTEXTSIMPLE
	5 5
	0x00000000 0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff
	0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff
	0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff
	0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff
	0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff`
)

// AsNRGBA returns an *image.NRGBA from a given string, which is assumed to be an image in the
// SKTEXTSIMPLE "codec".
func AsNRGBA(s string) *image.NRGBA {
	buf := bytes.NewBufferString(s)
	img, err := text.Decode(buf)
	if err != nil {
		// This indicates an error with the static test data.
		panic(fmt.Sprintf("Failed to decode a valid image: %s", err))
	}
	return img.(*image.NRGBA)
}
