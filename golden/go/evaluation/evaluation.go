package evaluation

import (
	"image"
	"os"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/tsuite"
)

var ctSuite *tsuite.CompatTestSuite = nil

// Load the test suite from disk.
func Load(testSuiteZipPath string) error {
	f, err := os.Open(testSuiteZipPath)
	if err != nil {
		return err
	}
	defer util.Close(f)

	ctSuite, err = tsuite.Load(f)
	return err
}

// Return the list of available test names.
func TestNames() []string {
	return ctSuite.TestNames()
}

// Evaluate a test. digest is the MD5 hash of the internal pixel buffer
// to speed up evaluation. The returned value is the probability of a pass.
func Evaluate(testName, digest string, img *image.NRGBA) float32 {
	return ctSuite.Evaluate(testName, digest, img)
}
