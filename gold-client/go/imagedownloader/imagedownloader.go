package imagedownloader

import (
	"context"
	"fmt"
	"io/ioutil"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/gold-client/go/httpclient"
	"go.skia.org/infra/golden/go/types"
)

// ImageDownloader implementations provide functions to download images from Gold.
type ImageDownloader interface {
	// DownloadImage returns the bytes belonging to a digest from a given instance.
	DownloadImage(ctx context.Context, goldURL string, digest types.Digest) ([]byte, error)
}

// httpImageDownloader implements the ImageDownloaderAPI by downloading images over HTTP.
type httpImageDownloader struct {
	httpClient httpclient.HTTPClient
}

func New(hc httpclient.HTTPClient) *httpImageDownloader {
	return &httpImageDownloader{httpClient: hc}
}

// DownloadImage implements the ImageDownloader API. It downloads the image from the instance
// (not from GCS itself), which removes the need for the service account to have read access.
func (h *httpImageDownloader) DownloadImage(_ context.Context, goldURL string, digest types.Digest) ([]byte, error) {
	u := fmt.Sprintf("%s/img/images/%s.png", goldURL, digest)
	resp, err := h.httpClient.Get(u)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting digest from url %s", u)
	}
	defer util.Close(resp.Body)
	return ioutil.ReadAll(resp.Body)
}

type DryRunImpl struct{}

// DownloadImage implements the ImageDownloader interface.
func (h *DryRunImpl) DownloadImage(_ context.Context, goldURL string, digest types.Digest) ([]byte, error) {
	return nil, skerr.Fmt("Dry run download image %s from %s", digest, goldURL)
}

// Make sure httpImageDownloader fulfills the ImageDownloader interface.
var _ ImageDownloader = (*httpImageDownloader)(nil)

// Make sure DryRunImpl fulfills the ImageDownloader interface.
var _ ImageDownloader = (*DryRunImpl)(nil)
