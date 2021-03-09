package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/cmd/gitilesfollower/mocks"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

func TestUpdateCycle_EmptyDB_UsesInitialCommit(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	mgl := mocks.GitilesLogger{}
	mgl.On("Log", testutils.AnyContext, "main", mock.Anything).Return([]*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: "4444444444444444444444444444444444444444",
				// The rest is ignored from Log
			},
		},
	}, nil)

	mgl.On("LogFirstParent", testutils.AnyContext, "1111111111111111111111111111111111111111", "4444444444444444444444444444444444444444").Return([]*vcsinfo.LongCommit{
		{ // These are returned with the most recent commits first
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    "4444444444444444444444444444444444444444",
				Author:  "author 4",
				Subject: "subject 4",
			},
			Timestamp: time.Date(2021, time.February, 25, 10, 4, 0, 0, time.UTC),
		},
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    "3333333333333333333333333333333333333333",
				Author:  "author 3",
				Subject: "subject 3",
			},
			Timestamp: time.Date(2021, time.February, 25, 10, 3, 0, 0, time.UTC),
		},
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    "2222222222222222222222222222222222222222",
				Author:  "author 2",
				Subject: "subject 2",
			},
			Timestamp: time.Date(2021, time.February, 25, 10, 2, 0, 0, time.UTC),
		},
		// LogFirstParent excludes the first one mentioned.
	}, nil)

	rfc := repoFollowerConfig{
		Common: config.Common{
			GitRepoBranch: "main",
		},
		InitialCommit: "1111111111111111111111111111111111111111",
	}
	require.NoError(t, updateCycle(ctx, db, &mgl, rfc))

	actualRows := sqltest.GetAllRows(ctx, t, db, "GitCommits", &schema.GitCommitRow{}).([]schema.GitCommitRow)
	assert.Equal(t, []schema.GitCommitRow{{
		GitHash:     "4444444444444444444444444444444444444444",
		CommitID:    "001000000003",
		CommitTime:  time.Date(2021, time.February, 25, 10, 4, 0, 0, time.UTC),
		AuthorEmail: "author 4",
		Subject:     "subject 4",
	}, {
		GitHash:     "3333333333333333333333333333333333333333",
		CommitID:    "001000000002",
		CommitTime:  time.Date(2021, time.February, 25, 10, 3, 0, 0, time.UTC),
		AuthorEmail: "author 3",
		Subject:     "subject 3",
	}, {
		GitHash:     "2222222222222222222222222222222222222222",
		CommitID:    "001000000001",
		CommitTime:  time.Date(2021, time.February, 25, 10, 2, 0, 0, time.UTC),
		AuthorEmail: "author 2",
		Subject:     "subject 2",
	}}, actualRows)
	// The initial commit is not stored in the DB nor queried, but is implicitly has id
	// equal to initialID.
}

func TestUpdateCycle_CommitsInDB_IncrementalUpdate(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := schema.Tables{GitCommits: []schema.GitCommitRow{{
		GitHash:     "4444444444444444444444444444444444444444",
		CommitID:    "001000000003",
		CommitTime:  time.Date(2021, time.February, 25, 10, 4, 0, 0, time.UTC),
		AuthorEmail: "author 4",
		Subject:     "subject 4",
	}, {
		GitHash:     "3333333333333333333333333333333333333333",
		CommitID:    "001000000002",
		CommitTime:  time.Date(2021, time.February, 25, 10, 3, 0, 0, time.UTC),
		AuthorEmail: "author 3",
		Subject:     "subject 3",
	}, {
		GitHash:     "2222222222222222222222222222222222222222",
		CommitID:    "001000000001",
		CommitTime:  time.Date(2021, time.February, 25, 10, 2, 0, 0, time.UTC),
		AuthorEmail: "author 2",
		Subject:     "subject 2",
	}}}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	mgl := mocks.GitilesLogger{}
	mgl.On("Log", testutils.AnyContext, "main", mock.Anything).Return([]*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: "6666666666666666666666666666666666666666",
				// The rest is ignored from Log
			},
		},
	}, nil)

	mgl.On("LogFirstParent", testutils.AnyContext, "4444444444444444444444444444444444444444", "6666666666666666666666666666666666666666").Return([]*vcsinfo.LongCommit{
		{ // These are returned with the most recent commits first
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    "6666666666666666666666666666666666666666",
				Author:  "author 6",
				Subject: "subject 6",
			},
			Timestamp: time.Date(2021, time.February, 25, 10, 6, 0, 0, time.UTC),
		},
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    "5555555555555555555555555555555555555555",
				Author:  "author 5",
				Subject: "subject 5",
			},
			Timestamp: time.Date(2021, time.February, 25, 10, 5, 0, 0, time.UTC),
		},
		// LogFirstParent excludes the first one mentioned.
	}, nil)

	rfc := repoFollowerConfig{
		Common: config.Common{
			GitRepoBranch: "main",
		},
		InitialCommit: "1111111111111111111111111111111111111111", // we expect this to not be used
	}
	require.NoError(t, updateCycle(ctx, db, &mgl, rfc))

	actualRows := sqltest.GetAllRows(ctx, t, db, "GitCommits", &schema.GitCommitRow{}).([]schema.GitCommitRow)
	assert.Equal(t, []schema.GitCommitRow{{
		GitHash:     "6666666666666666666666666666666666666666",
		CommitID:    "001000000005",
		CommitTime:  time.Date(2021, time.February, 25, 10, 6, 0, 0, time.UTC),
		AuthorEmail: "author 6",
		Subject:     "subject 6",
	}, {
		GitHash:     "5555555555555555555555555555555555555555",
		CommitID:    "001000000004",
		CommitTime:  time.Date(2021, time.February, 25, 10, 5, 0, 0, time.UTC),
		AuthorEmail: "author 5",
		Subject:     "subject 5",
	}, {
		GitHash:     "4444444444444444444444444444444444444444",
		CommitID:    "001000000003",
		CommitTime:  time.Date(2021, time.February, 25, 10, 4, 0, 0, time.UTC),
		AuthorEmail: "author 4",
		Subject:     "subject 4",
	}, {
		GitHash:     "3333333333333333333333333333333333333333",
		CommitID:    "001000000002",
		CommitTime:  time.Date(2021, time.February, 25, 10, 3, 0, 0, time.UTC),
		AuthorEmail: "author 3",
		Subject:     "subject 3",
	}, {
		GitHash:     "2222222222222222222222222222222222222222",
		CommitID:    "001000000001",
		CommitTime:  time.Date(2021, time.February, 25, 10, 2, 0, 0, time.UTC),
		AuthorEmail: "author 2",
		Subject:     "subject 2",
	}}, actualRows)
}

func TestUpdateCycle_NoNewCommits_NothingChanges(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := schema.Tables{GitCommits: []schema.GitCommitRow{{
		GitHash:     "4444444444444444444444444444444444444444",
		CommitID:    "001000000003",
		CommitTime:  time.Date(2021, time.February, 25, 10, 4, 0, 0, time.UTC),
		AuthorEmail: "author 4",
		Subject:     "subject 4",
	}, {
		GitHash:  "3333333333333333333333333333333333333333",
		CommitID: "001000000002",
		// Notice this commit comes the latest temporally, but commit_id is what should be use
		// to determine recency.
		CommitTime:  time.Date(2025, time.December, 25, 10, 3, 0, 0, time.UTC),
		AuthorEmail: "author 3",
		Subject:     "subject 3",
	}, {
		GitHash:     "2222222222222222222222222222222222222222",
		CommitID:    "001000000001",
		CommitTime:  time.Date(2021, time.February, 25, 10, 2, 0, 0, time.UTC),
		AuthorEmail: "author 2",
		Subject:     "subject 2",
	}}}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	mgl := mocks.GitilesLogger{}
	mgl.On("Log", testutils.AnyContext, "main", mock.Anything).Return([]*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: "4444444444444444444444444444444444444444",
				// The rest is ignored from Log
			},
		},
	}, nil)

	rfc := repoFollowerConfig{
		Common: config.Common{
			GitRepoBranch: "main",
		},
		InitialCommit: "1111111111111111111111111111111111111111", // we expect this to not be used
	}
	require.NoError(t, updateCycle(ctx, db, &mgl, rfc))

	actualRows := sqltest.GetAllRows(ctx, t, db, "GitCommits", &schema.GitCommitRow{}).([]schema.GitCommitRow)
	assert.Equal(t, []schema.GitCommitRow{{
		GitHash:     "4444444444444444444444444444444444444444",
		CommitID:    "001000000003",
		CommitTime:  time.Date(2021, time.February, 25, 10, 4, 0, 0, time.UTC),
		AuthorEmail: "author 4",
		Subject:     "subject 4",
	}, {
		GitHash:     "3333333333333333333333333333333333333333",
		CommitID:    "001000000002",
		CommitTime:  time.Date(2025, time.December, 25, 10, 3, 0, 0, time.UTC),
		AuthorEmail: "author 3",
		Subject:     "subject 3",
	}, {
		GitHash:     "2222222222222222222222222222222222222222",
		CommitID:    "001000000001",
		CommitTime:  time.Date(2021, time.February, 25, 10, 2, 0, 0, time.UTC),
		AuthorEmail: "author 2",
		Subject:     "subject 2",
	}}, actualRows)
}
