package diff

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/image/text"
)

const (
	TESTDATA_DIR = "testdata"
)

func TestDiffMetrics(t *testing.T) {
	unittest.MediumTest(t)
	// Assert different images with the same dimensions.
	assertDiffs(t, "4029959456464745507", "16465366847175223174",
		&DiffMetrics{
			NumDiffPixels:    16,
			PixelDiffPercent: 0.0064,
			MaxRGBADiffs:     [4]int{54, 100, 125, 0},
			DimDiffer:        false})
	assertDiffs(t, "5024150605949408692", "11069776588985027208",
		&DiffMetrics{
			NumDiffPixels:    2233,
			PixelDiffPercent: 0.8932,
			MaxRGBADiffs:     [4]int{0, 0, 1, 0},
			DimDiffer:        false})
	// Assert the same image.
	assertDiffs(t, "5024150605949408692", "5024150605949408692",
		&DiffMetrics{
			NumDiffPixels:    0,
			PixelDiffPercent: 0,
			MaxRGBADiffs:     [4]int{0, 0, 0, 0},
			DimDiffer:        false})
	// Assert different images with different dimensions.
	assertDiffs(t, "ffce5042b4ac4a57bd7c8657b557d495", "fffbcca7e8913ec45b88cc2c6a3a73ad",
		&DiffMetrics{
			NumDiffPixels:    571674,
			PixelDiffPercent: 89.324066,
			MaxRGBADiffs:     [4]int{255, 255, 255, 0},
			DimDiffer:        true})
	// Assert with images that match in dimensions but where all pixels differ.
	assertDiffs(t, "4029959456464745507", "4029959456464745507-inverted",
		&DiffMetrics{
			NumDiffPixels:    250000,
			PixelDiffPercent: 100.0,
			MaxRGBADiffs:     [4]int{255, 255, 255, 0},
			DimDiffer:        false})

	// Assert different images where neither fits into the other.
	assertDiffs(t, "fffbcca7e8913ec45b88cc2c6a3a73ad", "fffbcca7e8913ec45b88cc2c6a3a73ad-rotated",
		&DiffMetrics{
			NumDiffPixels:    172466,
			PixelDiffPercent: 74.8550347222,
			MaxRGBADiffs:     [4]int{255, 255, 255, 0},
			DimDiffer:        true})
	// Make sure the metric is symmetric.
	assertDiffs(t, "fffbcca7e8913ec45b88cc2c6a3a73ad-rotated", "fffbcca7e8913ec45b88cc2c6a3a73ad",
		&DiffMetrics{
			NumDiffPixels:    172466,
			PixelDiffPercent: 74.8550347222,
			MaxRGBADiffs:     [4]int{255, 255, 255, 0},
			DimDiffer:        true})

	// Compare two images where one has an alpha channel and the other doesn't.
	assertDiffs(t, "b716a12d5b98d04b15db1d9dd82c82ea", "df1591dde35907399734ea19feb76663",
		&DiffMetrics{
			NumDiffPixels:    8750,
			PixelDiffPercent: 2.8483074,
			MaxRGBADiffs:     [4]int{255, 2, 255, 0},
			DimDiffer:        false})

	// Compare two images where the alpha differs.
	assertDiffs(t, "df1591dde35907399734ea19feb76663", "df1591dde35907399734ea19feb76663-6-alpha-diff",
		&DiffMetrics{
			NumDiffPixels:    6,
			PixelDiffPercent: 0.001953125,
			MaxRGBADiffs:     [4]int{0, 0, 0, 235},
			DimDiffer:        false})
}

const SRC1 = `! SKTEXTSIMPLE
1 5
0x00000000
0x01000000
0x00010000
0x00000100
0x00000001`

// SRC2 is different in each pixel from SRC1 by one in each channel.
const SRC2 = `! SKTEXTSIMPLE
1 5
0x01000000
0x02000000
0x00020000
0x00000200
0x00000002`

// SRC3 is different in each pixel from SRC1 by 6 in each channel.
const SRC3 = `! SKTEXTSIMPLE
1 5
0x06000000
0x07000000
0x00070000
0x00000700
0x00000007`

const SRC4 = `! SKTEXTSIMPLE
1 5
0xffffffff
0xffffffff
0xffffffff
0xffffffff
0xffffffff`

// SRC2 is different in each pixel from SRC1 by one in each channel.
const SRC5 = `! SKTEXTSIMPLE
5 1
0x01000000 0x02000000 0x00020000 0x00000200 0x00000002`

// EXPECTED_1_2 Should have all the pixels as the pixel diff color with an
// offset of 1, except the last pixel which is only different in the alpha by
// an offset of 1.
const EXPECTED_1_2 = `! SKTEXTSIMPLE
1 5
0xfdd0a2ff
0xfdd0a2ff
0xfdd0a2ff
0xfdd0a2ff
0xc6dbefff`

// EXPECTED_1_3 Should have all the pixels as the pixel diff color with an
// offset of 6, except the last pixel which is only different in the alpha by
// an offet of 6.
const EXPECTED_1_3 = `! SKTEXTSIMPLE
1 5
0xfd8d3cff
0xfd8d3cff
0xfd8d3cff
0xfd8d3cff
0x6baed6ff`

// EXPECTED_1_4 Should have all the pixels as the pixel diff color with an
// offset of 6, except the last pixel which is only different in the alpha by
// an offet of 6.
const EXPECTED_1_4 = `! SKTEXTSIMPLE
1 5
0x7f2704ff
0x7f2704ff
0x7f2704ff
0x7f2704ff
0x7f2704ff`

// EXPECTED_NO_DIFF should be all black transparent since there are no differences.
const EXPECTED_NO_DIFF = `! SKTEXTSIMPLE
1 5
0x00000000
0x00000000
0x00000000
0x00000000
0x00000000`

const EXPECTED_2_5 = `! SKTEXTSIMPLE
5 5
0x00000000 0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff
0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff
0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff
0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff
0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff 0x7f2704ff`

// imageFromString decodes the SKTEXT image from the string.
func imageFromString(t *testing.T, s string) *image.NRGBA {
	buf := bytes.NewBufferString(s)
	img, err := text.Decode(buf)
	if err != nil {
		t.Fatalf("Failed to decode a valid image: %s", err)
	}
	return img.(*image.NRGBA)
}

// lineDiff lists the differences in the lines of a and b.
func lineDiff(t *testing.T, a, b string) {
	aslice := strings.Split(a, "\n")
	bslice := strings.Split(b, "\n")
	if len(aslice) != len(bslice) {
		t.Fatal("Can't diff text, mismatched number of lines.\n")
		return
	}
	for i, s := range aslice {
		if s != bslice[i] {
			t.Errorf("Line %d: %q != %q\n", i+1, s, bslice[i])
		}
	}
}

// assertImagesEqual asserts that the two images are identical.
func assertImagesEqual(t *testing.T, got, want *image.NRGBA) {
	// Do the compare by converting them to sktext format and doing a string
	// compare.
	gotbuf := &bytes.Buffer{}
	if err := text.Encode(gotbuf, got); err != nil {
		t.Fatalf("Failed to encode: %s", err)
	}
	wantbuf := &bytes.Buffer{}
	if err := text.Encode(wantbuf, want); err != nil {
		t.Fatalf("Failed to encode: %s", err)
	}
	if gotbuf.String() != wantbuf.String() {
		t.Errorf("Pixel mismatch:\nGot:\n\n%v\n\nWant:\n\n%v\n", gotbuf, wantbuf)
		// Also print out the lines that are different, to make debugging easier.
		lineDiff(t, gotbuf.String(), wantbuf.String())
	}
}

// assertDiffMatch asserts that you get expected when you diff
// src1 and src2.
//
// Note that all images are in sktext format as strings.
func assertDiffMatch(t *testing.T, expected, src1, src2 string, expectedDiffMetrics ...*DiffMetrics) {
	dm, got := PixelDiff(imageFromString(t, src1), imageFromString(t, src2))
	want := imageFromString(t, expected)
	assertImagesEqual(t, got, want)

	for _, expDM := range expectedDiffMetrics {
		assert.Equal(t, expDM, dm)
	}
}

// TestDiffImages tests that the diff images produced are correct.
func TestDiffImages(t *testing.T) {
	unittest.MediumTest(t)
	assertDiffMatch(t, EXPECTED_NO_DIFF, SRC1, SRC1)
	assertDiffMatch(t, EXPECTED_NO_DIFF, SRC2, SRC2)
	assertDiffMatch(t, EXPECTED_1_2, SRC1, SRC2)
	assertDiffMatch(t, EXPECTED_1_2, SRC2, SRC1)
	assertDiffMatch(t, EXPECTED_1_3, SRC3, SRC1)
	assertDiffMatch(t, EXPECTED_1_3, SRC1, SRC3)
	assertDiffMatch(t, EXPECTED_1_4, SRC1, SRC4)
	assertDiffMatch(t, EXPECTED_1_4, SRC4, SRC1)
	assertDiffMatch(t, EXPECTED_2_5, SRC2, SRC5, &DiffMetrics{
		NumDiffPixels:    24,
		PixelDiffPercent: (24.0 / 25.0) * 100,
		MaxRGBADiffs:     [4]int{0, 0, 0, 0},
		DimDiffer:        true,
	})
}

// assertDiffs asserts that the DiffMetrics reported by Diffing the two images
// matches the expected DiffMetrics.
func assertDiffs(t *testing.T, d1, d2 string, expectedDiffMetrics *DiffMetrics) {
	img1, err := openNRGBAFromFile(filepath.Join(TESTDATA_DIR, d1+".png"))
	if err != nil {
		t.Fatal("Failed to open test file: ", err)
	}
	img2, err := openNRGBAFromFile(filepath.Join(TESTDATA_DIR, d2+".png"))
	if err != nil {
		t.Fatal("Failed to open test file: ", err)
	}

	diffMetrics, _ := PixelDiff(img1, img2)
	assert.Equal(t, expectedDiffMetrics, diffMetrics)
}

func TestDeltaOffset(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		offset int
		want   int
	}{
		{
			offset: 1,
			want:   0,
		},
		{
			offset: 2,
			want:   1,
		},
		{
			offset: 5,
			want:   1,
		},
		{
			offset: 6,
			want:   2,
		},
		{
			offset: 100,
			want:   4,
		},
		{
			offset: 1024,
			want:   6,
		},
	}

	for _, tc := range testCases {
		if got, want := deltaOffset(tc.offset), tc.want; got != want {
			t.Errorf("deltaOffset(%d): Got %v Want %v", tc.offset, got, want)
		}
	}

}

func TestCombinedDiffMetric(t *testing.T) {
	unittest.SmallTest(t)
	dm := &DiffMetrics{
		MaxRGBADiffs:     [4]int{255, 255, 255, 255},
		PixelDiffPercent: 1.0,
	}
	assert.InDelta(t, 1.0, CombinedDiffMetric(dm, nil, nil), 0.000001)
	dm = &DiffMetrics{
		MaxRGBADiffs:     [4]int{255, 255, 255, 255},
		PixelDiffPercent: 0.5,
	}
	assert.InDelta(t, math.Sqrt(0.5), CombinedDiffMetric(dm, nil, nil), 0.000001)
}

func loadBenchmarkImage(fileName string) image.Image {
	img, err := openNRGBAFromFile(filepath.Join(TESTDATA_DIR, fileName))
	if err != nil {
		sklog.Fatal("Failed to open test file: ", err)
	}
	return img
}

func benchmarkDiff(b *testing.B, img1, img2 image.Image) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PixelDiff(img1, img2)
	}
}

const (
	img1 = "4029959456464745507.png"              // 500x500.
	img2 = "4029959456464745507-inverted.png"     // 500x500.
	img3 = "b716a12d5b98d04b15db1d9dd82c82ea.png" // 640x480.
)

func BenchmarkDiffIdentical(b *testing.B) {
	benchmarkDiff(b, loadBenchmarkImage(img1), loadBenchmarkImage(img1))
}

func BenchmarkDiffSameSize(b *testing.B) {
	benchmarkDiff(b, loadBenchmarkImage(img1), loadBenchmarkImage(img2))
}

func BenchmarkDiffDifferentSize(b *testing.B) {
	benchmarkDiff(b, loadBenchmarkImage(img1), loadBenchmarkImage(img3))
}

// openNRGBAFromFile opens the given file path to a PNG file and returns the image as image.NRGBA.
func openNRGBAFromFile(fileName string) (*image.NRGBA, error) {
	var img *image.NRGBA
	err := util.WithReadFile(fileName, func(r io.Reader) error {
		im, err := png.Decode(r)
		if err != nil {
			return err
		}
		img = GetNRGBA(im)
		return nil
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return img, nil
}
