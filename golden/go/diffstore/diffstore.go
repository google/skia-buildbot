package diffstore

import (
	"bytes"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/rtcache"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
)

const (
	// DEFAULT_IMG_DIR_NAME is the directory where the  digest images are stored.
	DEFAULT_IMG_DIR_NAME = "images"

	// DEFAULT_DIFFIMG_DIR_NAME is the directory where the diff images are stored.
	DEFAULT_DIFFIMG_DIR_NAME = "diffs"

	// DEFAULT_GS_IMG_DIR_NAME is the default image directory in GS.
	DEFAULT_GS_IMG_DIR_NAME = "dm-images-v1"

	// DEFAULT_TEMPFILE_DIR_NAME is the name of the temp directory.
	DEFAULT_TEMPFILE_DIR_NAME = "__temp"

	// METRICSDB_NAME is the name of the boltdb caching diff metrics.
	METRICSDB_NAME = "diff.DiffMetricss.db"

	// METRICS_BUCKET is the name of the bucket in the metrics DB.
	METRICS_BUCKET = "metrics"

	// MAX_DIFFMETRICS_CACHE is the maximum number of diff metrics to cache.
	MAX_DIFFMETRICS_CACHE = 1000000

	// MAX_IMAGE_CACHE is the maximum number of images to cache.
	MAX_IMAGE_CACHE = 30000
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

	// metricDB stores the diff metrics in a boltdb databasel.
	metricsDB *bolt.DB

	// diffMetricsCodec encodes/decodes diff.DiffMetrics instances to JSON.
	diffMetricsCodec util.LRUCodec

	// wg is used to synchronize background operations like saving files. Used for testing.
	wg sync.WaitGroup
}

// New returns a new instance of MemDiffStore.
func New(client *http.Client, baseDir, gsBucketName, gsImageBaseDir string) (diff.DiffStore, error) {
	// Set up image retrieval, caching and serving.
	imgDir := fileutil.Must(fileutil.EnsureDirExists(filepath.Join(baseDir, DEFAULT_IMG_DIR_NAME)))
	imgLoader, err := newImgLoader(client, imgDir, gsBucketName, gsImageBaseDir, MAX_IMAGE_CACHE)
	if err != err {
		return nil, err
	}

	metricsDB, err := bolt.Open(filepath.Join(baseDir, METRICSDB_NAME), 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to open metricsDB: %s", err)
	}

	ret := &MemDiffStore{
		baseDir:          baseDir,
		localDiffDir:     fileutil.Must(fileutil.EnsureDirExists(filepath.Join(baseDir, DEFAULT_DIFFIMG_DIR_NAME))),
		imgLoader:        imgLoader,
		metricsDB:        metricsDB,
		diffMetricsCodec: util.JSONCodec(&diff.DiffMetrics{}),
	}

	ret.diffMetricsCache = rtcache.New(ret.diffMetricsWorker, MAX_DIFFMETRICS_CACHE, runtime.NumCPU())
	return ret, nil
}

// WarmDigests prefetches images based on the given list of digests.
func (d *MemDiffStore) WarmDigests(priority int64, digests []string) {
	d.imgLoader.Warm(rtcache.PriorityTimeCombined(priority), digests)
}

// WarmDiffs puts the diff metrics for the cross product of leftDigests x rightDigests into the cache for the
// given diff metric and with the given priority. This means if there are multiple subsets of the digests
// with varying priority (ignored vs "regular") we can call this multiple times.
func (d *MemDiffStore) WarmDiffs(priority int64, leftDigests []string, rightDigests []string) {
	priority = rtcache.PriorityTimeCombined(priority)
	diffIDs := getDiffIds(leftDigests, rightDigests)
	glog.Infof("Warming %d diffs", len(diffIDs))
	d.wg.Add(len(diffIDs))
	for _, id := range diffIDs {
		go func(id string) {
			defer d.wg.Done()
			if err := d.diffMetricsCache.Warm(priority, id); err != nil {
				glog.Errorf("Unable to warm diff %s. Got error: %s", id, err)
			}
		}(id)
	}
}

func (d *MemDiffStore) sync() {
	d.wg.Wait()
}

// See DiffStore interface.
func (d *MemDiffStore) Get(priority int64, mainDigest string, rightDigests []string) (map[string]*diff.DiffMetrics, error) {
	if mainDigest == "" {
		return nil, fmt.Errorf("Received empty dMain digest.")
	}

	// Get the left main digest to make sure we can compare something.
	if _, err := d.imgLoader.Get(priority, []string{mainDigest}); err != nil {
		return nil, fmt.Errorf("Unable to retrieve main digest %s. Got error: %s", mainDigest, err)
	}

	diffMap := make(map[string]*diff.DiffMetrics, len(rightDigests))
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
					glog.Errorf("Unable to calculate diff for %s. Got error: %s", id, err)
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

// TODO(stephana): Implement UnavailableDigests and PurgeDigests when/if we
// re-add the endpoints to deal with image errors.

// UnavailableDigests implements the DiffStore interface.
func (m *MemDiffStore) UnavailableDigests() map[string]*diff.DigestFailure {
	return nil
}

// PurgeDigests implements the DiffStore interface.
func (m *MemDiffStore) PurgeDigests(digests []string, purgeGS bool) error {
	return nil
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

		// Limit the requests to directories with the images and diff images.
		if dir := path[:idx]; (dir != DEFAULT_DIFFIMG_DIR_NAME) && (dir != DEFAULT_IMG_DIR_NAME) {
			http.NotFound(w, r)
			return
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
	if dm, err := d.loadDiffMetric(id); err != nil {
		glog.Errorf("Error trying to load diff metric: %s", err)
	} else if dm != nil {
		return dm, nil
	}

	// Get the images.
	imgs, err := d.imgLoader.Get(priority, []string{leftDigest, rightDigest})
	if err != nil {
		return nil, err
	}

	// We are guaranteed to have two images at this point.
	diffRec, diffImg := diff.CalcDiff(imgs[0], imgs[1])

	// encode the result image and save it to disk. If encoding causes an error
	// we return an error.
	var buf bytes.Buffer
	if err = encodeImg(&buf, diffImg); err != nil {
		return nil, err
	}

	// save the diff.DiffMetrics and the diffImage.
	d.saveDiffInfoAsync(id, leftDigest, rightDigest, diffRec, buf.Bytes())
	return diffRec, nil
}

// saveDiffInfoAsync saves the given diff information to disk asynchronously.
func (d *MemDiffStore) saveDiffInfoAsync(diffID, leftDigest, rightDigest string, dr *diff.DiffMetrics, imgBytes []byte) {
	d.wg.Add(2)
	go func() {
		defer d.wg.Done()
		if err := d.saveDiffMetric(diffID, dr); err != nil {
			glog.Errorf("Error saving diff metric: %s", err)
		}
	}()

	go func() {
		defer d.wg.Done()
		imageFileName := getDiffImgFileName(leftDigest, rightDigest)
		if err := saveFileRadixPath(d.localDiffDir, imageFileName, bytes.NewBuffer(imgBytes)); err != nil {
			glog.Error(err)
		}
	}()
}

// loadDiffMetric loads a diffMetric from disk.
func (d *MemDiffStore) loadDiffMetric(id string) (*diff.DiffMetrics, error) {
	var jsonData []byte = nil
	viewFn := func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(METRICS_BUCKET))
		if bucket == nil {
			return nil
		}

		jsonData = bucket.Get([]byte(id))
		return nil
	}

	if err := d.metricsDB.View(viewFn); err != nil {
		return nil, err
	}

	if jsonData == nil {
		return nil, nil
	}

	ret, err := d.diffMetricsCodec.Decode(jsonData)
	if err != nil {
		return nil, err
	}
	return ret.(*diff.DiffMetrics), nil
}

// saveDiffMetric stores a diffmetric on disk.
func (d *MemDiffStore) saveDiffMetric(id string, dr *diff.DiffMetrics) error {
	jsonData, err := d.diffMetricsCodec.Encode(dr)
	if err != nil {
		return err
	}

	updateFn := func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(METRICS_BUCKET))
		if err != nil {
			return err
		}

		return bucket.Put([]byte(id), jsonData)
	}

	err = d.metricsDB.Update(updateFn)
	return err
}

func getDiffBasename(d1, d2 string) string {
	if d1 < d2 {
		return fmt.Sprintf("%s-%s", d1, d2)
	}
	return fmt.Sprintf("%s-%s", d2, d1)
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

// combineDigests returns a sorted, colon-separated concatination of two digests
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

// makeDiffMap creates a map[string]map[string]*DiffRecor map that is big
// enough to store the difference between all digests in leftKeys and
// 'rightLen' items.
func makeDiffMap(leftKeys []string, rightLen int) map[string]map[string]*diff.DiffMetrics {
	ret := make(map[string]map[string]*diff.DiffMetrics, len(leftKeys))
	for _, k := range leftKeys {
		ret[k] = make(map[string]*diff.DiffMetrics, rightLen)
	}
	return ret
}
