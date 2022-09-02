package testutils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/vfs/shared_tests"
)

// Ideally, this would be in the go/gitiles package, but we want to use the
// mocks in this package.
func TestFS(t *testing.T) {

	ctx := context.Background()
	repoURL := "https://fake.repo.git"
	urlMock := mockhttpclient.NewURLMock()
	repo := gitiles.NewRepo(repoURL, urlMock.Client())
	tmp := shared_tests.MakeTestFiles(t)
	gd := git.GitDir(tmp)
	_, err := gd.Git(ctx, "init")
	require.NoError(t, err)
	_, err = gd.Git(ctx, "add", ".")
	require.NoError(t, err)
	_, err = gd.Git(ctx, "commit", "-m", "initial commit")
	require.NoError(t, err)
	hash, err := gd.RevParse(ctx, "HEAD")
	require.NoError(t, err)

	mr := NewMockRepo(t, repoURL, gd, urlMock)
	mr.MockGetCommit(ctx, hash)
	mr.MockReadFile(ctx, ".", hash)
	mr.MockReadFile(ctx, "rootFile", hash)
	mr.MockReadFile(ctx, "subdir", hash)
	mr.MockReadFile(ctx, "subdir/subDirFile", hash)
	mr.MockReadFile(ctx, ".", hash)
	mr.MockReadFile(ctx, ".", hash)
	mr.MockReadFile(ctx, "rootFile", hash)
	mr.MockReadFile(ctx, "subdir", hash)

	fs, err := repo.VFS(ctx, hash)
	require.NoError(t, err)
	shared_tests.TestFS(ctx, t, fs)
	require.True(t, urlMock.Empty())
}
