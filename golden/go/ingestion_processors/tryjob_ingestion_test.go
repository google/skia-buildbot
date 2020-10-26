package ingestion_processors

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/clstore"
	mock_clstore "go.skia.org/infra/golden/go/clstore/mocks"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/code_review/gerrit_crs"
	mock_crs "go.skia.org/infra/golden/go/code_review/mocks"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	mock_cis "go.skia.org/infra/golden/go/continuous_integration/mocks"
	"go.skia.org/infra/golden/go/ingestion"
	ingestion_mocks "go.skia.org/infra/golden/go/ingestion/mocks"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/tjstore"
	mock_tjstore "go.skia.org/infra/golden/go/tjstore/mocks"
	"go.skia.org/infra/golden/go/types"
)

const (
	legacyGoldCtlFile = "testdata/legacy-tryjob-goldctl.json"
	githubGoldCtlFile = "testdata/github-goldctl.json"
)

func TestGerritBuildbucketFactory(t *testing.T) {
	unittest.LargeTest(t) // should use the emulator

	config := ingestion.Config{
		ExtraParams: map[string]string{
			firestoreProjectIDParam: "should-use-emulator",
			firestoreNamespaceParam: "testing",

			codeReviewSystemsParam: "gerrit,gerrit-internal",
			gerritURLParam:         "https://example-review.googlesource.com",
			gerritInternalURLParam: "https://example-internal-review.googlesource.com",

			continuousIntegrationSystemsParam: "buildbucket",
		},
	}
	ctx := gerrit_crs.TestContext(context.Background())
	p, err := newModularTryjobProcessor(ctx, nil, config, httputils.NewTimeoutClient())
	require.NoError(t, err)
	require.NotNil(t, p)

	gtp, ok := p.(*goldTryjobProcessor)
	require.True(t, ok)
	assert.Len(t, gtp.reviewSystems, 2)
	assert.Len(t, gtp.cisClients, 1)
	assert.Contains(t, gtp.cisClients, buildbucketCIS)
}

func TestGitHubCirrusBuildbucketFactory(t *testing.T) {
	unittest.LargeTest(t) // should use the emulator

	config := ingestion.Config{
		ExtraParams: map[string]string{
			firestoreProjectIDParam: "should-use-emulator",
			firestoreNamespaceParam: "testing",

			codeReviewSystemsParam:     "github",
			githubRepoParam:            "google/skia",
			githubCredentialsPathParam: "testdata/fake_token", // this is actually a file on disk.

			continuousIntegrationSystemsParam: "cirrus,buildbucket",
		},
	}

	ctx := gerrit_crs.TestContext(context.Background())
	p, err := newModularTryjobProcessor(ctx, nil, config, httputils.NewTimeoutClient())
	require.NoError(t, err)
	require.NotNil(t, p)

	gtp, ok := p.(*goldTryjobProcessor)
	require.True(t, ok)
	assert.Len(t, gtp.reviewSystems, 1)
	assert.Len(t, gtp.cisClients, 2)
	assert.Contains(t, gtp.cisClients, cirrusCIS)
	assert.Contains(t, gtp.cisClients, buildbucketCIS)
}

// TestTryJobProcessFreshStartSunnyDay tests the scenario in which we see data uploaded to Gerrit
// for a brand new CL, PS, and TryJob. There are no ignore rules and the known digests don't contain
// gerritDigest.
func TestTryJobProcessFreshStartSunnyDay(t *testing.T) {
	unittest.SmallTest(t)
	mcls := makeEmptyCLStore()
	mtjs := makeEmptyTJStore()
	defer mcls.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)

	mcls.On("PutChangeList", testutils.AnyContext, clWithUpdatedTime(t, gerritCLID, gerritCLDate)).Return(nil).Once()
	mcls.On("PutPatchSet", testutils.AnyContext, makeGerritPatchSet()).Return(nil).Once()

	mtjs.On("PutTryJob", testutils.AnyContext, gerritCombinedID, makeGerritBuildbucketTryJob()).Return(nil).Once()
	mtjs.On("PutResults", testutils.AnyContext, gerritCombinedID, gerritTJID, buildbucketCIS, makeTryJobResults(), anyTime).Return(nil).Once()

	gtp := goldTryjobProcessor{
		cisClients: makeBuildbucketCIS(),
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: makeGerritCRS(),
				Store:  mcls,
				// URLTemplate unused here
			},
		},
		tryJobStore: mtjs,
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(legacyGoldCtlFile)
	require.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	require.NoError(t, err)
}

// TestTryJobProcessFreshStartGitHub tests the scenario in which we see data uploaded to GitHub for
// a brand new CL, PS, and TryJob. The PS is derived by id, not by order. The uploaded digest
// was not previously seen or triaged on master and is not covered by an ignore rule, so the
// created PatchSet object should be marked as having UntriagedDigests.
func TestTryJobProcessFreshStartGitHub(t *testing.T) {
	unittest.SmallTest(t)
	mcls := makeEmptyCLStore()
	mtjs := makeEmptyTJStore()
	// We want to assert that the Process calls each of PutChangeList, PutPatchSet, and PutTryJob
	// with the new, correct objects object. Further, it should call PutResults with the
	// appropriate TryJobResults.
	defer mcls.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)

	mcls.On("PutChangeList", testutils.AnyContext, clWithUpdatedTime(t, githubCLID, makeGitHubCirrusTryJob().Updated)).Return(nil)
	mcls.On("PutPatchSet", testutils.AnyContext, code_review.PatchSet{
		SystemID:     githubPSID,
		ChangeListID: githubCLID,
		Order:        githubPSOrder,
		GitHash:      githubPSID,
	}).Return(nil).Once()

	combinedID := tjstore.CombinedPSID{CL: githubCLID, PS: githubPSID, CRS: "github"}
	mtjs.On("PutTryJob", testutils.AnyContext, combinedID, makeGitHubCirrusTryJob()).Return(nil)
	mtjs.On("PutResults", testutils.AnyContext, combinedID, githubTJID, cirrusCIS, makeGitHubTryJobResults(), anyTime).Return(nil)

	gtp := goldTryjobProcessor{
		cisClients: makeCirrusCIS(),
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     githubCRS,
				Client: makeGitHubCRS(),
				Store:  mcls,
				// URLTemplate unused here
			},
		},
		tryJobStore: mtjs,
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(githubGoldCtlFile)
	require.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	require.NoError(t, err)
}

// TestProcess_MultipleCIS_CorrectlyLooksUpTryJobs processes three results from different CIS
// (and has the CIS return an error to short-circuit the process code) and verifies that the
// two CIS we knew about were correctly contacted and the result with an unknown CIS was ignored.
func TestProcess_MultipleCIS_CorrectlyLooksUpTryJobs(t *testing.T) {
	unittest.SmallTest(t)
	mcls := &mock_clstore.Store{}
	// We can return whatever here, since we plan to error out when the tryjob gets read.
	mcls.On("GetChangeList", testutils.AnyContext, githubCLID).Return(makeChangeList(), nil)
	mcls.On("GetPatchSet", testutils.AnyContext, githubCLID, githubPSID).Return(makeGerritPatchSets()[0], nil)

	bbClient := &mock_cis.Client{}
	bbClient.On("GetTryJob", testutils.AnyContext, mock.Anything).Return(ci.TryJob{}, errors.New("buildbucket error")).Once()
	defer bbClient.AssertExpectations(t) // make sure GetTryJob is called exactly once.

	cirrusClient := &mock_cis.Client{}
	cirrusClient.On("GetTryJob", testutils.AnyContext, mock.Anything).Return(ci.TryJob{}, errors.New("cirrus error")).Once()
	defer cirrusClient.AssertExpectations(t) // make sure GetTryJob is called exactly once.

	errorfulCISClients := map[string]ci.Client{
		buildbucketCIS: bbClient,
		cirrusCIS:      cirrusClient,
	}

	gtp := goldTryjobProcessor{
		cisClients: errorfulCISClients,
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:    githubCRS,
				Store: mcls,
				// Client, URLTemplate unused here
			},
		},
		tryJobStore: makeEmptyTJStore(),
	}

	err := gtp.Process(context.Background(), githubIngestionResultFromCIS(t, buildbucketCIS))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "buildbucket error")

	err = gtp.Process(context.Background(), githubIngestionResultFromCIS(t, cirrusCIS))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cirrus error")

	err = gtp.Process(context.Background(), githubIngestionResultFromCIS(t, "unknown"))
	require.Error(t, err)
	assert.Equal(t, err, ingestion.IgnoreResultsFileErr)
}

func githubIngestionResultFromCIS(t *testing.T, cis string) ingestion.ResultFileLocation {
	// We provide the bare minimum to be a valid result
	gr := jsonio.GoldResults{
		Key: map[string]string{
			types.CorpusField: "arbitrary",
		},
		Results: []*jsonio.Result{
			{
				Key: map[string]string{
					types.PrimaryKeyField: "whatever",
				},
				// arbitrary, yet valid, md5 hash
				Digest: "46eb78c9711cb79197d47f448ba51338",
			},
		},
		// arbitrary, yet valid, sha1 (git) hash
		GitHash:                     "6eb2b22a052a9913fe3b9170fc217e84def40598",
		ChangeListID:                githubCLID,
		PatchSetID:                  githubPSID,
		CodeReviewSystem:            githubCRS,
		TryJobID:                    "whatever",
		ContinuousIntegrationSystem: cis,
	}
	b, err := json.Marshal(gr)
	require.NoError(t, err)

	// These two fields are arbitrary and don't affect the test.
	const name = "does not matter"
	ts := time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)

	return ingestion_mocks.MockResultFileLocationWithContent(name, b, ts)
}

// TestTryJobProcessCLExistsSunnyDay tests that the ingestion works when the CL already exists.
func TestTryJobProcessCLExistsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)
	mcls := &mock_clstore.Store{}
	mtjs := makeEmptyTJStore()
	// We want to assert that the Process calls PutChangeList (with updated time), PutPatchSet
	// (with the correct new object), PutTryJob with the new TryJob object and PutResults with
	// the appropriate TryJobResults.
	defer mcls.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)

	mcls.On("GetChangeList", testutils.AnyContext, gerritCLID).Return(makeChangeList(), nil)
	mcls.On("GetPatchSetByOrder", testutils.AnyContext, gerritCLID, gerritPSOrder).Return(code_review.PatchSet{}, clstore.ErrNotFound)
	mcls.On("PutPatchSet", testutils.AnyContext, makeGerritPatchSet()).Return(nil)
	mcls.On("PutChangeList", testutils.AnyContext, clWithUpdatedTime(t, gerritCLID, gerritCLDate)).Return(nil)

	mtjs.On("PutTryJob", testutils.AnyContext, gerritCombinedID, makeGerritBuildbucketTryJob()).Return(nil)
	mtjs.On("PutResults", testutils.AnyContext, gerritCombinedID, gerritTJID, buildbucketCIS, makeTryJobResults(), anyTime).Return(nil)

	gtp := goldTryjobProcessor{
		cisClients: makeBuildbucketCIS(),
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: makeGerritCRS(),
				Store:  mcls,
				// URLTemplate unused here
			},
		},
		tryJobStore: mtjs,
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(legacyGoldCtlFile)
	require.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	require.NoError(t, err)
}

// TestTryJobProcessCLExistsPreviouslyAbandoned tests that the ingestion works when the
// CL already exists, but was marked abandoned at some point (and presumably was re-opened).
func TestTryJobProcessCLExistsPreviouslyAbandoned(t *testing.T) {
	unittest.SmallTest(t)
	mcls := &mock_clstore.Store{}
	mtjs := makeEmptyTJStore()
	// We want to assert that the Process calls PutChangeList (with updated time and no longer
	// abandoned), PutPatchSet with the new object, PutTryJob with the new TryJob object,
	// and PutResults with the appropriate TryJobResults.
	defer mcls.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)

	cl := makeChangeList()
	cl.Status = code_review.Abandoned
	mcls.On("GetChangeList", testutils.AnyContext, gerritCLID).Return(cl, nil)
	mcls.On("GetPatchSetByOrder", testutils.AnyContext, gerritCLID, gerritPSOrder).Return(code_review.PatchSet{}, clstore.ErrNotFound)
	mcls.On("PutPatchSet", testutils.AnyContext, makeGerritPatchSet()).Return(nil)
	mcls.On("PutChangeList", testutils.AnyContext, clWithUpdatedTime(t, gerritCLID, gerritCLDate)).Return(nil)

	mtjs.On("PutTryJob", testutils.AnyContext, gerritCombinedID, makeGerritBuildbucketTryJob()).Return(nil)
	mtjs.On("PutResults", testutils.AnyContext, gerritCombinedID, gerritTJID, buildbucketCIS, makeTryJobResults(), anyTime).Return(nil)

	gtp := goldTryjobProcessor{
		cisClients: makeBuildbucketCIS(),
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:     gerritCRS,
				Client: makeGerritCRS(),
				Store:  mcls,
				// URLTemplate unused here
			},
		},
		tryJobStore: mtjs,
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(legacyGoldCtlFile)
	require.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	require.NoError(t, err)
}

// TestTryJobProcessPSExistsSunnyDay tests that the ingestion works when the
// CL and the PS already exists.
func TestTryJobProcessPSExistsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)
	mcls := &mock_clstore.Store{}
	mtjs := makeEmptyTJStore()
	// We want to assert that the Process calls PutChangeList and PutPatchSet (with updated times),
	// PutTryJob with the new TryJob object, and PutResults with the appropriate TryJobResults.
	defer mcls.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)

	mcls.On("GetChangeList", testutils.AnyContext, gerritCLID).Return(makeChangeList(), nil)
	mcls.On("GetPatchSetByOrder", testutils.AnyContext, gerritCLID, gerritPSOrder).Return(makeGerritPatchSet(), nil)
	mcls.On("PutChangeList", testutils.AnyContext, clWithUpdatedTime(t, gerritCLID, gerritCLDate)).Return(nil)
	mcls.On("PutPatchSet", testutils.AnyContext, makeGerritPatchSet()).Return(nil)

	mtjs.On("PutTryJob", testutils.AnyContext, gerritCombinedID, makeGerritBuildbucketTryJob()).Return(nil)
	mtjs.On("PutResults", testutils.AnyContext, gerritCombinedID, gerritTJID, buildbucketCIS, makeTryJobResults(), anyTime).Return(nil)

	gtp := goldTryjobProcessor{
		cisClients: makeBuildbucketCIS(),
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:    gerritCRS,
				Store: mcls,
				// Client, URLTemplate unused here
			},
		},
		tryJobStore: mtjs,
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(legacyGoldCtlFile)
	require.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	require.NoError(t, err)
}

func makeEmptyCLStore() *mock_clstore.Store {
	mcls := &mock_clstore.Store{}
	mcls.On("GetChangeList", testutils.AnyContext, mock.Anything).Return(code_review.ChangeList{}, clstore.ErrNotFound).Maybe()
	mcls.On("GetPatchSetByOrder", testutils.AnyContext, mock.Anything, mock.Anything).Return(code_review.PatchSet{}, clstore.ErrNotFound).Maybe()
	mcls.On("GetPatchSet", testutils.AnyContext, mock.Anything, mock.Anything).Return(code_review.PatchSet{}, clstore.ErrNotFound).Maybe()

	return mcls
}

func makeEmptyTJStore() *mock_tjstore.Store {
	mtjs := &mock_tjstore.Store{}
	mtjs.On("GetTryJob", testutils.AnyContext, mock.Anything, mock.Anything).Return(ci.TryJob{}, tjstore.ErrNotFound).Maybe()
	return mtjs
}

// Below is the gerrit data that belongs to legacyGoldCtlFile. It doesn't need to be a super
// complex example because we have tests that test parseDMResults directly.
const (
	gerritCLID    = "1762193"
	gerritPSID    = "e1681c90cf6a4c3b6be2bc4b4cea59849c16a438"
	gerritPSOrder = 2
	gerritTJID    = "8904604368086838672"
)

var (
	gerritCombinedID = tjstore.CombinedPSID{CL: gerritCLID, PS: gerritPSID, CRS: gerritCRS}

	gerritCLDate = time.Date(2019, time.August, 19, 18, 17, 16, 0, time.UTC)

	anyTime = mock.MatchedBy(func(time.Time) bool { return true })
)

// These are functions to avoid mutations causing issues for future tests/checks
func makeTryJobResults() []tjstore.TryJobResult {
	return []tjstore.TryJobResult{
		{
			Digest: "690f72c0b56ae014c8ac66e7f25c0779",
			GroupParams: paramtools.Params{
				"device_id":     "0x1cb3",
				"device_string": "None",
				"msaa":          "True",
				"vendor_id":     "0x10de",
				"vendor_string": "None",
			},
			ResultParams: paramtools.Params{
				"name":        "Pixel_CanvasDisplayLinearRGBUnaccelerated2DGPUCompositing",
				"source_type": "chrome-gpu",
			},
			Options: paramtools.Params{
				"ext": "png",
			},
		},
	}
}

func makeChangeList() code_review.ChangeList {
	return code_review.ChangeList{
		SystemID: "1762193",
		Owner:    "test@example.com",
		Status:   code_review.Open,
		Subject:  "initial commit",
		Updated:  gerritCLDate,
	}
}

// clWithUpdatedTime returns a matcher that will assert the CL has properly had its Updated field
// updated.
func clWithUpdatedTime(t *testing.T, clID string, originalDate time.Time) interface{} {
	return mock.MatchedBy(func(cl code_review.ChangeList) bool {
		assert.Equal(t, clID, cl.SystemID)
		assert.Equal(t, code_review.Open, cl.Status)
		// Make sure the time is updated to be later than the original one (which was in November
		// or August, depending on the testcase). Since this test was authored after 1 Dec 2019 and
		// the Updated is set to time.Now(), we can just check that we are after then.
		assert.True(t, cl.Updated.After(originalDate))
		// assert messages are easier to debug than "not matched" errors, so say that we matched,
		// but know the test will fail if any of the above asserts fail.
		return true
	})
}

func makeGerritPatchSets() []code_review.PatchSet {
	return []code_review.PatchSet{
		{
			SystemID:     "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1",
			ChangeListID: "1762193",
			Order:        1,
			GitHash:      "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1",
		},
		{
			SystemID:     gerritPSID,
			ChangeListID: "1762193",
			Order:        gerritPSOrder,
			GitHash:      gerritPSID,
		},
		{
			SystemID:     "b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2",
			ChangeListID: "1762193",
			Order:        3,
			GitHash:      "b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2",
		},
	}
}

func makeGerritPatchSet() code_review.PatchSet {
	ps := makeGerritPatchSets()[1]
	return ps
}

func makeGerritBuildbucketTryJob() ci.TryJob {
	return ci.TryJob{
		SystemID:    gerritTJID,
		System:      buildbucketCIS,
		DisplayName: "my-task",
		Updated:     time.Date(2019, time.August, 19, 18, 20, 10, 0, time.UTC),
	}
}

func makeBuildbucketCIS() map[string]ci.Client {
	mcis := &mock_cis.Client{}
	mcis.On("GetTryJob", testutils.AnyContext, gerritTJID).Return(makeGerritBuildbucketTryJob(), nil)
	return map[string]ci.Client{
		buildbucketCIS: mcis,
	}
}

func makeGerritCRS() *mock_crs.Client {
	mcrs := &mock_crs.Client{}
	mcrs.On("GetChangeList", testutils.AnyContext, gerritCLID).Return(makeChangeList(), nil)
	mcrs.On("GetPatchSets", testutils.AnyContext, gerritCLID).Return(makeGerritPatchSets(), nil)
	return mcrs
}

// Below is the gerrit data that belongs to githubGoldCtlFile, which is based on real data.
const (
	githubCLID    = "44474"
	githubPSOrder = 1
	githubPSID    = "fe1cad6c1a5d6dc7cea47f09efdd49f197a7f017"
	githubTJID    = "5489081055707136"
)

func makeGitHubCirrusTryJob() ci.TryJob {
	return ci.TryJob{
		SystemID:    githubTJID,
		System:      cirrusCIS,
		DisplayName: "my-github-task",
		Updated:     time.Date(2019, time.November, 19, 18, 20, 10, 0, time.UTC),
	}
}

func makeGitHubTryJobResults() []tjstore.TryJobResult {
	return []tjstore.TryJobResult{
		{
			Digest: "87599f3dec5b56dc110f1b63dc747182",
			GroupParams: paramtools.Params{
				"Platform": "windows",
			},
			ResultParams: paramtools.Params{
				"name":        "cupertino.date_picker_test.datetime.initial",
				"source_type": "flutter",
			},
			Options: paramtools.Params{
				"ext": "png",
			},
		},
		{
			Digest: "7d04fc1ef547a8e092495dab4294b4cd",
			GroupParams: paramtools.Params{
				"Platform": "windows",
			},
			ResultParams: paramtools.Params{
				"name":        "cupertino.date_picker_test.datetime.drag",
				"source_type": "flutter",
			},
			Options: paramtools.Params{
				"ext": "png",
			},
		},
	}
}

func makeCirrusCIS() map[string]ci.Client {
	mcis := &mock_cis.Client{}
	mcis.On("GetTryJob", testutils.AnyContext, githubTJID).Return(makeGitHubCirrusTryJob(), nil)
	return map[string]ci.Client{
		cirrusCIS: mcis,
	}
}

func makeGitHubCRS() *mock_crs.Client {
	cl := code_review.ChangeList{
		SystemID: githubCLID,
		Owner:    "test@example.com",
		Status:   code_review.Open,
		Subject:  "initial commit",
		Updated:  time.Date(2019, time.November, 19, 18, 17, 16, 0, time.UTC),
	}

	xps := []code_review.PatchSet{
		{
			SystemID:     "fe1cad6c1a5d6dc7cea47f09efdd49f197a7f017",
			ChangeListID: githubCLID,
			Order:        githubPSOrder,
			GitHash:      "fe1cad6c1a5d6dc7cea47f09efdd49f197a7f017",
		},
	}
	mcrs := &mock_crs.Client{}
	mcrs.On("GetChangeList", testutils.AnyContext, githubCLID).Return(cl, nil)
	mcrs.On("GetPatchSets", testutils.AnyContext, githubCLID).Return(xps, nil)
	return mcrs
}
