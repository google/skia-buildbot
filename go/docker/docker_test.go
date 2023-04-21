package docker

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/mockhttpclient"
)

func TestGetDigest(t *testing.T) {
	ctx := context.Background()

	const (
		fakeRegistry   = "fake-registry"
		fakeRepository = "my-image"
		fakeTag        = "latest"
	)

	md := mockhttpclient.MockGetDialogue([]byte(`{"response-is": "ignored}`))
	md.RequestHeader(acceptHeader, acceptContentType)
	md.ResponseHeader(digestHeader, "fake-digest")
	urlmock := mockhttpclient.NewURLMock()
	fakeURL := fmt.Sprintf(manifestURLTemplate, fakeRegistry, fakeRepository, fakeTag)
	urlmock.MockOnce(fakeURL, md)

	digest, err := GetDigest(ctx, urlmock.Client(), fakeRegistry, fakeRepository, fakeTag)
	require.NoError(t, err)
	require.Equal(t, "fake-digest", digest)
}
