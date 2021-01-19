package bazel

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInBazel(t *testing.T) {
	BazelTest(t)
	require.True(t, InBazel())
}

func TestRunfilesDir_UsedToLocateAKnownRunfile_Success(t *testing.T) {
	BazelTest(t)
	runfile := filepath.Join(RunfilesDir(), "bazel/go/bazel/testdata/hello.txt")
	bytes, err := ioutil.ReadFile(runfile)
	require.NoError(t, err)
	require.Equal(t, "Hello, world!", strings.TrimSpace(string(bytes)))
}
