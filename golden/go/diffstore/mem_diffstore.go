package diffstore

import (
	"bytes"
	"context"
	"image"
	"net/http"
	"runtime"
	"strings"

	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/rtcache"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/common"
	"go.skia.org/infra/golden/go/diffstore/failurestore"
	"go.skia.org/infra/golden/go/diffstore/metricsstore"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/validation"
)

const (
	// DefaultGCSImgDir is the default image directory in GCS.
	DefaultGCSImgDir = "dm-images-v1"

	// imgWebPath is the directory where the images are stored.
	imgWebPath = "images"

	// diffsWebPath is the directory where the diff images are stored.
	diffsWebPath = "diffs"

	// bytesPerEncodedImage is the estimated number of bytes an uncompressed images consumes.
	// Used to conservatively estimate the maximum number of items in the cache.
	// See go/gold-in-memory-images for more on the encoded images.
	bytesPerEncodedImage = 1024 * 1024

	// bytesPerDecodedImage is a conservative estimate of the amount of bytes a typical image
	// will take up decoded. This means 4 bytes per pixel with images usually being no bigger than
	// 1000px by 1000px.
	bytesPerDecodedImage = 4 * 1000 * 1000

	// bytesPerDiffMetric is the estimated number of bytes per diff metric.
	// Used to conservatively estimate the maximum number of items in the cache.
	bytesPerDiffMetric = 100

	// In the steady state, most of the diffs have already been calculated and stored, so spinning
	// up workers to fetch them from the metricsstore is somewhat cheap. We should only rarely have
	// to actually compute the diff metric (which requires a short burst of CPU usage).
	maxDiffWorkers = 5000

	cacheSizeMetrics = "diffstore_cache_size"
)

// MemDiffStore implements the diff.DiffStore interface.
type MemDiffStore struct {
	// diffMetricsCache caches and calculates diff metrics.
	diffMetricsCache rtcache.ReadThroughCache

	// decodedImageCache caches the pixels of decoded images.
	decodedImageCache rtcache.ReadThroughCache

	// imgLoader fetches and caches images.
	imgLoader *ImageLoader

	// metricsStore persists diff metrics.
	metricsStore metricsstore.MetricsStore
}

// NewMemDiffStore returns a new instance of MemDiffStore.
// 'gigs' is the approximate number of gigs to use for caching. This is not the
// exact amount memory that will be used, but a tuning parameter to increase
// or decrease memory used. If 'gigs' is 0 nothing will be cached in memory.
func NewMemDiffStore(client gcs.GCSClient, gsImageBaseDir string, gigs int, mStore metricsstore.MetricsStore, fStore failurestore.FailureStore) (*MemDiffStore, error) {
	imageCacheCount, diffCacheCount := getCacheCounts(gigs)

	// Set up image retrieval, caching and serving.
	sklog.Debugf("Creating img loader with cache of size %d", imageCacheCount)
	imgLoader, err := NewImgLoader(client, fStore, gsImageBaseDir, imageCacheCount)
	if err != nil {
		return nil, skerr.Wrapf(err, "creating img loader with dir %s", gsImageBaseDir)
	}

	ret := &MemDiffStore{
		imgLoader:    imgLoader,
		metricsStore: mStore,
	}

	// TODO(kjlubick) These read-through caches may make this code unnecessarily hard to follow.
	//   Consider just using an LRU directly.
	sklog.Debugf("Creating diffMetricsCache of size %d", diffCacheCount)
	if ret.diffMetricsCache, err = rtcache.New(ret.diffMetricsWorker, diffCacheCount, maxDiffWorkers); err != nil {
		return nil, skerr.Wrapf(err, "creating diffMeticsCache of size %d with max of %d workers", diffCacheCount, maxDiffWorkers)
	}

	// Limit the amount of decoding that can be done at once to the number of CPUs, since that can
	// be pretty CPU intensive.
	if ret.decodedImageCache, err = rtcache.New(ret.decodedImageWorker, imageCacheCount, runtime.NumCPU()); err != nil {
		return nil, skerr.Wrapf(err, "creating decodedImageCache of size %d with max of %d workers", imageCacheCount, runtime.NumCPU())
	}

	return ret, nil
}

// Get implements the DiffStore interface.
func (m *MemDiffStore) Get(ctx context.Context, mainDigest types.Digest, rightDigests types.DigestSlice) (map[types.Digest]*diff.DiffMetrics, error) {
	if mainDigest == "" {
		return nil, skerr.Fmt("Received empty dMain digest.")
	}

	diffIDs := make([]string, 0, len(rightDigests))
	digests := make(types.DigestSlice, 0, len(rightDigests))
	for _, right := range rightDigests {
		// Don't compare the digest to itself, if somehow, mainDigest is in rightDigests.
		if mainDigest == right {
			continue
		}
		diffIDs = append(diffIDs, common.DiffID(mainDigest, right))
		digests = append(digests, right)
	}

	diffMetrics, err := m.diffMetricsCache.GetAll(ctx, diffIDs)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting diffs for %s against %d other digests", mainDigest, len(digests))
	}

	diffMap := make(map[types.Digest]*diff.DiffMetrics, len(digests))
	for i, dm := range diffMetrics {
		// this nil check is likely just being paranoid.
		if dm != nil {
			diffMap[digests[i]] = dm.(*diff.DiffMetrics)
		}
	}

	metrics2.GetInt64Metric(cacheSizeMetrics, map[string]string{
		"type": "metrics",
	}).Update(int64(m.diffMetricsCache.Len()))

	return diffMap, nil
}

// UnavailableDigests implements the DiffStore interface.
func (m *MemDiffStore) UnavailableDigests(ctx context.Context) (map[types.Digest]*diff.DigestFailure, error) {
	return m.imgLoader.failureStore.UnavailableDigests(ctx)
}

// PurgeDigests implements the DiffStore interface.
func (m *MemDiffStore) PurgeDigests(ctx context.Context, digests types.DigestSlice, purgeGCS bool) error {
	// We remove the given digests from the various places where they might
	// be stored. None of the purge steps should return an error if the digests
	// related information is missing. So any error indicates a bigger problem in the
	// underlying system, i.e.issues with disk etc., that needs to be investigated
	// by hand. Since we remove the digests from the failureStore last, we will
	// not loose the vital information of what digests failed in the first place.

	// Remove the images from, the image cache, disk and GCS if necessary.
	if err := m.imgLoader.purgeImages(ctx, digests, purgeGCS); err != nil {
		return skerr.Wrapf(err, "purging %v (fromGCS: %t)", digests, purgeGCS)
	}

	// Remove the diff metrics from the cache if they exist.
	digestSet := make(types.DigestSet, len(digests))
	for _, d := range digests {
		digestSet[d] = true
	}
	removeKeys := make([]string, 0, len(digests))
	for _, key := range m.diffMetricsCache.Keys() {
		d1, d2 := common.SplitDiffID(key)
		if digestSet[d1] || digestSet[d2] {
			removeKeys = append(removeKeys, key)
		}
	}
	m.diffMetricsCache.Remove(removeKeys)

	if err := m.metricsStore.PurgeDiffMetrics(ctx, digests); err != nil {
		return skerr.Wrapf(err, "purging diff metrics for %v", digests)
	}

	return m.imgLoader.failureStore.PurgeDigestFailures(ctx, digests)
}

// ImageHandler implements the DiffStore interface.
func (m *MemDiffStore) ImageHandler(urlPrefix string) (http.Handler, error) {
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		// Go's image package has no color profile support and we convert to 8-bit NRGBA to diff,
		// but our source images may have embedded color profiles and be up to 16-bit. So we must
		// at least take care to serve the original .pngs unaltered.
		//
		// TODO(lovisolo): Diff in NRGBA64?
		// TODO(lovisolo): Make sure each pair of images is in the same color space before diffing?
		//                 (They probably are today but it'd be a good sanity check to make sure.)

		dotExt := "." + common.IMG_EXTENSION
		urlPath := r.URL.Path
		sklog.Debugf("diffstore handling %s", urlPath)
		idx := strings.Index(urlPath, "/")
		if idx == -1 {
			noCacheNotFound(w, r)
			return
		}
		dir := urlPath[:idx]

		// Limit the requests to directories with the images and diff images.
		if dir != diffsWebPath && dir != imgWebPath {
			noCacheNotFound(w, r)
			return
		}

		// Get the file that was requested and verify that it's a valid PNG file.
		file := urlPath[idx+1:]
		if (len(file) <= len(dotExt)) || (!strings.HasSuffix(file, dotExt)) {
			noCacheNotFound(w, r)
			return
		}

		// Trim the image extension to get the image ID.
		imgID := urlPath[idx+1 : len(urlPath)-len(dotExt)]
		imgDigest := types.Digest(imgID)

		// Cache images for 12 hours.
		w.Header().Set("Cache-Control", "public, max-age=43200")

		if dir == imgWebPath {
			// Validate the requested image ID.
			if !validation.IsValidDigest(imgID) {
				noCacheNotFound(w, r)
				return
			}

			// Retrieve the image from the in-memory cache.
			imgs, err := m.imgLoader.Get(r.Context(), types.DigestSlice{imgDigest})
			if err != nil {
				sklog.Errorf("Error retrieving digest: %s", imgID)
				noCacheNotFound(w, r)
				return
			}

			// Write output image to the http.ResponseWriter. Content-Type is set automatically
			// based on the first 512 bytes of written data. See docs for ResponseWriter.Write()
			// for details.
			if _, err := w.Write(imgs[0]); err != nil {
				sklog.Errorf("Error writing image to http.ResponseWriter: %s", err)
				noCacheNotFound(w, r)
			}
		} else {
			// Validate the requested diff image ID.
			if !validation.IsValidDiffImgID(imgID) {
				noCacheNotFound(w, r)
				return
			}

			// Extract the left and right image digests.
			leftDigest, rightDigest := common.SplitDiffID(imgID)

			// Retrieve the images from the in-memory cache.
			imgs, err := m.decodedImageCache.GetAll(r.Context(), []string{string(leftDigest), string(rightDigest)})
			if err != nil {
				sklog.Errorf("Error retrieving and decoding digests to compute diff: %s", imgID)
				noCacheNotFound(w, r)
				return
			}

			// Compute the diff image.
			leftImg, rightImg := imgs[0].(*image.NRGBA), imgs[1].(*image.NRGBA)
			_, diffImg := diff.PixelDiff(leftImg, rightImg)

			// Write output image to the http.ResponseWriter. Content-Type is set automatically
			// based on the first 512 bytes of written data. See docs for ResponseWriter.Write()
			// for details.
			//
			// The encoding step below does not take color profiles into account. This is fine since
			// both the left and right images used to compute the diff are in the same color space,
			// and also because the resulting diff image is just a visual approximation of the
			// differences between the left and right images.
			if err := common.EncodeImg(w, diffImg); err != nil {
				sklog.Errorf("Error encoding diff image: %s", err)
				noCacheNotFound(w, r)
			}
		}
	}

	sklog.Infof("Created diffstore")

	// The above function relies on the URL prefix being stripped.
	return http.StripPrefix(urlPrefix, http.HandlerFunc(handlerFunc)), nil
}

// noCacheNotFound disables caching and returns a 404.
func noCacheNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	http.NotFound(w, r)
}

// diffMetricsWorker calculates the diff if it's not in the cache.
func (m *MemDiffStore) diffMetricsWorker(ctx context.Context, ids []string) ([]interface{}, error) {
	// Load the metrics from the store if they are there.
	var missedIndexes []int
	rv := make([]interface{}, len(ids))
	if xdm, err := m.metricsStore.LoadDiffMetrics(ctx, ids); err != nil {
		sklog.Warningf("Could not load diff metrics from cache for %s, going to recompute them all (err: %s)", ids, err)
		for i := range ids {
			missedIndexes = append(missedIndexes, i)
		}
	} else {
		for i, dm := range xdm {
			if dm != nil {
				rv[i] = dm
			} else {
				missedIndexes = append(missedIndexes, i)
			}
		}
		if len(missedIndexes) == 0 {
			return rv, nil
		}
	}

	for _, missed := range missedIndexes {
		t := metrics2.NewTimer("gold_compute_diff_cycle")
		id := ids[missed]
		leftDigest, rightDigest := common.SplitDiffID(id)

		imgs, err := m.decodedImageCache.GetAll(ctx, []string{string(leftDigest), string(rightDigest)})
		if err != nil {
			return nil, skerr.Wrapf(err, "retrieving and decoding the following digests: %s, %s", leftDigest, rightDigest)
		}

		// Compute the diff image.
		leftImg, rightImg := imgs[0].(*image.NRGBA), imgs[1].(*image.NRGBA)
		diffMetrics := diff.ComputeDiffMetrics(leftImg, rightImg)

		if err := m.metricsStore.SaveDiffMetrics(ctx, id, diffMetrics); err != nil {
			sklog.Warningf("Warning - could not store diff metric: %s", err)
		}
		rv[missed] = diffMetrics
		t.Stop()
	}

	return rv, nil
}

// decodedImageWorker gets the images from imgLoader (which has a cache to save downloads) and
// decodes them serially. The number of ids is either 1 or 2 (since we only reason about up to
// to images at a time).
func (m *MemDiffStore) decodedImageWorker(ctx context.Context, ids []string) ([]interface{}, error) {
	imgs, err := m.imgLoader.Get(ctx, asDigests(ids))
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching the images")
	}

	rv := make([]interface{}, 0, len(ids))
	for i, imgBytes := range imgs {
		img, err := common.DecodeImg(bytes.NewReader(imgBytes))
		if err != nil {
			return nil, skerr.Wrapf(err, "decoding image with id %s", ids[i])
		}
		rv = append(rv, img)
	}
	return rv, nil
}

// getCacheCounts returns the number of images and diff metrics to cache
// based on the number of GiB provided. We assume that we want to store N images
// and 100 * N diffmetrics and solve the corresponding equation.
// For a given test, there usually aren't more than about 100 digests,
// as per empirical evidence with Skia.
func getCacheCounts(gigs int) (numImages, numDiffs int) {
	if gigs <= 0 {
		return 0, 0
	}

	bytesGig := int64(gigs) * 1024 * 1024 * 1024
	N := bytesGig / (100*bytesPerDiffMetric + bytesPerEncodedImage + bytesPerDecodedImage)

	return int(N), 100 * int(N)
}
