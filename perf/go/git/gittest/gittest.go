// Package gittest has utilities for testing perf/go/git.
package gittest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
//
// The repo is populated with 8 commits, one minute apart, starting at StartTime.
//
// The hashes for each commit are going to be random and so are returned also.
func NewForTest(t *testing.T) (context.Context, pool.Pool, *testutils.GitBuilder, []string, provider.Provider, *config.InstanceConfig) {
	ctx := context.Background()
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

	// Init our sql database.
	db := sqltest.NewCockroachDBForTests(t, "dbgit")

	// Get tmp dir to use for repo checkout.
	tmpDir, err := os.MkdirTemp("", "git_repo")
	require.NoError(t, err)

	// Create the cleanup function.
	t.Cleanup(func() {
		cancel()
		err = os.RemoveAll(tmpDir)
		assert.NoError(t, err)
		gb.Cleanup()
	})

	instanceConfig := &config.InstanceConfig{
		GitRepoConfig: config.GitRepoConfig{
			URL: gb.Dir(),
			Dir: filepath.Join(tmpDir, "checkout"),
		},
	}
	gp, err := git_checkout.New(ctx, instanceConfig)
	require.NoError(t, err)
	return ctx, db, gb, hashes, gp, instanceConfig
}
