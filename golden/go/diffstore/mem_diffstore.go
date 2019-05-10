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

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/rtcache"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/types"
)

const (
	// DEFAULT_IMG_DIR_NAME is the directory where the images are stored.
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

	// maxGoRoutines is the maximum number of Go-routines we allow in MemDiffStore.
	// This was determined empirically: we had instances running with 500k go-routines without problems.
	maxGoRoutines = 500000
)

// MemDiffStore implements the diff.DiffStore interface.
type MemDiffStore struct {
	// baseDir contains the root directory of where all data are stored.
	baseDir string

	// localDiffDir is the directory where diff images are written to.
	localDiffDir string

	// diffMetricsCache caches and calculates diff metrics and images.
	diffMetricsCache rtcache.ReadThroughCache

	// imgLoader fetches and caches images.
	imgLoader *ImageLoader

	// metricsStore persists diff metrics.
	metricsStore *metricsStore

	// wg is used to synchronize background operations like saving files. Used for testing.
	wg sync.WaitGroup

	// mapper contains various functions for creating image IDs, paths and diff metrics.
	mapper DiffStoreMapper

	// maxGoRoutinesCh is a buffered channel that is used to limit the number of goroutines for diffing.
	maxGoRoutinesCh chan bool
}

// NewMemDiffStore returns a new instance of MemDiffStore.
// 'gigs' is the approximate number of gigs to use for caching. This is not the
// exact amount memory that will be used, but a tuning parameter to increase
// or decrease memory used. If 'gigs' is 0 nothing will be cached in memory.
// If diffFn is not specified, the diff.DefaultDiffFn will be used. If codec is
// not specified, a JSON codec for the diff.DiffMetrics struct will be used.
// If mapper is not specified, GoldIDPathMapper will be used.
func NewMemDiffStore(client *http.Client, baseDir string, gsBucketNames []string, gsImageBaseDir string, gigs int, mapper DiffStoreMapper) (diff.DiffStore, error) {
	imageCacheCount, diffCacheCount := getCacheCounts(gigs)

	imgPath := filepath.Join(baseDir, DEFAULT_IMG_DIR_NAME)
	imgDir, err := fileutil.EnsureDirExists(imgPath)
	if err != nil {
		return nil, skerr.Fmt("Could not make image directory %s: %s", imgPath, err)
	}

	diffPath := filepath.Join(baseDir, DEFAULT_DIFFIMG_DIR_NAME)
	difDir, err := fileutil.EnsureDirExists(diffPath)
	if err != nil {
		return nil, skerr.Fmt("Could not make diff image directory %s: %s", diffPath, err)
	}

	// Set up image retrieval, caching and serving.
	imgLoader, err := NewImgLoader(client, baseDir, imgDir, gsBucketNames, gsImageBaseDir, imageCacheCount, mapper)
	if err != err {
		return nil, skerr.Fmt("Could not create img loader %s", err)
	}

	mStore, err := newMetricsStore(baseDir, mapper, mapper)
	if err != nil {
		return nil, skerr.Fmt("Could not create metrics store %s", err)
	}

	ret := &MemDiffStore{
		baseDir:         baseDir,
		localDiffDir:    difDir,
		imgLoader:       imgLoader,
		metricsStore:    mStore,
		mapper:          mapper,
		maxGoRoutinesCh: make(chan bool, maxGoRoutines),
	}

	if ret.diffMetricsCache, err = rtcache.New(ret.diffMetricsWorker, diffCacheCount, runtime.NumCPU()); err != nil {
		return nil, skerr.Fmt("Could not create diffMetricsCache: %s", err)
	}
	return ret, nil
}

// TODO(stephana): Remove the conversion below once all caches are fixed.

// ConvertLegacy converts the legacy metrics cache to the new format in the background.
func (d *MemDiffStore) ConvertLegacy() {
	d.metricsStore.convertDatabaseFromLegacy()
}

// WarmDigests fetches images based on the given list of digests. It does
// not cache the images but makes sure they are downloaded from GCS.
func (d *MemDiffStore) WarmDigests(priority int64, digests types.DigestSlice, sync bool) {
	missingDigests := make(types.DigestSlice, 0, len(digests))
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
func (d *MemDiffStore) WarmDiffs(priority int64, leftDigests types.DigestSlice, rightDigests types.DigestSlice) {
	priority = rtcache.PriorityTimeCombined(priority)
	diffIDs := d.getDiffIds(leftDigests, rightDigests)
	sklog.Infof("Warming %d diffs", len(diffIDs))
	d.wg.Add(len(diffIDs))
	for _, id := range diffIDs {
		d.maxGoRoutinesCh <- true

		go func(diffId string) {
			defer func() {
				d.wg.Done()
				<-d.maxGoRoutinesCh
			}()

			if err := d.diffMetricsCache.Warm(priority, diffId); err != nil {
				sklog.Errorf("Unable to warm diff %s. Got error: %s", diffId, err)
			}
		}(id)
	}
}

func (d *MemDiffStore) sync() {
	d.wg.Wait()
}

// See DiffStore interface.
func (d *MemDiffStore) Get(priority int64, mainDigest types.Digest, rightDigests types.DigestSlice) (map[types.Digest]interface{}, error) {
	if mainDigest == "" {
		return nil, fmt.Errorf("Received empty dMain digest.")
	}

	diffMap := make(map[types.Digest]interface{}, len(rightDigests))
	var wg sync.WaitGroup
	var mutex sync.Mutex
	for _, right := range rightDigests {
		// Don't compare the digest to itself.
		if mainDigest != right {
			wg.Add(1)
			d.maxGoRoutinesCh <- true

			go func(right types.Digest) {
				defer func() {
					wg.Done()
					<-d.maxGoRoutinesCh
				}()
				id := d.mapper.DiffID(mainDigest, right)
				ret, err := d.diffMetricsCache.Get(priority, id)
				if err != nil {
					sklog.Errorf("Unable to calculate diff for %s. Got error: %s", id, err)
					return
				}
				mutex.Lock()
				defer mutex.Unlock()
				diffMap[right] = ret
			}(right)
		}
	}
	wg.Wait()
	return diffMap, nil
}

// UnavailableDigests implements the DiffStore interface.
func (m *MemDiffStore) UnavailableDigests() map[types.Digest]*diff.DigestFailure {
	return m.imgLoader.failureStore.unavailableDigests()
}

// PurgeDigests implements the DiffStore interface.
func (m *MemDiffStore) PurgeDigests(digests types.DigestSlice, purgeGCS bool) error {
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
	digestSet := make(types.DigestSet, len(digests))
	for _, d := range digests {
		digestSet[d] = true
	}
	removeKeys := make([]string, 0, len(digests))
	for _, key := range m.diffMetricsCache.Keys() {
		d1, d2 := m.mapper.SplitDiffID(key)
		if digestSet[d1] || digestSet[d2] {
			removeKeys = append(removeKeys, key)
		}
	}
	m.diffMetricsCache.Remove(removeKeys)

	if err := m.metricsStore.purgeDiffMetrics(digests); err != nil {
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

	dotExt := "." + IMG_EXTENSION

	// Setup the file server and define the handler function.
	fileServer := http.FileServer(http.Dir(absPath))
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		sklog.Debugf("diffstore handling %s", path)
		idx := strings.Index(path, "/")
		if idx == -1 {
			noCacheNotFound(w, r)
			return
		}
		dir := path[:idx]

		// Limit the requests to directories with the images and diff images.
		if (dir != DEFAULT_DIFFIMG_DIR_NAME) && (dir != DEFAULT_IMG_DIR_NAME) {
			noCacheNotFound(w, r)
			return
		}

		// Get the file that was requested and verify that it's a valid PNG file.
		file := path[idx+1:]
		if (len(file) <= len(dotExt)) || (!strings.HasSuffix(file, dotExt)) {
			noCacheNotFound(w, r)
			return
		}

		// Trim the image extension to get the image ID.
		imgID := path[idx+1 : len(path)-len(dotExt)]
		var localRelPath string
		if dir == DEFAULT_IMG_DIR_NAME {
			// Validate the requested image ID.
			if !m.mapper.IsValidImgID(imgID) {
				noCacheNotFound(w, r)
				return
			}

			imgDigest := types.Digest(imgID)

			// Make sure the file exists. If not fetch it. Should be the exception.
			if !m.imgLoader.IsOnDisk(imgDigest) {
				_, pendingWrites, err := m.imgLoader.Get(diff.PRIORITY_NOW, types.DigestSlice{imgDigest})
				if err != nil {
					sklog.Errorf("Errorf retrieving digests: %s", imgDigest)
					noCacheNotFound(w, r)
					return
				}
				// Wait until the images are written to disk.
				pendingWrites.Wait()
			}
			localRelPath, _, _ = m.mapper.ImagePaths(imgDigest)
		} else {
			// Validate the requested diff image ID.
			if !m.mapper.IsValidDiffImgID(imgID) {
				noCacheNotFound(w, r)
				return
			}

			localRelPath = m.mapper.DiffPath(m.mapper.SplitDiffID(imgID))
		}

		// Rewrite the path to include the mapper's custom local path construction
		// format.
		r.URL.Path = filepath.Join(dir, localRelPath)

		// Cache images for 12 hours.
		w.Header().Set("Cache-Control", "public, max-age=43200")
		fileServer.ServeHTTP(w, r)
	}

	sklog.Infof("Created diffstore with base directory: %s", m.baseDir)

	// The above function relies on the URL prefix being stripped.
	return http.StripPrefix(urlPrefix, http.HandlerFunc(handlerFunc)), nil
}

// noCacheNotFound disables caching and returns a 404.
func noCacheNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	http.NotFound(w, r)
}

// diffMetricsWorker calculates the diff if it's not in the cache.
func (d *MemDiffStore) diffMetricsWorker(priority int64, id string) (interface{}, error) {
	leftDigest, rightDigest := d.mapper.SplitDiffID(id)

	// Load it from disk cache if necessary.
	if dm, err := d.metricsStore.loadDiffMetrics(id); err != nil {
		sklog.Errorf("Error trying to load diff metric: %s", err)
	} else if dm != nil {
		return dm, nil
	}

	// Get the images, but we don't need to wait for them to be written to disk,
	// since we are not e.g. serving them over http.
	imgs, _, err := d.imgLoader.Get(priority, types.DigestSlice{leftDigest, rightDigest})
	if err != nil {
		return nil, err
	}

	// We are guaranteed to have two images at this point.
	diffMetrics, diffImg := d.mapper.DiffFn(imgs[0], imgs[1])

	// Encode the result image and save it to disk. If encoding causes an error
	// we return an error.
	var buf bytes.Buffer
	if err = encodeImg(&buf, diffImg); err != nil {
		return nil, err
	}

	// Save the diffMetrics and the diffImage.
	d.saveDiffInfoAsync(id, leftDigest, rightDigest, diffMetrics, buf.Bytes())
	return diffMetrics, nil
}

// saveDiffInfoAsync saves the given diff information to disk asynchronously.
func (d *MemDiffStore) saveDiffInfoAsync(diffID string, leftDigest, rightDigest types.Digest, diffMetrics interface{}, imgBytes []byte) {
	d.wg.Add(2)
	d.maxGoRoutinesCh <- true
	go func() {
		defer func() {
			d.wg.Done()
			<-d.maxGoRoutinesCh
		}()
		if err := d.metricsStore.saveDiffMetrics(diffID, diffMetrics); err != nil {
			sklog.Errorf("Error saving diff metric: %s", err)
		}
	}()

	d.maxGoRoutinesCh <- true
	go func() {
		defer func() {
			d.wg.Done()
			<-d.maxGoRoutinesCh
		}()

		// Get the local file path using the mapper and save the diff image there.
		localDiffPath := d.mapper.DiffPath(leftDigest, rightDigest)
		if err := saveFilePath(filepath.Join(d.localDiffDir, localDiffPath), bytes.NewBuffer(imgBytes)); err != nil {
			sklog.Error(err)
		}
	}()
}

// Returns all combinations of leftDigests and rightDigests using the given
// DiffID function of the DiffStore's mapper.
func (d *MemDiffStore) getDiffIds(leftDigests, rightDigests types.DigestSlice) []string {
	// reminder: diffIds are "digest1:digest2"
	diffIDsSet := make(util.StringSet, len(leftDigests)*len(rightDigests))
	for _, left := range leftDigests {
		for _, right := range rightDigests {
			if left != right {
				diffIDsSet[d.mapper.DiffID(left, right)] = true
			}
		}
	}
	return diffIDsSet.Keys()
}

// getCacheCounts returns the number of images and diff metrics to cache
// based on the number of GiB provided.
// We are assume that we want to store x images and x^2 diffmetrics and
// solve the corresponding quadratic equation.
func getCacheCounts(gigs int) (int, int) {
	if gigs <= 0 {
		return 0, 0
	}

	// We are looking for x (number of images we can cache) where x is found by solving
	// a * x^2 + b * x = c
	diffSize := float64(BYTES_PER_DIFF_METRIC)             // a
	imgSize := float64(BYTES_PER_IMAGE)                    // b
	bytesGig := float64(uint64(gigs) * 1024 * 1024 * 1024) // c
	// To solve, use the quadratic formula and round to an int
	imgCount := int((-imgSize + math.Sqrt(imgSize*imgSize+4*diffSize*bytesGig)) / (2 * diffSize))
	return imgCount, imgCount * imgCount
}
