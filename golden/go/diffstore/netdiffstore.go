package diffstore

import (
	"net/http"
	"time"

	"go.skia.org/infra/golden/go/diff"
)

type NetDiffStore struct {
	client *http.Client
}

func NewNetDiffStore(addr string) diff.DiffStore {
	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}

	return &NetDiffStore{
		client: &http.Client{Transport: tr},
	}
}

	Get(priority int64, mainDigest string, rightDigests []string) (map[string]*DiffMetrics, error)

	// ImageHandler returns a http.Handler for the given path prefix. The caller
	// can then serve images of the format:
	//        <urlPrefix>/images/<digests>.png
	//        <irlPrefix>/diffs/<digest1>-<digests2>.png
	ImageHandler(urlPrefix string) (http.Handler, error)

	// WarmDigest will fetche the given digests.
	WarmDigests(priority int64, digests []string)

	// WarmDiffs will calculate the difference between every digests in
	// leftDigests and every in digests in rightDigests.
	WarmDiffs(priority int64, leftDigests []string, rightDigests []string)

	// UnavailableDigests returns map[digest]*DigestFailure which can be used
	// to check whether a digest could not be processed and to provide details
	// about failures.
	UnavailableDigests() map[string]*DigestFailure

	// PurgeDigests removes all information related to the indicated digests
	// (image, diffmetric) from local caches. If purgeGCS is true it will also
	// purge the digests image from Google storage, forcing that the digest
	// be re-uploaded by the build bots.
	PurgeDigests(digests []string, purgeGCS bool) error
