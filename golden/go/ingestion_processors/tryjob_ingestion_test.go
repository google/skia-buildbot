package ingestion_processors

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	ingestion_mocks "go.skia.org/infra/go/ingestion/mocks"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/testutils/unittest"
	mockclstore "go.skia.org/infra/golden/go/clstore/mocks"
	mockcrs "go.skia.org/infra/golden/go/code_review/mocks"
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

func TestTryJobProcessSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mcls := &mockclstore.Store{}
	mtjs := &mocktjstore.Store{}
	mcrs := &mockcrs.Client{}
	mcis := &mockcis.Client{}
	defer mcls.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)
	defer mcrs.AssertExpectations(t)
	defer mcis.AssertExpectations(t)

	expectedTryJobResults := []tjstore.TryJobResult{
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
	expectedCLID := "1762193"
	expectedPSID := "2"
	expectedCombinedID := tjstore.MakeCombinedID(expectedCLID, expectedPSID, "gerrit")

	mtjs.On("PutResults", anyctx, expectedCombinedID, expectedTryJobResults).Return(nil)

	gtp := goldTryjobProcessor{
		reviewClient:      mcrs,
		integrationClient: mcis,
		changelistStore:   mcls,
		tryjobStore:       mtjs,
	}

	fsResult, err := ingestion_mocks.MockResultFileLocationFromFile(legacyGoldCtlFile)
	assert.NoError(t, err)

	err = gtp.Process(context.Background(), fsResult)
	assert.NoError(t, err)
}

var anyctx = mock.AnythingOfType("*context.emptyCtx")
