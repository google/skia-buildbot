package cq

import (
	"regexp"
	"testing"

	"github.com/bazelbuild/buildtools/build"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git"
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
