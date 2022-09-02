package mirrors

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/testutils"
)

var (
	// This config path is only used for asserting the start command. It does
	// not have to exist and will not be created.
	testVerdaccioConfigPath = "/tmp/test-config-path1"
)

func TestStartVerdaccioMirror_VerdaccioStartCommandIsCalled_Success(t *testing.T) {

	// Mock the executil call.
	ctx := executil.FakeTestsContext("Test_FakeExe_Verdaccio_Start_Cmd")

	m := &VerdaccioMirror{
		verdaccioConfigPath: testVerdaccioConfigPath,
	}
	m.startVerdaccioMirror(ctx, 1111, nil)
	require.Equal(t, executil.FakeCommandsReturned(ctx), 1)
}

func TestDownloadedPackageTarballs(t *testing.T) {

	m := &VerdaccioMirror{downloadedPackageTarballs: map[string]interface{}{}}
	require.False(t, m.IsPackageTarballDownloaded("pkg1.tgz"))
	m.AddToDownloadedPackageTarballs("pkg1.tgz")
	require.True(t, m.IsPackageTarballDownloaded("pkg1.tgz"))
}

func TestGetDownloadedPackageNames(t *testing.T) {
	storageDir := testutils.TestDataDir(t)

	m := &VerdaccioMirror{verdaccioStorageDir: storageDir}
	packages, err := m.GetDownloadedPackageNames()
	require.NoError(t, err)
	require.Equal(t, []string{"pkg1", "pkg2"}, packages)
}

func TestGetTarballsInMirrorStorage(t *testing.T) {
	storageDir := testutils.TestDataDir(t)

	tarballs, err := GetTarballsInMirrorStorage(storageDir)
	require.NoError(t, err)
	_, pkg1ok := tarballs["pkg1.tgz"]
	require.True(t, pkg1ok)
	_, pkg2ok := tarballs["pkg2.tgz"]
	require.True(t, pkg2ok)
	_, pkg3ok := tarballs["pkg3.tgz"]
	require.False(t, pkg3ok)
}

// This is not a real test, but a fake implementation of the executable in question.
// By convention, we prefix these with FakeExe to make that clear.
func Test_FakeExe_Verdaccio_Start_Cmd(t *testing.T) {
	// Since this is a normal go test, it will get run on the usual test suite. We check for the
	// special environment variable and if it is not set, we do nothing.
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	fmt.Printf("")
	require.Equal(t, []string{"verdaccio", fmt.Sprintf("--config=%s", testVerdaccioConfigPath), "--listen=1111", "--audit-level=high"}, args)

	os.Exit(0) // exit 0 prevents golang from outputting test stuff like "=== RUN", "---Fail".
}
