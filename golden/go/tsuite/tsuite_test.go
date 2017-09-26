package tsuite

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/diff"

	"go.skia.org/infra/golden/go/types"

	assert "github.com/stretchr/testify/require"
)

const testDataDir = "testdata"

var (
	digests = []string{
		"b716a12d5b98d04b15db1d9dd82c82ea",
		"df1591dde35907399734ea19feb76663",
		"ffce5042b4ac4a57bd7c8657b557d495",
		"fffbcca7e8913ec45b88cc2c6a3a73ad",
	}

	testNames = []string{
		"convex-lineonly-paths",
		"imagefilterscropexpand",
		"gradients_interesting",
		"arithmode",
		"blurrects",
		"repeated_bitmap_jpg",
	}
)

func TestTSuiteSaveLoad(t *testing.T) {
	testutils.LargeTest(t)

	classifier := NewMemorizer()
	for _, digest := range digests {
		image, err := diff.OpenNRGBAFromFile(filepath.Join(testDataDir, digest+".png"))
		assert.NoError(t, err)
		classifier.Add(digest, image, types.POSITIVE)
	}

	suite := New()
	for _, testName := range testNames {
		suite.Add(testName, classifier)
	}

	var buf bytes.Buffer
	assert.NoError(t, suite.Save(&buf))
	bufBytes := buf.Bytes()

	foundSuite, err := Load(bytes.NewReader(bufBytes), int64(len(bufBytes)))
	assert.NoError(t, err)
	assert.Equal(t, suite.Tests, foundSuite.Tests)
	assert.Equal(t, len(suite.classifiers), len(foundSuite.classifiers))

	outputFileName := "knowledge.zip"
	assert.NoError(t, suite.SaveToFile(outputFileName))
	defer func() {
		assert.NoError(t, os.Remove(outputFileName))
	}()

	foundSuite, err = LoadFromFile(outputFileName)
	assert.NoError(t, err)

	assert.Equal(t, suite.Tests, foundSuite.Tests)
	assert.Equal(t, len(suite.classifiers), len(foundSuite.classifiers))
}
