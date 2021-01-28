// Package test contains tests for //go/bazel/go/bazel.go. These tests are placed here to avoid a
// circular dependency with //go/testutils/unittest/unittest.go.
package test

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestInBazel(t *testing.T) {
	unittest.SmallTest(t)
	unittest.BazelOnlyTest(t)

	require.True(t, bazel.InBazel())
}

func TestRunfilesDir_UsedToLocateAKnownRunfile_Success(t *testing.T) {
	unittest.SmallTest(t)
	unittest.BazelOnlyTest(t)

	runfile := filepath.Join(bazel.RunfilesDir(), "bazel/go/bazel/test/testdata/hello.txt")
	bytes, err := ioutil.ReadFile(runfile)
	require.NoError(t, err)
	require.Equal(t, "Hello, world!", strings.TrimSpace(string(bytes)))
}
