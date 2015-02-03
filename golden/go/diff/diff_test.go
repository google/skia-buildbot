package diff

import (
	"image"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/skia-dev/glog"
)

const (
	TESTDATA_DIR = "testdata"
)

func TestDiff(t *testing.T) {
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
