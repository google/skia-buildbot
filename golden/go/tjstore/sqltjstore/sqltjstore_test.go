package sqltjstore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/tjstore"
)

var kitchenData = datakitchensink.Build()

func TestGetTryJob_OnlyExistsAfterPut(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, kitchenData))

	store := New(db)

	const unqualifiedID = "some_nonexisting_tryjob"
	const cis = datakitchensink.BuildBucketInternalCIS
	psID := tjstore.CombinedPSID{
		CL:  datakitchensink.ChangelistIDThatAddsNewTests,
		CRS: datakitchensink.GerritInternalCRS,
		PS:  datakitchensink.PatchsetIDAddsNewCorpus,
	}

	// Should not exist initially
	_, err := store.GetTryJob(ctx, unqualifiedID, cis)
	require.Error(t, err)
	assert.Equal(t, tjstore.ErrNotFound, err)

	tj := ci.TryJob{
		SystemID:    unqualifiedID,
		System:      cis,
		DisplayName: "Fancy-New-Tryjob",
		Updated:     time.Date(2021, time.January, 19, 18, 17, 0, 0, time.UTC),
	}

	err = store.PutTryJob(ctx, psID, tj)
	require.NoError(t, err)

	actual, err := store.GetTryJob(ctx, unqualifiedID, cis)
	require.NoError(t, err)
	assert.Equal(t, tj, actual)

	// Check the created SQL here so we can trust PutTryJob elsewhere if needed.
	row := db.QueryRow(ctx, `SELECT * FROM Tryjobs WHERE tryjob_id = 'buildbucketInternal_some_nonexisting_tryjob'`)
	var rowData schema.TryjobRow
	require.NoError(t, row.Scan(&rowData.TryjobID, &rowData.System, &rowData.ChangelistID,
		&rowData.PatchsetID, &rowData.DisplayName, &rowData.LastIngestedData))
	rowData.LastIngestedData = rowData.LastIngestedData.UTC()
	assert.Equal(t, schema.TryjobRow{
		TryjobID:         "buildbucketInternal_some_nonexisting_tryjob",
		System:           "buildbucketInternal",
		ChangelistID:     "gerrit_internal_CL_new_tests",
		PatchsetID:       "gerrit_internal_PS_adds_new_corpus",
		DisplayName:      "Fancy-New-Tryjob",
		LastIngestedData: time.Date(2021, time.January, 19, 18, 17, 0, 0, time.UTC),
	}, rowData)
}

func TestPutTryJob_ChangelistDoesNotExist_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	store := New(db)

	const unqualifiedID = "some_nonexisting_tryjob"
	const cis = datakitchensink.BuildBucketCIS
	psID := tjstore.CombinedPSID{
		CL:  "Changelist does not exist",
		CRS: datakitchensink.GerritCRS,
		PS:  datakitchensink.PatchSetIDFixesIPadButNotIPhone,
	}
	tj := ci.TryJob{
		SystemID:    unqualifiedID,
		System:      cis,
		DisplayName: "Fancy-New-Tryjob",
		Updated:     time.Date(2021, time.January, 19, 18, 17, 0, 0, time.UTC),
	}

	err := store.PutTryJob(ctx, psID, tj)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `foreign key constraint "fk_changelist_id_ref_changelists"`)
}

func TestPutTryJob_PatchsetDoesNotExist_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, kitchenData))
	store := New(db)

	const unqualifiedID = "some_nonexisting_tryjob"
	const cis = datakitchensink.BuildBucketCIS
	psID := tjstore.CombinedPSID{
		CL:  datakitchensink.ChangelistIDThatAttemptsToFixIOS,
		CRS: datakitchensink.GerritCRS,
		PS:  "Patchset does not exist",
	}
	tj := ci.TryJob{
		SystemID:    unqualifiedID,
		System:      cis,
		DisplayName: "Fancy-New-Tryjob",
		Updated:     time.Date(2021, time.January, 19, 18, 17, 0, 0, time.UTC),
	}

	err := store.PutTryJob(ctx, psID, tj)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `foreign key constraint "fk_patchset_id_ref_patchsets"`)
}

func TestGetTryJobs_ValidID_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, kitchenData))
	store := New(db)

	actual, err := store.GetTryJobs(ctx, tjstore.CombinedPSID{
		CL:  datakitchensink.ChangelistIDThatAttemptsToFixIOS,
		CRS: datakitchensink.GerritCRS,
		PS:  datakitchensink.PatchSetIDFixesIPadButNotIPhone,
	})
	require.NoError(t, err)
	assert.Equal(t, []ci.TryJob{{
		SystemID:    datakitchensink.Tryjob01IPhoneRGB,
		System:      datakitchensink.BuildBucketCIS,
		DisplayName: "Test-iPhone-RGB",
		Updated:     datakitchensink.Tryjob01LastIngested,
	}, {
		SystemID:    datakitchensink.Tryjob02IPad,
		System:      datakitchensink.BuildBucketCIS,
		DisplayName: "Test-iPad-ALL",
		Updated:     datakitchensink.Tryjob02LastIngested,
	}, {
		SystemID:    datakitchensink.Tryjob03TaimenRGB,
		System:      datakitchensink.BuildBucketCIS,
		DisplayName: "Test-taimen-RGB",
		Updated:     datakitchensink.Tryjob03LastIngested,
	}}, actual)
}

func TestGetTryJobs_InvalidID_ReturnsNoResults(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, kitchenData))
	store := New(db)

	actual, err := store.GetTryJobs(ctx, tjstore.CombinedPSID{
		CL:  datakitchensink.ChangelistIDThatAttemptsToFixIOS,
		CRS: datakitchensink.GerritCRS,
		PS:  "Patchset does not exist",
	})
	require.NoError(t, err)
	assert.Empty(t, actual)
}
