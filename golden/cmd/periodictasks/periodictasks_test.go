package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
)

func TestGatherFromPrimaryBranch_ReportsAllGroupingsOnPrimaryBranch(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	pub := fakePublisher{}
	g := diffWorkGatherer{
		publisher:  &pub,
		windowSize: 100,
		db:         db,
	}
	require.NoError(t, g.gatherFromPrimaryBranch(ctx))

	assert.ElementsMatch(t, []diff.WorkerMessage{
		{
			Grouping: paramtools.Params{
				types.CorpusField:     dks.RoundCorpus,
				types.PrimaryKeyField: dks.CircleTest,
			},
			AdditionalLeft: nil, AdditionalRight: nil,
		},
		{
			Grouping: paramtools.Params{
				types.CorpusField:     dks.CornersCorpus,
				types.PrimaryKeyField: dks.SquareTest,
			},
			AdditionalLeft: nil, AdditionalRight: nil,
		},
		{
			Grouping: paramtools.Params{
				types.CorpusField:     dks.CornersCorpus,
				types.PrimaryKeyField: dks.TriangleTest,
			},
			AdditionalLeft: nil, AdditionalRight: nil,
		},
	}, pub.messages)
}

func TestGatherFromChangelists_OnlyReportsGroupingsWithDataNotOnPrimaryBranch(t *testing.T) {
	unittest.LargeTest(t)
	fakeNow := time.Date(2021, time.June, 1, 0, 0, 0, 0, time.UTC)
	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)

	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	pub := fakePublisher{}
	g := diffWorkGatherer{
		publisher:        &pub,
		windowSize:       100,
		db:               db,
		mostRecentCLScan: time.Time{}, // Setting this at time.Zero will get us data from all CLS
	}
	require.NoError(t, g.gatherFromChangelists(ctx))

	assert.ElementsMatch(t, []diff.WorkerMessage{
		{ // This entry is from the iOS CL
			Grouping: paramtools.Params{
				types.CorpusField:     dks.RoundCorpus,
				types.PrimaryKeyField: dks.CircleTest,
			},
			AdditionalLeft: []types.Digest{
				dks.DigestC06Pos_CL, dks.DigestC07Unt_CL,
			}, AdditionalRight: nil,
		},
		{ // This entry is from the new tests CL
			Grouping: paramtools.Params{
				types.CorpusField:     dks.RoundCorpus,
				types.PrimaryKeyField: dks.RoundRectTest,
			},
			AdditionalLeft: []types.Digest{
				dks.DigestE01Pos_CL, dks.DigestE02Pos_CL, dks.DigestE03Unt_CL,
			}, AdditionalRight: nil,
		},
		{ // This entry is from the new tests CL
			Grouping: paramtools.Params{
				types.CorpusField:     dks.TextCorpus,
				types.PrimaryKeyField: dks.SevenTest,
			},
			AdditionalLeft: []types.Digest{
				dks.DigestBlank, dks.DigestD01Pos_CL,
			}, AdditionalRight: nil,
		},
	}, pub.messages)
	assert.Equal(t, fakeNow, g.mostRecentCLScan)
}

type fakePublisher struct {
	messages []diff.WorkerMessage
}

func (p *fakePublisher) PublishWork(_ context.Context, grouping paramtools.Params, left, right []types.Digest) error {
	// We need to copy the data that has been given to us in case the caller mutates it.
	var leftCopy []types.Digest
	if len(left) != 0 {
		leftCopy = make([]types.Digest, len(left))
		copy(leftCopy, left)
	}
	var rightCopy []types.Digest
	if len(right) != 0 {
		rightCopy = make([]types.Digest, len(right))
		copy(leftCopy, right)
	}
	msg := diff.WorkerMessage{
		Grouping:        grouping.Copy(),
		AdditionalLeft:  leftCopy,
		AdditionalRight: rightCopy,
	}
	p.messages = append(p.messages, msg)
	return nil
}
