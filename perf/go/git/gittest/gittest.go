// Package gittest has utilities for testing perf/go/git.
package gittest

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cipd_git "go.skia.org/infra/bazel/external/cipd/git"
	"go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

const (
	// CockroachDatabaseName is the name of the database in CockroachDB that
	// NewForTest will create.
	CockroachDatabaseName = "git"
)

// CleanupFunc is the type of clean up function that NewForTest returns.
type CleanupFunc func()

var (
	// StartTime is the time of the first commit.
	StartTime = time.Unix(1680000000, 0)
)

// NewForTest returns all the necessary variables needed to test against infra/go/git.
//
// The repo is populated with 8 commits, one minute apart, starting at StartTime.
//
// The hashes for each commit are going to be random and so are returned also.
func NewForTest(t *testing.T) (context.Context, *pgxpool.Pool, *testutils.GitBuilder, []string, *config.InstanceConfig, CleanupFunc) {
	ctx := cipd_git.UseGitFinder(context.Background())
	ctx, cancel := context.WithCancel(ctx)

	// Create a git repo for testing purposes.
	gb := testutils.GitInit(t, ctx)
	hashes := []string{}
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(2*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "bar.txt", StartTime.Add(3*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(4*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(5*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "bar.txt", StartTime.Add(6*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(7*time.Minute)))

	// Init our sql database.
	db, sqlCleanup := sqltest.NewCockroachDBForTests(t, "git")

	// Get tmp dir to use for repo checkout.
	tmpDir, err := ioutil.TempDir("", "git")
	require.NoError(t, err)

	// Create the cleanup function.
	clean := func() {
		cancel()
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
	return ctx, db, gb, hashes, instanceConfig, clean
}
