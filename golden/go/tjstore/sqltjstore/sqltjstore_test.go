package sqltjstore

import (
	"context"
	"crypto/md5"
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/tjstore"
	"go.skia.org/infra/golden/go/types"
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
		ChangelistID:     "gerrit-internal_CL_new_tests",
		PatchsetID:       "gerrit-internal_PS_adds_new_corpus",
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

func TestGetResults_ValidID_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, kitchenData))
	store := New(db)

	results, err := store.GetResults(ctx, tjstore.CombinedPSID{
		CL:  datakitchensink.ChangelistIDThatAttemptsToFixIOS,
		CRS: datakitchensink.GerritCRS,
		PS:  datakitchensink.PatchSetIDFixesIPadButNotIPhone,
	}, time.Time{})
	require.NoError(t, err)

	assert.ElementsMatch(t, []tjstore.TryJobResult{{
		ResultParams: paramtools.Params{
			datakitchensink.OSKey:        datakitchensink.IOS,
			datakitchensink.DeviceKey:    datakitchensink.IPadDevice,
			types.CorpusField:            datakitchensink.CornersCorpus,
			types.PrimaryKeyField:        datakitchensink.SquareTest,
			datakitchensink.ColorModeKey: datakitchensink.GreyColorMode,
		},
		Options: paramtools.Params{"ext": "png"},
		Digest:  datakitchensink.DigestA02Pos,
	}, {
		ResultParams: paramtools.Params{
			datakitchensink.OSKey:        datakitchensink.IOS,
			datakitchensink.DeviceKey:    datakitchensink.IPadDevice,
			types.CorpusField:            datakitchensink.CornersCorpus,
			types.PrimaryKeyField:        datakitchensink.TriangleTest,
			datakitchensink.ColorModeKey: datakitchensink.GreyColorMode,
		},
		Options: paramtools.Params{"ext": "png"},
		Digest:  datakitchensink.DigestB02Pos,
	}, {
		ResultParams: paramtools.Params{
			datakitchensink.OSKey:        datakitchensink.IOS,
			datakitchensink.DeviceKey:    datakitchensink.IPadDevice,
			types.CorpusField:            datakitchensink.RoundCorpus,
			types.PrimaryKeyField:        datakitchensink.CircleTest,
			datakitchensink.ColorModeKey: datakitchensink.GreyColorMode,
		},
		Options: paramtools.Params{"ext": "png"},
		Digest:  datakitchensink.DigestC02Pos,
	}, {
		ResultParams: paramtools.Params{
			datakitchensink.OSKey:        datakitchensink.IOS,
			datakitchensink.DeviceKey:    datakitchensink.IPadDevice,
			types.CorpusField:            datakitchensink.CornersCorpus,
			types.PrimaryKeyField:        datakitchensink.SquareTest,
			datakitchensink.ColorModeKey: datakitchensink.RGBColorMode,
		},
		Options: paramtools.Params{"ext": "png"},
		Digest:  datakitchensink.DigestA01Pos,
	}, {
		ResultParams: paramtools.Params{
			datakitchensink.OSKey:        datakitchensink.IOS,
			datakitchensink.DeviceKey:    datakitchensink.IPadDevice,
			types.CorpusField:            datakitchensink.CornersCorpus,
			types.PrimaryKeyField:        datakitchensink.TriangleTest,
			datakitchensink.ColorModeKey: datakitchensink.RGBColorMode,
		},
		Options: paramtools.Params{"ext": "png"},
		Digest:  datakitchensink.DigestB01Pos,
	}, {
		ResultParams: paramtools.Params{
			datakitchensink.OSKey:        datakitchensink.IOS,
			datakitchensink.DeviceKey:    datakitchensink.IPadDevice,
			types.CorpusField:            datakitchensink.RoundCorpus,
			types.PrimaryKeyField:        datakitchensink.CircleTest,
			datakitchensink.ColorModeKey: datakitchensink.RGBColorMode,
		},
		Options: paramtools.Params{"ext": "png"},
		Digest:  datakitchensink.DigestC06Pos_CL,
	}, {
		ResultParams: paramtools.Params{
			datakitchensink.OSKey:        datakitchensink.IOS,
			datakitchensink.DeviceKey:    datakitchensink.IPhoneDevice,
			types.CorpusField:            datakitchensink.CornersCorpus,
			types.PrimaryKeyField:        datakitchensink.SquareTest,
			datakitchensink.ColorModeKey: datakitchensink.RGBColorMode,
		},
		Options: paramtools.Params{"ext": "png"},
		Digest:  datakitchensink.DigestA01Pos,
	}, {
		ResultParams: paramtools.Params{
			datakitchensink.OSKey:        datakitchensink.IOS,
			datakitchensink.DeviceKey:    datakitchensink.IPhoneDevice,
			types.CorpusField:            datakitchensink.CornersCorpus,
			types.PrimaryKeyField:        datakitchensink.TriangleTest,
			datakitchensink.ColorModeKey: datakitchensink.RGBColorMode,
		},
		Options: paramtools.Params{"ext": "png"},
		Digest:  datakitchensink.DigestB01Pos,
	}, {
		ResultParams: paramtools.Params{
			datakitchensink.OSKey:        datakitchensink.IOS,
			datakitchensink.DeviceKey:    datakitchensink.IPhoneDevice,
			types.CorpusField:            datakitchensink.RoundCorpus,
			types.PrimaryKeyField:        datakitchensink.CircleTest,
			datakitchensink.ColorModeKey: datakitchensink.RGBColorMode,
		},
		Options: paramtools.Params{"ext": "png"},
		Digest:  datakitchensink.DigestC07Unt_CL,
	}, {
		ResultParams: paramtools.Params{
			datakitchensink.OSKey:        datakitchensink.AndroidOS,
			datakitchensink.DeviceKey:    datakitchensink.TaimenDevice,
			types.CorpusField:            datakitchensink.CornersCorpus,
			types.PrimaryKeyField:        datakitchensink.SquareTest,
			datakitchensink.ColorModeKey: datakitchensink.RGBColorMode,
		},
		Options: paramtools.Params{"ext": "png"},
		Digest:  datakitchensink.DigestA09Neg,
	}, {
		ResultParams: paramtools.Params{
			datakitchensink.OSKey:        datakitchensink.AndroidOS,
			datakitchensink.DeviceKey:    datakitchensink.TaimenDevice,
			types.CorpusField:            datakitchensink.CornersCorpus,
			types.PrimaryKeyField:        datakitchensink.TriangleTest,
			datakitchensink.ColorModeKey: datakitchensink.RGBColorMode,
		},
		Options: paramtools.Params{"ext": "png"},
		Digest:  datakitchensink.DigestB01Pos,
	}, {
		ResultParams: paramtools.Params{
			datakitchensink.OSKey:        datakitchensink.AndroidOS,
			datakitchensink.DeviceKey:    datakitchensink.TaimenDevice,
			types.CorpusField:            datakitchensink.RoundCorpus,
			types.PrimaryKeyField:        datakitchensink.CircleTest,
			datakitchensink.ColorModeKey: datakitchensink.RGBColorMode,
		},
		Options: paramtools.Params{"ext": "png"},
		Digest:  datakitchensink.DigestC05Unt,
	}}, results)
}

func TestGetResults_TimeIncludesOneTryjob_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, kitchenData))
	store := New(db)

	// PatchSetIDFixesIPadButNotIPhone has data from 3 TryJobs. Of the three, the last one to be
	// ingested was Tryjob01, so we set the time cutoff before that to see that we only get the
	// results from that Tryjob (the iphone results)
	ts := datakitchensink.Tryjob01LastIngested.Add(-time.Second)
	results, err := store.GetResults(ctx, tjstore.CombinedPSID{
		CL:  datakitchensink.ChangelistIDThatAttemptsToFixIOS,
		CRS: datakitchensink.GerritCRS,
		PS:  datakitchensink.PatchSetIDFixesIPadButNotIPhone,
	}, ts)
	require.NoError(t, err)

	assert.ElementsMatch(t, []tjstore.TryJobResult{{
		ResultParams: paramtools.Params{
			datakitchensink.OSKey:        datakitchensink.IOS,
			datakitchensink.DeviceKey:    datakitchensink.IPhoneDevice,
			types.CorpusField:            datakitchensink.CornersCorpus,
			types.PrimaryKeyField:        datakitchensink.SquareTest,
			datakitchensink.ColorModeKey: datakitchensink.RGBColorMode,
		},
		Options: paramtools.Params{"ext": "png"},
		Digest:  datakitchensink.DigestA01Pos,
	}, {
		ResultParams: paramtools.Params{
			datakitchensink.OSKey:        datakitchensink.IOS,
			datakitchensink.DeviceKey:    datakitchensink.IPhoneDevice,
			types.CorpusField:            datakitchensink.CornersCorpus,
			types.PrimaryKeyField:        datakitchensink.TriangleTest,
			datakitchensink.ColorModeKey: datakitchensink.RGBColorMode,
		},
		Options: paramtools.Params{"ext": "png"},
		Digest:  datakitchensink.DigestB01Pos,
	}, {
		ResultParams: paramtools.Params{
			datakitchensink.OSKey:        datakitchensink.IOS,
			datakitchensink.DeviceKey:    datakitchensink.IPhoneDevice,
			types.CorpusField:            datakitchensink.RoundCorpus,
			types.PrimaryKeyField:        datakitchensink.CircleTest,
			datakitchensink.ColorModeKey: datakitchensink.RGBColorMode,
		},
		Options: paramtools.Params{"ext": "png"},
		Digest:  datakitchensink.DigestC07Unt_CL,
	}}, results)
}

func TestGetResults_TimeIncludesNoTryjobs_ReturnsEmptyResults(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, kitchenData))
	store := New(db)

	// PatchSetIDFixesIPadButNotIPhone has data from 3 TryJobs. Of the three, the last one to be
	// ingested was Tryjob01, so if we put the time cutoff after this, there should be no data.
	ts := datakitchensink.Tryjob01LastIngested.Add(time.Second)
	results, err := store.GetResults(ctx, tjstore.CombinedPSID{
		CL:  datakitchensink.ChangelistIDThatAttemptsToFixIOS,
		CRS: datakitchensink.GerritCRS,
		PS:  datakitchensink.PatchSetIDFixesIPadButNotIPhone,
	}, ts)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestPutResults_NewTest_CreatesRowsInForeignTables(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, kitchenData))
	store := New(db)

	cID := tjstore.CombinedPSID{
		CL:  datakitchensink.ChangelistIDThatAddsNewTests,
		CRS: datakitchensink.GerritInternalCRS,
		PS:  datakitchensink.PatchsetIDAddsNewCorpus,
	}
	ts := time.Date(2021, time.January, 20, 19, 18, 17, 0, time.UTC)
	const dataFileSource = "gs://my-bucket/my-file.json"
	digestBytes, err := sql.DigestToBytes(datakitchensink.DigestBlank)
	require.NoError(t, err)

	// This should be a never-before-seen trace as well as a never-before-seen options set.
	err = store.PutResults(ctx, cID, datakitchensink.Tryjob04Windows, datakitchensink.BuildBucketInternalCIS,
		dataFileSource, []tjstore.TryJobResult{{
			GroupParams: paramtools.Params{
				datakitchensink.OSKey:     datakitchensink.Windows10dot3OS,
				datakitchensink.DeviceKey: datakitchensink.QuadroDevice,
			},
			ResultParams: paramtools.Params{
				types.CorpusField:     "my new corpus",
				types.PrimaryKeyField: "my new test",
			},
			Options: paramtools.Params{"ext": "png", "ignore": "1"},
			Digest:  datakitchensink.DigestBlank,
		}}, ts)
	require.NoError(t, err)

	newGroupingID := assertGroupingExists(t, db, paramtools.Params{
		types.CorpusField:     "my new corpus",
		types.PrimaryKeyField: "my new test",
	})
	newTraceID := assertTraceCreated(t, db, paramtools.Params{
		datakitchensink.OSKey:     datakitchensink.Windows10dot3OS,
		datakitchensink.DeviceKey: datakitchensink.QuadroDevice,
		types.CorpusField:         "my new corpus",
		types.PrimaryKeyField:     "my new test",
	}, newGroupingID)
	newOptionsID := assertOptionsExist(t, db, paramtools.Params{"ext": "png", "ignore": "1"})
	newSourceFileID := assertSourceFileIngested(t, db, dataFileSource, ts)
	assertTryjobTimestampWasUpdated(t, db, "buildbucketInternal_tryjob_04_windows", ts)

	row := db.QueryRow(ctx, `
SELECT * FROM SecondaryBranchValues WHERE secondary_branch_trace_id = $1`, newTraceID)
	var sbvr schema.SecondaryBranchValueRow
	require.NoError(t, row.Scan(&sbvr.BranchName, &sbvr.VersionName, &sbvr.TraceID,
		&sbvr.Digest, &sbvr.GroupingID, &sbvr.OptionsID, &sbvr.SourceFileID, &sbvr.TryjobID))

	assert.Equal(t, schema.SecondaryBranchValueRow{
		BranchName:   "gerrit-internal_CL_new_tests",
		VersionName:  "gerrit-internal_PS_adds_new_corpus",
		TraceID:      newTraceID,
		Digest:       digestBytes,
		GroupingID:   newGroupingID,
		OptionsID:    newOptionsID,
		SourceFileID: newSourceFileID,
		TryjobID:     "buildbucketInternal_tryjob_04_windows",
	}, sbvr)

	// Spot check that caching works
	assert.True(t, store.keyValueCache.Contains(string(newTraceID)))
	assert.True(t, store.keyValueCache.Contains(string(newGroupingID)))
	assert.True(t, store.keyValueCache.Contains(string(newOptionsID)))
	assert.False(t, store.keyValueCache.Contains(string(newSourceFileID)))
}

func TestPutResults_DataAlreadyExists_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, kitchenData))
	store := New(db)

	cID := tjstore.CombinedPSID{
		CL:  datakitchensink.ChangelistIDThatAddsNewTests,
		CRS: datakitchensink.GerritInternalCRS,
		PS:  datakitchensink.PatchsetIDAddsNewCorpus,
	}
	ts := time.Date(2021, time.January, 20, 19, 18, 17, 0, time.UTC)
	// We create a new tryjob to have the new data to make it easy to verify which data was added
	// (because the timestamp will be applied to the tryjob, letting us pull just that tryjob's
	// data with GetResults).
	err := store.PutTryJob(ctx, cID, ci.TryJob{
		SystemID:    "my_new_tryjob",
		System:      datakitchensink.BuildBucketInternalCIS,
		DisplayName: "A brand new tryjob",
		Updated:     ts.Add(-time.Minute),
	})
	require.NoError(t, err)

	// Make sure we have an expected number of results before we overwrite things.
	const expectedResultsCount = 40
	count := 0
	row := db.QueryRow(ctx, `SELECT count(*) FROM SecondaryBranchValues`)
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, expectedResultsCount, count)

	err = store.PutResults(ctx, cID, "my_new_tryjob", datakitchensink.BuildBucketInternalCIS,
		"gs://somefile.json", []tjstore.TryJobResult{{
			GroupParams: paramtools.Params{
				datakitchensink.OSKey:        datakitchensink.Windows10dot3OS,
				datakitchensink.DeviceKey:    datakitchensink.QuadroDevice,
				datakitchensink.ColorModeKey: datakitchensink.RGBColorMode,
			},
			ResultParams: paramtools.Params{
				types.CorpusField:     datakitchensink.CornersCorpus,
				types.PrimaryKeyField: datakitchensink.SquareTest,
			},
			Options: paramtools.Params{"ext": "png"},
			Digest:  datakitchensink.DigestBlank,
		}, {
			GroupParams: paramtools.Params{
				datakitchensink.OSKey:        datakitchensink.Windows10dot3OS,
				datakitchensink.DeviceKey:    datakitchensink.QuadroDevice,
				datakitchensink.ColorModeKey: datakitchensink.GreyColorMode,
			},
			ResultParams: paramtools.Params{
				types.CorpusField:     datakitchensink.RoundCorpus,
				types.PrimaryKeyField: datakitchensink.CircleTest,
			},
			Options: paramtools.Params{"ext": "png"},
			Digest:  datakitchensink.DigestBlank,
		}}, ts)
	require.NoError(t, err)

	// Choose the time so we only get the data we just added.
	results, err := store.GetResults(ctx, cID, ts.Add(-time.Second))
	require.NoError(t, err)
	assert.ElementsMatch(t, []tjstore.TryJobResult{{
		ResultParams: paramtools.Params{
			datakitchensink.OSKey:        datakitchensink.Windows10dot3OS,
			datakitchensink.DeviceKey:    datakitchensink.QuadroDevice,
			datakitchensink.ColorModeKey: datakitchensink.RGBColorMode,
			types.CorpusField:            datakitchensink.CornersCorpus,
			types.PrimaryKeyField:        datakitchensink.SquareTest,
		},
		Options: paramtools.Params{"ext": "png"},
		Digest:  datakitchensink.DigestBlank,
	}, {
		ResultParams: paramtools.Params{
			datakitchensink.OSKey:        datakitchensink.Windows10dot3OS,
			datakitchensink.DeviceKey:    datakitchensink.QuadroDevice,
			datakitchensink.ColorModeKey: datakitchensink.GreyColorMode,
			types.CorpusField:            datakitchensink.RoundCorpus,
			types.PrimaryKeyField:        datakitchensink.CircleTest,
		},
		Options: paramtools.Params{"ext": "png"},
		Digest:  datakitchensink.DigestBlank,
	}}, results)

	// And the same number after, to make sure this test overwrites existing data and doesn't just
	// make new data (unintentionally).
	count = 0
	row = db.QueryRow(ctx, `SELECT count(*) FROM SecondaryBranchValues`)
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, expectedResultsCount, count)
}

func assertGroupingExists(t *testing.T, db *pgxpool.Pool, params paramtools.Params) schema.GroupingID {
	_, groupingID := sql.SerializeMap(params)
	row := db.QueryRow(context.Background(), `SELECT count(*) FROM Groupings WHERE grouping_id = $1`, groupingID)
	n := 0
	require.NoError(t, row.Scan(&n))
	assert.Equal(t, 1, n, "grouping %x for %v not created", groupingID, params)
	return groupingID
}

func assertTraceCreated(t *testing.T, db *pgxpool.Pool, params paramtools.Params, expectedGroupingID schema.GroupingID) schema.TraceID {
	_, traceID := sql.SerializeMap(params)
	row := db.QueryRow(context.Background(), `SELECT * FROM Traces WHERE trace_id = $1`, traceID)
	var actual schema.TraceRow
	var matches pgtype.Bool
	require.NoError(t, row.Scan(&actual.TraceID, &actual.Corpus, &actual.GroupingID, &actual.Keys, &matches))
	assert.Equal(t, schema.TraceRow{
		TraceID:    traceID,
		Corpus:     params[types.CorpusField],
		GroupingID: expectedGroupingID,
		Keys:       params,
	}, actual)
	assert.Equal(t, pgtype.Null, matches.Status)
	return traceID
}

func assertOptionsExist(t *testing.T, db *pgxpool.Pool, params paramtools.Params) schema.OptionsID {
	_, optionsID := sql.SerializeMap(params)
	row := db.QueryRow(context.Background(), `SELECT count(*) FROM Options WHERE options_id = $1`, optionsID)
	n := 0
	require.NoError(t, row.Scan(&n))
	assert.Equal(t, 1, n, "options %x for %v not created", optionsID, params)
	return optionsID
}

func assertSourceFileIngested(t *testing.T, db *pgxpool.Pool, fileName string, lastIngested time.Time) schema.SourceFileID {
	f := md5.Sum([]byte(fileName))
	fileID := f[:]
	row := db.QueryRow(context.Background(), `SELECT * FROM SourceFiles WHERE source_file_id = $1`, fileID)
	var actual schema.SourceFileRow
	require.NoError(t, row.Scan(&actual.SourceFileID, &actual.SourceFile, &actual.LastIngested))
	actual.LastIngested = actual.LastIngested.UTC()
	assert.Equal(t, schema.SourceFileRow{
		SourceFileID: fileID,
		SourceFile:   fileName,
		LastIngested: lastIngested,
	}, actual)
	return fileID
}

func assertTryjobTimestampWasUpdated(t *testing.T, db *pgxpool.Pool, tjID string, ts time.Time) {
	row := db.QueryRow(context.Background(), `SELECT last_ingested_data FROM TryJobs WHERE tryjob_id = $1`, tjID)
	var actual time.Time
	require.NoError(t, row.Scan(&actual))
	assert.Equal(t, ts, actual.UTC())
}
