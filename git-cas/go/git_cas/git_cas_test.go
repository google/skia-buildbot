package git_cas

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func makeRepo(ctx context.Context, t *testing.T) (*git.Repo, func()) {
	tmp, cleanup := testutils.TempDir(t)
	repo := &git.Repo{git.GitDir(tmp)}
	_, err := repo.Git(ctx, "--bare", "init")
	require.NoError(t, err)
	return repo, cleanup
}

func assertTreesEqual(ctx context.Context, t *testing.T, a, b string) {
	// fileInfo trims os.FileInfo to just the parts we can reasonably
	// compare, plus a hash.
	type fileInfo struct {
		Name string
		Mode os.FileMode
		Hash string
	}
	hash := func(path string) string {
		git, _, _, err := git_common.FindGit(ctx)
		require.NoError(t, err)
		out, err := exec.RunCwd(ctx, ".", git, "hash-object", path)
		require.NoError(t, err)
		return strings.TrimSpace(out)
	}
	getTree := func(d string) map[string]fileInfo {
		rv := map[string]fileInfo{}
		require.NoError(t, filepath.Walk(d, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// Git doesn't store empty directories.
			// TODO(borenet): Is this okay?
			if info.IsDir() {
				return nil
			}
			rv[strings.Replace(path, d, ".", -1)] = fileInfo{
				Name: strings.Replace(info.Name(), filepath.Base(d), ".", -1),
				Mode: info.Mode(),
				Hash: hash(path),
			}
			return nil
		}))
		return rv
	}
	treeA := getTree(a)
	treeB := getTree(b)
	assertdeep.Equal(t, treeA, treeB)
}

func TestGitCAS(t *testing.T) {
	unittest.LargeTest(t)

	// Remote repo. In production this would be on a server.
	ctx := context.Background()
	remote, cleanupRemote := makeRepo(ctx, t)
	defer cleanupRemote()

	// Local repo, synced from the remote.
	local, cleanupLocal := makeRepo(ctx, t)
	defer cleanupLocal()
	_, err := local.Git(ctx, "remote", "add", "origin", remote.Dir())
	require.NoError(t, err)

	// Test helper function.
	test := func(fn func(src string)) {
		src, cleanupSrc := testutils.TempDir(t)
		defer cleanupSrc()
		dst, cleanupDst := testutils.TempDir(t)
		defer cleanupDst()
		fn(src)
		hash, err := Upload(ctx, local, src)
		require.NoError(t, err)
		require.NoError(t, Download(ctx, local, dst, hash))
		assertTreesEqual(ctx, t, src, dst)

		// Create a new local copy, ensure that we can still download
		// the files.
		local2, cleanupLocal2 := makeRepo(ctx, t)
		defer cleanupLocal2()
		_, err = local2.Git(ctx, "remote", "add", "origin", remote.Dir())
		require.NoError(t, err)
		dst2, cleanupDst2 := testutils.TempDir(t)
		defer cleanupDst2()
		require.NoError(t, Download(ctx, local2, dst2, hash))
		assertTreesEqual(ctx, t, src, dst2)
	}

	// Simple test cases.
	test(func(src string) {})
	test(func(src string) {
		testutils.WriteFile(t, filepath.Join(src, "myfile"), "myfile-contents")
	})
	test(func(src string) {
		// Empty directories won't get synced, but also won't cause an
		// error.
		require.NoError(t, os.Mkdir(filepath.Join(src, "mydir"), os.ModePerm))
	})
	test(func(src string) {
		testutils.WriteFile(t, filepath.Join(src, "myfile"), "myfile-contents")
		require.NoError(t, os.Mkdir(filepath.Join(src, "mydir"), os.ModePerm))
		testutils.WriteFile(t, filepath.Join(src, "mydir", "other"), "other-contents")
	})
}
