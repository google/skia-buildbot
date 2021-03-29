// Package docsy transforms raw documents via Hugo and a Docsy template into
// final documentation.
package docsy

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestRender_HappyPath(t *testing.T) {
	unittest.SmallTest(t)

	d := New("/usr/bin/hugo", "/my-docsy-dir", "relative/path/into/checkout")

	ctx := executil.FakeTestsContext("Test_FakeExe_Hugo_Success")
	err := d.Render(ctx, "/input", "/output")
	require.NoError(t, err)
}

func Test_FakeExe_Hugo_Success(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"/usr/bin/hugo", "--source=/my-docsy-dir", "--destination=/output", "--config=/input/config.toml", "--contentDir=/input"}, args)

	// Force exit so we don't get PASS in the output.
	os.Exit(0)
}

func TestRender_Failure(t *testing.T) {
	unittest.SmallTest(t)

	d := New("/usr/bin/hugo", "/my-docsy-dir", "relative/path/into/checkout")
	ctx := executil.FakeTestsContext("Test_FakeExe_Hugo_Failure")
	err := d.Render(ctx, "/input", "/output")
	require.Error(t, err)
}

func Test_FakeExe_Hugo_Failure(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}

	// Force exit so we don't get PASS in the output.
	os.Exit(1)
}
