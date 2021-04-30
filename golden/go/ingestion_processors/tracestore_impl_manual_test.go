package ingestion_processors

import (
	"context"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/ingestion"
)

func TestLookupChromeOSMetadata_Success(t *testing.T) {
	unittest.ManualTest(t)

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	require.NoError(t, err)
	b := &btProcessor{
		source: &ingestion.GCSSource{
			Client: client,
		},
	}
	const testURL = "gs://chromeos-image-archive/sentry-release/R92-13944.0.0/manifest.xml"
	gitHash, err := b.lookupChromeOSMetadata(ctx, testURL)
	require.NoError(t, err)
	// This is the commit that the tast-tests repo was at in the given URL.
	assert.Equal(t, "f8eb6fa2e313974fe9b66d665793ba15ebe520a7", gitHash)
}
