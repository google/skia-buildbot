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
	mn := &mocks.NowSource{}

	ctx := WithContext(context.Background(), mg, mh, mi, mn)

	assert.Same(t, mg, extractGCSUploader(ctx))
	assert.Same(t, mh, extractHTTPClient(ctx))
	assert.Same(t, mi, extractImageDownloader(ctx))
	assert.Same(t, mn, extractNowSource(ctx))
}

// In tests, we sometimes mock out the implementations before something later calls WithContext.
// In those cases, we want the original value instead of the potentially not-mocked version.
func TestExtractProperties_ValuesSetTwice_ReturnFirstValues(t *testing.T) {
	unittest.SmallTest(t)

	original := &gcsuploader.DryRunImpl{}

	ctx := WithContext(context.Background(), original, nil, nil, nil)

	mg := &mocks.GCSUploader{}
	mh := &mocks.HTTPClient{}
	mi := &mocks.ImageDownloader{}
	mn := &mocks.NowSource{}

	ctx = WithContext(ctx, mg, mh, mi, mn)

	assert.Same(t, original, extractGCSUploader(ctx))
	assert.Same(t, mh, extractHTTPClient(ctx))
	assert.Same(t, mi, extractImageDownloader(ctx))
	assert.Same(t, mn, extractNowSource(ctx))
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

func TestExtractNowSource_ValueNotSet_Panics(t *testing.T) {
	unittest.SmallTest(t)

	assert.Panics(t, func() {
		extractNowSource(context.Background())
	})
}

func TestExtractLogWriter_ValueNotSet_ReturnsNonNilValue(t *testing.T) {
	unittest.SmallTest(t)

	w := extractLogWriter(context.Background())
	assert.NotNil(t, w)
}

func TestExtractErrorWriter_ValueNotSet_ReturnsNonNilValue(t *testing.T) {
	unittest.SmallTest(t)

	w := extractErrorWriter(context.Background())
	assert.NotNil(t, w)
}
