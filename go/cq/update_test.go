package cq

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/bazelbuild/buildtools/build"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

const filename = "main.star"

func TestCloneBranch(t *testing.T) {
	unittest.SmallTest(t)

	t.Run("clone all", func(t *testing.T) {
		actual, err := WithUpdateCQConfigBytes(filename, []byte(fakeConfig), func(f *build.File) error {
			return CloneBranch(f, git.MasterBranch, "clone", true, true, nil)
		})
		require.NoError(t, err)
		require.Equal(t, cloneAllExpect, string(actual))
	})

	t.Run("clone without experimental", func(t *testing.T) {
		actual, err := WithUpdateCQConfigBytes(filename, []byte(fakeConfig), func(f *build.File) error {
			return CloneBranch(f, git.MasterBranch, "clone", false, true, nil)
		})
		require.NoError(t, err)
		require.Equal(t, cloneNoExperimentalExpect, string(actual))
	})

	t.Run("clone without tree check", func(t *testing.T) {
		actual, err := WithUpdateCQConfigBytes(filename, []byte(fakeConfig), func(f *build.File) error {
			return CloneBranch(f, git.MasterBranch, "clone", true, false, nil)
		})
		require.NoError(t, err)
		require.Equal(t, cloneNoTreeCheckExpect, string(actual))
	})

	t.Run("clone exclude regex", func(t *testing.T) {
		excludeRe := regexp.MustCompile("skia")
		actual, err := WithUpdateCQConfigBytes(filename, []byte(fakeConfig), func(f *build.File) error {
			return CloneBranch(f, git.MasterBranch, "clone", true, true, []*regexp.Regexp{excludeRe})
		})
		require.NoError(t, err)
		require.Equal(t, cloneExcludeRegexExpect, string(actual))
	})
}

func TestDeleteBranch(t *testing.T) {
	unittest.SmallTest(t)

	actual, err := WithUpdateCQConfigBytes(filename, []byte(fakeConfig), func(f *build.File) error {
		return DeleteBranch(f, git.MasterBranch)
	})
	require.NoError(t, err)
	require.Equal(t, deleteExpect, string(actual))
}

func TestWithUpdateCQConfig(t *testing.T) {
	unittest.LargeTest(t)

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
