package ingestion_processors

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/ingestion"
	ingestion_mocks "go.skia.org/infra/go/ingestion/mocks"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/clstore"
	mock_clstore "go.skia.org/infra/golden/go/clstore/mocks"
	"go.skia.org/infra/golden/go/code_review"
	mock_crs "go.skia.org/infra/golden/go/code_review/mocks"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	mock_cis "go.skia.org/infra/golden/go/continuous_integration/mocks"
	"go.skia.org/infra/golden/go/expectations"
	mock_expectations "go.skia.org/infra/golden/go/expectations/mocks"
	"go.skia.org/infra/golden/go/ignore"
	mock_ignorestore "go.skia.org/infra/golden/go/ignore/mocks"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/mocks"
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

	config := &sharedconfig.IngesterConfig{
		ExtraParams: map[string]string{
			firestoreProjectIDParam: "should-use-emulator",
			firestoreNamespaceParam: "testing",

			// This is a fictitious bucket and path.
			gcsKnownHashesParam: "example/hashes_files/gold-example-hashes.txt",

			codeReviewSystemParam: "gerrit",
			gerritURLParam:        "https://example-review.googlesource.com",

			continuousIntegrationSystemsParam: "buildbucket",
		},
	}

	p, err := newModularTryjobProcessor(context.Background(), nil, config, httputils.NewTimeoutClient())
	require.NoError(t, err)
	require.NotNil(t, p)

	gtp, ok := p.(*goldTryjobProcessor)
	require.True(t, ok)
	assert.NotNil(t, gtp.reviewClient)
	assert.Len(t, gtp.cisClients, 1)
	assert.Contains(t, gtp.cisClients, buildbucketCIS)
}

func TestGitHubCirrusBuildbucketFactory(t *testing.T) {
	unittest.LargeTest(t) // should use the emulator

	config := &sharedconfig.IngesterConfig{
		ExtraParams: map[string]string{
			firestoreProjectIDParam: "should-use-emulator",
			firestoreNamespaceParam: "testing",

			// This is a fictitious bucket and path.
			gcsKnownHashesParam: "example/hashes_files/gold-example-hashes.txt",

			codeReviewSystemParam:      "github",
			githubRepoParam:            "google/skia",
			githubCredentialsPathParam: "testdata/fake_token", // this is actually a file on disk.

			continuousIntegrationSystemsParam: "cirrus,buildbucket",
		},
	}

	p, err := newModularTryjobProcessor(context.Background(), nil, config, httputils.NewTimeoutClient())
	require.NoError(t, err)
	require.NotNil(t, p)

	gtp, ok := p.(*goldTryjobProcessor)
	require.True(t, ok)
	assert.NotNil(t, gtp.reviewClient)
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
	mcls.On("PutPatchSet", testutils.AnyContext, makeGerritPatchSet(false)).Return(nil).Once()

	mtjs.On("PutTryJob", testutils.AnyContext, gerritCombinedID, makeGerritBuildbucketTryJob()).Return(nil).Once()
	mtjs.On("PutResults", testutils.AnyContext, gerritCombinedID, gerritTJID, buildbucketCIS, makeTryJobResults()).Return(nil).Once()

	gtp := goldTryjobProcessor{
		changeListStore: mcls,
		crsName:         gerritCRS,
		expStore:        makeGerritExpectationsWithCL(gerritCLID, gerritCRS),
		gcsClient:       makeGCSClientWithoutMatchingDigests(t),
		ignoreStore:     makeEmptyIgnoreStore(),
		cisClients:      makeBuildbucketCIS(),
		reviewClient:    makeGerritCRS(),
		tryJobStore:     mtjs,
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(legacyGoldCtlFile)
	require.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	require.NoError(t, err)
}

// TestTryJobProcessFreshStartUntriaged tests the scenario in which we see data uploaded
// to Gerrit for a brand new CL, PS, and TryJob. Additionally, the tryjob result has a digest
// that has not been seen before (and is thus Untriaged).
func TestTryJobProcessFreshStartUntriaged(t *testing.T) {
	unittest.SmallTest(t)
	mcls := makeEmptyCLStore()
	mtjs := makeEmptyTJStore()
	// We want to assert that the Process calls each of PutChangeList, PutPatchSet, and PutTryJob
	// with the new, correct objects object. Further, it should call PutResults with the
	// appropriate TryJobResults.
	defer mcls.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)

	mcls.On("PutChangeList", testutils.AnyContext, clWithUpdatedTime(t, gerritCLID, gerritCLDate)).Return(nil).Once()
	mcls.On("PutPatchSet", testutils.AnyContext, makeGerritPatchSet(true)).Return(nil).Once()

	mtjs.On("PutTryJob", testutils.AnyContext, gerritCombinedID, makeGerritBuildbucketTryJob()).Return(nil).Once()
	mtjs.On("PutResults", testutils.AnyContext, gerritCombinedID, gerritTJID, buildbucketCIS, makeTryJobResults()).Return(nil).Once()

	gtp := goldTryjobProcessor{
		changeListStore: mcls,
		crsName:         gerritCRS,
		expStore:        makeEmptyExpectations(),
		gcsClient:       makeGCSClientWithoutMatchingDigests(t),
		ignoreStore:     makeEmptyIgnoreStore(),
		cisClients:      makeBuildbucketCIS(),
		reviewClient:    makeGerritCRS(),
		tryJobStore:     mtjs,
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
		SystemID:            githubPSID,
		ChangeListID:        githubCLID,
		Order:               githubPSOrder,
		GitHash:             githubPSID,
		HasUntriagedDigests: true,
	}).Return(nil).Once()

	combinedID := tjstore.CombinedPSID{CL: githubCLID, PS: githubPSID, CRS: "github"}
	mtjs.On("PutTryJob", testutils.AnyContext, combinedID, makeGitHubCirrusTryJob()).Return(nil)
	mtjs.On("PutResults", testutils.AnyContext, combinedID, githubTJID, cirrusCIS, makeGitHubTryJobResults()).Return(nil)

	gtp := goldTryjobProcessor{
		changeListStore: mcls,
		crsName:         githubCRS,
		expStore:        makeEmptyExpectations(),
		gcsClient:       makeGCSClientWithoutMatchingDigests(t),
		ignoreStore:     makeEmptyIgnoreStore(),
		cisClients:      makeCirrusCIS(),
		reviewClient:    makeGitHubCRS(),
		tryJobStore:     mtjs,
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
		changeListStore: mcls,
		crsName:         githubCRS,
		cisClients:      errorfulCISClients,
		tryJobStore:     makeEmptyTJStore(),
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
	mcls.On("PutPatchSet", testutils.AnyContext, makeGerritPatchSet(false)).Return(nil)
	mcls.On("PutChangeList", testutils.AnyContext, clWithUpdatedTime(t, gerritCLID, gerritCLDate)).Return(nil)

	mtjs.On("PutTryJob", testutils.AnyContext, gerritCombinedID, makeGerritBuildbucketTryJob()).Return(nil)
	mtjs.On("PutResults", testutils.AnyContext, gerritCombinedID, gerritTJID, buildbucketCIS, makeTryJobResults()).Return(nil)

	gtp := goldTryjobProcessor{
		changeListStore: mcls,
		crsName:         gerritCRS,
		expStore:        makeGerritExpectationsWithCL(gerritCLID, gerritCRS),
		gcsClient:       makeGCSClientWithoutMatchingDigests(t),
		ignoreStore:     makeEmptyIgnoreStore(),
		cisClients:      makeBuildbucketCIS(),
		reviewClient:    makeGerritCRS(),
		tryJobStore:     mtjs,
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
	mcls.On("PutPatchSet", testutils.AnyContext, makeGerritPatchSet(false)).Return(nil)
	mcls.On("PutChangeList", testutils.AnyContext, clWithUpdatedTime(t, gerritCLID, gerritCLDate)).Return(nil)

	mtjs.On("PutTryJob", testutils.AnyContext, gerritCombinedID, makeGerritBuildbucketTryJob()).Return(nil)
	mtjs.On("PutResults", testutils.AnyContext, gerritCombinedID, gerritTJID, buildbucketCIS, makeTryJobResults()).Return(nil)

	gtp := goldTryjobProcessor{
		changeListStore: mcls,
		crsName:         gerritCRS,
		expStore:        makeGerritExpectationsWithCL(gerritCLID, gerritCRS),
		gcsClient:       makeGCSClientWithoutMatchingDigests(t),
		ignoreStore:     makeEmptyIgnoreStore(),
		cisClients:      makeBuildbucketCIS(),
		reviewClient:    makeGerritCRS(),
		tryJobStore:     mtjs,
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
	mcls.On("GetPatchSetByOrder", testutils.AnyContext, gerritCLID, gerritPSOrder).Return(makeGerritPatchSet(false), nil)
	mcls.On("PutChangeList", testutils.AnyContext, clWithUpdatedTime(t, gerritCLID, gerritCLDate)).Return(nil)
	mcls.On("PutPatchSet", testutils.AnyContext, makeGerritPatchSet(false)).Return(nil)

	mtjs.On("PutTryJob", testutils.AnyContext, gerritCombinedID, makeGerritBuildbucketTryJob()).Return(nil)
	mtjs.On("PutResults", testutils.AnyContext, gerritCombinedID, gerritTJID, buildbucketCIS, makeTryJobResults()).Return(nil)

	gtp := goldTryjobProcessor{
		changeListStore: mcls,
		crsName:         gerritCRS,
		expStore:        makeGerritExpectationsWithCL(gerritCLID, gerritCRS),
		gcsClient:       makeGCSClientWithoutMatchingDigests(t),
		ignoreStore:     makeEmptyIgnoreStore(),
		cisClients:      makeBuildbucketCIS(),
		tryJobStore:     mtjs,
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(legacyGoldCtlFile)
	require.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	require.NoError(t, err)
}

// TestTryJobProcess_IngestedResultAlreadyTriagedPositive tests the case that an ingested result is
// part of a TryJob that is in the tjstore and a ChangeList that is in clstore. This result
// is already triaged (marked positive), so we should expect to see the PatchSet gets "touched"
// (i.e. Put into the store, which updates the timestamp), but not marked as having Untriaged
// digests.
func TestTryJobProcess_IngestedResultAlreadyTriagedPositive(t *testing.T) {
	unittest.SmallTest(t)
	mcls := &mock_clstore.Store{}
	mtjs := &mock_tjstore.Store{}
	// We want to assert that the Process calls PutPatchSet with HasUntriagedDigests = false,
	// and PutResults with the appropriate TryJobResults
	defer mcls.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)

	mcls.On("GetChangeList", testutils.AnyContext, gerritCLID).Return(makeChangeList(), nil)
	mcls.On("GetPatchSetByOrder", testutils.AnyContext, gerritCLID, gerritPSOrder).Return(makeGerritPatchSet(false), nil)
	mcls.On("PutPatchSet", testutils.AnyContext, makeGerritPatchSet(false /* = hasUntriagedDigests*/)).Return(nil)

	mtjs.On("GetTryJob", testutils.AnyContext, gerritTJID, buildbucketCIS).Return(makeGerritBuildbucketTryJob(), nil)
	mtjs.On("PutResults", testutils.AnyContext, gerritCombinedID, gerritTJID, buildbucketCIS, makeTryJobResults()).Return(nil)

	gtp := goldTryjobProcessor{
		changeListStore: mcls,
		tryJobStore:     mtjs,
		gcsClient:       makeGCSClientWithoutMatchingDigests(t),
		ignoreStore:     makeEmptyIgnoreStore(),
		expStore:        makeGerritExpectationsWithCL(gerritCLID, gerritCRS),
		cisClients:      makeBuildbucketCIS(),
		crsName:         gerritCRS,
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(legacyGoldCtlFile)
	require.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	require.NoError(t, err)
}

// TestTryJobProcess_IngestedResultAlreadyUntriagedOnMaster tests the cases that an ingested result
// is part of a TryJob that is in the tjstore and a ChangeList that is in clstore. This result
// is already on master, but untriaged, so we should expect to see the PatchSet gets "touched"
// (i.e. Put into the store, which updates the timestamp), but not marked as having Untriaged
// digests. The reason we don't mark this CL/PS as having Untriaged digests is because it is not
// the CL's fault the untriaged digest is there, it was pre-existing.
func TestTryJobProcess_IngestedResultAlreadyUntriagedOnMaster(t *testing.T) {
	unittest.SmallTest(t)
	mcls := &mock_clstore.Store{}
	mtjs := &mock_tjstore.Store{}
	// We want to assert that the Process calls PutPatchSet with HasUntriagedDigests = false,
	// and PutResults with the appropriate TryJobResults
	defer mcls.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)

	mcls.On("GetChangeList", testutils.AnyContext, gerritCLID).Return(makeChangeList(), nil)
	mcls.On("GetPatchSetByOrder", testutils.AnyContext, gerritCLID, gerritPSOrder).Return(makeGerritPatchSet(false), nil)
	mcls.On("PutPatchSet", testutils.AnyContext, makeGerritPatchSet(false /* = hasUntriagedDigests*/)).Return(nil)

	mtjs.On("GetTryJob", testutils.AnyContext, gerritTJID, buildbucketCIS).Return(makeGerritBuildbucketTryJob(), nil)
	mtjs.On("PutResults", testutils.AnyContext, gerritCombinedID, gerritTJID, buildbucketCIS, makeTryJobResults()).Return(nil)

	gtp := goldTryjobProcessor{
		changeListStore: mcls,
		tryJobStore:     mtjs,
		gcsClient:       makeGCSClientWithGerritDigest(t),
		ignoreStore:     makeEmptyIgnoreStore(),
		expStore:        makeEmptyExpectations(),
		cisClients:      makeBuildbucketCIS(),
		crsName:         gerritCRS,
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(legacyGoldCtlFile)
	require.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	require.NoError(t, err)
}

// TestTryJobProcess_IngestedResultIgnored tests the cases that an ingested result is part of a
// TryJob that is in the tjstore and a ChangeList that is in clstore. This result is Untriaged, but
// matches one of the ignore rules, so we should expect to see the PatchSet gets "touched"
// (i.e. Put into the store, which updates the timestamp), but not marked as having Untriaged
// digests.
func TestTryJobProcess_IngestedResultIgnored(t *testing.T) {
	unittest.SmallTest(t)
	mcls := &mock_clstore.Store{}
	mtjs := &mock_tjstore.Store{}
	// We want to assert that the Process calls PutPatchSet with HasUntriagedDigests = false,
	// and PutResults with the appropriate TryJobResults
	defer mcls.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)

	mcls.On("GetChangeList", testutils.AnyContext, gerritCLID).Return(makeChangeList(), nil)
	mcls.On("GetPatchSetByOrder", testutils.AnyContext, gerritCLID, gerritPSOrder).Return(makeGerritPatchSet(false), nil)
	mcls.On("PutPatchSet", testutils.AnyContext, makeGerritPatchSet(false /* = hasUntriagedDigests*/)).Return(nil)

	mtjs.On("GetTryJob", testutils.AnyContext, gerritTJID, buildbucketCIS).Return(makeGerritBuildbucketTryJob(), nil)
	mtjs.On("PutResults", testutils.AnyContext, gerritCombinedID, gerritTJID, buildbucketCIS, makeTryJobResults()).Return(nil)

	gtp := goldTryjobProcessor{
		changeListStore: mcls,
		tryJobStore:     mtjs,
		gcsClient:       makeGCSClientWithoutMatchingDigests(t),
		ignoreStore:     makeIgnoreStoreWhichIgnoresGerritTrace(),
		expStore:        makeEmptyExpectations(),
		cisClients:      makeBuildbucketCIS(),
		crsName:         gerritCRS,
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(legacyGoldCtlFile)
	require.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	require.NoError(t, err)
}

// TestTryJobProcess_CLIntroducedNewUntriagedDigest tests the cases that an ingested result is
// part of a TryJob that is in the tjstore and a ChangeList that is in clstore. This result is
// Untriaged and 1) was not already on master and 2) does not match any ignore rules, so we
// say that the CL is responsible for this Untriaged digest and update the corresponding
// PatchSet in clstore to reflect this.
func TestTryJobProcess_CLIntroducedNewUntriagedDigest(t *testing.T) {
	unittest.SmallTest(t)
	mcls := &mock_clstore.Store{}
	mtjs := &mock_tjstore.Store{}
	// We want to assert that the Process calls PutPatchSet with HasUntriagedDigests = true,
	// and PutResults with the appropriate TryJobResults
	defer mcls.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)

	mcls.On("GetChangeList", testutils.AnyContext, gerritCLID).Return(makeChangeList(), nil)
	mcls.On("GetPatchSetByOrder", testutils.AnyContext, gerritCLID, gerritPSOrder).Return(makeGerritPatchSet(false), nil)
	mcls.On("PutPatchSet", testutils.AnyContext, makeGerritPatchSet(true /* = hasUntriagedDigests*/)).Return(nil)

	mtjs.On("GetTryJob", testutils.AnyContext, gerritTJID, buildbucketCIS).Return(makeGerritBuildbucketTryJob(), nil)
	mtjs.On("PutResults", testutils.AnyContext, gerritCombinedID, gerritTJID, buildbucketCIS, makeTryJobResults()).Return(nil)

	gtp := goldTryjobProcessor{
		changeListStore: mcls,
		tryJobStore:     mtjs,
		gcsClient:       makeGCSClientWithoutMatchingDigests(t),
		ignoreStore:     makeEmptyIgnoreStore(),
		expStore:        makeEmptyExpectations(),
		cisClients:      makeBuildbucketCIS(),
		crsName:         gerritCRS,
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(legacyGoldCtlFile)
	require.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	require.NoError(t, err)
}

// TestTryJobProcess_ExpectationStoreFailure makes sure we don't ingest a set of results if we
// cannot fetch the expectations for a given CL.
func TestTryJobProcess_ExpectationStoreFailure(t *testing.T) {
	unittest.SmallTest(t)
	mcls := &mock_clstore.Store{}
	mtjs := &mock_tjstore.Store{}
	mes := &mock_expectations.Store{}
	failingExpStore := &mock_expectations.Store{}

	mcls.On("GetChangeList", testutils.AnyContext, gerritCLID).Return(makeChangeList(), nil)
	mcls.On("GetPatchSetByOrder", testutils.AnyContext, gerritCLID, gerritPSOrder).Return(makeGerritPatchSet(false), nil)

	mtjs.On("GetTryJob", testutils.AnyContext, gerritTJID, buildbucketCIS).Return(makeGerritBuildbucketTryJob(), nil)

	failingExpStore.On("Get", testutils.AnyContext).Return(nil, errors.New("broken expstore"))
	mes.On("ForChangeList", mock.Anything, mock.Anything).Return(failingExpStore)

	gtp := goldTryjobProcessor{
		changeListStore: mcls,
		tryJobStore:     mtjs,
		gcsClient:       makeGCSClientWithoutMatchingDigests(t),
		ignoreStore:     makeEmptyIgnoreStore(),
		cisClients:      makeBuildbucketCIS(),
		expStore:        mes,
		crsName:         gerritCRS,
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(legacyGoldCtlFile)
	require.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken expstore")
}

// TestTryJobProcess_GCSClientFailure makes sure we don't ingest a set of results if we cannot
// fetch the known digests from GCS.
func TestTryJobProcess_GCSClientFailure(t *testing.T) {
	unittest.SmallTest(t)
	mcls := &mock_clstore.Store{}
	mtjs := &mock_tjstore.Store{}
	failingGCSClient := &mocks.GCSClient{}

	mcls.On("GetChangeList", testutils.AnyContext, gerritCLID).Return(makeChangeList(), nil)
	mcls.On("GetPatchSetByOrder", testutils.AnyContext, gerritCLID, gerritPSOrder).Return(makeGerritPatchSet(false), nil)

	mtjs.On("GetTryJob", testutils.AnyContext, gerritTJID, buildbucketCIS).Return(makeGerritBuildbucketTryJob(), nil)

	failingGCSClient.On("LoadKnownDigests", testutils.AnyContext, mock.Anything).Return(errors.New("gcs offline"))

	gtp := goldTryjobProcessor{
		changeListStore: mcls,
		tryJobStore:     mtjs,
		gcsClient:       failingGCSClient,
		ignoreStore:     makeEmptyIgnoreStore(),
		expStore:        makeEmptyExpectations(),
		cisClients:      makeBuildbucketCIS(),
		crsName:         gerritCRS,
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(legacyGoldCtlFile)
	require.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gcs offline")
}

// TestTryJobProcess_IgnoreStoreFailure makes sure we don't ingest a set of results if we cannot
// fetch the current ignore rules.
func TestTryJobProcess_IgnoreStoreFailure(t *testing.T) {
	unittest.SmallTest(t)
	mcls := &mock_clstore.Store{}
	mtjs := &mock_tjstore.Store{}
	failingIgnoreStore := &mock_ignorestore.Store{}

	mcls.On("GetChangeList", testutils.AnyContext, gerritCLID).Return(makeChangeList(), nil)
	mcls.On("GetPatchSetByOrder", testutils.AnyContext, gerritCLID, gerritPSOrder).Return(makeGerritPatchSet(false), nil)

	mtjs.On("GetTryJob", testutils.AnyContext, gerritTJID, buildbucketCIS).Return(makeGerritBuildbucketTryJob(), nil)

	failingIgnoreStore.On("List", testutils.AnyContext).Return(nil, errors.New("network down"))

	gtp := goldTryjobProcessor{
		changeListStore: mcls,
		tryJobStore:     mtjs,
		gcsClient:       makeGCSClientWithoutMatchingDigests(t),
		ignoreStore:     failingIgnoreStore,
		expStore:        makeEmptyExpectations(),
		cisClients:      makeBuildbucketCIS(),
		crsName:         gerritCRS,
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(legacyGoldCtlFile)
	require.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "network down")
}

// makeEmptyExpectations returns a series of expectations.Store that has everything be untriaged.
func makeEmptyExpectations() *mock_expectations.Store {
	mes := &mock_expectations.Store{}
	issueStore := &mock_expectations.Store{}
	mes.On("ForChangeList", mock.Anything, mock.Anything).Return(issueStore, nil).Maybe()
	var ie expectations.Expectations
	issueStore.On("Get", testutils.AnyContext).Return(&ie, nil)
	var e expectations.Expectations
	mes.On("Get", testutils.AnyContext).Return(&e, nil)
	return mes
}

func makeGCSClientWithoutMatchingDigests(t *testing.T) *mocks.GCSClient {
	const randomDigests = `0cc175b9c0f1b6a831c399e269772661
92eb5ffee6ae2fec3ad71c777531578f
4a8a08f09d37b73795649038408b5f33
`
	mgc := &mocks.GCSClient{}
	mgc.On("LoadKnownDigests", testutils.AnyContext, mock.Anything).Run(func(args mock.Arguments) {
		w := args.Get(1).(io.Writer)
		_, err := w.Write([]byte(randomDigests))
		assert.NoError(t, err)
	}).Return(nil)
	return mgc
}

func makeEmptyIgnoreStore() *mock_ignorestore.Store {
	mis := &mock_ignorestore.Store{}
	mis.On("List", testutils.AnyContext).Return(nil, nil)
	return mis
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
	gerritCLID     = "1762193"
	gerritPSID     = "e1681c90cf6a4c3b6be2bc4b4cea59849c16a438"
	gerritPSOrder  = 2
	gerritTJID     = "8904604368086838672"
	gerritDigest   = "690f72c0b56ae014c8ac66e7f25c0779"
	gerritTestName = "Pixel_CanvasDisplayLinearRGBUnaccelerated2DGPUCompositing"
)

var (
	gerritCombinedID = tjstore.CombinedPSID{CL: gerritCLID, PS: gerritPSID, CRS: gerritCRS}

	gerritCLDate = time.Date(2019, time.August, 19, 18, 17, 16, 0, time.UTC)
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

func makeGerritPatchSet(hasUntriagedDigests bool) code_review.PatchSet {
	ps := makeGerritPatchSets()[1]
	ps.HasUntriagedDigests = hasUntriagedDigests
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

// makeGerritExpectationsWithCL returns a series of expectations.Store that make the gerritTestName
// marked as positive.
func makeGerritExpectationsWithCL(clID, crs string) *mock_expectations.Store {
	mes := &mock_expectations.Store{}
	issueStore := &mock_expectations.Store{}
	mes.On("ForChangeList", clID, crs).Return(issueStore, nil)
	var ie expectations.Expectations
	issueStore.On("Get", testutils.AnyContext).Return(&ie, nil)
	var e expectations.Expectations
	e.Set(gerritTestName, gerritDigest, expectations.Positive)
	mes.On("Get", testutils.AnyContext).Return(&e, nil)
	return mes
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

func makeGCSClientWithGerritDigest(t *testing.T) *mocks.GCSClient {
	// The last digest in this list is gerritDigest
	const randomDigests = `0cc175b9c0f1b6a831c399e269772661
92eb5ffee6ae2fec3ad71c777531578f
4a8a08f09d37b73795649038408b5f33
690f72c0b56ae014c8ac66e7f25c0779
`
	mgc := &mocks.GCSClient{}
	mgc.On("LoadKnownDigests", testutils.AnyContext, mock.Anything).Run(func(args mock.Arguments) {
		w := args.Get(1).(io.Writer)
		_, err := w.Write([]byte(randomDigests))
		assert.NoError(t, err)
	}).Return(nil)
	return mgc
}

func makeIgnoreStoreWhichIgnoresGerritTrace() *mock_ignorestore.Store {
	mis := &mock_ignorestore.Store{}
	mis.On("List", testutils.AnyContext).Return([]ignore.Rule{
		{
			ID:        "abc123123",
			CreatedBy: "user@example.com",
			UpdatedBy: "admin@example.com",
			// This time doesn't matter, we should apply the ignore even if it's expired.
			Expires: time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC),
			Query:   "device_id=0x1cb3",
			Note:    "This query will match the legacy-tryjob-goldctl.json file",
		},
	}, nil)
	return mis
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
