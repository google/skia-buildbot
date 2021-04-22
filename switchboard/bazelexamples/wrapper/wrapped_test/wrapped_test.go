package wrapped_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	wrapperScriptTempFileEnvVar = "WRAPPER_SCRIPT_TEMP_FILE"
	expectedTempFileContents = "Hello, world!\n"
)

func TestWrappedTest(t *testing.T) {
	unittest.BazelOnlyTest(t)

	// The wrapper script should have set this environment variable.
	tempFile := os.Getenv(wrapperScriptTempFileEnvVar)
	require.NotEmpty(t, tempFile)

	// The wrapper script should have written a known string into the temp file.
	b, err := os.ReadFile(tempFile)
	require.NoError(t, err)
	require.Equal(t, expectedTempFileContents, string(b))
}
