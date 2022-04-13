package rbe

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
)

// TODO(borenet): Is there a better test instance we could be using?
const testInstance = "projects/chromium-swarm-dev/instances/default_instance"

// setup creates and returns a Client instance
func setup(t *testing.T) (context.Context, *Client) {
	unittest.ManualTest(t)
	unittest.LargeTest(t)

	ctx := context.Background()
	ts, err := google.DefaultTokenSource(ctx, compute.CloudPlatformScope)
	require.NoError(t, err)
	client, err := NewClient(ctx, testInstance, ts)
	require.NoError(t, err)
	return ctx, client
}

type node struct {
	name         string
	size         int64
	isDir        bool
	isExecutable bool
	contents     []byte
}

// readTree reads the directory tree rooted in the given location and creates
// an object to represent it.
func readTree(t *testing.T, dir string) map[string]*node {
	tree := map[string]*node{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		// Skip the root itself.
		if path == dir {
			return nil
		}
		n := &node{
			name:         info.Name(),
			size:         info.Size(),
			isDir:        info.IsDir(),
			isExecutable: info.Mode()&0111 != 0,
		}
		if !info.IsDir() {
			contents, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			n.contents = contents
		}
		tree[strings.Replace(path, dir, ".", -1)] = n
		return nil
	})
	require.NoError(t, err)
	return tree
}

// AssertTreesEqual asserts that the directory trees rooted in the given
// locations have exactly the same contents.
func AssertTreesEqual(t *testing.T, a, b string) {
	treeA := readTree(t, a)
	treeB := readTree(t, b)
	assertdeep.Equal(t, treeA, treeB)
}

// testUploadDownload creates a temporary directory, runs the given function
// which adds files and directories, then uploads and downloads, asserting that
// the resulting directory is identical.
func testUploadDownload(ctx context.Context, t *testing.T, client *Client, work func(string)) {
	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, wd)
	work(wd)
	digest, err := client.Upload(ctx, wd, []string{"."}, nil)
	require.NoError(t, err)
	dest, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, dest)
	require.NoError(t, client.Download(ctx, dest, digest))
	AssertTreesEqual(t, wd, dest)
}

func d(t *testing.T, wd, name string) {
	require.NoError(t, os.MkdirAll(filepath.Join(wd, name), os.ModePerm))
}

func f(t *testing.T, wd, name, contents string, executable bool) {
	dir := path.Dir(name)
	if dir != "" {
		d(t, wd, dir)
	}
	mode := os.FileMode(0644)
	if executable {
		mode |= 0111
	}
	require.NoError(t, ioutil.WriteFile(filepath.Join(wd, name), []byte(contents), mode))
}

func TestUploadDownload(t *testing.T) {
	ctx, client := setup(t)

	// Empty tree.
	testUploadDownload(ctx, t, client, func(wd string) {})

	// Single file.
	testUploadDownload(ctx, t, client, func(wd string) {
		f(t, wd, "fake", "fakecontents", false)
	})

	// Executable file.
	testUploadDownload(ctx, t, client, func(wd string) {
		f(t, wd, "fake", "fakecontents", true)
	})

	// Empty directory.
	testUploadDownload(ctx, t, client, func(wd string) {
		d(t, wd, "emptydir")
	})

	// Deeply nested file.
	testUploadDownload(ctx, t, client, func(wd string) {
		f(t, wd, "a/b/c/d/fake", "fakecontents", false)
	})

	// Deeply nested dir.
	testUploadDownload(ctx, t, client, func(wd string) {
		d(t, wd, "a/b/c/d/dir")
	})

	// Multiple files and dirs
	testUploadDownload(ctx, t, client, func(wd string) {
		f(t, wd, "file1", "file1contents", false)
		f(t, wd, "file2", "file2contents", true)
		d(t, wd, "subdir")
		f(t, wd, "subdir/fake", "fakecontents", false)
		f(t, wd, "subdir/fake2", "fakecontents2", false)
	})
}

func TestMerge(t *testing.T) {
	ctx, client := setup(t)

	// upload is a helper function for creating and uploading a directory tree.
	upload := func(work func(string)) (string, map[string]*node) {
		wd, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		defer testutils.RemoveAll(t, wd)
		work(wd)
		digest, err := client.Upload(ctx, wd, []string{"."}, nil)
		require.NoError(t, err)
		return digest, readTree(t, wd)
	}

	// Create the two entries to merge.
	digest1, tree1 := upload(func(wd string) {
		f(t, wd, "fake", "fakecontents", false)
		f(t, wd, "duplicated", "duplicatedcontents", true)
		d(t, wd, "subdir")
		f(t, wd, "subdir/subfile", "subfilecontents", false)
		d(t, wd, "missing-from-tree2")
		f(t, wd, "missing-from-tree2/somefile", "missing-from-tree2 contents", false)
	})
	digest2, tree2 := upload(func(wd string) {
		f(t, wd, "otherfile", "blahblah", true)
		f(t, wd, "duplicated", "duplicatedcontents", true)
		d(t, wd, "subdir")
		f(t, wd, "subdir/subfile2", "subfilecontents2", false)
		d(t, wd, "missing-from-tree1")
		f(t, wd, "missing-from-tree1/somefile", "missing-from-tree1 contents", false)
	})

	// Merge the digests.
	mergeDigest, err := client.Merge(ctx, []string{digest1, digest2})
	require.NoError(t, err)
	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, wd)
	require.NoError(t, client.Download(ctx, wd, mergeDigest))
	mergeTree := readTree(t, wd)

	// Ensure that each of the digests is present in the merged version. Ensure
	// that the merged tree is the same as the result of merging the two
	// individual trees.
	expectMergeTree := map[string]*node{}
	for k, v1 := range tree1 {
		v2, ok := mergeTree[k]
		require.True(t, ok)
		assertdeep.Equal(t, v1, v2)
		expectMergeTree[k] = v1
	}
	for k, v1 := range tree2 {
		v2, ok := mergeTree[k]
		require.True(t, ok)
		assertdeep.Equal(t, v1, v2)
		expectMergeTree[k] = v1
	}
	assertdeep.Equal(t, expectMergeTree, mergeTree)
}

func TestUpload_Exclude(t *testing.T) {
	ctx, client := setup(t)

	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, wd)

	f(t, wd, "keepme", "blahblah", false)
	f(t, wd, "skipme", "abcdef", false)

	digest, err := client.Upload(ctx, wd, []string{"."}, []string{".*ipm.*"})
	require.NoError(t, err)
	dest, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, dest)
	require.NoError(t, client.Download(ctx, dest, digest))

	// Remove the skipped file and verify that the trees are equal.
	require.NoError(t, os.Remove(filepath.Join(wd, "skipme")))
	AssertTreesEqual(t, wd, dest)

	// Exclude ".git"
	d(t, wd, ".git")
	f(t, wd, ".git/objects", "blahblah", false)
	// Make sure we don't accidentally exclude similar file names.
	f(t, wd, "agit", "ldsfklasdfkl", false)
	f(t, wd, ".gitignore", "ldsfklasdfkl", false)
	f(t, wd, "fake.git", "blahblah", false)
	digest, err = client.Upload(ctx, wd, []string{"."}, []string{".*ipm.*", ExcludeGitDir})
	require.NoError(t, err)
	dest, err = ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, dest)
	require.NoError(t, client.Download(ctx, dest, digest))

	// Remove the skipped file and verify that the trees are equal.
	require.NoError(t, os.RemoveAll(filepath.Join(wd, ".git")))
	AssertTreesEqual(t, wd, dest)
}
