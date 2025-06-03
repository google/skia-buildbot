package ingestion_processors

import (
	"context"
	"sync"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/code_review/gerrit_crs"
	mock_crs "go.skia.org/infra/golden/go/code_review/mocks"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	mock_cis "go.skia.org/infra/golden/go/continuous_integration/mocks"
	"go.skia.org/infra/golden/go/continuous_integration/simple_cis"
	"go.skia.org/infra/golden/go/ingestion"
	"go.skia.org/infra/golden/go/ingestion_processors/mocks"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
)

func TestTryjobSQL_SingleCRSAndCIS_Success(t *testing.T) {

	configParams := map[string]string{
		codeReviewSystemsParam: "gerrit,gerrit-internal",
		gerritURLParam:         "https://example-review.googlesource.com",
		gerritInternalURLParam: "https://example-internal-review.googlesource.com",

		continuousIntegrationSystemsParam: "buildbucket",
	}
	ctx := gerrit_crs.TestContext(context.Background())
	p, err := TryjobSQL(ctx, nil, configParams, httputils.NewTimeoutClient(), nil)
	require.NoError(t, err)
	require.NotNil(t, p)

	gtp, ok := p.(*goldTryjobProcessor)
	require.True(t, ok)
	assert.Len(t, gtp.reviewSystems, 2)
	assert.Len(t, gtp.cisClients, 1)
	assert.Contains(t, gtp.cisClients, buildbucketCIS)
	assert.Nil(t, gtp.lookupSystem)
}

func TestTryjobSQL_SingleCRSDoubleCIS_Success(t *testing.T) {

	configParams := map[string]string{
		codeReviewSystemsParam:     "github",
		githubRepoParam:            "google/skia",
		githubCredentialsPathParam: "testdata/fake_token", // this is actually a file on disk.

		continuousIntegrationSystemsParam: "cirrus,buildbucket",
	}

	ctx := gerrit_crs.TestContext(context.Background())
	p, err := TryjobSQL(ctx, nil, configParams, httputils.NewTimeoutClient(), nil)
	require.NoError(t, err)
	require.NotNil(t, p)

	gtp, ok := p.(*goldTryjobProcessor)
	require.True(t, ok)
	assert.Len(t, gtp.reviewSystems, 1)
	assert.Len(t, gtp.cisClients, 2)
	assert.Contains(t, gtp.cisClients, cirrusCIS)
	assert.Contains(t, gtp.cisClients, buildbucketCIS)
	assert.Nil(t, gtp.lookupSystem)
}

func TestTryjobSQL_LookupCRS_Success(t *testing.T) {

	configParams := map[string]string{
		lookupParam:            "buildbucket",
		codeReviewSystemsParam: "gerrit",
		gerritURLParam:         "https://example-review.googlesource.com",

		continuousIntegrationSystemsParam: "buildbucket",
	}

	ctx := gerrit_crs.TestContext(context.Background())
	p, err := TryjobSQL(ctx, nil, configParams, httputils.NewTimeoutClient(), nil)
	require.NoError(t, err)
	require.NotNil(t, p)

	gtp, ok := p.(*goldTryjobProcessor)
	require.True(t, ok)
	assert.Len(t, gtp.reviewSystems, 1)
	assert.Len(t, gtp.cisClients, 1)
	assert.NotNil(t, gtp.lookupSystem)
}

func TestTryjobSQL_Process_FirstFileForCL_Success_cdb(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	const clID = dks.ChangelistIDThatAttemptsToFixIOS
	const psID = dks.PatchSetIDFixesIPadButNotIPhone
	const tjID = dks.Tryjob01IPhoneRGB
	const expectedPSOrder = 3
	const squareTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"square","os":"iOS","source_type":"corners"}`
	const triangleTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"triangle","os":"iOS","source_type":"corners"}`
	const circleTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"circle","os":"iOS","source_type":"round"}`

	const qualifiedCL = "gerrit_CL_fix_ios"
	const qualifiedPS = "gerrit_PS_fixes_ipad_but_not_iphone"
	const qualifiedTJ = "buildbucket_tryjob_01_iphonergb"
	mcrs := &mock_crs.Client{}
	mcrs.On("GetChangelist", testutils.AnyContext, clID).Return(code_review.Changelist{
		SystemID: clID,
		Owner:    dks.UserOne,
		Status:   code_review.Open,
		Subject:  "Fix iOS",
		// This time should get overwritten by the fakeIngestionTime
		Updated: time.Date(2020, time.December, 5, 15, 0, 0, 0, time.UTC),
	}, nil)
	mcrs.On("GetPatchset", testutils.AnyContext, clID, "", expectedPSOrder).Return(code_review.Patchset{
		SystemID:     psID,
		ChangelistID: clID,
		Order:        expectedPSOrder,
		GitHash:      "ffff111111111111111111111111111111111111",
		Created:      time.Date(2020, time.December, 6, 6, 0, 0, 0, time.UTC),
	}, nil)

	mcis := &mock_cis.Client{}
	mcis.On("GetTryJob", testutils.AnyContext, tjID).Return(ci.TryJob{
		SystemID:    tjID,
		System:      dks.BuildBucketCIS,
		DisplayName: "Test-iPhone-RGB",
	}, nil)

	// This file has data from 3 traces across 2 corpora. The data is for the patchset with order 3.
	src := fakeGCSSourceFromFile(t, "from_goldctl_legacy_fields.json")
	gtp := initCaches(goldTryjobProcessor{
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		db:     db,
		source: src,
	})

	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, dks.Tryjob01FileIPhoneRGB)
	require.NoError(t, err)

	actualSourceFiles := sqltest.GetAllRows(ctx, t, db, "SourceFiles", &schema.SourceFileRow{}).([]schema.SourceFileRow)
	assert.Equal(t, []schema.SourceFileRow{{
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		SourceFile:   dks.Tryjob01FileIPhoneRGB,
		LastIngested: fakeIngestionTime,
	}}, actualSourceFiles)

	actualChangelists := sqltest.GetAllRows(ctx, t, db, "Changelists", &schema.ChangelistRow{}).([]schema.ChangelistRow)
	assert.Equal(t, []schema.ChangelistRow{{
		ChangelistID:     qualifiedCL,
		System:           dks.GerritCRS,
		Status:           schema.StatusOpen,
		OwnerEmail:       dks.UserOne,
		Subject:          "Fix iOS",
		LastIngestedData: fakeIngestionTime,
	}}, actualChangelists)

	actualPatchsets := sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{}).([]schema.PatchsetRow)
	assert.Equal(t, []schema.PatchsetRow{{
		PatchsetID:   qualifiedPS,
		System:       dks.GerritCRS,
		ChangelistID: qualifiedCL,
		Order:        3,
		GitHash:      "ffff111111111111111111111111111111111111",
		Created:      time.Date(2020, time.December, 6, 6, 0, 0, 0, time.UTC),
	}}, actualPatchsets)

	actualTryjobs := sqltest.GetAllRows(ctx, t, db, "Tryjobs", &schema.TryjobRow{}).([]schema.TryjobRow)
	assert.Equal(t, []schema.TryjobRow{{
		TryjobID:         qualifiedTJ,
		System:           dks.BuildBucketCIS,
		ChangelistID:     qualifiedCL,
		PatchsetID:       qualifiedPS,
		DisplayName:      "Test-iPhone-RGB",
		LastIngestedData: fakeIngestionTime,
	}}, actualTryjobs)

	actualGroupings := sqltest.GetAllRows(ctx, t, db, "Groupings", &schema.GroupingRow{}).([]schema.GroupingRow)
	assert.ElementsMatch(t, []schema.GroupingRow{{
		GroupingID: h(circleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.RoundCorpus,
			types.PrimaryKeyField: dks.CircleTest,
		},
	}, {
		GroupingID: h(squareGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
		},
	}, {
		GroupingID: h(triangleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.TriangleTest,
		},
	}}, actualGroupings)

	actualOptions := sqltest.GetAllRows(ctx, t, db, "Options", &schema.OptionsRow{}).([]schema.OptionsRow)
	assert.ElementsMatch(t, []schema.OptionsRow{{
		OptionsID: h(pngOptions),
		Keys: map[string]string{
			"ext": "png",
		},
	}}, actualOptions)

	actualTraces := sqltest.GetAllRows(ctx, t, db, "Traces", &schema.TraceRow{}).([]schema.TraceRow)
	assert.Equal(t, []schema.TraceRow{{
		TraceID:    h(circleTraceKeys),
		Corpus:     dks.RoundCorpus,
		GroupingID: h(circleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.RoundCorpus,
			types.PrimaryKeyField: dks.CircleTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.IOS,
			dks.DeviceKey:         dks.IPhoneDevice,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}, {
		TraceID:    h(squareTraceKeys),
		Corpus:     dks.CornersCorpus,
		GroupingID: h(squareGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.IOS,
			dks.DeviceKey:         dks.IPhoneDevice,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}, {
		TraceID:    h(triangleTraceKeys),
		Corpus:     dks.CornersCorpus,
		GroupingID: h(triangleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.TriangleTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.IOS,
			dks.DeviceKey:         dks.IPhoneDevice,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}}, actualTraces)

	actualParams := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchParams", &schema.SecondaryBranchParamRow{}).([]schema.SecondaryBranchParamRow)
	assert.ElementsMatch(t, []schema.SecondaryBranchParamRow{
		{Key: dks.ColorModeKey, Value: dks.RGBColorMode, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: dks.DeviceKey, Value: dks.IPhoneDevice, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.PrimaryKeyField, Value: dks.CircleTest, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.PrimaryKeyField, Value: dks.SquareTest, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.PrimaryKeyField, Value: dks.TriangleTest, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: dks.OSKey, Value: dks.IOS, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.CorpusField, Value: dks.CornersCorpus, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.CorpusField, Value: dks.RoundCorpus, BranchName: qualifiedCL, VersionName: qualifiedPS},
	}, actualParams)

	actualValues := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchValues", &schema.SecondaryBranchValueRow{}).([]schema.SecondaryBranchValueRow)
	assert.ElementsMatch(t, []schema.SecondaryBranchValueRow{{
		BranchName: qualifiedCL, VersionName: qualifiedPS,
		TraceID:      h(squareTraceKeys),
		Digest:       d(dks.DigestA01Pos),
		GroupingID:   h(squareGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     qualifiedTJ,
	}, {
		BranchName: qualifiedCL, VersionName: qualifiedPS,
		TraceID:      h(triangleTraceKeys),
		Digest:       d(dks.DigestB01Pos),
		GroupingID:   h(triangleGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     qualifiedTJ,
	}, {
		BranchName: qualifiedCL, VersionName: qualifiedPS,
		TraceID:      h(circleTraceKeys),
		Digest:       d(dks.DigestC07Unt_CL),
		GroupingID:   h(circleGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     qualifiedTJ,
	}}, actualValues)

	// We only write to SecondaryBranchExpectations when something is explicitly triaged.
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "SecondaryBranchExpectations", &schema.SecondaryBranchExpectationRow{}))

	// Unlike the primary branch ingestion, these should be empty
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "CommitsWithData", &schema.CommitWithDataRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "TraceValues", &schema.TraceValueRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "ValuesAtHead", &schema.ValueAtHeadRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "Expectations", &schema.ExpectationRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "PrimaryBranchParams", &schema.PrimaryBranchParamRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "TiledTraceDigests", &schema.TiledTraceDigestRow{}))
}

func TestTryjobSQL_Process_FirstFileForCL_Success(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewSpannerDBForTestsWithProductionSchema(ctx, t)

	const clID = dks.ChangelistIDThatAttemptsToFixIOS
	const psID = dks.PatchSetIDFixesIPadButNotIPhone
	const tjID = dks.Tryjob01IPhoneRGB
	const expectedPSOrder = 3
	const squareTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"square","os":"iOS","source_type":"corners"}`
	const triangleTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"triangle","os":"iOS","source_type":"corners"}`
	const circleTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"circle","os":"iOS","source_type":"round"}`

	const qualifiedCL = "gerrit_CL_fix_ios"
	const qualifiedPS = "gerrit_PS_fixes_ipad_but_not_iphone"
	const qualifiedTJ = "buildbucket_tryjob_01_iphonergb"
	mcrs := &mock_crs.Client{}
	mcrs.On("GetChangelist", testutils.AnyContext, clID).Return(code_review.Changelist{
		SystemID: clID,
		Owner:    dks.UserOne,
		Status:   code_review.Open,
		Subject:  "Fix iOS",
		// This time should get overwritten by the fakeIngestionTime
		Updated: time.Date(2020, time.December, 5, 15, 0, 0, 0, time.UTC),
	}, nil)
	mcrs.On("GetPatchset", testutils.AnyContext, clID, "", expectedPSOrder).Return(code_review.Patchset{
		SystemID:     psID,
		ChangelistID: clID,
		Order:        expectedPSOrder,
		GitHash:      "ffff111111111111111111111111111111111111",
		Created:      time.Date(2020, time.December, 6, 6, 0, 0, 0, time.UTC),
	}, nil)

	mcis := &mock_cis.Client{}
	mcis.On("GetTryJob", testutils.AnyContext, tjID).Return(ci.TryJob{
		SystemID:    tjID,
		System:      dks.BuildBucketCIS,
		DisplayName: "Test-iPhone-RGB",
	}, nil)

	// This file has data from 3 traces across 2 corpora. The data is for the patchset with order 3.
	src := fakeGCSSourceFromFile(t, "from_goldctl_legacy_fields.json")
	gtp := initCaches(goldTryjobProcessor{
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		db:     db,
		source: src,
	})

	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, dks.Tryjob01FileIPhoneRGB)
	require.NoError(t, err)

	actualSourceFiles := sqltest.GetAllRows(ctx, t, db, "SourceFiles", &schema.SourceFileRow{}).([]schema.SourceFileRow)
	assert.Equal(t, []schema.SourceFileRow{{
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		SourceFile:   dks.Tryjob01FileIPhoneRGB,
		LastIngested: fakeIngestionTime,
	}}, actualSourceFiles)

	actualChangelists := sqltest.GetAllRows(ctx, t, db, "Changelists", &schema.ChangelistRow{}).([]schema.ChangelistRow)
	assert.Equal(t, []schema.ChangelistRow{{
		ChangelistID:     qualifiedCL,
		System:           dks.GerritCRS,
		Status:           schema.StatusOpen,
		OwnerEmail:       dks.UserOne,
		Subject:          "Fix iOS",
		LastIngestedData: fakeIngestionTime,
	}}, actualChangelists)

	actualPatchsets := sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{}).([]schema.PatchsetRow)
	assert.Equal(t, []schema.PatchsetRow{{
		PatchsetID:   qualifiedPS,
		System:       dks.GerritCRS,
		ChangelistID: qualifiedCL,
		Order:        3,
		GitHash:      "ffff111111111111111111111111111111111111",
		Created:      time.Date(2020, time.December, 6, 6, 0, 0, 0, time.UTC),
	}}, actualPatchsets)

	actualTryjobs := sqltest.GetAllRows(ctx, t, db, "Tryjobs", &schema.TryjobRow{}).([]schema.TryjobRow)
	assert.Equal(t, []schema.TryjobRow{{
		TryjobID:         qualifiedTJ,
		System:           dks.BuildBucketCIS,
		ChangelistID:     qualifiedCL,
		PatchsetID:       qualifiedPS,
		DisplayName:      "Test-iPhone-RGB",
		LastIngestedData: fakeIngestionTime,
	}}, actualTryjobs)

	actualGroupings := sqltest.GetAllRows(ctx, t, db, "Groupings", &schema.GroupingRow{}).([]schema.GroupingRow)
	assert.ElementsMatch(t, []schema.GroupingRow{{
		GroupingID: h(circleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.RoundCorpus,
			types.PrimaryKeyField: dks.CircleTest,
		},
	}, {
		GroupingID: h(squareGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
		},
	}, {
		GroupingID: h(triangleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.TriangleTest,
		},
	}}, actualGroupings)

	actualOptions := sqltest.GetAllRows(ctx, t, db, "Options", &schema.OptionsRow{}).([]schema.OptionsRow)
	assert.ElementsMatch(t, []schema.OptionsRow{{
		OptionsID: h(pngOptions),
		Keys: map[string]string{
			"ext": "png",
		},
	}}, actualOptions)

	actualTraces := sqltest.GetAllRows(ctx, t, db, "Traces", &schema.TraceRow{}).([]schema.TraceRow)
	assert.ElementsMatch(t, []schema.TraceRow{{
		TraceID:    h(circleTraceKeys),
		Corpus:     dks.RoundCorpus,
		GroupingID: h(circleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.RoundCorpus,
			types.PrimaryKeyField: dks.CircleTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.IOS,
			dks.DeviceKey:         dks.IPhoneDevice,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}, {
		TraceID:    h(squareTraceKeys),
		Corpus:     dks.CornersCorpus,
		GroupingID: h(squareGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.IOS,
			dks.DeviceKey:         dks.IPhoneDevice,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}, {
		TraceID:    h(triangleTraceKeys),
		Corpus:     dks.CornersCorpus,
		GroupingID: h(triangleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.TriangleTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.IOS,
			dks.DeviceKey:         dks.IPhoneDevice,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}}, actualTraces)

	actualParams := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchParams", &schema.SecondaryBranchParamRow{}).([]schema.SecondaryBranchParamRow)
	assert.ElementsMatch(t, []schema.SecondaryBranchParamRow{
		{Key: dks.ColorModeKey, Value: dks.RGBColorMode, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: dks.DeviceKey, Value: dks.IPhoneDevice, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.PrimaryKeyField, Value: dks.CircleTest, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.PrimaryKeyField, Value: dks.SquareTest, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.PrimaryKeyField, Value: dks.TriangleTest, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: dks.OSKey, Value: dks.IOS, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.CorpusField, Value: dks.CornersCorpus, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.CorpusField, Value: dks.RoundCorpus, BranchName: qualifiedCL, VersionName: qualifiedPS},
	}, actualParams)

	actualValues := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchValues", &schema.SecondaryBranchValueRow{}).([]schema.SecondaryBranchValueRow)
	assert.ElementsMatch(t, []schema.SecondaryBranchValueRow{{
		BranchName: qualifiedCL, VersionName: qualifiedPS,
		TraceID:      h(squareTraceKeys),
		Digest:       d(dks.DigestA01Pos),
		GroupingID:   h(squareGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     qualifiedTJ,
	}, {
		BranchName: qualifiedCL, VersionName: qualifiedPS,
		TraceID:      h(triangleTraceKeys),
		Digest:       d(dks.DigestB01Pos),
		GroupingID:   h(triangleGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     qualifiedTJ,
	}, {
		BranchName: qualifiedCL, VersionName: qualifiedPS,
		TraceID:      h(circleTraceKeys),
		Digest:       d(dks.DigestC07Unt_CL),
		GroupingID:   h(circleGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     qualifiedTJ,
	}}, actualValues)

	// We only write to SecondaryBranchExpectations when something is explicitly triaged.
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "SecondaryBranchExpectations", &schema.SecondaryBranchExpectationRow{}))

	// Unlike the primary branch ingestion, these should be empty
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "CommitsWithData", &schema.CommitWithDataRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "TraceValues", &schema.TraceValueRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "ValuesAtHead", &schema.ValueAtHeadRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "Expectations", &schema.ExpectationRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "PrimaryBranchParams", &schema.PrimaryBranchParamRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "TiledTraceDigests", &schema.TiledTraceDigestRow{}))
}

func TestTryjobSQL_Process_UsesRubberstampCRSAndSimpleCIS_Success_cdb(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	const squareTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"square","os":"iOS","source_type":"corners"}`
	const triangleTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"triangle","os":"iOS","source_type":"corners"}`
	const circleTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"circle","os":"iOS","source_type":"round"}`

	const qualifiedCL = "gerrit_CL_fix_ios"
	const qualifiedRubberstampPS = "gerrit_CL_fix_ios||3"
	const qualifiedTJ = "buildbucket_tryjob_01_iphonergb"

	// This file has data from 3 traces across 2 corpora. The data is for the patchset with order 3.
	src := fakeGCSSourceFromFile(t, "from_goldctl_legacy_fields.json")
	gtp := initCaches(goldTryjobProcessor{
		cisClients: map[string]ci.Client{
			buildbucketCIS: simple_cis.New(dks.BuildBucketCIS),
		},
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: rubberstampCRS{},
			},
		},
		db:     db,
		source: src,
	})

	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, dks.Tryjob01FileIPhoneRGB)
	require.NoError(t, err)

	actualChangelists := sqltest.GetAllRows(ctx, t, db, "Changelists", &schema.ChangelistRow{}).([]schema.ChangelistRow)
	assert.Equal(t, []schema.ChangelistRow{{
		ChangelistID:     qualifiedCL,
		System:           dks.GerritCRS,
		Status:           schema.StatusOpen,
		OwnerEmail:       "<unknown>",
		Subject:          "<unknown>",
		LastIngestedData: fakeIngestionTime,
	}}, actualChangelists)

	actualPatchsets := sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{}).([]schema.PatchsetRow)
	assert.Equal(t, []schema.PatchsetRow{{
		PatchsetID:   qualifiedRubberstampPS,
		System:       dks.GerritCRS,
		ChangelistID: qualifiedCL,
		Order:        3,
		GitHash:      "<unknown>",
		Created:      beginningOfTime,
	}}, actualPatchsets)

	actualTryjobs := sqltest.GetAllRows(ctx, t, db, "Tryjobs", &schema.TryjobRow{}).([]schema.TryjobRow)
	assert.Equal(t, []schema.TryjobRow{{
		TryjobID:         qualifiedTJ,
		System:           dks.BuildBucketCIS,
		ChangelistID:     qualifiedCL,
		PatchsetID:       qualifiedRubberstampPS,
		DisplayName:      "tryjob_01_iphonergb",
		LastIngestedData: fakeIngestionTime,
	}}, actualTryjobs)

	actualValues := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchValues", &schema.SecondaryBranchValueRow{}).([]schema.SecondaryBranchValueRow)
	assert.ElementsMatch(t, []schema.SecondaryBranchValueRow{{
		BranchName: qualifiedCL, VersionName: qualifiedRubberstampPS,
		TraceID:      h(squareTraceKeys),
		Digest:       d(dks.DigestA01Pos),
		GroupingID:   h(squareGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     qualifiedTJ,
	}, {
		BranchName: qualifiedCL, VersionName: qualifiedRubberstampPS,
		TraceID:      h(triangleTraceKeys),
		Digest:       d(dks.DigestB01Pos),
		GroupingID:   h(triangleGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     qualifiedTJ,
	}, {
		BranchName: qualifiedCL, VersionName: qualifiedRubberstampPS,
		TraceID:      h(circleTraceKeys),
		Digest:       d(dks.DigestC07Unt_CL),
		GroupingID:   h(circleGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     qualifiedTJ,
	}}, actualValues)
}

func TestTryjobSQL_Process_UsesRubberstampCRSAndSimpleCIS_Success(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewSpannerDBForTestsWithProductionSchema(ctx, t)

	const squareTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"square","os":"iOS","source_type":"corners"}`
	const triangleTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"triangle","os":"iOS","source_type":"corners"}`
	const circleTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"circle","os":"iOS","source_type":"round"}`

	const qualifiedCL = "gerrit_CL_fix_ios"
	const qualifiedRubberstampPS = "gerrit_CL_fix_ios||3"
	const qualifiedTJ = "buildbucket_tryjob_01_iphonergb"

	// This file has data from 3 traces across 2 corpora. The data is for the patchset with order 3.
	src := fakeGCSSourceFromFile(t, "from_goldctl_legacy_fields.json")
	gtp := initCaches(goldTryjobProcessor{
		cisClients: map[string]ci.Client{
			buildbucketCIS: simple_cis.New(dks.BuildBucketCIS),
		},
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: rubberstampCRS{},
			},
		},
		db:     db,
		source: src,
	})

	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, dks.Tryjob01FileIPhoneRGB)
	require.NoError(t, err)

	actualChangelists := sqltest.GetAllRows(ctx, t, db, "Changelists", &schema.ChangelistRow{}).([]schema.ChangelistRow)
	assert.Equal(t, []schema.ChangelistRow{{
		ChangelistID:     qualifiedCL,
		System:           dks.GerritCRS,
		Status:           schema.StatusOpen,
		OwnerEmail:       "<unknown>",
		Subject:          "<unknown>",
		LastIngestedData: fakeIngestionTime,
	}}, actualChangelists)

	actualPatchsets := sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{}).([]schema.PatchsetRow)
	assert.Equal(t, []schema.PatchsetRow{{
		PatchsetID:   qualifiedRubberstampPS,
		System:       dks.GerritCRS,
		ChangelistID: qualifiedCL,
		Order:        3,
		GitHash:      "<unknown>",
		Created:      beginningOfTime,
	}}, actualPatchsets)

	actualTryjobs := sqltest.GetAllRows(ctx, t, db, "Tryjobs", &schema.TryjobRow{}).([]schema.TryjobRow)
	assert.Equal(t, []schema.TryjobRow{{
		TryjobID:         qualifiedTJ,
		System:           dks.BuildBucketCIS,
		ChangelistID:     qualifiedCL,
		PatchsetID:       qualifiedRubberstampPS,
		DisplayName:      "tryjob_01_iphonergb",
		LastIngestedData: fakeIngestionTime,
	}}, actualTryjobs)

	actualValues := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchValues", &schema.SecondaryBranchValueRow{}).([]schema.SecondaryBranchValueRow)
	assert.ElementsMatch(t, []schema.SecondaryBranchValueRow{{
		BranchName: qualifiedCL, VersionName: qualifiedRubberstampPS,
		TraceID:      h(squareTraceKeys),
		Digest:       d(dks.DigestA01Pos),
		GroupingID:   h(squareGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     qualifiedTJ,
	}, {
		BranchName: qualifiedCL, VersionName: qualifiedRubberstampPS,
		TraceID:      h(triangleTraceKeys),
		Digest:       d(dks.DigestB01Pos),
		GroupingID:   h(triangleGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     qualifiedTJ,
	}, {
		BranchName: qualifiedCL, VersionName: qualifiedRubberstampPS,
		TraceID:      h(circleTraceKeys),
		Digest:       d(dks.DigestC07Unt_CL),
		GroupingID:   h(circleGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     qualifiedTJ,
	}}, actualValues)
}

func TestTryjobSQL_Process_SomeDataExists_Success_cdb(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	const qualifiedCL = "gerrit_CL_fix_ios"
	const qualifiedPS = "gerrit_PS_fixes_ipad_but_not_iphone"
	const qualifiedTJ = "buildbucket_tryjob_01_iphonergb"
	const squareTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"square","os":"iOS","source_type":"corners"}`
	const triangleTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"triangle","os":"iOS","source_type":"corners"}`
	const circleTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"circle","os":"iOS","source_type":"round"}`

	// Load all the tables we write to with one existing row.
	existingData := schema.Tables{
		Changelists: []schema.ChangelistRow{{
			ChangelistID:     qualifiedCL,
			System:           dks.GerritCRS,
			Status:           schema.StatusOpen,
			OwnerEmail:       dks.UserOne,
			Subject:          "Fix iOS [sentinel]",
			LastIngestedData: beginningOfTime, // should be updated.
		}},
		Patchsets: []schema.PatchsetRow{{
			PatchsetID:    qualifiedPS,
			System:        dks.GerritCRS,
			ChangelistID:  qualifiedCL,
			Order:         3,
			GitHash:       "ffff111111111111111111111111111111111111",
			CommentedOnCL: true, // sentinel values
			Created:       time.Date(2021, time.March, 26, 13, 6, 3, 0, time.UTC),
		}},
		Tryjobs: []schema.TryjobRow{{
			TryjobID:         qualifiedTJ,
			System:           dks.BuildBucketCIS,
			ChangelistID:     qualifiedCL,
			PatchsetID:       qualifiedPS,
			DisplayName:      "Test-iPhone-RGB-sentinel",
			LastIngestedData: beginningOfTime, // should be updated.
		}},
		Groupings: []schema.GroupingRow{{
			GroupingID: h(squareGrouping),
			Keys: paramtools.Params{
				types.CorpusField:     dks.CornersCorpus,
				types.PrimaryKeyField: dks.SquareTest,
			},
		}},
		Options: []schema.OptionsRow{{
			OptionsID: h(pngOptions),
			Keys: map[string]string{
				"ext": "png",
			},
		}},
		Traces: []schema.TraceRow{{
			TraceID:    h(circleTraceKeys),
			Corpus:     dks.RoundCorpus,
			GroupingID: h(circleGrouping),
			Keys: map[string]string{
				types.CorpusField:     dks.RoundCorpus,
				types.PrimaryKeyField: dks.CircleTest,
				dks.ColorModeKey:      dks.RGBColorMode,
				dks.OSKey:             dks.IOS,
				dks.DeviceKey:         dks.IPhoneDevice,
			},
			MatchesAnyIgnoreRule: schema.NBFalse, // should not be overwritten
		}},
		SecondaryBranchParams: []schema.SecondaryBranchParamRow{{
			Key:         dks.ColorModeKey,
			Value:       dks.RGBColorMode,
			BranchName:  qualifiedCL,
			VersionName: qualifiedPS,
		}},
		SecondaryBranchValues: []schema.SecondaryBranchValueRow{{
			BranchName: qualifiedCL, VersionName: qualifiedPS,
			TraceID:      h(squareTraceKeys),
			Digest:       d(dks.DigestA01Pos),
			GroupingID:   h(squareGrouping),
			OptionsID:    h(pngOptions),
			SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
			TryjobID:     "Should be overwritten",
		}},
		SourceFiles: []schema.SourceFileRow{{
			SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
			SourceFile:   dks.Tryjob01FileIPhoneRGB,
			LastIngested: time.Date(2020, time.March, 1, 1, 1, 1, 0, time.UTC),
		}},
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	mcrs := &mock_crs.Client{}
	mcis := &mock_cis.Client{}

	// This file has data from 3 traces across 2 corpora. The data is for the patchset with order 3.
	src := fakeGCSSourceFromFile(t, "from_goldctl_recent_fields.json")
	gtp := initCaches(goldTryjobProcessor{
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		db:     db,
		source: src,
	})

	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, dks.Tryjob01FileIPhoneRGB)
	require.NoError(t, err)

	actualSourceFiles := sqltest.GetAllRows(ctx, t, db, "SourceFiles", &schema.SourceFileRow{}).([]schema.SourceFileRow)
	assert.Equal(t, []schema.SourceFileRow{{
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		SourceFile:   dks.Tryjob01FileIPhoneRGB,
		LastIngested: fakeIngestionTime,
	}}, actualSourceFiles)

	actualChangelists := sqltest.GetAllRows(ctx, t, db, "Changelists", &schema.ChangelistRow{}).([]schema.ChangelistRow)
	assert.Equal(t, []schema.ChangelistRow{{
		ChangelistID:     qualifiedCL,
		System:           dks.GerritCRS,
		Status:           schema.StatusOpen,
		OwnerEmail:       dks.UserOne,
		Subject:          "Fix iOS [sentinel]",
		LastIngestedData: fakeIngestionTime,
	}}, actualChangelists)

	actualPatchsets := sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{}).([]schema.PatchsetRow)
	assert.Equal(t, []schema.PatchsetRow{{
		PatchsetID:    qualifiedPS,
		System:        dks.GerritCRS,
		ChangelistID:  qualifiedCL,
		Order:         3,
		GitHash:       "ffff111111111111111111111111111111111111",
		CommentedOnCL: true,
		Created:       time.Date(2021, time.March, 26, 13, 6, 3, 0, time.UTC),
	}}, actualPatchsets)

	actualTryjobs := sqltest.GetAllRows(ctx, t, db, "Tryjobs", &schema.TryjobRow{}).([]schema.TryjobRow)
	assert.Equal(t, []schema.TryjobRow{{
		TryjobID:         qualifiedTJ,
		System:           dks.BuildBucketCIS,
		ChangelistID:     qualifiedCL,
		PatchsetID:       qualifiedPS,
		DisplayName:      "Test-iPhone-RGB-sentinel",
		LastIngestedData: fakeIngestionTime,
	}}, actualTryjobs)

	actualGroupings := sqltest.GetAllRows(ctx, t, db, "Groupings", &schema.GroupingRow{}).([]schema.GroupingRow)
	assert.ElementsMatch(t, []schema.GroupingRow{{
		GroupingID: h(circleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.RoundCorpus,
			types.PrimaryKeyField: dks.CircleTest,
		},
	}, {
		GroupingID: h(squareGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
		},
	}, {
		GroupingID: h(triangleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.TriangleTest,
		},
	}}, actualGroupings)

	actualOptions := sqltest.GetAllRows(ctx, t, db, "Options", &schema.OptionsRow{}).([]schema.OptionsRow)
	assert.ElementsMatch(t, []schema.OptionsRow{{
		OptionsID: h(pngOptions),
		Keys: map[string]string{
			"ext": "png",
		},
	}}, actualOptions)

	actualTraces := sqltest.GetAllRows(ctx, t, db, "Traces", &schema.TraceRow{}).([]schema.TraceRow)
	assert.ElementsMatch(t, []schema.TraceRow{{
		TraceID:    h(circleTraceKeys),
		Corpus:     dks.RoundCorpus,
		GroupingID: h(circleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.RoundCorpus,
			types.PrimaryKeyField: dks.CircleTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.IOS,
			dks.DeviceKey:         dks.IPhoneDevice,
		},
		MatchesAnyIgnoreRule: schema.NBFalse, // existing status not overwritten
	}, {
		TraceID:    h(squareTraceKeys),
		Corpus:     dks.CornersCorpus,
		GroupingID: h(squareGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.IOS,
			dks.DeviceKey:         dks.IPhoneDevice,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}, {
		TraceID:    h(triangleTraceKeys),
		Corpus:     dks.CornersCorpus,
		GroupingID: h(triangleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.TriangleTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.IOS,
			dks.DeviceKey:         dks.IPhoneDevice,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}}, actualTraces)

	actualParams := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchParams", &schema.SecondaryBranchParamRow{}).([]schema.SecondaryBranchParamRow)
	assert.ElementsMatch(t, []schema.SecondaryBranchParamRow{
		{Key: dks.ColorModeKey, Value: dks.RGBColorMode, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: dks.DeviceKey, Value: dks.IPhoneDevice, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.PrimaryKeyField, Value: dks.CircleTest, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.PrimaryKeyField, Value: dks.SquareTest, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.PrimaryKeyField, Value: dks.TriangleTest, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: dks.OSKey, Value: dks.IOS, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.CorpusField, Value: dks.CornersCorpus, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.CorpusField, Value: dks.RoundCorpus, BranchName: qualifiedCL, VersionName: qualifiedPS},
	}, actualParams)

	actualValues := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchValues", &schema.SecondaryBranchValueRow{}).([]schema.SecondaryBranchValueRow)
	assert.ElementsMatch(t, []schema.SecondaryBranchValueRow{{
		BranchName: qualifiedCL, VersionName: qualifiedPS,
		TraceID:      h(squareTraceKeys),
		Digest:       d(dks.DigestA01Pos),
		GroupingID:   h(squareGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     qualifiedTJ,
	}, {
		BranchName: qualifiedCL, VersionName: qualifiedPS,
		TraceID:      h(triangleTraceKeys),
		Digest:       d(dks.DigestB01Pos),
		GroupingID:   h(triangleGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     qualifiedTJ,
	}, {
		BranchName: qualifiedCL, VersionName: qualifiedPS,
		TraceID:      h(circleTraceKeys),
		Digest:       d(dks.DigestC07Unt_CL),
		GroupingID:   h(circleGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     qualifiedTJ,
	}}, actualValues)

	// We only write to SecondaryBranchExpectations when something is explicitly triaged.
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "SecondaryBranchExpectations", &schema.SecondaryBranchExpectationRow{}))

	// Unlike the primary branch ingestion, these should be empty
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "CommitsWithData", &schema.CommitWithDataRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "TraceValues", &schema.TraceValueRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "ValuesAtHead", &schema.ValueAtHeadRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "Expectations", &schema.ExpectationRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "PrimaryBranchParams", &schema.PrimaryBranchParamRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "TiledTraceDigests", &schema.TiledTraceDigestRow{}))
}

func TestTryjobSQL_Process_SomeDataExists_Success(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewSpannerDBForTestsWithProductionSchema(ctx, t)
	const qualifiedCL = "gerrit_CL_fix_ios"
	const qualifiedPS = "gerrit_PS_fixes_ipad_but_not_iphone"
	const qualifiedTJ = "buildbucket_tryjob_01_iphonergb"
	const squareTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"square","os":"iOS","source_type":"corners"}`
	const triangleTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"triangle","os":"iOS","source_type":"corners"}`
	const circleTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"circle","os":"iOS","source_type":"round"}`

	// Load all the tables we write to with one existing row.
	existingData := schema.Tables{
		Changelists: []schema.ChangelistRow{{
			ChangelistID:     qualifiedCL,
			System:           dks.GerritCRS,
			Status:           schema.StatusOpen,
			OwnerEmail:       dks.UserOne,
			Subject:          "Fix iOS [sentinel]",
			LastIngestedData: beginningOfTime, // should be updated.
		}},
		Patchsets: []schema.PatchsetRow{{
			PatchsetID:    qualifiedPS,
			System:        dks.GerritCRS,
			ChangelistID:  qualifiedCL,
			Order:         3,
			GitHash:       "ffff111111111111111111111111111111111111",
			CommentedOnCL: true, // sentinel values
			Created:       time.Date(2021, time.March, 26, 13, 6, 3, 0, time.UTC),
		}},
		Tryjobs: []schema.TryjobRow{{
			TryjobID:         qualifiedTJ,
			System:           dks.BuildBucketCIS,
			ChangelistID:     qualifiedCL,
			PatchsetID:       qualifiedPS,
			DisplayName:      "Test-iPhone-RGB-sentinel",
			LastIngestedData: beginningOfTime, // should be updated.
		}},
		Groupings: []schema.GroupingRow{{
			GroupingID: h(squareGrouping),
			Keys: paramtools.Params{
				types.CorpusField:     dks.CornersCorpus,
				types.PrimaryKeyField: dks.SquareTest,
			},
		}},
		Options: []schema.OptionsRow{{
			OptionsID: h(pngOptions),
			Keys: map[string]string{
				"ext": "png",
			},
		}},
		Traces: []schema.TraceRow{{
			TraceID:    h(circleTraceKeys),
			Corpus:     dks.RoundCorpus,
			GroupingID: h(circleGrouping),
			Keys: map[string]string{
				types.CorpusField:     dks.RoundCorpus,
				types.PrimaryKeyField: dks.CircleTest,
				dks.ColorModeKey:      dks.RGBColorMode,
				dks.OSKey:             dks.IOS,
				dks.DeviceKey:         dks.IPhoneDevice,
			},
			MatchesAnyIgnoreRule: schema.NBFalse, // should not be overwritten
		}},
		SecondaryBranchParams: []schema.SecondaryBranchParamRow{{
			Key:         dks.ColorModeKey,
			Value:       dks.RGBColorMode,
			BranchName:  qualifiedCL,
			VersionName: qualifiedPS,
		}},
		SecondaryBranchValues: []schema.SecondaryBranchValueRow{{
			BranchName: qualifiedCL, VersionName: qualifiedPS,
			TraceID:      h(squareTraceKeys),
			Digest:       d(dks.DigestA01Pos),
			GroupingID:   h(squareGrouping),
			OptionsID:    h(pngOptions),
			SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
			TryjobID:     "Should be overwritten",
		}},
		SourceFiles: []schema.SourceFileRow{{
			SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
			SourceFile:   dks.Tryjob01FileIPhoneRGB,
			LastIngested: time.Date(2020, time.March, 1, 1, 1, 1, 0, time.UTC),
		}},
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	mcrs := &mock_crs.Client{}
	mcis := &mock_cis.Client{}

	// This file has data from 3 traces across 2 corpora. The data is for the patchset with order 3.
	src := fakeGCSSourceFromFile(t, "from_goldctl_recent_fields.json")
	gtp := initCaches(goldTryjobProcessor{
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		db:     db,
		source: src,
	})

	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, dks.Tryjob01FileIPhoneRGB)
	require.NoError(t, err)

	actualSourceFiles := sqltest.GetAllRows(ctx, t, db, "SourceFiles", &schema.SourceFileRow{}).([]schema.SourceFileRow)
	assert.Equal(t, []schema.SourceFileRow{{
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		SourceFile:   dks.Tryjob01FileIPhoneRGB,
		LastIngested: fakeIngestionTime,
	}}, actualSourceFiles)

	actualChangelists := sqltest.GetAllRows(ctx, t, db, "Changelists", &schema.ChangelistRow{}).([]schema.ChangelistRow)
	assert.Equal(t, []schema.ChangelistRow{{
		ChangelistID:     qualifiedCL,
		System:           dks.GerritCRS,
		Status:           schema.StatusOpen,
		OwnerEmail:       dks.UserOne,
		Subject:          "Fix iOS [sentinel]",
		LastIngestedData: fakeIngestionTime,
	}}, actualChangelists)

	actualPatchsets := sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{}).([]schema.PatchsetRow)
	assert.Equal(t, []schema.PatchsetRow{{
		PatchsetID:    qualifiedPS,
		System:        dks.GerritCRS,
		ChangelistID:  qualifiedCL,
		Order:         3,
		GitHash:       "ffff111111111111111111111111111111111111",
		CommentedOnCL: true,
		Created:       time.Date(2021, time.March, 26, 13, 6, 3, 0, time.UTC),
	}}, actualPatchsets)

	actualTryjobs := sqltest.GetAllRows(ctx, t, db, "Tryjobs", &schema.TryjobRow{}).([]schema.TryjobRow)
	assert.Equal(t, []schema.TryjobRow{{
		TryjobID:         qualifiedTJ,
		System:           dks.BuildBucketCIS,
		ChangelistID:     qualifiedCL,
		PatchsetID:       qualifiedPS,
		DisplayName:      "Test-iPhone-RGB-sentinel",
		LastIngestedData: fakeIngestionTime,
	}}, actualTryjobs)

	actualGroupings := sqltest.GetAllRows(ctx, t, db, "Groupings", &schema.GroupingRow{}).([]schema.GroupingRow)
	assert.ElementsMatch(t, []schema.GroupingRow{{
		GroupingID: h(circleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.RoundCorpus,
			types.PrimaryKeyField: dks.CircleTest,
		},
	}, {
		GroupingID: h(squareGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
		},
	}, {
		GroupingID: h(triangleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.TriangleTest,
		},
	}}, actualGroupings)

	actualOptions := sqltest.GetAllRows(ctx, t, db, "Options", &schema.OptionsRow{}).([]schema.OptionsRow)
	assert.ElementsMatch(t, []schema.OptionsRow{{
		OptionsID: h(pngOptions),
		Keys: map[string]string{
			"ext": "png",
		},
	}}, actualOptions)

	actualTraces := sqltest.GetAllRows(ctx, t, db, "Traces", &schema.TraceRow{}).([]schema.TraceRow)
	assert.ElementsMatch(t, []schema.TraceRow{{
		TraceID:    h(circleTraceKeys),
		Corpus:     dks.RoundCorpus,
		GroupingID: h(circleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.RoundCorpus,
			types.PrimaryKeyField: dks.CircleTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.IOS,
			dks.DeviceKey:         dks.IPhoneDevice,
		},
		MatchesAnyIgnoreRule: schema.NBFalse, // existing status not overwritten
	}, {
		TraceID:    h(squareTraceKeys),
		Corpus:     dks.CornersCorpus,
		GroupingID: h(squareGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.IOS,
			dks.DeviceKey:         dks.IPhoneDevice,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}, {
		TraceID:    h(triangleTraceKeys),
		Corpus:     dks.CornersCorpus,
		GroupingID: h(triangleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.TriangleTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.IOS,
			dks.DeviceKey:         dks.IPhoneDevice,
		},
		MatchesAnyIgnoreRule: schema.NBNull,
	}}, actualTraces)

	actualParams := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchParams", &schema.SecondaryBranchParamRow{}).([]schema.SecondaryBranchParamRow)
	assert.ElementsMatch(t, []schema.SecondaryBranchParamRow{
		{Key: dks.ColorModeKey, Value: dks.RGBColorMode, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: dks.DeviceKey, Value: dks.IPhoneDevice, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.PrimaryKeyField, Value: dks.CircleTest, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.PrimaryKeyField, Value: dks.SquareTest, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.PrimaryKeyField, Value: dks.TriangleTest, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: dks.OSKey, Value: dks.IOS, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.CorpusField, Value: dks.CornersCorpus, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.CorpusField, Value: dks.RoundCorpus, BranchName: qualifiedCL, VersionName: qualifiedPS},
	}, actualParams)

	actualValues := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchValues", &schema.SecondaryBranchValueRow{}).([]schema.SecondaryBranchValueRow)
	assert.ElementsMatch(t, []schema.SecondaryBranchValueRow{{
		BranchName: qualifiedCL, VersionName: qualifiedPS,
		TraceID:      h(squareTraceKeys),
		Digest:       d(dks.DigestA01Pos),
		GroupingID:   h(squareGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     qualifiedTJ,
	}, {
		BranchName: qualifiedCL, VersionName: qualifiedPS,
		TraceID:      h(triangleTraceKeys),
		Digest:       d(dks.DigestB01Pos),
		GroupingID:   h(triangleGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     qualifiedTJ,
	}, {
		BranchName: qualifiedCL, VersionName: qualifiedPS,
		TraceID:      h(circleTraceKeys),
		Digest:       d(dks.DigestC07Unt_CL),
		GroupingID:   h(circleGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     qualifiedTJ,
	}}, actualValues)

	// We only write to SecondaryBranchExpectations when something is explicitly triaged.
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "SecondaryBranchExpectations", &schema.SecondaryBranchExpectationRow{}))

	// Unlike the primary branch ingestion, these should be empty
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "CommitsWithData", &schema.CommitWithDataRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "TraceValues", &schema.TraceValueRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "ValuesAtHead", &schema.ValueAtHeadRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "Expectations", &schema.ExpectationRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "PrimaryBranchParams", &schema.PrimaryBranchParamRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "TiledTraceDigests", &schema.TiledTraceDigestRow{}))
}

func TestTryjobSQL_Process_TryjobRerunAtSameCLPS_MultipleDatapointsForTraceAtSamePatchset_cdb(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	const qualifiedCL = "gerrit_" + dks.ChangelistIDWithMultipleDatapointsPerTrace
	const qualifiedPS = "gerrit_" + dks.PatchsetIDWithMultipleDatapointsPerTrace
	const qualifiedTJ = "buildbucket_" + dks.Tryjob09Windows
	const qualifiedTJRerun = "buildbucket_" + dks.Tryjob10Windows
	const squareTraceKeys = `{"color mode":"RGB","device":"QuadroP400","name":"square","os":"Windows10.3","source_type":"corners"}`

	// Load all the tables we write to with one existing row.
	existingData := schema.Tables{
		Changelists: []schema.ChangelistRow{{
			ChangelistID:     qualifiedCL,
			System:           dks.GerritCRS,
			Status:           schema.StatusOpen,
			OwnerEmail:       dks.UserOne,
			Subject:          "multiple datapoints",
			LastIngestedData: time.Date(2020, time.December, 12, 10, 1, 0, 0, time.UTC), // Should be updated.
		}},
		Patchsets: []schema.PatchsetRow{{
			PatchsetID:    qualifiedPS,
			System:        dks.GerritCRS,
			ChangelistID:  qualifiedCL,
			Order:         1,
			GitHash:       "ccccccccccccccccccccccccccccccccccc66666",
			CommentedOnCL: true,
			Created:       time.Date(2020, time.December, 11, 0, 0, 0, 0, time.UTC),
		}},
		Tryjobs: []schema.TryjobRow{{
			TryjobID:         qualifiedTJ,
			System:           dks.BuildBucketCIS,
			ChangelistID:     qualifiedCL,
			PatchsetID:       qualifiedPS,
			DisplayName:      "Test-Windows10.3-Some",
			LastIngestedData: time.Date(2020, time.December, 12, 10, 0, 0, 0, time.UTC),
		}},
		Groupings: []schema.GroupingRow{{
			GroupingID: h(squareGrouping),
			Keys: paramtools.Params{
				types.CorpusField:     dks.CornersCorpus,
				types.PrimaryKeyField: dks.SquareTest,
			},
		}},
		Options: []schema.OptionsRow{{
			OptionsID: h(pngOptions),
			Keys: map[string]string{
				"ext": "png",
			},
		}},
		Traces: []schema.TraceRow{{
			TraceID:    h(squareTraceKeys),
			Corpus:     dks.CornersCorpus,
			GroupingID: h(squareGrouping),
			Keys: map[string]string{
				types.CorpusField:     dks.CornersCorpus,
				types.PrimaryKeyField: dks.SquareTest,
				dks.ColorModeKey:      dks.RGBColorMode,
				dks.OSKey:             dks.Windows10dot3OS,
				dks.DeviceKey:         dks.QuadroDevice,
			},
			MatchesAnyIgnoreRule: schema.NBFalse,
		}},
		SecondaryBranchParams: []schema.SecondaryBranchParamRow{
			{Key: dks.ColorModeKey, Value: dks.RGBColorMode, BranchName: qualifiedCL, VersionName: qualifiedPS},
			{Key: dks.DeviceKey, Value: dks.QuadroDevice, BranchName: qualifiedCL, VersionName: qualifiedPS},
			{Key: types.PrimaryKeyField, Value: dks.SquareTest, BranchName: qualifiedCL, VersionName: qualifiedPS},
			{Key: dks.OSKey, Value: dks.Windows10dot3OS, BranchName: qualifiedCL, VersionName: qualifiedPS},
			{Key: types.CorpusField, Value: dks.CornersCorpus, BranchName: qualifiedCL, VersionName: qualifiedPS},
		},
		SecondaryBranchValues: []schema.SecondaryBranchValueRow{{
			BranchName: qualifiedCL, VersionName: qualifiedPS,
			TraceID:      h(squareTraceKeys),
			Digest:       d(dks.DigestC03Unt),
			GroupingID:   h(squareGrouping),
			OptionsID:    h(pngOptions),
			SourceFileID: h(dks.Tryjob09FileWindows),
			TryjobID:     qualifiedTJ,
		}},
		SourceFiles: []schema.SourceFileRow{{
			SourceFileID: h(dks.Tryjob09FileWindows),
			SourceFile:   dks.Tryjob09FileWindows,
			LastIngested: time.Date(2020, time.December, 12, 10, 1, 0, 0, time.UTC),
		}},
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	mcrs := &mock_crs.Client{}
	mcis := &mock_cis.Client{}
	mcis.On("GetTryJob", testutils.AnyContext, dks.Tryjob10Windows).Return(ci.TryJob{
		SystemID:    dks.Tryjob10Windows,
		System:      dks.BuildBucketCIS,
		DisplayName: "Test-Windows10.3-Some",
	}, nil)

	// At this point the database has one CL with a single patchset. A tryjob ran and produced a
	// single datapoint.
	//
	// The following file is the result of re-running the same tryjob at the same patchset, which
	// produces a new datapoint. The digest is different because the test is flaky.
	src := fakeGCSSourceFromFile(t, "tryjob_rerun.json")
	gtp := initCaches(goldTryjobProcessor{
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		db:     db,
		source: src,
	})

	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, dks.Tryjob10FileWindows)
	require.NoError(t, err)

	actualSourceFiles := sqltest.GetAllRows(ctx, t, db, "SourceFiles", &schema.SourceFileRow{}).([]schema.SourceFileRow)
	assert.ElementsMatch(t, []schema.SourceFileRow{{
		SourceFileID: h(dks.Tryjob09FileWindows),
		SourceFile:   dks.Tryjob09FileWindows,
		LastIngested: time.Date(2020, time.December, 12, 10, 1, 0, 0, time.UTC),
	}, {
		// New file.
		SourceFileID: h(dks.Tryjob10FileWindows),
		SourceFile:   dks.Tryjob10FileWindows,
		LastIngested: fakeIngestionTime,
	}}, actualSourceFiles)

	actualChangelists := sqltest.GetAllRows(ctx, t, db, "Changelists", &schema.ChangelistRow{}).([]schema.ChangelistRow)
	assert.Equal(t, []schema.ChangelistRow{{
		ChangelistID:     qualifiedCL,
		System:           dks.GerritCRS,
		Status:           schema.StatusOpen,
		OwnerEmail:       dks.UserOne,
		Subject:          "multiple datapoints",
		LastIngestedData: fakeIngestionTime, // Updated.
	}}, actualChangelists)

	actualPatchsets := sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{}).([]schema.PatchsetRow)
	assert.Equal(t, []schema.PatchsetRow{{
		PatchsetID:    qualifiedPS,
		System:        dks.GerritCRS,
		ChangelistID:  qualifiedCL,
		Order:         1,
		GitHash:       "ccccccccccccccccccccccccccccccccccc66666",
		CommentedOnCL: true,
		Created:       time.Date(2020, time.December, 11, 0, 0, 0, 0, time.UTC),
	}}, actualPatchsets)

	actualTryjobs := sqltest.GetAllRows(ctx, t, db, "Tryjobs", &schema.TryjobRow{}).([]schema.TryjobRow)
	assert.ElementsMatch(t, []schema.TryjobRow{{
		TryjobID:         qualifiedTJ,
		System:           dks.BuildBucketCIS,
		ChangelistID:     qualifiedCL,
		PatchsetID:       qualifiedPS,
		DisplayName:      "Test-Windows10.3-Some",
		LastIngestedData: time.Date(2020, time.December, 12, 10, 0, 0, 0, time.UTC),
	}, {
		// New tryjob.
		TryjobID:         qualifiedTJRerun,
		System:           dks.BuildBucketCIS,
		ChangelistID:     qualifiedCL,
		PatchsetID:       qualifiedPS,
		DisplayName:      "Test-Windows10.3-Some",
		LastIngestedData: fakeIngestionTime,
	}}, actualTryjobs)

	actualGroupings := sqltest.GetAllRows(ctx, t, db, "Groupings", &schema.GroupingRow{}).([]schema.GroupingRow)
	assert.ElementsMatch(t, []schema.GroupingRow{{
		GroupingID: h(squareGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
		},
	}}, actualGroupings)

	actualOptions := sqltest.GetAllRows(ctx, t, db, "Options", &schema.OptionsRow{}).([]schema.OptionsRow)
	assert.ElementsMatch(t, []schema.OptionsRow{{
		OptionsID: h(pngOptions),
		Keys: map[string]string{
			"ext": "png",
		},
	}}, actualOptions)

	actualTraces := sqltest.GetAllRows(ctx, t, db, "Traces", &schema.TraceRow{}).([]schema.TraceRow)
	assert.Equal(t, []schema.TraceRow{{
		TraceID:    h(squareTraceKeys),
		Corpus:     dks.CornersCorpus,
		GroupingID: h(squareGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.Windows10dot3OS,
			dks.DeviceKey:         dks.QuadroDevice,
		},
		MatchesAnyIgnoreRule: schema.NBFalse,
	}}, actualTraces)

	actualParams := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchParams", &schema.SecondaryBranchParamRow{}).([]schema.SecondaryBranchParamRow)
	assert.ElementsMatch(t, []schema.SecondaryBranchParamRow{
		{Key: dks.ColorModeKey, Value: dks.RGBColorMode, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: dks.DeviceKey, Value: dks.QuadroDevice, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.PrimaryKeyField, Value: dks.SquareTest, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: dks.OSKey, Value: dks.Windows10dot3OS, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.CorpusField, Value: dks.CornersCorpus, BranchName: qualifiedCL, VersionName: qualifiedPS},
	}, actualParams)

	actualValues := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchValues", &schema.SecondaryBranchValueRow{}).([]schema.SecondaryBranchValueRow)
	assert.ElementsMatch(t, []schema.SecondaryBranchValueRow{{
		BranchName:   qualifiedCL,
		VersionName:  qualifiedPS,
		TraceID:      h(squareTraceKeys),
		Digest:       d(dks.DigestC03Unt),
		GroupingID:   h(squareGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob09FileWindows),
		TryjobID:     qualifiedTJ,
	}, {
		// New datapoint.
		BranchName:   qualifiedCL,
		VersionName:  qualifiedPS,
		TraceID:      h(squareTraceKeys),
		Digest:       d(dks.DigestC04Unt),
		GroupingID:   h(squareGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob10FileWindows),
		TryjobID:     qualifiedTJRerun,
	}}, actualValues)

	// We only write to SecondaryBranchExpectations when something is explicitly triaged.
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "SecondaryBranchExpectations", &schema.SecondaryBranchExpectationRow{}))

	// Unlike the primary branch ingestion, these should be empty
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "CommitsWithData", &schema.CommitWithDataRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "TraceValues", &schema.TraceValueRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "ValuesAtHead", &schema.ValueAtHeadRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "Expectations", &schema.ExpectationRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "PrimaryBranchParams", &schema.PrimaryBranchParamRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "TiledTraceDigests", &schema.TiledTraceDigestRow{}))
}

func TestTryjobSQL_Process_TryjobRerunAtSameCLPS_MultipleDatapointsForTraceAtSamePatchset(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewSpannerDBForTestsWithProductionSchema(ctx, t)
	const qualifiedCL = "gerrit_" + dks.ChangelistIDWithMultipleDatapointsPerTrace
	const qualifiedPS = "gerrit_" + dks.PatchsetIDWithMultipleDatapointsPerTrace
	const qualifiedTJ = "buildbucket_" + dks.Tryjob09Windows
	const qualifiedTJRerun = "buildbucket_" + dks.Tryjob10Windows
	const squareTraceKeys = `{"color mode":"RGB","device":"QuadroP400","name":"square","os":"Windows10.3","source_type":"corners"}`

	// Load all the tables we write to with one existing row.
	existingData := schema.Tables{
		Changelists: []schema.ChangelistRow{{
			ChangelistID:     qualifiedCL,
			System:           dks.GerritCRS,
			Status:           schema.StatusOpen,
			OwnerEmail:       dks.UserOne,
			Subject:          "multiple datapoints",
			LastIngestedData: time.Date(2020, time.December, 12, 10, 1, 0, 0, time.UTC), // Should be updated.
		}},
		Patchsets: []schema.PatchsetRow{{
			PatchsetID:    qualifiedPS,
			System:        dks.GerritCRS,
			ChangelistID:  qualifiedCL,
			Order:         1,
			GitHash:       "ccccccccccccccccccccccccccccccccccc66666",
			CommentedOnCL: true,
			Created:       time.Date(2020, time.December, 11, 0, 0, 0, 0, time.UTC),
		}},
		Tryjobs: []schema.TryjobRow{{
			TryjobID:         qualifiedTJ,
			System:           dks.BuildBucketCIS,
			ChangelistID:     qualifiedCL,
			PatchsetID:       qualifiedPS,
			DisplayName:      "Test-Windows10.3-Some",
			LastIngestedData: time.Date(2020, time.December, 12, 10, 0, 0, 0, time.UTC),
		}},
		Groupings: []schema.GroupingRow{{
			GroupingID: h(squareGrouping),
			Keys: paramtools.Params{
				types.CorpusField:     dks.CornersCorpus,
				types.PrimaryKeyField: dks.SquareTest,
			},
		}},
		Options: []schema.OptionsRow{{
			OptionsID: h(pngOptions),
			Keys: map[string]string{
				"ext": "png",
			},
		}},
		Traces: []schema.TraceRow{{
			TraceID:    h(squareTraceKeys),
			Corpus:     dks.CornersCorpus,
			GroupingID: h(squareGrouping),
			Keys: map[string]string{
				types.CorpusField:     dks.CornersCorpus,
				types.PrimaryKeyField: dks.SquareTest,
				dks.ColorModeKey:      dks.RGBColorMode,
				dks.OSKey:             dks.Windows10dot3OS,
				dks.DeviceKey:         dks.QuadroDevice,
			},
			MatchesAnyIgnoreRule: schema.NBFalse,
		}},
		SecondaryBranchParams: []schema.SecondaryBranchParamRow{
			{Key: dks.ColorModeKey, Value: dks.RGBColorMode, BranchName: qualifiedCL, VersionName: qualifiedPS},
			{Key: dks.DeviceKey, Value: dks.QuadroDevice, BranchName: qualifiedCL, VersionName: qualifiedPS},
			{Key: types.PrimaryKeyField, Value: dks.SquareTest, BranchName: qualifiedCL, VersionName: qualifiedPS},
			{Key: dks.OSKey, Value: dks.Windows10dot3OS, BranchName: qualifiedCL, VersionName: qualifiedPS},
			{Key: types.CorpusField, Value: dks.CornersCorpus, BranchName: qualifiedCL, VersionName: qualifiedPS},
		},
		SecondaryBranchValues: []schema.SecondaryBranchValueRow{{
			BranchName: qualifiedCL, VersionName: qualifiedPS,
			TraceID:      h(squareTraceKeys),
			Digest:       d(dks.DigestC03Unt),
			GroupingID:   h(squareGrouping),
			OptionsID:    h(pngOptions),
			SourceFileID: h(dks.Tryjob09FileWindows),
			TryjobID:     qualifiedTJ,
		}},
		SourceFiles: []schema.SourceFileRow{{
			SourceFileID: h(dks.Tryjob09FileWindows),
			SourceFile:   dks.Tryjob09FileWindows,
			LastIngested: time.Date(2020, time.December, 12, 10, 1, 0, 0, time.UTC),
		}},
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	mcrs := &mock_crs.Client{}
	mcis := &mock_cis.Client{}
	mcis.On("GetTryJob", testutils.AnyContext, dks.Tryjob10Windows).Return(ci.TryJob{
		SystemID:    dks.Tryjob10Windows,
		System:      dks.BuildBucketCIS,
		DisplayName: "Test-Windows10.3-Some",
	}, nil)

	// At this point the database has one CL with a single patchset. A tryjob ran and produced a
	// single datapoint.
	//
	// The following file is the result of re-running the same tryjob at the same patchset, which
	// produces a new datapoint. The digest is different because the test is flaky.
	src := fakeGCSSourceFromFile(t, "tryjob_rerun.json")
	gtp := initCaches(goldTryjobProcessor{
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		db:     db,
		source: src,
	})

	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, dks.Tryjob10FileWindows)
	require.NoError(t, err)

	actualSourceFiles := sqltest.GetAllRows(ctx, t, db, "SourceFiles", &schema.SourceFileRow{}).([]schema.SourceFileRow)
	assert.ElementsMatch(t, []schema.SourceFileRow{{
		SourceFileID: h(dks.Tryjob09FileWindows),
		SourceFile:   dks.Tryjob09FileWindows,
		LastIngested: time.Date(2020, time.December, 12, 10, 1, 0, 0, time.UTC),
	}, {
		// New file.
		SourceFileID: h(dks.Tryjob10FileWindows),
		SourceFile:   dks.Tryjob10FileWindows,
		LastIngested: fakeIngestionTime,
	}}, actualSourceFiles)

	actualChangelists := sqltest.GetAllRows(ctx, t, db, "Changelists", &schema.ChangelistRow{}).([]schema.ChangelistRow)
	assert.Equal(t, []schema.ChangelistRow{{
		ChangelistID:     qualifiedCL,
		System:           dks.GerritCRS,
		Status:           schema.StatusOpen,
		OwnerEmail:       dks.UserOne,
		Subject:          "multiple datapoints",
		LastIngestedData: fakeIngestionTime, // Updated.
	}}, actualChangelists)

	actualPatchsets := sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{}).([]schema.PatchsetRow)
	assert.Equal(t, []schema.PatchsetRow{{
		PatchsetID:    qualifiedPS,
		System:        dks.GerritCRS,
		ChangelistID:  qualifiedCL,
		Order:         1,
		GitHash:       "ccccccccccccccccccccccccccccccccccc66666",
		CommentedOnCL: true,
		Created:       time.Date(2020, time.December, 11, 0, 0, 0, 0, time.UTC),
	}}, actualPatchsets)

	actualTryjobs := sqltest.GetAllRows(ctx, t, db, "Tryjobs", &schema.TryjobRow{}).([]schema.TryjobRow)
	assert.ElementsMatch(t, []schema.TryjobRow{{
		TryjobID:         qualifiedTJ,
		System:           dks.BuildBucketCIS,
		ChangelistID:     qualifiedCL,
		PatchsetID:       qualifiedPS,
		DisplayName:      "Test-Windows10.3-Some",
		LastIngestedData: time.Date(2020, time.December, 12, 10, 0, 0, 0, time.UTC),
	}, {
		// New tryjob.
		TryjobID:         qualifiedTJRerun,
		System:           dks.BuildBucketCIS,
		ChangelistID:     qualifiedCL,
		PatchsetID:       qualifiedPS,
		DisplayName:      "Test-Windows10.3-Some",
		LastIngestedData: fakeIngestionTime,
	}}, actualTryjobs)

	actualGroupings := sqltest.GetAllRows(ctx, t, db, "Groupings", &schema.GroupingRow{}).([]schema.GroupingRow)
	assert.ElementsMatch(t, []schema.GroupingRow{{
		GroupingID: h(squareGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
		},
	}}, actualGroupings)

	actualOptions := sqltest.GetAllRows(ctx, t, db, "Options", &schema.OptionsRow{}).([]schema.OptionsRow)
	assert.ElementsMatch(t, []schema.OptionsRow{{
		OptionsID: h(pngOptions),
		Keys: map[string]string{
			"ext": "png",
		},
	}}, actualOptions)

	actualTraces := sqltest.GetAllRows(ctx, t, db, "Traces", &schema.TraceRow{}).([]schema.TraceRow)
	assert.Equal(t, []schema.TraceRow{{
		TraceID:    h(squareTraceKeys),
		Corpus:     dks.CornersCorpus,
		GroupingID: h(squareGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
			dks.ColorModeKey:      dks.RGBColorMode,
			dks.OSKey:             dks.Windows10dot3OS,
			dks.DeviceKey:         dks.QuadroDevice,
		},
		MatchesAnyIgnoreRule: schema.NBFalse,
	}}, actualTraces)

	actualParams := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchParams", &schema.SecondaryBranchParamRow{}).([]schema.SecondaryBranchParamRow)
	assert.ElementsMatch(t, []schema.SecondaryBranchParamRow{
		{Key: dks.ColorModeKey, Value: dks.RGBColorMode, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: dks.DeviceKey, Value: dks.QuadroDevice, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.PrimaryKeyField, Value: dks.SquareTest, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: dks.OSKey, Value: dks.Windows10dot3OS, BranchName: qualifiedCL, VersionName: qualifiedPS},
		{Key: types.CorpusField, Value: dks.CornersCorpus, BranchName: qualifiedCL, VersionName: qualifiedPS},
	}, actualParams)

	actualValues := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchValues", &schema.SecondaryBranchValueRow{}).([]schema.SecondaryBranchValueRow)
	assert.ElementsMatch(t, []schema.SecondaryBranchValueRow{{
		BranchName:   qualifiedCL,
		VersionName:  qualifiedPS,
		TraceID:      h(squareTraceKeys),
		Digest:       d(dks.DigestC03Unt),
		GroupingID:   h(squareGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob09FileWindows),
		TryjobID:     qualifiedTJ,
	}, {
		// New datapoint.
		BranchName:   qualifiedCL,
		VersionName:  qualifiedPS,
		TraceID:      h(squareTraceKeys),
		Digest:       d(dks.DigestC04Unt),
		GroupingID:   h(squareGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob10FileWindows),
		TryjobID:     qualifiedTJRerun,
	}}, actualValues)

	// We only write to SecondaryBranchExpectations when something is explicitly triaged.
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "SecondaryBranchExpectations", &schema.SecondaryBranchExpectationRow{}))

	// Unlike the primary branch ingestion, these should be empty
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "CommitsWithData", &schema.CommitWithDataRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "TraceValues", &schema.TraceValueRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "ValuesAtHead", &schema.ValueAtHeadRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "Expectations", &schema.ExpectationRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "PrimaryBranchParams", &schema.PrimaryBranchParamRow{}))
	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "TiledTraceDigests", &schema.TiledTraceDigestRow{}))
}

func TestTryjobSQL_Process_PatchsetExistsAndSuppliedByOrder_Success_cdb(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	const qualifiedCL = "gerrit_CL_fix_ios"
	const qualifiedPS = "gerrit_PS_fixes_ipad_but_not_iphone"
	const qualifiedTJ = "buildbucket_tryjob_01_iphonergb"

	// Load all the tables we write to with one existing row.
	existingData := schema.Tables{
		Changelists: []schema.ChangelistRow{{
			ChangelistID:     qualifiedCL,
			System:           dks.GerritCRS,
			Status:           schema.StatusOpen,
			OwnerEmail:       dks.UserOne,
			Subject:          "Fix iOS [sentinel]",
			LastIngestedData: beginningOfTime, // should be updated.
		}},
		Patchsets: []schema.PatchsetRow{{
			PatchsetID:    qualifiedPS,
			System:        dks.GerritCRS,
			ChangelistID:  qualifiedCL,
			Order:         3,
			GitHash:       "ffff111111111111111111111111111111111111",
			CommentedOnCL: true, // sentinel values
			Created:       time.Date(2021, time.March, 26, 13, 6, 3, 0, time.UTC),
		}},
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	mcrs := &mock_crs.Client{}
	mcis := &mock_cis.Client{}

	mcis.On("GetTryJob", testutils.AnyContext, dks.Tryjob01IPhoneRGB).Return(ci.TryJob{
		SystemID:    dks.Tryjob01IPhoneRGB,
		System:      dks.BuildBucketCIS,
		DisplayName: "Test-iPhone-RGB",
	}, nil)

	// This file has data from 3 traces across 2 corpora. The data is for the patchset with order 3.
	src := fakeGCSSourceFromFile(t, "from_goldctl_patchset_order_provided.json")
	gtp := initCaches(goldTryjobProcessor{
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		db:     db,
		source: src,
	})

	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, dks.Tryjob01FileIPhoneRGB)
	require.NoError(t, err)

	actualSourceFiles := sqltest.GetAllRows(ctx, t, db, "SourceFiles", &schema.SourceFileRow{}).([]schema.SourceFileRow)
	assert.Equal(t, []schema.SourceFileRow{{
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		SourceFile:   dks.Tryjob01FileIPhoneRGB,
		LastIngested: fakeIngestionTime,
	}}, actualSourceFiles)

	actualChangelists := sqltest.GetAllRows(ctx, t, db, "Changelists", &schema.ChangelistRow{}).([]schema.ChangelistRow)
	assert.Equal(t, []schema.ChangelistRow{{
		ChangelistID:     qualifiedCL,
		System:           dks.GerritCRS,
		Status:           schema.StatusOpen,
		OwnerEmail:       dks.UserOne,
		Subject:          "Fix iOS [sentinel]",
		LastIngestedData: fakeIngestionTime,
	}}, actualChangelists)

	actualPatchsets := sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{}).([]schema.PatchsetRow)
	assert.Equal(t, []schema.PatchsetRow{{
		PatchsetID:    qualifiedPS,
		System:        dks.GerritCRS,
		ChangelistID:  qualifiedCL,
		Order:         3,
		GitHash:       "ffff111111111111111111111111111111111111",
		CommentedOnCL: true,
		Created:       time.Date(2021, time.March, 26, 13, 6, 3, 0, time.UTC),
	}}, actualPatchsets)

	actualTryjobs := sqltest.GetAllRows(ctx, t, db, "Tryjobs", &schema.TryjobRow{}).([]schema.TryjobRow)
	assert.Equal(t, []schema.TryjobRow{{
		TryjobID:         qualifiedTJ,
		System:           dks.BuildBucketCIS,
		ChangelistID:     qualifiedCL,
		PatchsetID:       qualifiedPS,
		DisplayName:      "Test-iPhone-RGB",
		LastIngestedData: fakeIngestionTime,
	}}, actualTryjobs)
}

func TestTryjobSQL_Process_PatchsetExistsAndSuppliedByOrder_Success(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewSpannerDBForTestsWithProductionSchema(ctx, t)
	const qualifiedCL = "gerrit_CL_fix_ios"
	const qualifiedPS = "gerrit_PS_fixes_ipad_but_not_iphone"
	const qualifiedTJ = "buildbucket_tryjob_01_iphonergb"

	// Load all the tables we write to with one existing row.
	existingData := schema.Tables{
		Changelists: []schema.ChangelistRow{{
			ChangelistID:     qualifiedCL,
			System:           dks.GerritCRS,
			Status:           schema.StatusOpen,
			OwnerEmail:       dks.UserOne,
			Subject:          "Fix iOS [sentinel]",
			LastIngestedData: beginningOfTime, // should be updated.
		}},
		Patchsets: []schema.PatchsetRow{{
			PatchsetID:    qualifiedPS,
			System:        dks.GerritCRS,
			ChangelistID:  qualifiedCL,
			Order:         3,
			GitHash:       "ffff111111111111111111111111111111111111",
			CommentedOnCL: true, // sentinel values
			Created:       time.Date(2021, time.March, 26, 13, 6, 3, 0, time.UTC),
		}},
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	mcrs := &mock_crs.Client{}
	mcis := &mock_cis.Client{}

	mcis.On("GetTryJob", testutils.AnyContext, dks.Tryjob01IPhoneRGB).Return(ci.TryJob{
		SystemID:    dks.Tryjob01IPhoneRGB,
		System:      dks.BuildBucketCIS,
		DisplayName: "Test-iPhone-RGB",
	}, nil)

	// This file has data from 3 traces across 2 corpora. The data is for the patchset with order 3.
	src := fakeGCSSourceFromFile(t, "from_goldctl_patchset_order_provided.json")
	gtp := initCaches(goldTryjobProcessor{
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		db:     db,
		source: src,
	})

	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, dks.Tryjob01FileIPhoneRGB)
	require.NoError(t, err)

	actualSourceFiles := sqltest.GetAllRows(ctx, t, db, "SourceFiles", &schema.SourceFileRow{}).([]schema.SourceFileRow)
	assert.Equal(t, []schema.SourceFileRow{{
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		SourceFile:   dks.Tryjob01FileIPhoneRGB,
		LastIngested: fakeIngestionTime,
	}}, actualSourceFiles)

	actualChangelists := sqltest.GetAllRows(ctx, t, db, "Changelists", &schema.ChangelistRow{}).([]schema.ChangelistRow)
	assert.Equal(t, []schema.ChangelistRow{{
		ChangelistID:     qualifiedCL,
		System:           dks.GerritCRS,
		Status:           schema.StatusOpen,
		OwnerEmail:       dks.UserOne,
		Subject:          "Fix iOS [sentinel]",
		LastIngestedData: fakeIngestionTime,
	}}, actualChangelists)

	actualPatchsets := sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{}).([]schema.PatchsetRow)
	assert.Equal(t, []schema.PatchsetRow{{
		PatchsetID:    qualifiedPS,
		System:        dks.GerritCRS,
		ChangelistID:  qualifiedCL,
		Order:         3,
		GitHash:       "ffff111111111111111111111111111111111111",
		CommentedOnCL: true,
		Created:       time.Date(2021, time.March, 26, 13, 6, 3, 0, time.UTC),
	}}, actualPatchsets)

	actualTryjobs := sqltest.GetAllRows(ctx, t, db, "Tryjobs", &schema.TryjobRow{}).([]schema.TryjobRow)
	assert.Equal(t, []schema.TryjobRow{{
		TryjobID:         qualifiedTJ,
		System:           dks.BuildBucketCIS,
		ChangelistID:     qualifiedCL,
		PatchsetID:       qualifiedPS,
		DisplayName:      "Test-iPhone-RGB",
		LastIngestedData: fakeIngestionTime,
	}}, actualTryjobs)
}

func TestTryjobSQL_Process_UnknownCL_ReturnsNonretriableError_cdb(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	mcrs := &mock_crs.Client{}
	mcrs.On("GetChangelist", testutils.AnyContext, dks.ChangelistIDThatAttemptsToFixIOS).Return(code_review.Changelist{}, code_review.ErrNotFound)
	mcis := &mock_cis.Client{}

	src := fakeGCSSourceFromFile(t, "from_goldctl_recent_fields.json")
	gtp := initCaches(goldTryjobProcessor{
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		db:     db,
		source: src,
	})

	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, dks.Tryjob01FileIPhoneRGB)
	require.Error(t, err)
	assert.NotEqual(t, ingestion.ErrRetryable, skerr.Unwrap(err))
	assert.Contains(t, err.Error(), "not found")

	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "Changelists", &schema.ChangelistRow{}))
}

func TestTryjobSQL_Process_UnknownCL_ReturnsNonretriableError(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewSpannerDBForTestsWithProductionSchema(ctx, t)

	mcrs := &mock_crs.Client{}
	mcrs.On("GetChangelist", testutils.AnyContext, dks.ChangelistIDThatAttemptsToFixIOS).Return(code_review.Changelist{}, code_review.ErrNotFound)
	mcis := &mock_cis.Client{}

	src := fakeGCSSourceFromFile(t, "from_goldctl_recent_fields.json")
	gtp := initCaches(goldTryjobProcessor{
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		db:     db,
		source: src,
	})

	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, dks.Tryjob01FileIPhoneRGB)
	require.Error(t, err)
	assert.NotEqual(t, ingestion.ErrRetryable, skerr.Unwrap(err))
	assert.Contains(t, err.Error(), "not found")

	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "Changelists", &schema.ChangelistRow{}))
}

func TestTryjobSQL_Process_UnknownPS_ReturnsNonretriableError_cdb(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	const clID = dks.ChangelistIDThatAttemptsToFixIOS
	const psID = dks.PatchSetIDFixesIPadButNotIPhone

	mcrs := &mock_crs.Client{}
	mcrs.On("GetChangelist", testutils.AnyContext, clID).Return(code_review.Changelist{
		SystemID: clID,
		Owner:    dks.UserOne,
		Status:   code_review.Open,
		Subject:  "Fix iOS",
		Updated:  time.Date(2020, time.December, 5, 15, 0, 0, 0, time.UTC),
	}, nil)
	mcrs.On("GetPatchset", testutils.AnyContext, clID, psID, 0).Return(code_review.Patchset{}, code_review.ErrNotFound)
	mcis := &mock_cis.Client{}

	src := fakeGCSSourceFromFile(t, "from_goldctl_recent_fields.json")
	gtp := initCaches(goldTryjobProcessor{
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		db:     db,
		source: src,
	})

	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, dks.Tryjob01FileIPhoneRGB)
	require.Error(t, err)
	assert.NotEqual(t, ingestion.ErrRetryable, skerr.Unwrap(err))
	assert.Contains(t, err.Error(), "not found")

	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{}))
}

func TestTryjobSQL_Process_UnknownPS_ReturnsNonretriableError(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewSpannerDBForTestsWithProductionSchema(ctx, t)

	const clID = dks.ChangelistIDThatAttemptsToFixIOS
	const psID = dks.PatchSetIDFixesIPadButNotIPhone

	mcrs := &mock_crs.Client{}
	mcrs.On("GetChangelist", testutils.AnyContext, clID).Return(code_review.Changelist{
		SystemID: clID,
		Owner:    dks.UserOne,
		Status:   code_review.Open,
		Subject:  "Fix iOS",
		Updated:  time.Date(2020, time.December, 5, 15, 0, 0, 0, time.UTC),
	}, nil)
	mcrs.On("GetPatchset", testutils.AnyContext, clID, psID, 0).Return(code_review.Patchset{}, code_review.ErrNotFound)
	mcis := &mock_cis.Client{}

	src := fakeGCSSourceFromFile(t, "from_goldctl_recent_fields.json")
	gtp := initCaches(goldTryjobProcessor{
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		db:     db,
		source: src,
	})

	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, dks.Tryjob01FileIPhoneRGB)
	require.Error(t, err)
	assert.NotEqual(t, ingestion.ErrRetryable, skerr.Unwrap(err))
	assert.Contains(t, err.Error(), "not found")

	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{}))
}

func TestTryjobSQL_Process_UnknownTryjob_ReturnsNonretriableError_cdb(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	const clID = dks.ChangelistIDThatAttemptsToFixIOS
	const psID = dks.PatchSetIDFixesIPadButNotIPhone
	const tjID = dks.Tryjob01IPhoneRGB

	mcrs := &mock_crs.Client{}
	mcrs.On("GetChangelist", testutils.AnyContext, clID).Return(code_review.Changelist{
		SystemID: clID,
		Owner:    dks.UserOne,
		Status:   code_review.Open,
		Subject:  "Fix iOS",
		Updated:  time.Date(2020, time.December, 5, 15, 0, 0, 0, time.UTC),
	}, nil)
	mcrs.On("GetPatchset", testutils.AnyContext, clID, psID, 0).Return(code_review.Patchset{
		SystemID:     psID,
		ChangelistID: clID,
		Order:        3,
		GitHash:      "ffff111111111111111111111111111111111111",
	}, nil)

	mcis := &mock_cis.Client{}
	mcis.On("GetTryJob", testutils.AnyContext, tjID).Return(ci.TryJob{}, ci.ErrNotFound)

	src := fakeGCSSourceFromFile(t, "from_goldctl_recent_fields.json")
	gtp := initCaches(goldTryjobProcessor{
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		db:     db,
		source: src,
	})

	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, dks.Tryjob01FileIPhoneRGB)
	require.Error(t, err)
	assert.NotEqual(t, ingestion.ErrRetryable, skerr.Unwrap(err))
	assert.Contains(t, err.Error(), "not found")

	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "Tryjobs", &schema.TryjobRow{}))
}

func TestTryjobSQL_Process_UnknownTryjob_ReturnsNonretriableError(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewSpannerDBForTestsWithProductionSchema(ctx, t)

	const clID = dks.ChangelistIDThatAttemptsToFixIOS
	const psID = dks.PatchSetIDFixesIPadButNotIPhone
	const tjID = dks.Tryjob01IPhoneRGB

	mcrs := &mock_crs.Client{}
	mcrs.On("GetChangelist", testutils.AnyContext, clID).Return(code_review.Changelist{
		SystemID: clID,
		Owner:    dks.UserOne,
		Status:   code_review.Open,
		Subject:  "Fix iOS",
		Updated:  time.Date(2020, time.December, 5, 15, 0, 0, 0, time.UTC),
	}, nil)
	mcrs.On("GetPatchset", testutils.AnyContext, clID, psID, 0).Return(code_review.Patchset{
		SystemID:     psID,
		ChangelistID: clID,
		Order:        3,
		GitHash:      "ffff111111111111111111111111111111111111",
	}, nil)

	mcis := &mock_cis.Client{}
	mcis.On("GetTryJob", testutils.AnyContext, tjID).Return(ci.TryJob{}, ci.ErrNotFound)

	src := fakeGCSSourceFromFile(t, "from_goldctl_recent_fields.json")
	gtp := initCaches(goldTryjobProcessor{
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		db:     db,
		source: src,
	})

	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, dks.Tryjob01FileIPhoneRGB)
	require.Error(t, err)
	assert.NotEqual(t, ingestion.ErrRetryable, skerr.Unwrap(err))
	assert.Contains(t, err.Error(), "not found")

	assert.Empty(t, sqltest.GetAllRows(ctx, t, db, "Tryjobs", &schema.TryjobRow{}))
}

func TestTryjobSQL_Process_SameFileMultipleTimesInParallel_Success_cdb(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	const clID = dks.ChangelistIDThatAttemptsToFixIOS
	const psID = dks.PatchSetIDFixesIPadButNotIPhone
	const tjID = dks.Tryjob01IPhoneRGB

	const qualifiedCL = "gerrit_CL_fix_ios"
	const qualifiedPS = "gerrit_PS_fixes_ipad_but_not_iphone"
	const qualifiedTJ = "buildbucket_tryjob_01_iphonergb"

	ctx = overwriteNow(ctx, fakeIngestionTime)
	wg := sync.WaitGroup{}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mcrs := &mock_crs.Client{}
			mcrs.On("GetChangelist", testutils.AnyContext, clID).Return(code_review.Changelist{
				SystemID: clID,
				Owner:    dks.UserOne,
				Status:   code_review.Open,
				Subject:  "Fix iOS",
				// This time should get overwritten by the fakeIngestionTime
				Updated: time.Date(2020, time.December, 5, 15, 0, 0, 0, time.UTC),
			}, nil)
			mcrs.On("GetPatchset", testutils.AnyContext, clID, psID, 0).Return(code_review.Patchset{
				SystemID:     psID,
				ChangelistID: clID,
				Order:        3,
				GitHash:      "ffff111111111111111111111111111111111111",
			}, nil)

			mcis := &mock_cis.Client{}
			mcis.On("GetTryJob", testutils.AnyContext, tjID).Return(ci.TryJob{
				SystemID:    tjID,
				System:      dks.BuildBucketCIS,
				DisplayName: "Test-iPhone-RGB",
			}, nil)

			src := fakeGCSSourceFromFile(t, "from_goldctl_recent_fields.json")
			gtp := initCaches(goldTryjobProcessor{
				cisClients: map[string]ci.Client{
					buildbucketCIS: mcis,
				},
				reviewSystems: []clstore.ReviewSystem{
					{
						ID:     gerritCRS,
						Client: mcrs,
					},
				},
				db:     db,
				source: src,
			})

			for j := 0; j < 10; j++ {
				if err := ctx.Err(); err != nil {
					return
				}
				if err := gtp.Process(ctx, dks.Tryjob01FileIPhoneRGB); err != nil {
					assert.NoError(t, err)
					return
				}
			}
		}()
	}
	wg.Wait()
	// spot check the data to make sure it was written
	actualTryjobs := sqltest.GetAllRows(ctx, t, db, "Tryjobs", &schema.TryjobRow{}).([]schema.TryjobRow)
	assert.Equal(t, []schema.TryjobRow{{
		TryjobID:         qualifiedTJ,
		System:           dks.BuildBucketCIS,
		ChangelistID:     qualifiedCL,
		PatchsetID:       qualifiedPS,
		DisplayName:      "Test-iPhone-RGB",
		LastIngestedData: fakeIngestionTime,
	}}, actualTryjobs)

	actualGroupings := sqltest.GetAllRows(ctx, t, db, "Groupings", &schema.GroupingRow{}).([]schema.GroupingRow)
	assert.ElementsMatch(t, []schema.GroupingRow{{
		GroupingID: h(circleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.RoundCorpus,
			types.PrimaryKeyField: dks.CircleTest,
		},
	}, {
		GroupingID: h(squareGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
		},
	}, {
		GroupingID: h(triangleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.TriangleTest,
		},
	}}, actualGroupings)
}

func TestTryjobSQL_Process_SameFileMultipleTimesInParallel_Success(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewSpannerDBForTestsWithProductionSchema(ctx, t)

	const clID = dks.ChangelistIDThatAttemptsToFixIOS
	const psID = dks.PatchSetIDFixesIPadButNotIPhone
	const tjID = dks.Tryjob01IPhoneRGB

	const qualifiedCL = "gerrit_CL_fix_ios"
	const qualifiedPS = "gerrit_PS_fixes_ipad_but_not_iphone"
	const qualifiedTJ = "buildbucket_tryjob_01_iphonergb"

	ctx = overwriteNow(ctx, fakeIngestionTime)
	wg := sync.WaitGroup{}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mcrs := &mock_crs.Client{}
			mcrs.On("GetChangelist", testutils.AnyContext, clID).Return(code_review.Changelist{
				SystemID: clID,
				Owner:    dks.UserOne,
				Status:   code_review.Open,
				Subject:  "Fix iOS",
				// This time should get overwritten by the fakeIngestionTime
				Updated: time.Date(2020, time.December, 5, 15, 0, 0, 0, time.UTC),
			}, nil)
			mcrs.On("GetPatchset", testutils.AnyContext, clID, psID, 0).Return(code_review.Patchset{
				SystemID:     psID,
				ChangelistID: clID,
				Order:        3,
				GitHash:      "ffff111111111111111111111111111111111111",
			}, nil)

			mcis := &mock_cis.Client{}
			mcis.On("GetTryJob", testutils.AnyContext, tjID).Return(ci.TryJob{
				SystemID:    tjID,
				System:      dks.BuildBucketCIS,
				DisplayName: "Test-iPhone-RGB",
			}, nil)

			src := fakeGCSSourceFromFile(t, "from_goldctl_recent_fields.json")
			gtp := initCaches(goldTryjobProcessor{
				cisClients: map[string]ci.Client{
					buildbucketCIS: mcis,
				},
				reviewSystems: []clstore.ReviewSystem{
					{
						ID:     gerritCRS,
						Client: mcrs,
					},
				},
				db:     db,
				source: src,
			})

			for j := 0; j < 10; j++ {
				if err := ctx.Err(); err != nil {
					return
				}
				if err := gtp.Process(ctx, dks.Tryjob01FileIPhoneRGB); err != nil {
					assert.NoError(t, err)
					return
				}
			}
		}()
	}
	wg.Wait()
	// spot check the data to make sure it was written
	actualTryjobs := sqltest.GetAllRows(ctx, t, db, "Tryjobs", &schema.TryjobRow{}).([]schema.TryjobRow)
	assert.Equal(t, []schema.TryjobRow{{
		TryjobID:         qualifiedTJ,
		System:           dks.BuildBucketCIS,
		ChangelistID:     qualifiedCL,
		PatchsetID:       qualifiedPS,
		DisplayName:      "Test-iPhone-RGB",
		LastIngestedData: fakeIngestionTime,
	}}, actualTryjobs)

	actualGroupings := sqltest.GetAllRows(ctx, t, db, "Groupings", &schema.GroupingRow{}).([]schema.GroupingRow)
	assert.ElementsMatch(t, []schema.GroupingRow{{
		GroupingID: h(circleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.RoundCorpus,
			types.PrimaryKeyField: dks.CircleTest,
		},
	}, {
		GroupingID: h(squareGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.SquareTest,
		},
	}, {
		GroupingID: h(triangleGrouping),
		Keys: map[string]string{
			types.CorpusField:     dks.CornersCorpus,
			types.PrimaryKeyField: dks.TriangleTest,
		},
	}}, actualGroupings)
}

func TestTryjobSQL_Process_CLPSNeedResolution_Success_cdb(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	const resolvedCRS = dks.GerritCRS
	const resolvedCL = dks.ChangelistIDThatAttemptsToFixIOS
	const resolvedPSOrder = 3

	const expectedTryjobID = dks.Tryjob01IPhoneRGB
	const expectedPSID = dks.PatchSetIDFixesIPadButNotIPhone

	const qualifiedCL = "gerrit_CL_fix_ios"
	const qualifiedPS = "gerrit_PS_fixes_ipad_but_not_iphone"
	const qualifiedTJ = "buildbucket_tryjob_01_iphonergb"

	ml := &mocks.LookupSystem{}
	ml.On("Lookup", testutils.AnyContext, expectedTryjobID).Return(resolvedCRS, resolvedCL, resolvedPSOrder, nil)

	mcrs := &mock_crs.Client{}
	mcrs.On("GetChangelist", testutils.AnyContext, resolvedCL).Return(code_review.Changelist{
		SystemID: resolvedCL,
		Owner:    dks.UserOne,
		Status:   code_review.Open,
		Subject:  "Fix iOS",
		// This time should get overwritten by the fakeIngestionTime
		Updated: time.Date(2020, time.December, 5, 15, 0, 0, 0, time.UTC),
	}, nil)
	mcrs.On("GetPatchset", testutils.AnyContext, resolvedCL, "", resolvedPSOrder).Return(code_review.Patchset{
		SystemID:     expectedPSID,
		ChangelistID: resolvedCL,
		Order:        resolvedPSOrder,
		GitHash:      "ffff111111111111111111111111111111111111",
		Created:      time.Date(2020, time.December, 5, 15, 6, 3, 0, time.UTC),
	}, nil)

	mcis := &mock_cis.Client{}
	mcis.On("GetTryJob", testutils.AnyContext, expectedTryjobID).Return(ci.TryJob{
		SystemID:    expectedTryjobID,
		System:      dks.BuildBucketCIS,
		DisplayName: "Test-iPhone-RGB",
	}, nil)

	src := fakeGCSSourceFromFile(t, "needs_lookup.json")
	gtp := initCaches(goldTryjobProcessor{
		source:       src,
		db:           db,
		lookupSystem: ml,
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
	})
	ctx = overwriteNow(ctx, fakeIngestionTime)
	require.NoError(t, gtp.Process(ctx, "needs_lookup.json"))

	// Spot check the resolved data
	actualChangelists := sqltest.GetAllRows(ctx, t, db, "Changelists", &schema.ChangelistRow{}).([]schema.ChangelistRow)
	assert.Equal(t, []schema.ChangelistRow{{
		ChangelistID:     qualifiedCL,
		System:           dks.GerritCRS,
		Status:           schema.StatusOpen,
		OwnerEmail:       dks.UserOne,
		Subject:          "Fix iOS",
		LastIngestedData: fakeIngestionTime,
	}}, actualChangelists)

	actualPatchsets := sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{}).([]schema.PatchsetRow)
	assert.Equal(t, []schema.PatchsetRow{{
		PatchsetID:   qualifiedPS,
		System:       dks.GerritCRS,
		ChangelistID: qualifiedCL,
		Order:        3,
		GitHash:      "ffff111111111111111111111111111111111111",
		Created:      time.Date(2020, time.December, 5, 15, 6, 3, 0, time.UTC),
	}}, actualPatchsets)

	actualTryjobs := sqltest.GetAllRows(ctx, t, db, "Tryjobs", &schema.TryjobRow{}).([]schema.TryjobRow)
	assert.Equal(t, []schema.TryjobRow{{
		TryjobID:         qualifiedTJ,
		System:           dks.BuildBucketCIS,
		ChangelistID:     qualifiedCL,
		PatchsetID:       qualifiedPS,
		DisplayName:      "Test-iPhone-RGB",
		LastIngestedData: fakeIngestionTime,
	}}, actualTryjobs)
}

func TestTryjobSQL_Process_CLPSNeedResolution_Success(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewSpannerDBForTestsWithProductionSchema(ctx, t)

	const resolvedCRS = dks.GerritCRS
	const resolvedCL = dks.ChangelistIDThatAttemptsToFixIOS
	const resolvedPSOrder = 3

	const expectedTryjobID = dks.Tryjob01IPhoneRGB
	const expectedPSID = dks.PatchSetIDFixesIPadButNotIPhone

	const qualifiedCL = "gerrit_CL_fix_ios"
	const qualifiedPS = "gerrit_PS_fixes_ipad_but_not_iphone"
	const qualifiedTJ = "buildbucket_tryjob_01_iphonergb"

	ml := &mocks.LookupSystem{}
	ml.On("Lookup", testutils.AnyContext, expectedTryjobID).Return(resolvedCRS, resolvedCL, resolvedPSOrder, nil)

	mcrs := &mock_crs.Client{}
	mcrs.On("GetChangelist", testutils.AnyContext, resolvedCL).Return(code_review.Changelist{
		SystemID: resolvedCL,
		Owner:    dks.UserOne,
		Status:   code_review.Open,
		Subject:  "Fix iOS",
		// This time should get overwritten by the fakeIngestionTime
		Updated: time.Date(2020, time.December, 5, 15, 0, 0, 0, time.UTC),
	}, nil)
	mcrs.On("GetPatchset", testutils.AnyContext, resolvedCL, "", resolvedPSOrder).Return(code_review.Patchset{
		SystemID:     expectedPSID,
		ChangelistID: resolvedCL,
		Order:        resolvedPSOrder,
		GitHash:      "ffff111111111111111111111111111111111111",
		Created:      time.Date(2020, time.December, 5, 15, 6, 3, 0, time.UTC),
	}, nil)

	mcis := &mock_cis.Client{}
	mcis.On("GetTryJob", testutils.AnyContext, expectedTryjobID).Return(ci.TryJob{
		SystemID:    expectedTryjobID,
		System:      dks.BuildBucketCIS,
		DisplayName: "Test-iPhone-RGB",
	}, nil)

	src := fakeGCSSourceFromFile(t, "needs_lookup.json")
	gtp := initCaches(goldTryjobProcessor{
		source:       src,
		db:           db,
		lookupSystem: ml,
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
	})
	ctx = overwriteNow(ctx, fakeIngestionTime)
	require.NoError(t, gtp.Process(ctx, "needs_lookup.json"))

	// Spot check the resolved data
	actualChangelists := sqltest.GetAllRows(ctx, t, db, "Changelists", &schema.ChangelistRow{}).([]schema.ChangelistRow)
	assert.Equal(t, []schema.ChangelistRow{{
		ChangelistID:     qualifiedCL,
		System:           dks.GerritCRS,
		Status:           schema.StatusOpen,
		OwnerEmail:       dks.UserOne,
		Subject:          "Fix iOS",
		LastIngestedData: fakeIngestionTime,
	}}, actualChangelists)

	actualPatchsets := sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{}).([]schema.PatchsetRow)
	assert.Equal(t, []schema.PatchsetRow{{
		PatchsetID:   qualifiedPS,
		System:       dks.GerritCRS,
		ChangelistID: qualifiedCL,
		Order:        3,
		GitHash:      "ffff111111111111111111111111111111111111",
		Created:      time.Date(2020, time.December, 5, 15, 6, 3, 0, time.UTC),
	}}, actualPatchsets)

	actualTryjobs := sqltest.GetAllRows(ctx, t, db, "Tryjobs", &schema.TryjobRow{}).([]schema.TryjobRow)
	assert.Equal(t, []schema.TryjobRow{{
		TryjobID:         qualifiedTJ,
		System:           dks.BuildBucketCIS,
		ChangelistID:     qualifiedCL,
		PatchsetID:       qualifiedPS,
		DisplayName:      "Test-iPhone-RGB",
		LastIngestedData: fakeIngestionTime,
	}}, actualTryjobs)
}

func TestTryjobSQL_Process_CLPSNeedResolution_UsesCache_cdb(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	const resolvedCRS = dks.GerritCRS
	const resolvedCL = dks.ChangelistIDThatAttemptsToFixIOS
	const resolvedPSOrder = 3

	const expectedTryjobID = dks.Tryjob01IPhoneRGB

	ml := &mocks.LookupSystem{}
	ml.On("Lookup", testutils.AnyContext, expectedTryjobID).
		Return(resolvedCRS, resolvedCL, resolvedPSOrder, nil).Once() // Will fail if not cached

	mcrs := &mock_crs.Client{}
	mcis := &mock_cis.Client{}

	src := fakeGCSSourceFromFile(t, "needs_lookup.json")
	gtp := initCaches(goldTryjobProcessor{
		source:       src,
		db:           db,
		lookupSystem: ml,
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
	})
	ctx = overwriteNow(ctx, fakeIngestionTime)
	require.NoError(t, gtp.Process(ctx, "needs_lookup.json"))
	require.NoError(t, gtp.Process(ctx, "needs_lookup.json"))
	require.NoError(t, gtp.Process(ctx, "needs_lookup.json"))
	require.NoError(t, gtp.Process(ctx, "needs_lookup.json"))
}

func TestTryjobSQL_Process_CLPSNeedResolution_UsesCache(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewSpannerDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	const resolvedCRS = dks.GerritCRS
	const resolvedCL = dks.ChangelistIDThatAttemptsToFixIOS
	const resolvedPSOrder = 3

	const expectedTryjobID = dks.Tryjob01IPhoneRGB

	ml := &mocks.LookupSystem{}
	ml.On("Lookup", testutils.AnyContext, expectedTryjobID).
		Return(resolvedCRS, resolvedCL, resolvedPSOrder, nil).Once() // Will fail if not cached

	mcrs := &mock_crs.Client{}
	mcis := &mock_cis.Client{}

	src := fakeGCSSourceFromFile(t, "needs_lookup.json")
	gtp := initCaches(goldTryjobProcessor{
		source:       src,
		db:           db,
		lookupSystem: ml,
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
	})
	ctx = overwriteNow(ctx, fakeIngestionTime)
	require.NoError(t, gtp.Process(ctx, "needs_lookup.json"))
	require.NoError(t, gtp.Process(ctx, "needs_lookup.json"))
	require.NoError(t, gtp.Process(ctx, "needs_lookup.json"))
	require.NoError(t, gtp.Process(ctx, "needs_lookup.json"))
}

func TestTryjobSQL_Process_LookupSystemNotConfigured_ReturnsError_cdb(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	mcrs := &mock_crs.Client{}
	mcis := &mock_cis.Client{}

	src := fakeGCSSourceFromFile(t, "needs_lookup.json")
	gtp := initCaches(goldTryjobProcessor{
		source: src,
		db:     db,
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
	})
	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, "needs_lookup.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestTryjobSQL_Process_LookupSystemNotConfigured_ReturnsError(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewSpannerDBForTestsWithProductionSchema(ctx, t)

	mcrs := &mock_crs.Client{}
	mcis := &mock_cis.Client{}

	src := fakeGCSSourceFromFile(t, "needs_lookup.json")
	gtp := initCaches(goldTryjobProcessor{
		source: src,
		db:     db,
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
			},
		},
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
	})
	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, "needs_lookup.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestTryjobSQL_Process_LookupReturnsDifferentSystem_ReturnsError_cdb(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	ml := &mocks.LookupSystem{}
	ml.On("Lookup", testutils.AnyContext, mock.Anything).Return("incorrect CRS", "whatever", 6, nil)

	mcrs := &mock_crs.Client{}
	mcis := &mock_cis.Client{}

	src := fakeGCSSourceFromFile(t, "needs_lookup.json")
	gtp := initCaches(goldTryjobProcessor{
		source:       src,
		db:           db,
		lookupSystem: ml,
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     "gerrit",
				Client: mcrs,
			},
		},
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
	})
	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, "needs_lookup.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "after lookup")
}

func TestTryjobSQL_Process_LookupReturnsDifferentSystem_ReturnsError(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewSpannerDBForTestsWithProductionSchema(ctx, t)

	ml := &mocks.LookupSystem{}
	ml.On("Lookup", testutils.AnyContext, mock.Anything).Return("incorrect CRS", "whatever", 6, nil)

	mcrs := &mock_crs.Client{}
	mcis := &mock_cis.Client{}

	src := fakeGCSSourceFromFile(t, "needs_lookup.json")
	gtp := initCaches(goldTryjobProcessor{
		source:       src,
		db:           db,
		lookupSystem: ml,
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     "gerrit",
				Client: mcrs,
			},
		},
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
	})
	ctx = overwriteNow(ctx, fakeIngestionTime)
	err := gtp.Process(ctx, "needs_lookup.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "after lookup")
}

func TestLookupAndCreatePS_PatchsetExistsForAnotherCL_ReturnsNonconflictingPSID_cdb(t *testing.T) {

	const gitHash = "000000000000000"
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := schema.Tables{
		Changelists: []schema.ChangelistRow{
			{
				ChangelistID:     "github_cl1111111",
				System:           "github",
				Status:           schema.StatusOpen,
				OwnerEmail:       "user@example.com",
				Subject:          "Some subject",
				LastIngestedData: time.Date(2021, time.October, 11, 11, 11, 11, 0, time.UTC),
			},
			{
				ChangelistID:     "github_cl2222222",
				System:           "github",
				Status:           schema.StatusOpen,
				OwnerEmail:       "user@example.com",
				Subject:          "Some other subject",
				LastIngestedData: time.Date(2021, time.October, 22, 22, 22, 22, 0, time.UTC),
			},
		},
		Patchsets: []schema.PatchsetRow{
			{
				PatchsetID:    "github_ps000000000000000",
				System:        "github",
				ChangelistID:  "github_cl1111111",
				Order:         4,
				GitHash:       gitHash,
				CommentedOnCL: false,
				Created:       time.Date(2021, time.October, 11, 11, 12, 0, 0, time.UTC),
			},
		},
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	const clID = "cl2222222"
	const psID = "ps000000000000000"

	mcrs := &mock_crs.Client{}
	mcrs.On("GetPatchset", testutils.AnyContext, "cl2222222", "ps000000000000000", 0).Return(code_review.Patchset{
		SystemID:      psID,
		ChangelistID:  clID,
		Order:         1,
		GitHash:       gitHash,
		Created:       time.Date(2021, time.October, 22, 22, 23, 0, 0, time.UTC),
		CommentedOnCL: false,
	}, nil)

	p := goldTryjobProcessor{
		db: db,
	}

	qualifiedPSID, err := p.lookupAndCreatePS(ctx, mcrs, clID, psID, "github", 0)
	require.NoError(t, err)
	// We expect to see the patchset id qualified even further with the CLID.
	assert.Equal(t, "github_ps000000000000000-cl2222222", qualifiedPSID)

	patchsets := sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{})
	assert.ElementsMatch(t, []schema.PatchsetRow{
		{
			PatchsetID:    "github_ps000000000000000",
			System:        "github",
			ChangelistID:  "github_cl1111111",
			Order:         4,
			GitHash:       gitHash,
			CommentedOnCL: false,
			Created:       time.Date(2021, time.October, 11, 11, 12, 0, 0, time.UTC),
		},
		{
			PatchsetID:    "github_ps000000000000000-cl2222222",
			System:        "github",
			ChangelistID:  "github_cl2222222",
			Order:         1,
			GitHash:       gitHash,
			CommentedOnCL: false,
			Created:       time.Date(2021, time.October, 22, 22, 23, 0, 0, time.UTC),
		},
	}, patchsets)
}

func TestLookupAndCreatePS_PatchsetExistsForAnotherCL_ReturnsNonconflictingPSID(t *testing.T) {

	const gitHash = "000000000000000"
	ctx := context.Background()
	db := sqltest.NewSpannerDBForTestsWithProductionSchema(ctx, t)
	existingData := schema.Tables{
		Changelists: []schema.ChangelistRow{
			{
				ChangelistID:     "github_cl1111111",
				System:           "github",
				Status:           schema.StatusOpen,
				OwnerEmail:       "user@example.com",
				Subject:          "Some subject",
				LastIngestedData: time.Date(2021, time.October, 11, 11, 11, 11, 0, time.UTC),
			},
			{
				ChangelistID:     "github_cl2222222",
				System:           "github",
				Status:           schema.StatusOpen,
				OwnerEmail:       "user@example.com",
				Subject:          "Some other subject",
				LastIngestedData: time.Date(2021, time.October, 22, 22, 22, 22, 0, time.UTC),
			},
		},
		Patchsets: []schema.PatchsetRow{
			{
				PatchsetID:    "github_ps000000000000000",
				System:        "github",
				ChangelistID:  "github_cl1111111",
				Order:         4,
				GitHash:       gitHash,
				CommentedOnCL: false,
				Created:       time.Date(2021, time.October, 11, 11, 12, 0, 0, time.UTC),
			},
		},
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	const clID = "cl2222222"
	const psID = "ps000000000000000"

	mcrs := &mock_crs.Client{}
	mcrs.On("GetPatchset", testutils.AnyContext, "cl2222222", "ps000000000000000", 0).Return(code_review.Patchset{
		SystemID:      psID,
		ChangelistID:  clID,
		Order:         1,
		GitHash:       gitHash,
		Created:       time.Date(2021, time.October, 22, 22, 23, 0, 0, time.UTC),
		CommentedOnCL: false,
	}, nil)

	p := goldTryjobProcessor{
		db: db,
	}

	qualifiedPSID, err := p.lookupAndCreatePS(ctx, mcrs, clID, psID, "github", 0)
	require.NoError(t, err)
	// We expect to see the patchset id qualified even further with the CLID.
	assert.Equal(t, "github_ps000000000000000-cl2222222", qualifiedPSID)

	patchsets := sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{})
	assert.ElementsMatch(t, []schema.PatchsetRow{
		{
			PatchsetID:    "github_ps000000000000000",
			System:        "github",
			ChangelistID:  "github_cl1111111",
			Order:         4,
			GitHash:       gitHash,
			CommentedOnCL: false,
			Created:       time.Date(2021, time.October, 11, 11, 12, 0, 0, time.UTC),
		},
		{
			PatchsetID:    "github_ps000000000000000-cl2222222",
			System:        "github",
			ChangelistID:  "github_cl2222222",
			Order:         1,
			GitHash:       gitHash,
			CommentedOnCL: false,
			Created:       time.Date(2021, time.October, 22, 22, 23, 0, 0, time.UTC),
		},
	}, patchsets)
}

func initCaches(processor goldTryjobProcessor) goldTryjobProcessor {
	ogCache, err := lru.New(optionsGroupingCacheSize)
	if err != nil {
		panic(err) // should only throw error on invalid size
	}
	paramsCache, err := lru.New(paramsCacheSize)
	if err != nil {
		panic(err) // should only throw error on invalid size
	}
	tCache, err := lru.New(traceCacheSize)
	if err != nil {
		panic(err) // should only throw error on invalid size
	}
	clCache, err := lru.New(clCacheSize)
	if err != nil {
		panic(err) // should only throw error on invalid size
	}
	processor.optionGroupingCache = ogCache
	processor.paramsCache = paramsCache
	processor.traceCache = tCache
	processor.clCache = clCache
	return processor
}

var beginningOfTime = ts("1970-01-01T00:00:00Z")

func ts(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
