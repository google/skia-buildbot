package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
)

func TestGatherFromPrimaryBranch_Success(t *testing.T) {
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

type fakePublisher struct {
	messages []diff.WorkerMessage
}

func (p *fakePublisher) PublishWork(_ context.Context, grouping paramtools.Params, left, right []types.Digest) error {
	msg := diff.WorkerMessage{
		Grouping:        grouping,
		AdditionalLeft:  left,
		AdditionalRight: right,
	}
	p.messages = append(p.messages, msg)
	return nil
}
