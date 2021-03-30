package search2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

func TestNewAndUntriagedSummaryForCL_OnePatchset_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db)
	rv, err := s.NewAndUntriagedSummaryForCL(ctx, dks.GerritCRS, dks.ChangelistIDThatAttemptsToFixIOS)
	require.NoError(t, err)

	assert.Equal(t, NewAndUntriagedSummary{
		ChangelistID: dks.ChangelistIDThatAttemptsToFixIOS,
		PatchsetSummaries: []PatchsetNewAndUntriagedSummary{{
			PatchsetNewImages: 2, // DigestC07Unt_CL and DigestC06Pos_CL
			// Despite the fact the CL also produced DigestC05Unt, C05 is already on the primary branch
			// and should thus be excluded from the TotalNewUntriagedImages. Only 1 of the two
			// CLs "new images" is untriaged, so that's what we report.
			PatchsetNewUntriagedImages: 1,
			PatchsetID:                 dks.PatchSetIDFixesIPadButNotIPhone,
			PatchsetOrder:              3,
		}},
	}, rv)
}

func TestNewAndUntriagedSummaryForCL_TwoPatchsets_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	s := New(db)
	rv, err := s.NewAndUntriagedSummaryForCL(ctx, dks.GerritInternalCRS, dks.ChangelistIDThatAddsNewTests)
	require.NoError(t, err)

	assert.Equal(t, NewAndUntriagedSummary{
		ChangelistID: dks.ChangelistIDThatAddsNewTests,
		// Should be sorted by PatchsetOrder
		PatchsetSummaries: []PatchsetNewAndUntriagedSummary{{
			// One grouping (Text-Seven) produced one image that had not been seen on that grouping
			// before (DigestBlank). This digest *had* been seen on the primary branch in a
			// different grouping, but that should not prevent us from letting a developer know.
			PatchsetNewImages:          1,
			PatchsetNewUntriagedImages: 1,
			PatchsetID:                 dks.PatchsetIDAddsNewCorpus,
			PatchsetOrder:              1,
		}, {
			// Two groupings (Text-Seven and Round-RoundRect) produced 1 and 3 new digests
			// respectively. DigestE03Unt_CL remains untriaged.
			PatchsetNewImages:          4,
			PatchsetNewUntriagedImages: 1,
			PatchsetID:                 dks.PatchsetIDAddsNewCorpusAndTest,
			PatchsetOrder:              4,
		}},
	}, rv)
}

// TODO(kjlubick) test case for:
//    - new images produced on PS1, PS2, and PS3, but triaged after PS3 (should all be counted as
//         "untriaged")
//    - Same image on different groupings counts multiple times.
//    - CL does not exist
//    - CL is closed (shouldn't matter)
//    - New digest triaged on different CL (should not be affected)
