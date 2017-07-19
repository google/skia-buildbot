package diffstore

import (
	"bytes"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/rtcache"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/validation"
)

const (
	// DEFAULT_IMG_DIR_NAME is the directory where the  digest images are stored.
	DEFAULT_IMG_DIR_NAME = "images"

	// DEFAULT_DIFFIMG_DIR_NAME is the directory where the diff images are stored.
	DEFAULT_DIFFIMG_DIR_NAME = "diffs"

	// DEFAULT_GCS_IMG_DIR_NAME is the default image directory in GCS.
	DEFAULT_GCS_IMG_DIR_NAME = "dm-images-v1"

	// DEFAULT_TEMPFILE_DIR_NAME is the name of the temp directory.
	DEFAULT_TEMPFILE_DIR_NAME = "__temp"

	// BYTES_PER_IMAGE is the estimated number of bytes an uncompressed images consumes.
	// Used to conservatively estimate the maximum number of items in the cache.
	BYTES_PER_IMAGE = 1024 * 1024

	// BYTES_PER_DIFF_METRIC is the estimated number of bytes per diff metric.
	// Used to conservatively estimate the maximum number of items in the cache.
	BYTES_PER_DIFF_METRIC = 100
)

// MemDiffStore implements the diff.DiffStore interface.
type MemDiffStore struct {
	// baseDir contains the root directory of where all data are stored.
	baseDir string

	// localDiffDir is the directory where diff images are written to.
	localDiffDir string

	// diffMetricsCache caches and calculates diff metrics and images.
	diffMetricsCache rtcache.ReadThroughCache

	// diffFn is called on two images to output diff metrics and the diff image
	diffFn diff.DiffFn

	// imgLoader fetches and caches images.
	imgLoader *ImageLoader

	// metricsStore persists diff metrics.
	metricsStore *metricsStore

	// wg is used to synchronize background operations like saving files. Used for testing.
	wg sync.WaitGroup
}

// NewMemDiffStore returns a new instance of MemDiffStore.
// 'gigs' is the approximate number of gigs to use for caching. This is not the
// exact amount memory that will be used, but a tuning parameter to increase
// or decrease memory used. If 'gigs' is 0 nothing will be cached in memory.
// If diffFn is nil, the diff.DefaultDiffFn will be used.
func NewMemDiffStore(client *http.Client, diffFn diff.DiffFn, baseDir string, gsBucketNames []string, gsImageBaseDir string, gigs int) (diff.DiffStore, error) {
	imageCacheCount, diffCacheCount := getCacheCounts(gigs)

	// Set up image retrieval, caching and serving.
	imgDir := fileutil.Must(fileutil.EnsureDirExists(filepath.Join(baseDir, DEFAULT_IMG_DIR_NAME)))
	imgLoader, err := newImgLoader(client, baseDir, imgDir, gsBucketNames, gsImageBaseDir, imageCacheCount)
	if err != err {
		return nil, err
	}

	mStore, err := newMetricStore(baseDir)
	if err != nil {
		return nil, err
	}

	// Default to diff.DefaultDiffFn if diffFn not specified
	if diffFn == nil {
		diffFn = diff.DefaultDiffFn
	}

	ret := &MemDiffStore{
		baseDir:      baseDir,
		localDiffDir: fileutil.Must(fileutil.EnsureDirExists(filepath.Join(baseDir, DEFAULT_DIFFIMG_DIR_NAME))),
		diffFn:       diffFn,
		imgLoader:    imgLoader,
		metricsStore: mStore,
	}

	if ret.diffMetricsCache, err = rtcache.New(ret.diffMetricsWorker, diffCacheCount, runtime.NumCPU()); err != nil {
		return nil, err
	}

	return ret, nil
}

// WarmDigests fetches images based on the given list of digests. It does
// not cache the images but makes sure they are downloaded from GCS.
func (d *MemDiffStore) WarmDigests(priority int64, digests []string, sync bool) {
	missingDigests := make([]string, 0, len(digests))
	for _, digest := range digests {
		if !d.imgLoader.IsOnDisk(digest) {
			missingDigests = append(missingDigests, digest)
		}
	}
	if len(missingDigests) > 0 {
		d.imgLoader.Warm(rtcache.PriorityTimeCombined(priority), missingDigests, sync)
	}
}

// WarmDiffs puts the diff metrics for the cross product of leftDigests x rightDigests into the cache for the
// given diff metric and with the given priority. This means if there are multiple subsets of the digests
// with varying priority (ignored vs "regular") we can call this multiple times.
func (d *MemDiffStore) WarmDiffs(priority int64, leftDigests []string, rightDigests []string) {
	priority = rtcache.PriorityTimeCombined(priority)
	diffIDs := getDiffIds(leftDigests, rightDigests)
	sklog.Infof("Warming %d diffs", len(diffIDs))
	d.wg.Add(len(diffIDs))
	for _, id := range diffIDs {
		go func(id string) {
			defer d.wg.Done()
			if err := d.diffMetricsCache.Warm(priority, id); err != nil {
				sklog.Errorf("Unable to warm diff %s. Got error: %s", id, err)
			}
		}(id)
	}
}

func (d *MemDiffStore) sync() {
	d.wg.Wait()
}

// See DiffStore interface.
func (d *MemDiffStore) Get(priority int64, mainDigest string, rightDigests []string) (map[string]interface{}, error) {
	if mainDigest == "" {
		return nil, fmt.Errorf("Received empty dMain digest.")
	}

	diffMap := make(map[string]interface{}, len(rightDigests))
	var wg sync.WaitGroup
	var mutex sync.Mutex
	for _, right := range rightDigests {
		// Don't compare the digest to itself.
		if mainDigest != right {
			wg.Add(1)
			go func(right string) {
				defer wg.Done()
				id := combineDigests(mainDigest, right)
				ret, err := d.diffMetricsCache.Get(priority, id)
				if err != nil {
					sklog.Errorf("Unable to calculate diff for %s. Got error: %s", id, err)
					return
				}
				mutex.Lock()
				defer mutex.Unlock()
				diffMap[right] = ret.(*diff.DiffMetrics)
			}(right)
		}
	}
	wg.Wait()
	return diffMap, nil
}

// UnavailableDigests implements the DiffStore interface.
func (m *MemDiffStore) UnavailableDigests() map[string]*diff.DigestFailure {
	return m.imgLoader.failureStore.unavailableDigests()
}

// PurgeDigests implements the DiffStore interface.
func (m *MemDiffStore) PurgeDigests(digests []string, purgeGCS bool) error {
	// We remove the given digests from the various places where they might
	// be stored. None of the purge steps should return an error if the digests
	// related information is missing. So any error indicates a bigger problem in the
	// underlying system, i.e.issues with disk etc., that needs to be investigated
	// by hand. Since we remove the digests from the failureStore last, we will
	// not loose the vital information of what digests failed in the first place.

	// Remove the images from, the image cache, disk and GCS if necessary.
	if err := m.imgLoader.PurgeImages(digests, purgeGCS); err != nil {
		return err
	}

	// Remove the diff metrics from the cache if they exist.
	digestSet := util.NewStringSet(digests)
	removeKeys := make([]string, 0, len(digests))
	for _, key := range m.diffMetricsCache.Keys() {
		d1, d2 := splitDigests(key)
		if digestSet[d1] || digestSet[d2] {
			removeKeys = append(removeKeys, key)
		}
	}
	m.diffMetricsCache.Remove(removeKeys)

	if err := m.metricsStore.purgeMetrics(digests); err != nil {
		return err
	}

	return m.imgLoader.failureStore.purgeDigestFailures(digests)
}

// ImageHandler implements the DiffStore interface.
func (m *MemDiffStore) ImageHandler(urlPrefix string) (http.Handler, error) {
	absPath, err := filepath.Abs(m.baseDir)
	if err != nil {
		return nil, fmt.Errorf("Unable to get abs path of %s. Got error: %s", m.baseDir, err)
	}

	// Setup the file server and define the handler function.
	fileServer := http.FileServer(http.Dir(absPath))
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		idx := strings.Index(path, "/")
		if idx == -1 {
			http.NotFound(w, r)
			return
		}
		dir := path[:idx]

		// Limit the requests to directories with the images and diff images.
		if (dir != DEFAULT_DIFFIMG_DIR_NAME) && (dir != DEFAULT_IMG_DIR_NAME) {
			http.NotFound(w, r)
			return
		}

		// Get the file name that was requested and validate it.
		_, fName := filepath.Split(path)
		if dir == DEFAULT_IMG_DIR_NAME {
			// Make sure the file exists. If not fetch it.Should be the exception.
			digest := strings.TrimRight(fName, "."+IMG_EXTENSION)
			if !validation.IsValidDigest(digest) {
				http.NotFound(w, r)
				return
			}

			if !m.imgLoader.IsOnDisk(digest) {
				if _, err = m.imgLoader.Get(diff.PRIORITY_NOW, []string{digest}); err != nil {
					sklog.Errorf("Errorf retrieving digests: %s", digest)
					http.NotFound(w, r)
					return
				}
			}
		} else {
			left, right := splitDiffImgFileName(fName)
			if !validation.IsValidDigest(left) || !validation.IsValidDigest(right) {
				http.NotFound(w, r)
				return
			}
		}

		// rewrite the paths to include the radix prefix.
		r.URL.Path = fileutil.TwoLevelRadixPath(path)

		// Cache images for 12 hours.
		w.Header().Set("Cache-control", "public, max-age=43200")
		fileServer.ServeHTTP(w, r)
	}

	// The above function relies on the URL prefix being stripped.
	return http.StripPrefix(urlPrefix, http.HandlerFunc(handlerFunc)), nil
}

// diffMetricsWorker calculates the diff if it's not in the cache.
func (d *MemDiffStore) diffMetricsWorker(priority int64, id string) (interface{}, error) {
	leftDigest, rightDigest := splitDigests(id)

	// Load it from disk cache if necessary.
	if dm, err := d.metricsStore.loadDiffMetric(id); err != nil {
		sklog.Errorf("Error trying to load diff metric: %s", err)
	} else if dm != nil {
		return dm, nil
	}

	// Get the images.
	imgs, err := d.imgLoader.Get(priority, []string{leftDigest, rightDigest})
	if err != nil {
		return nil, err
	}

	// We are guaranteed to have two images at this point.
	diffRec, diffImg := d.diffFn(imgs[0], imgs[1])

	// encode the result image and save it to disk. If encoding causes an error
	// we return an error.
	var buf bytes.Buffer
	if err = encodeImg(&buf, diffImg); err != nil {
		return nil, err
	}

	// save the diff.DiffMetrics and the diffImage.
	diffMetrics := diffRec.(*diff.DiffMetrics)
	d.saveDiffInfoAsync(id, leftDigest, rightDigest, diffMetrics, buf.Bytes())
	return diffMetrics, nil
}

// saveDiffInfoAsync saves the given diff information to disk asynchronously.
func (d *MemDiffStore) saveDiffInfoAsync(diffID, leftDigest, rightDigest string, dr *diff.DiffMetrics, imgBytes []byte) {
	d.wg.Add(2)
	go func() {
		defer d.wg.Done()
		if err := d.metricsStore.saveDiffMetric(diffID, dr); err != nil {
			sklog.Errorf("Error saving diff metric: %s", err)
		}
	}()

	go func() {
		defer d.wg.Done()
		imageFileName := getDiffImgFileName(leftDigest, rightDigest)
		if err := saveFileRadixPath(d.localDiffDir, imageFileName, bytes.NewBuffer(imgBytes)); err != nil {
			sklog.Error(err)
		}
	}()
}

func getDiffBasename(d1, d2 string) string {
	if d1 < d2 {
		return fmt.Sprintf("%s-%s", d1, d2)
	}
	return fmt.Sprintf("%s-%s", d2, d1)
}

// splitDiffImgFileName splits a diff image file name that was previously
// created with getDiffImgFileName and returns the two digests that
// were compared to create the diff image. It returns two empty strings
// if the given string does not match the expected structure, but does
// not validate the digests in any way.
func splitDiffImgFileName(fName string) (string, string) {
	combined := strings.TrimRight(fName, "."+IMG_EXTENSION)
	digests := strings.Split(combined, "-")
	if len(digests) != 2 {
		return "", ""
	}
	return digests[0], digests[1]
}

func getDiffImgFileName(digest1, digest2 string) string {
	b := getDiffBasename(digest1, digest2)
	return fmt.Sprintf("%s.%s", b, IMG_EXTENSION)
}

// Returns all combinations of leftDigests and rightDigests except for when
// they are identical. The combineDigests function is used to
func getDiffIds(leftDigests, rightDigests []string) []string {
	diffIDsSet := make(util.StringSet, len(leftDigests)*len(rightDigests))
	for _, left := range leftDigests {
		for _, right := range rightDigests {
			if left != right {
				diffIDsSet[combineDigests(left, right)] = true
			}
		}
	}
	return diffIDsSet.Keys()
}

// combineDigests returns a sorted, colon-separated concatenation of two digests
func combineDigests(d1, d2 string) string {
	if d2 > d1 {
		d1, d2 = d2, d1
	}
	return d1 + ":" + d2
}

// splitDigests splits two colon-separated digests and returns them.
func splitDigests(d1d2 string) (string, string) {
	ret := strings.Split(d1d2, ":")
	return ret[0], ret[1]
}

// makeDiffMap creates a map[string]map[string]*DiffRecord map that is big
// enough to store the difference between all digests in leftKeys and
// 'rightLen' items.
func makeDiffMap(leftKeys []string, rightLen int) map[string]map[string]*diff.DiffMetrics {
	ret := make(map[string]map[string]*diff.DiffMetrics, len(leftKeys))
	for _, k := range leftKeys {
		ret[k] = make(map[string]*diff.DiffMetrics, rightLen)
	}
	return ret
}

// getCacheCounts returns the number of images and diff metrics to cache
// based on the number of GiB provided.
// We are assume that we want to store x images and x^2 diffmetrics and
// solve the corresponding quadratic equation.
func getCacheCounts(gigs int) (int, int) {
	if gigs <= 0 {
		return 0, 0
	}

	imgSize := float64(BYTES_PER_IMAGE)
	diffSize := float64(BYTES_PER_DIFF_METRIC)
	bytesGig := float64(uint64(gigs) * 1024 * 1024 * 1024)
	imgCount := int((-imgSize + math.Sqrt(imgSize*imgSize+4*diffSize*bytesGig)) / (2 * diffSize))
	return imgCount, imgCount * imgCount
}
