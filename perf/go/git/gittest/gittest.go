// Package gittest has utilities for testing perf/go/git.
package gittest

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	cipd_git "go.skia.org/infra/bazel/external/cipd/git"
	"go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/git/providers/git_checkout"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

const (
	// CockroachDatabaseName is the name of the database in CockroachDB that
	// NewForTest will create.
	CockroachDatabaseName = "git"
)

var (
	// StartTime is the time of the first commit.
	StartTime = time.Unix(1680000000, 0)
)

// NewForTest returns all the necessary variables needed to test against infra/go/git.
// It initializes a new Spanner database which is auto-closed after the test.
func NewForTest(t *testing.T) (context.Context, pool.Pool, *testutils.GitBuilder, []string, provider.Provider, *config.InstanceConfig) {
	db := sqltest.NewSpannerDBForTests(t, "dbgit")
	ctx, gb, hashes, gp, instanceConfig := NewForTestWithDB(t, db)
	return ctx, db, gb, hashes, gp, instanceConfig
}

// NewForTestWithDB returns all the necessary variables needed to test against infra/go/git,
// using the provided database instead of creating a new one.
func NewForTestWithDB(t *testing.T, db pool.Pool) (context.Context, *testutils.GitBuilder, []string, provider.Provider, *config.InstanceConfig) {
	ctx, gb, hashes, gp, instanceConfig, cleanup := NewForTestWithDBNoCleanup(t, db)
	t.Cleanup(cleanup)
	return ctx, gb, hashes, gp, instanceConfig
}

// NewForTestWithDBNoCleanup is like NewForTestWithDB but does not register cleanup on t.
// It returns a cleanup function that must be called to release resources.
func NewForTestWithDBNoCleanup(t *testing.T, db pool.Pool) (context.Context, *testutils.GitBuilder, []string, provider.Provider, *config.InstanceConfig, func()) {
	ctx := cipd_git.UseGitFinder(context.Background())
	ctx, cancel := context.WithCancel(ctx)

	// Create a git repo for testing purposes.
	gb := testutils.GitInit(t, ctx)
	gb.DisableAutoPush = true
	hashes := []string{}
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(2*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "bar.txt", StartTime.Add(3*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(4*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(5*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "bar.txt", StartTime.Add(6*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(7*time.Minute)))

	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(8*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(9*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(10*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(11*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(12*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(13*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(14*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(15*time.Minute)))

	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(16*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(17*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(18*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(19*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(20*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(21*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(22*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(23*time.Minute)))
	gb.Push(ctx)

	// Get tmp dir to use for repo checkout.
	tmpDir, err := os.MkdirTemp("", "git_repo")
	require.NoError(t, err)

	cleanup := func() {
		cancel()
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Printf("Error cleaning up tmpDir %q: %v", tmpDir, err)
		}
		if err := os.RemoveAll(gb.Dir()); err != nil {
			log.Printf("Error cleaning up gb.Dir %q: %v", gb.Dir(), err)
		}
	}

	instanceConfig := &config.InstanceConfig{
		GitRepoConfig: config.GitRepoConfig{
			URL: gb.Dir(),
			Dir: filepath.Join(tmpDir, "checkout"),
		},
		DataStoreConfig: config.DataStoreConfig{
			DataStoreType: config.SpannerDataStoreType,
		},
	}
	gp, err := git_checkout.New(ctx, instanceConfig)
	require.NoError(t, err)
	return ctx, gb, hashes, gp, instanceConfig, cleanup
}
