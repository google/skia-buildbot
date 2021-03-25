package ingestion_processors

import (
	"context"
	"testing"

	ci "go.skia.org/infra/golden/go/continuous_integration"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review/gerrit_crs"
	mock_crs "go.skia.org/infra/golden/go/code_review/mocks"
	mock_cis "go.skia.org/infra/golden/go/continuous_integration/mocks"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
)

func TestTryjobSQL_SingleCRSAndCIS_Success(t *testing.T) {
	unittest.SmallTest(t)

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
}

func TestTryjobSQL_SingleCRSDoubleCIS_Success(t *testing.T) {
	unittest.SmallTest(t)

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
}

func TestTryjobSQL_Process_FirstFileForCL_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	// This file has data from 3 traces across 2 corpora. The data is for the patchset with order 3.
	const clID = dks.ChangelistIDThatAttemptsToFixIOS
	const psID = dks.PatchSetIDFixesIPadButNotIPhone
	const squareTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"square","os":"iOS","source_type":"corners"}`
	const triangleTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"triangle","os":"iOS","source_type":"corners"}`
	const circleTraceKeys = `{"color mode":"RGB","device":"iPhone12,1","name":"circle","os":"iOS","source_type":"round"}`

	mcis := &mock_cis.Client{}
	mcrs := &mock_crs.Client{}

	src := fakeGCSSourceFromFile(t, "from_goldctl_legacy_fields.json")
	gtp := goldTryjobProcessor{
		cisClients: map[string]ci.Client{
			buildbucketCIS: mcis,
		},
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: mcrs,
				// Store and URLTemplate unused here
			},
		},
		db:     db,
		source: src,
	}

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
		ChangelistID:     "gerrit_CL_fix_ios",
		System:           "gerrit",
		Status:           schema.StatusOpen,
		OwnerEmail:       dks.UserOne,
		Subject:          "Fix iOS",
		LastIngestedData: fakeIngestionTime,
	}}, actualChangelists)

	actualPatchsets := sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{}).([]schema.PatchsetRow)
	assert.Equal(t, []schema.PatchsetRow{{
		PatchsetID:   "gerrit_PS_fixes_ipad_but_not_iphone",
		System:       "gerrit",
		ChangelistID: "gerrit_CL_fix_ios",
		Order:        3,
		GitHash:      "ffff111111111111111111111111111111111111",
	}}, actualPatchsets)

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
	assert.Equal(t, []schema.SecondaryBranchParamRow{
		{Key: dks.ColorModeKey, Value: dks.RGBColorMode, BranchName: clID, VersionName: psID},
		{Key: dks.DeviceKey, Value: dks.QuadroDevice, BranchName: clID, VersionName: psID},
		{Key: "ext", Value: "png", BranchName: clID, VersionName: psID},
		{Key: types.PrimaryKeyField, Value: dks.CircleTest, BranchName: clID, VersionName: psID},
		{Key: types.PrimaryKeyField, Value: dks.SquareTest, BranchName: clID, VersionName: psID},
		{Key: types.PrimaryKeyField, Value: dks.TriangleTest, BranchName: clID, VersionName: psID},
		{Key: dks.OSKey, Value: dks.Windows10dot2OS, BranchName: clID, VersionName: psID},
		{Key: types.CorpusField, Value: dks.CornersCorpus, BranchName: clID, VersionName: psID},
		{Key: types.CorpusField, Value: dks.RoundCorpus, BranchName: clID, VersionName: psID},
	}, actualParams)

	actualValues := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchValues", &schema.SecondaryBranchValueRow{}).([]schema.SecondaryBranchValueRow)
	assert.ElementsMatch(t, []schema.SecondaryBranchValueRow{{
		BranchName: clID, VersionName: psID,
		TraceID:      h(squareTraceKeys),
		Digest:       d(dks.DigestA01Pos),
		GroupingID:   h(squareGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     dks.Tryjob01IPhoneRGB,
	}, {
		BranchName: clID, VersionName: psID,
		TraceID:      h(triangleTraceKeys),
		Digest:       d(dks.DigestB01Pos),
		GroupingID:   h(triangleGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     dks.Tryjob01IPhoneRGB,
	}, {
		BranchName: clID, VersionName: psID,
		TraceID:      h(circleTraceKeys),
		Digest:       d(dks.DigestC07Unt_CL),
		GroupingID:   h(circleGrouping),
		OptionsID:    h(pngOptions),
		SourceFileID: h(dks.Tryjob01FileIPhoneRGB),
		TryjobID:     dks.Tryjob01IPhoneRGB,
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

// TestTryJobProcessFreshStartGitHub tests the scenario in which we see data uploaded to GitHub for
// a brand new CL, PS, and TryJob. The PS is derived by id, not by order. The uploaded digest
// was not previously seen or triaged on master and is not covered by an ignore rule, so the
// created Patchset object should be marked as having UntriagedDigests.
func TestTryJobProcessFreshStartGitHub(t *testing.T) {
	t.Skip("rebuild")
}

// TestProcess_MultipleCIS_CorrectlyLooksUpTryJobs processes three results from different CIS
// (and has the CIS return an error to short-circuit the process code) and verifies that the
// two CIS we knew about were correctly contacted and the result with an unknown CIS was ignored.
func TestProcess_MultipleCIS_CorrectlyLooksUpTryJobs(t *testing.T) {
	t.Skip("rebuild")
}

// TestTryJobProcessCLExistsSunnyDay tests that the ingestion works when the CL already exists.
func TestTryJobProcessCLExistsSunnyDay(t *testing.T) {
	t.Skip("rebuild")
}

// TestTryJobProcessCLExistsPreviouslyAbandoned tests that the ingestion works when the
// CL already exists, but was marked abandoned at some point (and presumably was re-opened).
func TestTryJobProcessCLExistsPreviouslyAbandoned(t *testing.T) {
	t.Skip("rebuild")
}

// TestTryJobProcessPSExistsSunnyDay tests that the ingestion works when the
// CL and the PS already exists.
func TestTryJobProcessPSExistsSunnyDay(t *testing.T) {
	t.Skip("rebuild")

}

func TestTryjobSQL_Process_CLPSNeedResolution_Success(t *testing.T) {
	t.Skip("waiting until after refactoring")

	src := fakeGCSSourceFromFile(t, "needs_lookup.json")
	gtp := goldTryjobProcessor{
		source: src,
	}

	require.NoError(t, gtp.Process(context.Background(), "needs_lookup.json"))
}
