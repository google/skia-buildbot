package diff

import (
	"bytes"
	"image"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/golden/go/image/text"
)

const (
	TESTDATA_DIR = "testdata"
)

func TestDiffMetrics(t *testing.T) {
	// Assert different images with the same dimensions.
	diffFilePath1 := filepath.Join(os.TempDir(), "diff1.png")
	defer os.Remove(diffFilePath1)
	assertDiffs(t, "4029959456464745507", "16465366847175223174",
		&DiffMetrics{
			NumDiffPixels:              16,
			PixelDiffPercent:           0.0064,
			PixelDiffFilePath:          "",
			ThumbnailPixelDiffFilePath: "",
			MaxRGBADiffs:               []int{54, 100, 125, 0},
			DimDiffer:                  false})
	diffFilePath2 := filepath.Join(os.TempDir(), "diff2.png")
	defer os.Remove(diffFilePath2)
	assertDiffs(t, "5024150605949408692", "11069776588985027208",
		&DiffMetrics{
			NumDiffPixels:              2233,
			PixelDiffPercent:           0.8932,
			PixelDiffFilePath:          "",
			ThumbnailPixelDiffFilePath: "",
			MaxRGBADiffs:               []int{0, 0, 1, 0},
			DimDiffer:                  false})
	// Assert the same image.
	diffFilePath3 := filepath.Join(os.TempDir(), "diff3.png")
	defer os.Remove(diffFilePath3)
	assertDiffs(t, "5024150605949408692", "5024150605949408692",
		&DiffMetrics{
			NumDiffPixels:              0,
			PixelDiffPercent:           0,
			PixelDiffFilePath:          "",
			ThumbnailPixelDiffFilePath: "",
			MaxRGBADiffs:               []int{0, 0, 0, 0},
			DimDiffer:                  false})
	// Assert different images with different dimensions.
	diffFilePath4 := filepath.Join(os.TempDir(), "diff4.png")
	defer os.Remove(diffFilePath4)
	assertDiffs(t, "ffce5042b4ac4a57bd7c8657b557d495", "fffbcca7e8913ec45b88cc2c6a3a73ad",
		&DiffMetrics{
			NumDiffPixels:              571674,
			PixelDiffPercent:           89.324066,
			PixelDiffFilePath:          "",
			ThumbnailPixelDiffFilePath: "",
			MaxRGBADiffs:               []int{255, 255, 255, 0},
			DimDiffer:                  true})
	// Assert with images that match in dimensions but where all pixels differ.
	diffFilePath5 := filepath.Join(os.TempDir(), "diff5.png")
	defer os.Remove(diffFilePath5)
	assertDiffs(t, "4029959456464745507", "4029959456464745507-inverted",
		&DiffMetrics{
			NumDiffPixels:              250000,
			PixelDiffPercent:           100.0,
			PixelDiffFilePath:          "",
			ThumbnailPixelDiffFilePath: "",
			MaxRGBADiffs:               []int{255, 255, 255, 0},
			DimDiffer:                  false})

	// Assert different images where neither fits into the other.
	diffFilePath6 := filepath.Join(os.TempDir(), "diff6.png")
	defer os.Remove(diffFilePath6)
	assertDiffs(t, "fffbcca7e8913ec45b88cc2c6a3a73ad", "fffbcca7e8913ec45b88cc2c6a3a73ad-rotated",
		&DiffMetrics{
			NumDiffPixels:              172466,
			PixelDiffPercent:           74.8550347222,
			PixelDiffFilePath:          "",
			ThumbnailPixelDiffFilePath: "",
			MaxRGBADiffs:               []int{255, 255, 255, 0},
			DimDiffer:                  true})
	// Make sure the metric is symmetric.
	diffFilePath7 := filepath.Join(os.TempDir(), "diff7.png")
	defer os.Remove(diffFilePath7)
	assertDiffs(t, "fffbcca7e8913ec45b88cc2c6a3a73ad-rotated", "fffbcca7e8913ec45b88cc2c6a3a73ad",
		&DiffMetrics{
			NumDiffPixels:              172466,
			PixelDiffPercent:           74.8550347222,
			PixelDiffFilePath:          "",
			ThumbnailPixelDiffFilePath: "",
			MaxRGBADiffs:               []int{255, 255, 255, 0},
			DimDiffer:                  true})

	// Compare two images where one has an alpha channel and the other doesn't.
	diffFilePath8 := filepath.Join(os.TempDir(), "diff8.png")
	defer os.Remove(diffFilePath8)
	assertDiffs(t, "b716a12d5b98d04b15db1d9dd82c82ea", "df1591dde35907399734ea19feb76663",
		&DiffMetrics{
			NumDiffPixels:              8750,
			PixelDiffPercent:           2.8483074,
			PixelDiffFilePath:          "",
			ThumbnailPixelDiffFilePath: "",
			MaxRGBADiffs:               []int{255, 2, 255, 0},
			DimDiffer:                  false})

	// Compare two images where the alpha differs.
	diffFilePath9 := filepath.Join(os.TempDir(), "diff9.png")
	defer os.Remove(diffFilePath9)
	assertDiffs(t, "df1591dde35907399734ea19feb76663", "df1591dde35907399734ea19feb76663-6-alpha-diff",
		&DiffMetrics{
			NumDiffPixels:              6,
			PixelDiffPercent:           0.001953125,
			PixelDiffFilePath:          "",
			ThumbnailPixelDiffFilePath: "",
			MaxRGBADiffs:               []int{0, 0, 0, 235},
			DimDiffer:                  false})
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
func assertDiffMatch(t *testing.T, expected, src1, src2 string) {
	_, got := Diff(imageFromString(t, src1), imageFromString(t, src2))
	want := imageFromString(t, expected)
	assertImagesEqual(t, got, want)
}

// TestDiffImages tests that the diff images produced are correct.
func TestDiffImages(t *testing.T) {
	assertDiffMatch(t, EXPECTED_NO_DIFF, SRC1, SRC1)
	assertDiffMatch(t, EXPECTED_NO_DIFF, SRC2, SRC2)
	assertDiffMatch(t, EXPECTED_1_2, SRC1, SRC2)
	assertDiffMatch(t, EXPECTED_1_2, SRC2, SRC1)
	assertDiffMatch(t, EXPECTED_1_3, SRC3, SRC1)
	assertDiffMatch(t, EXPECTED_1_3, SRC1, SRC3)
	assertDiffMatch(t, EXPECTED_1_4, SRC1, SRC4)
	assertDiffMatch(t, EXPECTED_1_4, SRC4, SRC1)
}

// assertDiffs asserts that the DiffMetrics reported by Diffing the two images
// matches the expected DiffMetrics.
func assertDiffs(t *testing.T, d1, d2 string, expectedDiffMetrics *DiffMetrics) {
	img1, err := OpenImage(filepath.Join(TESTDATA_DIR, d1+".png"))
	if err != nil {
		t.Fatal("Failed to open test file: ", err)
	}
	img2, err := OpenImage(filepath.Join(TESTDATA_DIR, d2+".png"))
	if err != nil {
		t.Fatal("Failed to open test file: ", err)
	}

	diffMetrics, _ := Diff(img1, img2)
	if err != nil {
		t.Error("Unexpected error: ", err)
	}
	if got, want := diffMetrics, expectedDiffMetrics; !reflect.DeepEqual(got, want) {
		t.Errorf("Image Diff: Got %v Want %v", got, want)
	}
}

func TestDeltaOffset(t *testing.T) {
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

var (
	img1 image.Image
	img2 image.Image
	once sync.Once
)

func loadBenchmarkImages() {
	var err error
	img1, err = OpenImage(filepath.Join(TESTDATA_DIR, "4029959456464745507.png"))
	if err != nil {
		glog.Fatal("Failed to open test file: ", err)
	}
	img2, err = OpenImage(filepath.Join(TESTDATA_DIR, "16465366847175223174.png"))
	if err != nil {
		glog.Fatal("Failed to open test file: ", err)
	}
}

func BenchmarkDiff(b *testing.B) {
	// Only load the images once so we aren't measuring that as part of the
	// benchmark.
	once.Do(loadBenchmarkImages)

	for i := 0; i < b.N; i++ {
		Diff(img1, img2)
	}
}
