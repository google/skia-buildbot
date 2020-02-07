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
	repo := &git.Repo{GitDir: git.GitDir(tmp)}
	_, err := repo.Git(ctx, "--bare", "init")
	require.NoError(t, err)
	return repo, cleanup
}

func makeGitCAS(ctx context.Context, t *testing.T, remote *git.Repo) (*GitCAS, func()) {
	tmp, cleanup := testutils.TempDir(t)
	rv, err := New(ctx, tmp, remote.Dir())
	require.NoError(t, err)
	return rv, cleanup
}

// fileInfo trims os.FileInfo to just the parts we can reasonably
// compare, plus a hash.
type fileInfo struct {
	Name string
	Mode os.FileMode
	Hash string
}

func hashFile(ctx context.Context, t *testing.T, path string) string {
	git, _, _, err := git_common.FindGit(ctx)
	require.NoError(t, err)
	out, err := exec.RunCwd(ctx, ".", git, "hash-object", path)
	require.NoError(t, err)
	return strings.TrimSpace(out)
}

func getTree(ctx context.Context, t *testing.T, d string) map[string]fileInfo {
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
		rel, err := filepath.Rel(d, path)
		require.NoError(t, err)
		rv[rel] = fileInfo{
			Name: info.Name(),
			// TODO(borenet): File modes don't match between my
			// machine and the bots. Probably need to play some
			// games with umask.
			//Mode: info.Mode(),
			Hash: hashFile(ctx, t, path),
		}
		return nil
	}))
	return rv
}

func assertTreesEqual(ctx context.Context, t *testing.T, a, b string) {
	treeA := getTree(ctx, t, a)
	treeB := getTree(ctx, t, b)
	assertdeep.Equal(t, treeA, treeB)
}

func TestGitCAS(t *testing.T) {
	unittest.LargeTest(t)

	// Remote repo. In production this would be on a server.
	ctx := context.Background()
	remote, cleanupRemote := makeRepo(ctx, t)
	defer cleanupRemote()

	// Local repo, synced from the remote.
	local, cleanupLocal := makeGitCAS(ctx, t, remote)
	defer cleanupLocal()

	// Test helper functions.
	up := func(fn func(src string)) (string, string, func()) {
		src, cleanupSrc := testutils.TempDir(t)
		fn(src)
		hash, err := local.Upload(ctx, src)
		require.NoError(t, err)
		return src, hash, cleanupSrc
	}
	test := func(fn func(src string)) {
		src, hash, cleanupSrc := up(fn)
		defer cleanupSrc()
		dst, cleanupDst := testutils.TempDir(t)
		defer cleanupDst()
		require.NoError(t, local.Download(ctx, dst, hash))
		assertTreesEqual(ctx, t, src, dst)

		// Create a new local copy, ensure that we can still download
		// the files.
		local2, cleanupLocal2 := makeGitCAS(ctx, t, remote)
		defer cleanupLocal2()
		dst2, cleanupDst2 := testutils.TempDir(t)
		defer cleanupDst2()
		require.NoError(t, local2.Download(ctx, dst2, hash))
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

	// Downloads should merge and overwrite existing contents without
	// producing an error.
	{
		var f1Hash, f2Hash, f3Hash string
		_, hash1, cleanupSrc1 := up(func(src string) {
			testutils.WriteFile(t, filepath.Join(src, "myfile"), "myfile-contents")
			require.NoError(t, os.Mkdir(filepath.Join(src, "mydir"), os.ModePerm))
			testutils.WriteFile(t, filepath.Join(src, "mydir", "other"), "other-contents")
			f2Hash = hashFile(ctx, t, filepath.Join(src, "mydir", "other"))
		})
		defer cleanupSrc1()
		_, hash2, cleanupSrc2 := up(func(src string) {
			testutils.WriteFile(t, filepath.Join(src, "myfile"), "myfile-contents2")
			f1Hash = hashFile(ctx, t, filepath.Join(src, "myfile"))
			require.NoError(t, os.Mkdir(filepath.Join(src, "mydir"), os.ModePerm))
			testutils.WriteFile(t, filepath.Join(src, "mydir", "other2"), "other-contents")
			f3Hash = hashFile(ctx, t, filepath.Join(src, "mydir", "other2"))
		})
		defer cleanupSrc2()
		dst, cleanupDst := testutils.TempDir(t)
		defer cleanupDst()
		require.NoError(t, local.Download(ctx, dst, hash1))
		require.NoError(t, local.Download(ctx, dst, hash2))
		actual := getTree(ctx, t, dst)
		assertdeep.Equal(t, map[string]fileInfo{
			filepath.Join(".", "myfile"): {
				Name: "myfile",
				//Mode: 0750, // TODO(borenet): Why doesn't this match os.ModePerm?
				Hash: f1Hash,
			},
			filepath.Join(".", "mydir", "other"): {
				Name: "other",
				//Mode: 0750,
				Hash: f2Hash,
			},
			filepath.Join(".", "mydir", "other2"): {
				Name: "other2",
				//Mode: 0750,
				Hash: f3Hash,
			},
		}, actual)
	}

	// Verify that the paths in the tree match the expectations. Ignore
	// the hashes and modes, assuming that we'll have gotten those correct
	// based on the previous test cases.
	checkTree := func(path string, expect []string) {
		actual := getTree(ctx, t, path)
		require.Equal(t, len(expect), len(actual))
		for _, e := range expect {
			_, ok := actual[e]
			require.True(t, ok)
		}
	}

	// UploadItems.
	{
		src, cleanupSrc := testutils.TempDir(t)
		defer cleanupSrc()
		testutils.WriteFile(t, filepath.Join(src, "myfile"), "myfile-contents")
		testutils.WriteFile(t, filepath.Join(src, "extra"), "bogus")
		require.NoError(t, os.Mkdir(filepath.Join(src, "mydir"), os.ModePerm))
		testutils.WriteFile(t, filepath.Join(src, "mydir", "other"), "other-contents")
		require.NoError(t, os.Mkdir(filepath.Join(src, "skipthis"), os.ModePerm))
		testutils.WriteFile(t, filepath.Join(src, "skipthis", "skipped"), "bye")
		items := []string{
			"myfile",
			"mydir",
		}
		hash, err := local.UploadItems(ctx, src, items)
		require.NoError(t, err)
		dst, cleanupDst := testutils.TempDir(t)
		defer cleanupDst()
		require.NoError(t, local.Download(ctx, dst, hash))
		checkTree(dst, []string{
			"myfile",
			filepath.Join("mydir", "other"),
		})
	}
}
