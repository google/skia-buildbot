package git

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vfs/shared_tests"
)

func TestFS(t *testing.T) {
	unittest.MediumTest(t)

	ctx := context.Background()
	tmp, cleanup := shared_tests.MakeTestFiles(t)
	defer cleanup()

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
