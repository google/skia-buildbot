package parent

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	cipd_git "go.skia.org/infra/bazel/external/cipd/git"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
)

const checkedInFile = "file-foo"
const checkedInSubmodule = "submodule-foo"

func setup(t *testing.T) (context.Context, *git_testutils.GitBuilder) {
	ctx := cipd_git.UseGitFinder(context.Background())
	g := git_testutils.GitInit(t, ctx)

	// Create submodule
	hash := "1111111111111111111111111111111111111111"
	cacheInfo := fmt.Sprintf("%s,%s,%s", "160000", hash, checkedInSubmodule)
	g.Git(ctx, "update-index", "--add", "--cacheinfo", cacheInfo)
	gitmoduleTemplate := "[submodule \"%s\"]\n" +
		"\tpath = %s\n" +
		"\turl = file://foo\n"
	g.Add(ctx, ".gitmodules", fmt.Sprintf(gitmoduleTemplate, checkedInSubmodule, checkedInSubmodule))

	// create file & commit
	g.CommitGenMsg(ctx, checkedInFile, "init")
	g.Git(ctx, "restore", checkedInSubmodule)
	return ctx, g
}

func TestReadFile(t *testing.T) {
	ctx, gb := setup(t)
	defer gb.Cleanup()

	tmpDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmpDir)

	co, err := git.NewCheckout(ctx, gb.Dir(), tmpDir)
	require.NoError(t, err)

	// Test Read file
	contents, err := getFile(ctx, co, checkedInFile)
	require.NoError(t, err)

	// Test Read submodule
	contents, err = getFile(ctx, co, checkedInSubmodule)
	require.NoError(t, err)
	require.Equal(t, "1111111111111111111111111111111111111111", contents)

	// Test file not found
	_, err = getFile(ctx, co, "bar")
	require.Error(t, err)
}

func TestWriteReadFile(t *testing.T) {
	ctx, gb := setup(t)
	defer gb.Cleanup()

	tmpDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmpDir)

	co, err := git.NewCheckout(ctx, gb.Dir(), tmpDir)

	// New file
	newHash := "2222222222222222222222222222222222222222"
	err = writeFile(ctx, co, checkedInSubmodule, newHash)
	require.NoError(t, err)

	_, err = co.Git(ctx, "commit", "-m", "update submodule #2")
	require.NoError(t, err)

	contents, err := getFile(ctx, co, checkedInSubmodule)
	require.NoError(t, err)
	require.Equal(t, newHash, contents)
}
