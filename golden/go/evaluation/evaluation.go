package evaluation

import (
	"image"

	"go.skia.org/infra/golden/go/tsuite"
)

var suite *tsuite.CompatTestSuite = nil

// Load the test suite from disk.
func Load(testSuiteZipPath string) error {
	var err error = nil
	suite, err = tsuite.Load(testSuiteZipPath)
	if err != nil {
		return err
	}
	return nil
}

// Return the list of available test names.
func TestNames() []string {
	return suite.TestNames()
}

// Evaluate a test. digest is the MD5 hash of the internal pixel buffer
// to speed up evaluation. The returned value is the probability of a pass.
func Evaluate(testName, digest string, img *image.NRGBA) float32 {
	ret, _ := suite.Evaluate(testName, digest, img)
	return ret
}
