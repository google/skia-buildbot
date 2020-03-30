// Package git is the minimal interface that Perf need to interact with a Git
// repo.
package git

import (
	"context"
	"database/sql"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
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

func newForTest(t *testing.T, dialect perfsql.Dialect) (context.Context, *Git, *testutils.GitBuilder, []string, cleanupFunc) {
	ctx := context.Background()

	// Create a git repo for testing purposes.
	gb := testutils.GitInit(t, ctx)
	hashes := []string{}
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", startTime))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", startTime.Add(time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", startTime.Add(2*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", startTime.Add(3*time.Minute)))

	// Init our sql database.
	var db *sql.DB
	var sqlCleanup sqltest.Cleanup
	if dialect == perfsql.SQLiteDialect {
		db, sqlCleanup = sqltest.NewSQLite3DBForTests(t)
	} else {
		db, sqlCleanup = sqltest.NewCockroachDBForTests(t, "git", sqltest.ApplyMigrations)
	}

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
		GitRepoConfig: config.GitRepoConfig{
			URL: gb.Dir(),
			Dir: filepath.Join(tmpDir, "checkout"),
		},
	}
	g, err := New(ctx, true, db, dialect, instanceConfig)
	require.NoError(t, err)
	return ctx, g, gb, hashes, clean
}

func TestSQLite(t *testing.T) {
	unittest.LargeTest(t)

	for name, subTest := range subTests {
		t.Run(name, func(t *testing.T) {
			ctx, g, gb, hashes, cleanup := newForTest(t, perfsql.SQLiteDialect)
			subTest(t, ctx, g, gb, hashes, cleanup)
		})
	}
}

func TestCockroachDB(t *testing.T) {
	unittest.LargeTest(t)

	for name, subTest := range subTests {
		t.Run(name, func(t *testing.T) {
			ctx, g, gb, hashes, cleanup := newForTest(t, perfsql.CockroachDBDialect)
			subTest(t, ctx, g, gb, hashes, cleanup)
		})
	}
}

// subTestFunction is a func we will call to test one aspect of *SQLTraceStore.
type subTestFunction func(t *testing.T, ctx context.Context, g *Git, gb *testutils.GitBuilder, hashes []string, cleanup cleanupFunc)

// subTests are all the tests we have for *SQLTraceStore.
var subTests = map[string]subTestFunction{
	"testSingleUpdateStep_NewCommitsAreFoundAfterUpdate": testSingleUpdateStep_NewCommitsAreFoundAfterUpdate,
	"testCommitNumberFromGitHash_Success":                testCommitNumberFromGitHash_Success,
	"testCommitNumberFromGitHash_ErrorOnUnknownGitHash":  testCommitNumberFromGitHash_ErrorOnUnknownGitHash,
	"testCommitNumberFromTime_Success":                   testCommitNumberFromTime_Success,
	"testCommitNumberFromTime_ErrorOnTimeTooOld":         testCommitNumberFromTime_ErrorOnTimeTooOld,
	"testCommitNumberFromTime_SuccessOnZeroTime":         testCommitNumberFromTime_SuccessOnZeroTime,
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

func testCommitNumberFromTime_SuccessOnZeroTime(t *testing.T, ctx context.Context, g *Git, gb *testutils.GitBuilder, hashes []string, cleanup cleanupFunc) {
	unittest.LargeTest(t)
	defer cleanup()

	commitNumber, err := g.CommitNumberFromTime(ctx, time.Time{})
	assert.NoError(t, err)
	assert.Equal(t, types.CommitNumber(len(hashes)-1), commitNumber)
}

func testCommitNumberFromTime_ErrorOnTimeTooOld(t *testing.T, ctx context.Context, g *Git, gb *testutils.GitBuilder, hashes []string, cleanup cleanupFunc) {
	unittest.LargeTest(t)
	defer cleanup()

	_, err := g.CommitNumberFromTime(ctx, startTime.Add(-1*time.Minute))
	require.Error(t, err)
}

func TestParseGitRevLogStream_Success(t *testing.T) {
	unittest.SmallTest(t)
	r := strings.NewReader(
		`commit 6079a7810530025d9877916895dd14eb8bb454c0
Joe Gregorio <joe@bitworking.org>
Change #9
1584837783
commit 977e0ef44bec17659faf8c5d4025c5a068354817
Joe Gregorio <joe@bitworking.org>
Change #8
1584837780`)
	count := 0
	hashes := []string{"6079a7810530025d9877916895dd14eb8bb454c0", "977e0ef44bec17659faf8c5d4025c5a068354817"}
	err := parseGitRevLogStream(ioutil.NopCloser(r), func(p parsedSingleCommit) error {
		assert.Equal(t, "Joe Gregorio <joe@bitworking.org>", p.author)
		assert.Equal(t, hashes[count], p.gitHash)
		count++
		return nil
	})
	assert.Equal(t, 2, count)
	assert.NoError(t, err)
}

func TestParseGitRevLogStream_EmptyFile_Success(t *testing.T) {
	unittest.SmallTest(t)
	r := strings.NewReader("")
	count := 0
	err := parseGitRevLogStream(ioutil.NopCloser(r), func(p parsedSingleCommit) error {
		count++
		return nil
	})
	assert.Equal(t, 0, count)
	assert.NoError(t, err)
}

func TestParseGitRevLogStream_ErrMissingTimestamp(t *testing.T) {
	unittest.SmallTest(t)
	r := strings.NewReader(
		`commit 6079a7810530025d9877916895dd14eb8bb454c0
Joe Gregorio <joe@bitworking.org>
Change #9`)
	count := 0
	err := parseGitRevLogStream(ioutil.NopCloser(r), func(p parsedSingleCommit) error {
		count++
		return nil
	})
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "expecting a timestamp")
}

func TestParseGitRevLogStream_ErrFailedToParseTimestamp(t *testing.T) {
	unittest.SmallTest(t)
	r := strings.NewReader(
		`commit 6079a7810530025d9877916895dd14eb8bb454c0
Joe Gregorio <joe@bitworking.org>
Change #9
ooops 1584837780`)
	count := 0
	err := parseGitRevLogStream(ioutil.NopCloser(r), func(p parsedSingleCommit) error {
		count++
		return nil
	})
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "Failed to parse timestamp")
}

func TestParseGitRevLogStream_ErrMissingSubject(t *testing.T) {
	unittest.SmallTest(t)
	r := strings.NewReader(
		`commit 6079a7810530025d9877916895dd14eb8bb454c0
Joe Gregorio <joe@bitworking.org>`)
	count := 0
	err := parseGitRevLogStream(ioutil.NopCloser(r), func(p parsedSingleCommit) error {
		count++
		return nil
	})
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "expecting a subject")
}

func TestParseGitRevLogStream_ErrMissingAuthor(t *testing.T) {
	unittest.SmallTest(t)
	r := strings.NewReader(
		`commit 6079a7810530025d9877916895dd14eb8bb454c0`)
	count := 0
	err := parseGitRevLogStream(ioutil.NopCloser(r), func(p parsedSingleCommit) error {
		count++
		return nil
	})
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "expecting an author")
}

func TestParseGitRevLogStream_ErrMalformedCommitLine(t *testing.T) {
	unittest.SmallTest(t)
	r := strings.NewReader(
		`something_not_commit 6079a7810530025d9877916895dd14eb8bb454c0`)
	count := 0
	err := parseGitRevLogStream(ioutil.NopCloser(r), func(p parsedSingleCommit) error {
		count++
		return nil
	})
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "expected commit at")
}
