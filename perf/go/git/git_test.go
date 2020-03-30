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

	// Create a git repo for testing purposes.
	gb := testutils.GitInit(t, ctx)
	hashes := []string{}
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", startTime))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", startTime.Add(time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", startTime.Add(2*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", startTime.Add(3*time.Minute)))

	// Init our sql database.
	db, sqlCleanup := sqltest.NewSQLite3DBForTests(t)

	// Get tmp dir to use for repo checkout.
	tmpDir, err := ioutil.TempDir("", "git")
	require.NoError(t, err)

	// Create the cleanup function.
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

func TestSQLite(t *testing.T) {
	unittest.LargeTest(t)

	for name, subTest := range subTests {
		t.Run(name, func(t *testing.T) {
			ctx, g, gb, hashes, cleanup := newForTest(t)
			subTest(t, ctx, g, gb, hashes, cleanup)
		})
	}
}

// subTestFunction is a func we will call to test one aspect of *SQLTraceStore.
type subTestFunction func(t *testing.T, ctx context.Context, g *Git, gb *testutils.GitBuilder, hashes []string, cleanup cleanupFunc)

// subTests are all the tests we have for *SQLTraceStore.
var subTests = map[string]subTestFunction{
	"testCommitNumberFromGitHash_Success":                testCommitNumberFromGitHash_Success,
	"testSingleUpdateStep_NewCommitsAreFoundAfterUpdate": testSingleUpdateStep_NewCommitsAreFoundAfterUpdate,
	"testCommitNumberFromGitHash_ErrorOnUnknownGitHash":  testCommitNumberFromGitHash_ErrorOnUnknownGitHash,
	"testCommitNumberFromTime_Success":                   testCommitNumberFromTime_Success,
	"testCommitNumberFromTime_ErrorOnTimeTooOld":         testCommitNumberFromTime_ErrorOnTimeTooOld,
}

func testCommitNumberFromGitHash_Success(t *testing.T, ctx context.Context, g *Git, gb *testutils.GitBuilder, hashes []string, cleanup cleanupFunc) {
	unittest.LargeTest(t)
	defer cleanup()

	commitNumber, err := g.CommitNumberFromGitHash(ctx, hashes[0])
	assert.NoError(t, err)
	assert.Equal(t, types.CommitNumber(0), commitNumber)
	commitNumber, err = g.CommitNumberFromGitHash(ctx, hashes[2])
	assert.NoError(t, err)
	assert.Equal(t, types.CommitNumber(2), commitNumber)
}

func testCommitNumberFromGitHash_ErrorOnUnknownGitHash(t *testing.T, ctx context.Context, g *Git, gb *testutils.GitBuilder, hashes []string, cleanup cleanupFunc) {
	unittest.LargeTest(t)
	defer cleanup()

	_, err := g.CommitNumberFromGitHash(ctx, hashes[0]+"obviously_not_a_valid_hash")
	assert.Error(t, err)
}

func testSingleUpdateStep_NewCommitsAreFoundAfterUpdate(t *testing.T, ctx context.Context, g *Git, gb *testutils.GitBuilder, hashes []string, cleanup cleanupFunc) {
	unittest.LargeTest(t)
	defer cleanup()

	// Add new commit to repo, but singleUpdateStep hasn't been done, so it
	// shouldn't appear in the database.
	newHash := gb.CommitGenAt(ctx, "foo.txt", startTime.Add(4*time.Minute))
	commitNumber, err := g.CommitNumberFromGitHash(ctx, newHash)
	require.Error(t, err)

	// After the update step we should find it.
	err = g.singleUpdateStep(ctx)
	require.NoError(t, err)
	commitNumber, err = g.CommitNumberFromGitHash(ctx, newHash)
	assert.Equal(t, types.CommitNumber(4), commitNumber)
}

func testCommitNumberFromTime_Success(t *testing.T, ctx context.Context, g *Git, gb *testutils.GitBuilder, hashes []string, cleanup cleanupFunc) {
	unittest.LargeTest(t)
	defer cleanup()

	commitNumber, err := g.CommitNumberFromTime(ctx, startTime.Add(1*time.Minute))
	assert.NoError(t, err)
	assert.Equal(t, types.CommitNumber(1), commitNumber)

	commitNumber, err = g.CommitNumberFromTime(ctx, startTime.Add(1*time.Minute+time.Second))
	assert.NoError(t, err)
	assert.Equal(t, types.CommitNumber(1), commitNumber)

	commitNumber, err = g.CommitNumberFromTime(ctx, startTime.Add(1*time.Minute-time.Second))
	assert.NoError(t, err)
	assert.Equal(t, types.CommitNumber(0), commitNumber)
}

func testCommitNumberFromTime_ErrorOnTimeTooOld(t *testing.T, ctx context.Context, g *Git, gb *testutils.GitBuilder, hashes []string, cleanup cleanupFunc) {
	unittest.LargeTest(t)
	defer cleanup()

	_, err := g.CommitNumberFromTime(ctx, startTime.Add(-1*time.Minute))
	require.Error(t, err)
}
