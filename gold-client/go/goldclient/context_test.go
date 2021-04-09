package goldclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/gold-client/go/gcsuploader"
	"go.skia.org/infra/gold-client/go/mocks"
)

func TestExtractProperties_ValuesSet_ReturnSetValues(t *testing.T) {
	unittest.SmallTest(t)

	mg := &mocks.GCSUploader{}
	mh := &mocks.HTTPClient{}
	mi := &mocks.ImageDownloader{}

	ctx := WithContext(context.Background(), mg, mh, mi)

	assert.Same(t, mg, extractGCSUploader(ctx))
	assert.Same(t, mh, extractHTTPClient(ctx))
	assert.Same(t, mi, extractImageDownloader(ctx))
}

// In tests, we sometimes mock out the implementations before something later calls WithContext.
// In those cases, we want the original value instead of the potentially not-mocked version.
func TestExtractProperties_ValuesSetTwice_ReturnFirstValues(t *testing.T) {
	unittest.SmallTest(t)

	original := &gcsuploader.DryRunImpl{}

	ctx := WithContext(context.Background(), original, nil, nil)

	mg := &mocks.GCSUploader{}
	mh := &mocks.HTTPClient{}
	mi := &mocks.ImageDownloader{}

	ctx = WithContext(ctx, mg, mh, mi)

	assert.Same(t, original, extractGCSUploader(ctx))
	assert.Same(t, mh, extractHTTPClient(ctx))
	assert.Same(t, mi, extractImageDownloader(ctx))
}

func TestExtractGCSUploader_ValueNotSet_Panics(t *testing.T) {
	unittest.SmallTest(t)

	assert.Panics(t, func() {
		extractGCSUploader(context.Background())
	})
}

func TestExtractHTTPClient_ValueNotSet_Panics(t *testing.T) {
	unittest.SmallTest(t)

	assert.Panics(t, func() {
		extractHTTPClient(context.Background())
	})
}

func TestExtractImageDownloader_ValueNotSet_Panics(t *testing.T) {
	unittest.SmallTest(t)

	assert.Panics(t, func() {
		extractImageDownloader(context.Background())
	})
}

func TestExtractLogWriter_ValueNotSet_ReturnsDefaultValue(t *testing.T) {
	unittest.SmallTest(t)

	w := extractLogWriter(context.Background())
	assert.NotNil(t, w)
}

func TestExtractErrorWriter_ValueNotSet_ReturnsDefaultValue(t *testing.T) {
	unittest.SmallTest(t)

	w := extractErrorWriter(context.Background())
	assert.NotNil(t, w)
}
