package gitiles

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	cipd_git "go.skia.org/infra/bazel/external/cipd/git"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitiles/mocks"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vfs/shared_tests"
)

const (
	fakeHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

func TestFS(t *testing.T) {
	ctx := cipd_git.UseGitFinder(context.Background())
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

	mr := gitiles_testutils.NewMockRepo(t, repoURL, gd, urlMock)
	mr.MockGetCommit(ctx, hash)
	mr.MockReadFile(ctx, ".", hash)
	mr.MockReadFile(ctx, "rootFile", hash)
	mr.MockReadFile(ctx, "subdir", hash)
	mr.MockReadFile(ctx, "subdir/subDirFile", hash)

	fs, err := New(ctx, repo, hash)
	require.NoError(t, err)
	shared_tests.TestFS(ctx, t, fs)
	require.True(t, urlMock.Empty())
}

func TestVFS_ReadOnly(t *testing.T) {
	repo := &mocks.GitilesRepo{}
	repo.On("ReadObject", testutils.AnyContext, shared_tests.FakeFileName, fakeHash).Return(shared_tests.FakeFileInfo, shared_tests.FakeContents, nil)
	fs := &FS{
		repo:            repo,
		hash:            fakeHash,
		cachedFileInfos: map[string]os.FileInfo{},
		cachedContents:  map[string][]byte{},
		changes:         map[string][]byte{},
	}
	shared_tests.TestVFS_ReadOnly(t, fs)
}
func TestVFS_ReadWrite(t *testing.T) {
	repo := &mocks.GitilesRepo{}
	repo.On("ReadObject", testutils.AnyContext, shared_tests.FakeFileName, fakeHash).Return(shared_tests.FakeFileInfo, shared_tests.FakeContents, nil)
	fs := &FS{
		repo:            repo,
		hash:            fakeHash,
		cachedFileInfos: map[string]os.FileInfo{},
		cachedContents:  map[string][]byte{},
		changes:         map[string][]byte{},
	}
	shared_tests.TestVFS_ReadWrite(t, fs)
}

func TestVFS_MultiWrite_ChangedToOriginal(t *testing.T) {
	repo := &mocks.GitilesRepo{}
	repo.On("ReadObject", testutils.AnyContext, shared_tests.FakeFileName, fakeHash).Return(shared_tests.FakeFileInfo, shared_tests.FakeContents, nil)
	fs := &FS{
		repo:            repo,
		hash:            fakeHash,
		cachedFileInfos: map[string]os.FileInfo{},
		cachedContents:  map[string][]byte{},
		changes:         map[string][]byte{},
	}
	shared_tests.TestVFS_MultiWrite_ChangedToOriginal(t, fs)
}
