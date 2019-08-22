package ingestion_processors

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	ingestion_mocks "go.skia.org/infra/go/ingestion/mocks"
	"go.skia.org/infra/go/sharedconfig"
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
)

func TestGerritBuildBucketFactory(t *testing.T) {
	unittest.SmallTest(t)

	config := &sharedconfig.IngesterConfig{
		ExtraParams: map[string]string{
			firestoreProjectIDParam: "should-use-emulator",
			firestoreNamespaceParam: "testing",

			codeReviewSystemParam: "gerrit",
			gerritURLParam:        "https://example-review.googlesource.com",

			continuousIntegrationSystemParam: "buildbucket",
			buildBucketNameParam:             "my.bucket.here",
		},
	}

	p, err := newGoldTryjobProcessor(nil, config, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, p)

	gtp, ok := p.(*goldTryjobProcessor)
	assert.True(t, ok)
	assert.NotNil(t, gtp.reviewClient)
	assert.NotNil(t, gtp.integrationClient)
}

// TestTryJobProcessFreshStartSunnyDay tests the scenario in which
// we see data uploaded for a brand new CL, PS, and TryJob.
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

	mcrs.On("GetChangeList", anyctx, sampleCLID).Return(makeChangeList(), nil)
	mcrs.On("GetPatchSets", anyctx, sampleCLID).Return(makePatchSets(), nil)

	mcls.On("GetChangeList", anyctx, sampleCLID).Return(code_review.ChangeList{}, clstore.NotFound)
	mcls.On("GetPatchSet", anyctx, sampleCLID, samplePSID).Return(code_review.PatchSet{}, clstore.NotFound)
	mcls.On("PutChangeList", anyctx, makeChangeList()).Return(nil)
	xps := makePatchSets()
	mcls.On("PutPatchSet", anyctx, sampleCLID, xps[0]).Return(nil)
	mcls.On("PutPatchSet", anyctx, sampleCLID, xps[1]).Return(nil)

	mcis.On("GetTryJob", anyctx, sampleTJID).Return(makeTryJob(), nil)

	mtjs.On("GetTryJob", anyctx, sampleTJID).Return(ci.TryJob{}, tjstore.NotFound)
	mtjs.On("PutTryJob", anyctx, sampleCombinedID, makeTryJob()).Return(nil)
	mtjs.On("PutResults", anyctx, sampleCombinedID, makeTryJobResults()).Return(nil)

	gtp := goldTryjobProcessor{
		reviewClient:      mcrs,
		integrationClient: mcis,
		changelistStore:   mcls,
		tryjobStore:       mtjs,
		crsName:           "gerrit",
		cisName:           "buildbucket",
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(legacyGoldCtlFile)
	assert.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	assert.NoError(t, err)
}

var (
	anyctx = mock.AnythingOfType("*context.emptyCtx")
)

// Below is the sample data that belongs to legacyGoldCtlFile
// It doesn't need to be a super complex example because we can have tests that
// test toTryJobResults directly, which should handle the more complex
// file types there.
const (
	sampleCLID = "1762193"
	samplePSID = "2"
	sampleTJID = "8904604368086838672"
)

var (
	sampleCombinedID = tjstore.CombinedPSID{CL: sampleCLID, PS: samplePSID, CRS: "gerrit"}
)

// These are functions to avoid mutations causing issues for future tests/checks
func makeTryJobResults() []tjstore.TryJobResult {
	return []tjstore.TryJobResult{
		{
			Digest: "690f72c0b56ae014c8ac66e7f25c0779",
			GroupParams: map[string]string{
				"device_id":     "0x1cb3",
				"device_string": "None",
				"model_name":    "",
				"msaa":          "True",
				"vendor_id":     "0x10de",
				"vendor_string": "None",
			},
			ResultParams: map[string]string{
				"name":        "Pixel_CanvasDisplayLinearRGBUnaccelerated2DGPUCompositing",
				"source_type": "chrome-gpu",
			},
			Options: map[string]string{
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
		Updated:  time.Date(2019, time.August, 19, 18, 17, 16, 0, time.UTC),
	}
}

func makePatchSets() []code_review.PatchSet {
	return []code_review.PatchSet{
		{
			SystemID:     "1",
			ChangeListID: "1762193",
			Order:        1,
			GitHash:      "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1",
			ParentHash:   "0000000000000000000000000000000000000000",
		},
		{
			SystemID:     "2",
			ChangeListID: "1762193",
			Order:        2,
			GitHash:      "e1681c90cf6a4c3b6be2bc4b4cea59849c16a438",
			ParentHash:   "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1",
		},
		{
			SystemID:     "3",
			ChangeListID: "1762193",
			Order:        3,
			GitHash:      "b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2",
			ParentHash:   "e1681c90cf6a4c3b6be2bc4b4cea59849c16a438",
		},
	}
}

func makeTryJob() ci.TryJob {
	return ci.TryJob{
		SystemID: "8904604368086838672",
		Name:     "iphone 7 test",
		Status:   ci.Running,
		Updated:  time.Date(2019, time.August, 19, 18, 20, 10, 0, time.UTC),
	}
}
