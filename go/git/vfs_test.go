package git

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/vfs/shared_tests"
)

func TestFS(t *testing.T) {
	ctx := context.Background()
	tmp := shared_tests.MakeTestFiles(t)
	gd := GitDir(tmp)
	_, err := gd.Git(ctx, "init")
	require.NoError(t, err)
	_, err = gd.Git(ctx, "add", ".")
	require.NoError(t, err)
	_, err = gd.Git(ctx, "commit", "-m", "initial commit")
	require.NoError(t, err)
	hash, err := gd.RevParse(ctx, "HEAD")
	require.NoError(t, err)

	fs, err := gd.VFS(ctx, hash)
	require.NoError(t, err)
	shared_tests.TestFS(ctx, t, fs)
}

func TestVFS_ReadOnly(t *testing.T) {
	ctx := context.Background()
	tmp := shared_tests.MakeTestFiles(t)
	gd := GitDir(tmp)
	_, err := gd.Git(ctx, "init")
	require.NoError(t, err)
	_, err = gd.Git(ctx, "add", ".")
	require.NoError(t, err)
	_, err = gd.Git(ctx, "commit", "-m", "initial commit")
	require.NoError(t, err)
	hash, err := gd.RevParse(ctx, "HEAD")
	require.NoError(t, err)

	fs, err := gd.VFS(ctx, hash)
	require.NoError(t, err)
	shared_tests.TestVFS_ReadOnly(t, fs)
}

// Skip the VFS tests which use Write, since that's unimplemented for this FS.
