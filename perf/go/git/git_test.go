// Package git is the minimal interface that Perf need to interact with a Git
// repo.
package git

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/types"
)

type cleanupFunc func()

func newForTest(t *testing.T) (context.Context, *Git, cleanupFunc) {
	ctx := context.Background()

	// Get tmp dir to use for repo checkout.
	tmpDir, err := ioutil.TempDir("", "git")
	require.NoError(t, err)

	clean := func() {
		err = os.RemoveAll(tmpDir)
		assert.NoError(t, err)
	}

	instanceConfig := &config.InstanceConfig{
		GitRepoConfig: config.GitRepoConfig{
			URL: "https://github.com/skia-dev/perf-demo-repo.git",
			Dir: tmpDir,
		},
	}
	g, err := New(ctx, instanceConfig)
	require.NoError(t, err)
	return ctx, g, clean
}
func TestGit_CommitNumberFromGitHash_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, cleanup := newForTest(t)
	defer cleanup()

	// This is a real commit from the repo at https://github.com/skia-dev/perf-demo-repo.git.
	commitNumber, err := g.CommitNumberFromGitHash(ctx, "fcd63691360443c852ab3bd832d0a9be7596e2d5")
	assert.NoError(t, err)
	assert.Equal(t, types.CommitNumber(1), commitNumber)
	assert.Equal(t, 1, g.commitNumberCache.Len())
}

func TestGit_CommitNumberFromGitHash_LookupFail(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, cleanup := newForTest(t)
	defer cleanup()

	commitNumber, err := g.CommitNumberFromGitHash(ctx, "this is not a valid git hash")
	assert.Error(t, err)
	assert.Equal(t, 0, g.commitNumberCache.Len())
	assert.Equal(t, types.BadCommitNumber, commitNumber)
}

func TestGit_New_FailCheckout(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()

	// Get tmp dir to use for repo checkout.
	tmpDir, err := ioutil.TempDir("", "git")
	require.NoError(t, err)

	defer func() {
		err = os.RemoveAll(tmpDir)
		assert.NoError(t, err)
	}()

	instanceConfig := &config.InstanceConfig{
		GitRepoConfig: config.GitRepoConfig{
			URL: "this is not a valid URL",
			Dir: tmpDir,
		},
	}
	_, err = New(ctx, instanceConfig)
	require.Error(t, err)
}
