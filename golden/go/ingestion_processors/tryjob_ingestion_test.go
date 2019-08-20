package ingestion_processors

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/testutils/unittest"
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
