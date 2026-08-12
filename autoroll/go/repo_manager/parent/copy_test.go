package parent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	vfs_mocks "go.skia.org/infra/go/vfs/mocks"
)

func TestCopyParentGetCopies_DstRelPathDoesNotExist(t *testing.T) {
	ctx := context.Background()

	parentFs := vfs_mocks.NewFS(t)
	childFs := vfs_mocks.NewFS(t)

	childFiles := map[string]string{
		"bar/file1.txt":     "hello",
		"bar/sub/file2.txt": "world",
		"somefile.txt":      "top-level",
	}
	vfs_mocks.SetupMockFS(t, childFs, childFiles)

	// DstRelPath does not exist in parentFs at all.
	vfs_mocks.SetupMockFS(t, parentFs, nil)

	copies := []*config.CopyParentConfig_CopyEntry{
		{
			SrcRelPath: ".",
			DstRelPath: "foo",
		},
	}

	changes, err := copyParentGetCopies(ctx, copies, parentFs, childFs)
	require.NoError(t, err)

	expectedChanges := map[string]string{
		"foo/bar/file1.txt":     "hello",
		"foo/bar/sub/file2.txt": "world",
		"foo/somefile.txt":      "top-level",
	}
	require.Equal(t, expectedChanges, changes)
}

func TestCopyParentGetCopies_ExistingAndModifiedAndDeleted(t *testing.T) {
	ctx := context.Background()

	parentFs := vfs_mocks.NewFS(t)
	childFs := vfs_mocks.NewFS(t)

	childFiles := map[string]string{
		"bar/nochange.txt":   "same",
		"bar/sub/update.txt": "new-val",
		"bar/sub/add.txt":    "add-me",
	}
	vfs_mocks.SetupMockFS(t, childFs, childFiles)

	parentFiles := map[string]string{
		"foo/nochange.txt":   "same",
		"foo/sub/update.txt": "old-val",
		"foo/sub/delete.txt": "delete-me",
	}
	vfs_mocks.SetupMockFS(t, parentFs, parentFiles)

	copies := []*config.CopyParentConfig_CopyEntry{
		{
			SrcRelPath: "bar",
			DstRelPath: "foo",
		},
	}

	changes, err := copyParentGetCopies(ctx, copies, parentFs, childFs)
	require.NoError(t, err)

	expectedChanges := map[string]string{
		"foo/sub/update.txt": "new-val",
		"foo/sub/delete.txt": "",
		"foo/sub/add.txt":    "add-me",
	}
	require.Equal(t, expectedChanges, changes)
}

func TestCopyParentGetCopies_ChildSrcRelPathDoesNotExist(t *testing.T) {
	ctx := context.Background()

	parentFs := vfs_mocks.NewFS(t)
	childFs := vfs_mocks.NewFS(t)

	// SrcRelPath does not exist in childFs.
	vfs_mocks.SetupMockFS(t, childFs, nil)

	// DstRelPath exists in parentFs.
	vfs_mocks.SetupMockFS(t, parentFs, map[string]string{
		"foo/file1.txt":     "hello",
		"foo/sub/file2.txt": "world",
	})

	copies := []*config.CopyParentConfig_CopyEntry{
		{
			SrcRelPath: "bar",
			DstRelPath: "foo",
		},
	}

	changes, err := copyParentGetCopies(ctx, copies, parentFs, childFs)
	require.NoError(t, err)

	expectedChanges := map[string]string{
		"foo/file1.txt":     "",
		"foo/sub/file2.txt": "",
	}
	require.Equal(t, expectedChanges, changes)
}
