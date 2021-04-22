package wrapped_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bmizerany/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestWrappedTest(t *testing.T) {
	unittest.BazelOnlyTest(t)

	// Temporary directory created by the test_on_env rule. This environment variable is set for both
	// the environment binary and the test binary.
	envDir := os.Getenv("ENV_DIR")
	require.NotEmpty(t, envDir, "The ENV_DIR environment variable is not set. Are we running inside a test_on_env test?")

	// Temporary file written by the environment binary.
	tempFile := filepath.Join(envDir, "tempfile")

	// The test will read the temp file once per second, and it will assert that its contents change
	// between each read.
	prevFileContents := ""
	for i := 0; i < 5; i++ {
		time.Sleep(1 * time.Second)

		b, err := os.ReadFile(tempFile)
		require.NoError(t, err)
		fileContents := string(b)

		assert.NotEqual(t, fileContents, prevFileContents)
		prevFileContents = fileContents
	}
}
