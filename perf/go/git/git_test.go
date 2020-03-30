// Package git is the minimal interface that Perf need to interact with a Git
// repo.
package git

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/config"
	perfsql "go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/types"
)

type cleanupFunc func()

var (
	startTime = time.Unix(1680000000, 0)
)

func newForTest(t *testing.T) (context.Context, *Git, *testutils.GitBuilder, []string, cleanupFunc) {
	ctx := context.Background()

	gb := testutils.GitInit(t, ctx)
	ts := startTime
	hashes := []string{}
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", ts))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", ts.Add(time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", ts.Add(2*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", ts.Add(3*time.Minute)))

	db, sqlCleanup := sqltest.NewSQLite3DBForTests(t)

	// Get tmp dir to use for repo checkout.
	tmpDir, err := ioutil.TempDir("", "git")
	require.NoError(t, err)

	clean := func() {
		err = os.RemoveAll(tmpDir)
		assert.NoError(t, err)
		sqlCleanup()
		gb.Cleanup()
	}

	instanceConfig := &config.InstanceConfig{
		DataStoreConfig: config.DataStoreConfig{
			DataStoreType: config.SQLite3DataStoreType,
		},
		GitRepoConfig: config.GitRepoConfig{
			URL: gb.Dir(),
			Dir: filepath.Join(tmpDir, "checkout"),
		},
	}
	g, err := New(ctx, true, db, perfsql.SQLiteDialect, instanceConfig)
	require.NoError(t, err)
	return ctx, g, gb, hashes, clean
}

func TestGit_CommitNumberFromGitHash_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, _, hashes, cleanup := newForTest(t)
	defer cleanup()

	commitNumber, err := g.CommitNumberFromGitHash(ctx, hashes[0])
	assert.NoError(t, err)
	assert.Equal(t, types.CommitNumber(0), commitNumber)
	commitNumber, err = g.CommitNumberFromGitHash(ctx, hashes[2])
	assert.NoError(t, err)
	assert.Equal(t, types.CommitNumber(2), commitNumber)
}

func TestGit_SingleUpdateStep_NewCommitsAreFound(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, gb, hashes, cleanup := newForTest(t)
	defer cleanup()

	newHash := gb.CommitGenAt(ctx, "foo.txt", startTime.Add(4*time.Minute))

	commitNumber, err := g.CommitNumberFromGitHash(ctx, hashes[0])
	assert.NoError(t, err)
	assert.Equal(t, types.CommitNumber(0), commitNumber)

	commitNumber, err = g.CommitNumberFromGitHash(ctx, newHash)
	require.Error(t, err)

	err = g.singleUpdateStep(ctx)
	require.NoError(t, err)
	commitNumber, err = g.CommitNumberFromGitHash(ctx, newHash)
	assert.Equal(t, types.CommitNumber(4), commitNumber)

}
