package cq

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/bazelbuild/buildtools/build"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestWithUpdateCQConfig(t *testing.T) {
	unittest.ManualTest(t)

	ctx := context.Background()

	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	mainStarFile := filepath.Join(tmp, filename)
	testutils.WriteFile(t, mainStarFile, fakeConfig)
	// We use a directory other than the default "generated", to verify that we
	// respect what the caller passed in.
	generatedDir := filepath.Join(tmp, "my-generated-configs")
	require.NoError(t, os.MkdirAll(generatedDir, os.ModePerm))

	require.NoError(t, WithUpdateCQConfig(ctx, mainStarFile, generatedDir, func(f *build.File) error {
		return DeleteBranch(f, "master")
	}))

	generatedFiles, err := os.ReadDir(generatedDir)
	require.NoError(t, err)
	generatedFileNames := make([]string, 0, len(generatedFiles))
	for _, f := range generatedFiles {
		generatedFileNames = append(generatedFileNames, f.Name())
	}
	require.Equal(t, []string{
		"commit-queue.cfg",
		"cr-buildbucket.cfg",
		"luci-logdog.cfg",
		"project.cfg",
	}, generatedFileNames)
}
