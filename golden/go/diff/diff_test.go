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
	one_by_five "go.skia.org/infra/golden/go/testutils/data_one_by_five"
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
	dm, got := PixelDiff(text.MustToNRGBA(src1), text.MustToNRGBA(src2))
	want := text.MustToNRGBA(expected)
	assertImagesEqual(t, got, want)

	for _, expDM := range expectedDiffMetrics {
		assert.Equal(t, expDM, dm)
	}
}

// TestDiffImages tests that the diff images produced are correct.
func TestDiffImages(t *testing.T) {
	unittest.MediumTest(t)
	assertDiffMatch(t, one_by_five.DiffNone, one_by_five.ImageOne, one_by_five.ImageOne)
	assertDiffMatch(t, one_by_five.DiffNone, one_by_five.ImageTwo, one_by_five.ImageTwo)
	assertDiffMatch(t, one_by_five.DiffImageOneAndTwo, one_by_five.ImageOne, one_by_five.ImageTwo)
	assertDiffMatch(t, one_by_five.DiffImageOneAndTwo, one_by_five.ImageTwo, one_by_five.ImageOne)
	assertDiffMatch(t, one_by_five.DiffImageOneAndThree, one_by_five.ImageThree, one_by_five.ImageOne)
	assertDiffMatch(t, one_by_five.DiffImageOneAndThree, one_by_five.ImageOne, one_by_five.ImageThree)
	assertDiffMatch(t, one_by_five.DiffImageOneAndFour, one_by_five.ImageOne, one_by_five.ImageFour)
	assertDiffMatch(t, one_by_five.DiffImageOneAndFour, one_by_five.ImageFour, one_by_five.ImageOne)
	assertDiffMatch(t, one_by_five.DiffImageTwoAndFive, one_by_five.ImageTwo, one_by_five.ImageFive, &DiffMetrics{
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
