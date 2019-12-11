package ingestion_processors

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	ingestion_mocks "go.skia.org/infra/go/ingestion/mocks"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/clstore"
	mockclstore "go.skia.org/infra/golden/go/clstore/mocks"
	"go.skia.org/infra/golden/go/code_review"
	mockcrs "go.skia.org/infra/golden/go/code_review/mocks"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	mockcis "go.skia.org/infra/golden/go/continuous_integration/mocks"
	"go.skia.org/infra/golden/go/tjstore"
	mocktjstore "go.skia.org/infra/golden/go/tjstore/mocks"
)

const (
	legacyGoldCtlFile = "testdata/legacy-tryjob-goldctl.json"
	githubGoldCtlFile = "testdata/github-goldctl.json"
)

func TestGerritBuildBucketFactory(t *testing.T) {
	unittest.LargeTest(t) // should use the emulator

	config := &sharedconfig.IngesterConfig{
		ExtraParams: map[string]string{
			firestoreProjectIDParam: "should-use-emulator",
			firestoreNamespaceParam: "testing",

			codeReviewSystemParam: "gerrit",
			gerritURLParam:        "https://example-review.googlesource.com",

			continuousIntegrationSystemParam: "buildbucket",
		},
	}

	p, err := newModularTryjobProcessor(context.Background(), nil, config, nil)
	require.NoError(t, err)
	require.NotNil(t, p)

	gtp, ok := p.(*goldTryjobProcessor)
	require.True(t, ok)
	require.NotNil(t, gtp.reviewClient)
	require.NotNil(t, gtp.integrationClient)
}

func TestGitHubCirrusFactory(t *testing.T) {
	unittest.SmallTest(t)

	config := &sharedconfig.IngesterConfig{
		ExtraParams: map[string]string{
			firestoreProjectIDParam: "should-use-emulator",
			firestoreNamespaceParam: "testing",

			codeReviewSystemParam:      "github",
			githubRepoParam:            "google/skia",
			githubCredentialsPathParam: "testdata/fake_token", // this is actually a file on disk.

			continuousIntegrationSystemParam: "cirrus",
		},
	}

	p, err := newModularTryjobProcessor(context.Background(), nil, config, nil)
	require.NoError(t, err)
	require.NotNil(t, p)

	gtp, ok := p.(*goldTryjobProcessor)
	require.True(t, ok)
	require.NotNil(t, gtp.reviewClient)
	require.NotNil(t, gtp.integrationClient)
}

// TestTryJobProcessFreshStartSunnyDay tests the scenario in which
// we see data uploaded to Gerrit for a brand new CL, PS, and TryJob.
func TestTryJobProcessFreshStartSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mcls := &mockclstore.Store{}
	mtjs := &mocktjstore.Store{}
	mcrs := &mockcrs.Client{}
	mcis := &mockcis.Client{}
	defer mcls.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)
	defer mcrs.AssertExpectations(t)
	defer mcis.AssertExpectations(t)

	mcrs.On("GetChangeList", testutils.AnyContext, sampleCLID).Return(makeChangeList(), nil)
	mcrs.On("GetPatchSets", testutils.AnyContext, sampleCLID).Return(makePatchSets(), nil)

	mcls.On("GetChangeList", testutils.AnyContext, sampleCLID).Return(code_review.ChangeList{}, clstore.ErrNotFound)
	mcls.On("GetPatchSetByOrder", testutils.AnyContext, sampleCLID, samplePSOrder).Return(code_review.PatchSet{}, clstore.ErrNotFound)
	mcls.On("PutChangeList", testutils.AnyContext, clWithUpdatedTime(t, sampleCLID, sampleCLDate)).Return(nil)
	xps := makePatchSets()
	mcls.On("PutPatchSet", testutils.AnyContext, xps[1]).Return(nil)

	mcis.On("GetTryJob", testutils.AnyContext, sampleTJID).Return(makeTryJob(), nil)

	mtjs.On("GetTryJob", testutils.AnyContext, sampleTJID).Return(ci.TryJob{}, tjstore.ErrNotFound)
	mtjs.On("PutTryJob", testutils.AnyContext, sampleCombinedID, makeTryJob()).Return(nil)
	mtjs.On("PutResults", testutils.AnyContext, sampleCombinedID, sampleTJID, makeTryJobResults()).Return(nil)

	gtp := goldTryjobProcessor{
		reviewClient:      mcrs,
		integrationClient: mcis,
		changeListStore:   mcls,
		tryJobStore:       mtjs,
		crsName:           "gerrit",
		cisName:           "buildbucket",
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(legacyGoldCtlFile)
	require.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	require.NoError(t, err)
}

// TestTryJobProcessFreshStartGitHub tests the scenario in which
// we see data uploaded to GitHub for a brand new CL, PS, and TryJob. The PS is derived by id, not
// by order.
func TestTryJobProcessFreshStartGitHub(t *testing.T) {
	unittest.SmallTest(t)

	const clID = "44474"
	const psOrder = 1
	const psID = "fe1cad6c1a5d6dc7cea47f09efdd49f197a7f017"
	const tjID = "5489081055707136"

	mcls := &mockclstore.Store{}
	mtjs := &mocktjstore.Store{}
	mcrs := &mockcrs.Client{}
	mcis := &mockcis.Client{}
	defer mcls.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)
	defer mcrs.AssertExpectations(t)
	defer mcis.AssertExpectations(t)

	cl := code_review.ChangeList{
		SystemID: clID,
		Owner:    "test@example.com",
		Status:   code_review.Open,
		Subject:  "initial commit",
		Updated:  time.Date(2019, time.November, 19, 18, 17, 16, 0, time.UTC),
	}

	xps := []code_review.PatchSet{
		{
			SystemID:     "fe1cad6c1a5d6dc7cea47f09efdd49f197a7f017",
			ChangeListID: clID,
			Order:        psOrder,
			GitHash:      "fe1cad6c1a5d6dc7cea47f09efdd49f197a7f017",
		},
	}

	combinedID := tjstore.CombinedPSID{CL: clID, PS: psID, CRS: "github"}

	originalDate := time.Date(2019, time.November, 19, 18, 20, 10, 0, time.UTC)
	tj := ci.TryJob{
		SystemID:    tjID,
		DisplayName: "my-task",
		Updated:     originalDate,
	}

	xtjr := []tjstore.TryJobResult{
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

	mcrs.On("GetChangeList", testutils.AnyContext, clID).Return(cl, nil)
	mcrs.On("GetPatchSets", testutils.AnyContext, clID).Return(xps, nil)

	mcls.On("GetChangeList", testutils.AnyContext, clID).Return(code_review.ChangeList{}, clstore.ErrNotFound)
	mcls.On("GetPatchSet", testutils.AnyContext, clID, psID).Return(code_review.PatchSet{}, clstore.ErrNotFound)
	mcls.On("PutChangeList", testutils.AnyContext, clWithUpdatedTime(t, clID, originalDate)).Return(nil)
	mcls.On("PutPatchSet", testutils.AnyContext, xps[psOrder-1]).Return(nil)

	mcis.On("GetTryJob", testutils.AnyContext, tjID).Return(tj, nil)

	mtjs.On("GetTryJob", testutils.AnyContext, tjID).Return(ci.TryJob{}, tjstore.ErrNotFound)
	mtjs.On("PutTryJob", testutils.AnyContext, combinedID, tj).Return(nil)
	mtjs.On("PutResults", testutils.AnyContext, combinedID, tjID, xtjr).Return(nil)

	gtp := goldTryjobProcessor{
		reviewClient:      mcrs,
		integrationClient: mcis,
		changeListStore:   mcls,
		tryJobStore:       mtjs,
		crsName:           "github",
		cisName:           "cirrus",
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(githubGoldCtlFile)
	require.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	require.NoError(t, err)
}

// TestTryJobProcessCLExistsSunnyDay tests that the ingestion works when the
// CL already exists.
func TestTryJobProcessCLExistsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mcls := &mockclstore.Store{}
	mtjs := &mocktjstore.Store{}
	mcrs := &mockcrs.Client{}
	mcis := &mockcis.Client{}
	defer mcls.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)
	defer mcrs.AssertExpectations(t)
	defer mcis.AssertExpectations(t)

	mcrs.On("GetPatchSets", testutils.AnyContext, sampleCLID).Return(makePatchSets(), nil)

	mcls.On("GetChangeList", testutils.AnyContext, sampleCLID).Return(makeChangeList(), nil)
	mcls.On("GetPatchSetByOrder", testutils.AnyContext, sampleCLID, samplePSOrder).Return(code_review.PatchSet{}, clstore.ErrNotFound)
	xps := makePatchSets()
	mcls.On("PutPatchSet", testutils.AnyContext, xps[1]).Return(nil)
	mcls.On("PutChangeList", testutils.AnyContext, clWithUpdatedTime(t, sampleCLID, sampleCLDate)).Return(nil)

	mcis.On("GetTryJob", testutils.AnyContext, sampleTJID).Return(makeTryJob(), nil)

	mtjs.On("GetTryJob", testutils.AnyContext, sampleTJID).Return(ci.TryJob{}, tjstore.ErrNotFound)
	mtjs.On("PutTryJob", testutils.AnyContext, sampleCombinedID, makeTryJob()).Return(nil)
	mtjs.On("PutResults", testutils.AnyContext, sampleCombinedID, sampleTJID, makeTryJobResults()).Return(nil)

	gtp := goldTryjobProcessor{
		reviewClient:      mcrs,
		integrationClient: mcis,
		changeListStore:   mcls,
		tryJobStore:       mtjs,
		crsName:           "gerrit",
		cisName:           "buildbucket",
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

	mcls := &mockclstore.Store{}
	mtjs := &mocktjstore.Store{}
	mcrs := &mockcrs.Client{}
	mcis := &mockcis.Client{}
	defer mcls.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)
	defer mcrs.AssertExpectations(t)
	defer mcis.AssertExpectations(t)

	mcls.On("GetChangeList", testutils.AnyContext, sampleCLID).Return(makeChangeList(), nil)
	xps := makePatchSets()
	mcls.On("GetPatchSetByOrder", testutils.AnyContext, sampleCLID, samplePSOrder).Return(xps[1], nil)
	mcls.On("PutChangeList", testutils.AnyContext, clWithUpdatedTime(t, sampleCLID, sampleCLDate)).Return(nil)

	mcis.On("GetTryJob", testutils.AnyContext, sampleTJID).Return(makeTryJob(), nil)

	mtjs.On("GetTryJob", testutils.AnyContext, sampleTJID).Return(ci.TryJob{}, tjstore.ErrNotFound)
	mtjs.On("PutTryJob", testutils.AnyContext, sampleCombinedID, makeTryJob()).Return(nil)
	mtjs.On("PutResults", testutils.AnyContext, sampleCombinedID, sampleTJID, makeTryJobResults()).Return(nil)

	gtp := goldTryjobProcessor{
		reviewClient:      mcrs,
		integrationClient: mcis,
		changeListStore:   mcls,
		tryJobStore:       mtjs,
		crsName:           "gerrit",
		cisName:           "buildbucket",
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(legacyGoldCtlFile)
	require.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	require.NoError(t, err)
}

// TestTryJobProcessTJExistsSunnyDay tests that the ingestion works when the
// CL, PS and TryJob already exists.
func TestTryJobProcessTJExistsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mcls := &mockclstore.Store{}
	mtjs := &mocktjstore.Store{}
	mcrs := &mockcrs.Client{}
	mcis := &mockcis.Client{}
	defer mcls.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)
	defer mcrs.AssertExpectations(t)
	defer mcis.AssertExpectations(t)

	mcls.On("GetChangeList", testutils.AnyContext, sampleCLID).Return(makeChangeList(), nil)
	xps := makePatchSets()
	mcls.On("GetPatchSetByOrder", testutils.AnyContext, sampleCLID, samplePSOrder).Return(xps[1], nil)

	mtjs.On("GetTryJob", testutils.AnyContext, sampleTJID).Return(makeTryJob(), nil)
	mtjs.On("PutResults", testutils.AnyContext, sampleCombinedID, sampleTJID, makeTryJobResults()).Return(nil)

	gtp := goldTryjobProcessor{
		reviewClient:      mcrs,
		integrationClient: mcis,
		changeListStore:   mcls,
		tryJobStore:       mtjs,
		crsName:           "gerrit",
		cisName:           "buildbucket",
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(legacyGoldCtlFile)
	require.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	require.NoError(t, err)
}

// Below is the sample data that belongs to legacyGoldCtlFile
// It doesn't need to be a super complex example because we have tests that
// test parseDMResults directly.
const (
	sampleCLID    = "1762193"
	samplePSID    = "e1681c90cf6a4c3b6be2bc4b4cea59849c16a438"
	samplePSOrder = 2
	sampleTJID    = "8904604368086838672"
)

var (
	sampleCombinedID = tjstore.CombinedPSID{CL: sampleCLID, PS: samplePSID, CRS: "gerrit"}

	sampleCLDate = time.Date(2019, time.August, 19, 18, 17, 16, 0, time.UTC)
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
		Updated:  sampleCLDate,
	}
}

// clWithUpdatedTime returns a matcher that will assert the CL has properly had its Updated field
// updated.
func clWithUpdatedTime(t *testing.T, clID string, originalDate time.Time) interface{} {
	return mock.MatchedBy(func(cl code_review.ChangeList) bool {
		assert.Equal(t, clID, cl.SystemID)
		// Make sure the time is updated to be later than the original one (which was in November
		// or August, depending on the testcase). Since this test was authored after 1 Dec 2019 and
		// the Updated is set to time.Now(), we can just check that we are after then.
		assert.True(t, cl.Updated.After(originalDate))
		// assert messages are easier to debug than "not matched" errors, so say that we matched,
		// but know the test will fail if any of the above asserts fail.
		return true
	})
}

func makePatchSets() []code_review.PatchSet {
	return []code_review.PatchSet{
		{
			SystemID:     "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1",
			ChangeListID: "1762193",
			Order:        1,
			GitHash:      "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1",
		},
		{
			SystemID:     samplePSID,
			ChangeListID: "1762193",
			Order:        samplePSOrder,
			GitHash:      samplePSID,
		},
		{
			SystemID:     "b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2",
			ChangeListID: "1762193",
			Order:        3,
			GitHash:      "b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2",
		},
	}
}

func makeTryJob() ci.TryJob {
	return ci.TryJob{
		SystemID:    "8904604368086838672",
		DisplayName: "my-task",
		Updated:     time.Date(2019, time.August, 19, 18, 20, 10, 0, time.UTC),
	}
}
