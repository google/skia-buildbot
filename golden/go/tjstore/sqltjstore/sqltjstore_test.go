package sqltjstore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/tjstore"
	"go.skia.org/infra/golden/go/types"
)

func TestGetTryJob_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, datakitchensink.Build()))

	store := New(db)

	actual, err := store.GetTryJob(ctx, datakitchensink.Tryjob02IPad, datakitchensink.BuildBucketCIS)
	require.NoError(t, err)
	assert.Equal(t, ci.TryJob{
		SystemID:    datakitchensink.Tryjob02IPad,
		System:      datakitchensink.BuildBucketCIS,
		DisplayName: "Test-iPad-ALL",
		Updated:     time.Date(2020, time.December, 10, 3, 2, 1, 0, time.UTC),
	}, actual)
}

func TestGetTryJobs_ValidID_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, datakitchensink.Build()))
	store := New(db)

	actual, err := store.GetTryJobs(ctx, tjstore.CombinedPSID{
		CL:  datakitchensink.ChangelistIDThatAttemptsToFixIOS,
		CRS: datakitchensink.GerritCRS,
		PS:  datakitchensink.PatchSetIDFixesIPadButNotIPhone,
	})
	require.NoError(t, err)
	// As per the Tryjob Store API, we sort them by display name
	assert.Equal(t, []ci.TryJob{{
		SystemID:    datakitchensink.Tryjob02IPad,
		System:      datakitchensink.BuildBucketCIS,
		DisplayName: "Test-iPad-ALL",
		Updated:     datakitchensink.Tryjob02LastIngested,
	}, {
		SystemID:    datakitchensink.Tryjob01IPhoneRGB,
		System:      datakitchensink.BuildBucketCIS,
		DisplayName: "Test-iPhone-RGB",
		Updated:     datakitchensink.Tryjob01LastIngested,
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
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, datakitchensink.Build()))
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
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, datakitchensink.Build()))
	store := New(db)

	results, err := store.GetResults(ctx, tjstore.CombinedPSID{
		CL:  datakitchensink.ChangelistIDThatAttemptsToFixIOS,
		CRS: datakitchensink.GerritCRS,
		PS:  datakitchensink.PatchSetIDFixesIPadButNotIPhone,
	}, time.Time{})
	require.NoError(t, err)

	assert.ElementsMatch(t, []tjstore.TryJobResult{{
		System:   datakitchensink.BuildBucketCIS,
		TryjobID: datakitchensink.Tryjob02IPad,
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
		System:   datakitchensink.BuildBucketCIS,
		TryjobID: datakitchensink.Tryjob02IPad,
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
		System:   datakitchensink.BuildBucketCIS,
		TryjobID: datakitchensink.Tryjob02IPad,
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
		System:   datakitchensink.BuildBucketCIS,
		TryjobID: datakitchensink.Tryjob02IPad,
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
		System:   datakitchensink.BuildBucketCIS,
		TryjobID: datakitchensink.Tryjob02IPad,
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
		System:   datakitchensink.BuildBucketCIS,
		TryjobID: datakitchensink.Tryjob02IPad,
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
		System:   datakitchensink.BuildBucketCIS,
		TryjobID: datakitchensink.Tryjob01IPhoneRGB,
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
		System:   datakitchensink.BuildBucketCIS,
		TryjobID: datakitchensink.Tryjob01IPhoneRGB,
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
		System:   datakitchensink.BuildBucketCIS,
		TryjobID: datakitchensink.Tryjob01IPhoneRGB,
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
		System:   datakitchensink.BuildBucketCIS,
		TryjobID: datakitchensink.Tryjob03TaimenRGB,
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
		System:   datakitchensink.BuildBucketCIS,
		TryjobID: datakitchensink.Tryjob03TaimenRGB,
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
		System:   datakitchensink.BuildBucketCIS,
		TryjobID: datakitchensink.Tryjob03TaimenRGB,
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
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, datakitchensink.Build()))
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
		System:   datakitchensink.BuildBucketCIS,
		TryjobID: datakitchensink.Tryjob01IPhoneRGB,
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
		System:   datakitchensink.BuildBucketCIS,
		TryjobID: datakitchensink.Tryjob01IPhoneRGB,
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
		System:   datakitchensink.BuildBucketCIS,
		TryjobID: datakitchensink.Tryjob01IPhoneRGB,
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
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, datakitchensink.Build()))
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
