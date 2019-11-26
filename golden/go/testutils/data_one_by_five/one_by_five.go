// Package data_one_by_five contains some sample images that are in the text format and are (mostly) one
// pixel wide by five pixels tall. It should be used for testing only.
package data_one_by_five

const (
	// ImageOne is a simple 1x5 image of a completely transparent black pixel, then three completely
	// transparent nearly-black pixels (with one value in each of red, then green, then blue)
	// and finally a nearly-transparent black pixel (one value of alpha).
	ImageOne = `! SKTEXTSIMPLE
	1 5
	0x00000000
	0x01000000
	0x00010000
	0x00000100
	0x00000001`

	// ImageTwo is different from ImageOne by one value in one channel per pixel.
	ImageTwo = `! SKTEXTSIMPLE
	1 5
	0x01000000
	0x02000000
	0x00020000
	0x00000200
	0x00000002`

	// ImageThree is different from ImageOne by six values in one channel per pixel.
	ImageThree = `! SKTEXTSIMPLE
	1 5
	0x06000000
	0x07000000
	0x00070000
	0x00000700
	0x00000007`

	// ImageFour is made up entirely of completely white opaque pixels.
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

	// DiffImageTwoAndFive is the computed image between ImageTwo and ImageFive. Note they
	// are different dimensions, so the diff's dimensions are the biggest width and height
	// of the two inputs.
	DiffImageTwoAndFive = `! SKTEXTSIMPLE
	5 5
	0x00000000 0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff
	0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff
	0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff
	0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff
	0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff`

	// DiffNone is the computed diff between two identical images. It
	DiffNone = `! SKTEXTSIMPLE
	1 5
	0x00000000
	0x00000000
	0x00000000
	0x00000000
	0x00000000`
)
