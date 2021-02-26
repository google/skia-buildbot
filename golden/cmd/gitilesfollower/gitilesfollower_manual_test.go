package main

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

func TestUpdateCycle_Load1501CommitsFromGitiles_Success(t *testing.T) {
	unittest.ManualTest(t)

	rfc := repoFollowerConfig{
		Common: config.Common{
			GitRepoBranch: "master",
			GitRepoURL:    "https://skia.googlesource.com/skia.git",
		},
		// Arbitrary commit from 14 Dec 2020
		InitialCommit: "9b395f55ea0f8f92103d33f1ea8e8217bee8aaea",
	}
	// Pretend the test is always running on the morning of 26 Feb 2021, with this
	// commit being the most recent. This commit is 1502 commits after the InitialCommit.
	// Since the gitiles API starts after the initial commit, we expect to load 1501 commits.
	const latestGitHash = "453f143dba3fea76bc777b7d6d933c4017f7e4e8"
	ctx := overrideLatestGitHash(latestGitHash)
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	gitilesClient := gitiles.NewRepo(rfc.GitRepoURL, httputils.NewTimeoutClient())
	err := updateCycle(ctx, db, gitilesClient, rfc)
	require.NoError(t, err)

	row := db.QueryRow(ctx, `SELECT count(*) FROM GitCommits`)
	n := 0
	require.NoError(t, row.Scan(&n))
	assert.Equal(t, 1501, n)

	// Spot check some of the entries
	gc := getNthGitCommit(ctx, t, db, 0)
	assert.Equal(t, schema.GitCommitRow{
		GitHash:     latestGitHash,
		CommitID:    "001000001501",
		CommitTime:  time.Date(2021, time.February, 26, 14, 01, 07, 0, time.UTC),
		AuthorEmail: "Joh...",
		Subject:     "Improve dead-code elimination check in SPIR-V.",
	}, gc)

	gc = getNthGitCommit(ctx, t, db, 500)
	assert.Equal(t, schema.GitCommitRow{
		GitHash:     "4d76f63e45f107c9e041ef7ae6534b00b4623044",
		CommitID:    "001000001001",
		CommitTime:  time.Date(2021, time.February, 3, 22, 50, 28, 0, time.UTC),
		AuthorEmail: "Bri...",
		Subject:     "Fix particle bug where uniforms are allocated too late",
	}, gc)

	gc = getNthGitCommit(ctx, t, db, 1000)
	assert.Equal(t, schema.GitCommitRow{
		GitHash:     "9e1cedda632e7fc20147e4e0b4b2f6dc3728283f",
		CommitID:    "001000000501",
		CommitTime:  time.Date(2021, time.January, 14, 14, 38, 18, 0, time.UTC),
		AuthorEmail: "Der...",
		Subject:     "Add generic uniform setter function to SkRuntimeShaderBuilder",
	}, gc)

	gc = getNthGitCommit(ctx, t, db, 1500)
	assert.Equal(t, schema.GitCommitRow{
		GitHash:     "f79b298b9d0fb57827e43967ee8bb799c37f7135",
		CommitID:    "001000000001",
		CommitTime:  time.Date(2020, time.December, 14, 20, 04, 07, 0, time.UTC),
		AuthorEmail: "Mic...",
		Subject:     "Remove UPDATE_DEVICE_CLIP macro",
	}, gc)
}

func getNthGitCommit(ctx context.Context, t *testing.T, db *pgxpool.Pool, n int) schema.GitCommitRow {
	row := db.QueryRow(ctx, `SELECT * FROM GitCommits ORDER BY commit_id DESC LIMIT 1 OFFSET $1`, n)
	var gc schema.GitCommitRow
	require.NoError(t, row.Scan(&gc.GitHash, &gc.CommitID, &gc.CommitTime, &gc.AuthorEmail, &gc.Subject))
	gc.CommitTime = gc.CommitTime.UTC()
	// We would rather not put real-world names and emails in public test assertions
	gc.AuthorEmail = gc.AuthorEmail[:3] + "..."
	return gc
}

func overrideLatestGitHash(gitHash string) context.Context {
	return context.WithValue(context.Background(), overrideLatestCommitKey, gitHash)
}
