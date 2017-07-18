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

// IDPathMapper is an interface that defines various functions for mapping
// between IDs and storage paths.
type IDPathMapper interface {
	// Takes two image IDs and returns a unique diff ID.
	// Note: DiffID(a,b) == DiffID(b, a) should hold.
	DiffID(leftImgID, rightImgID string) string

	// Inverse function of DiffID.
	// SplitDiffID(DiffID(a,b)) should return (a,b) or (b,a).
	SplitDiffID(diffID string) (string, string)

	// DiffPath returns the local file path for the diff image of two images.
	// This path is used to store the diff image on disk and serve it over HTTP.
	DiffPath(leftImgID, rightImgID string) string

	// ImagePath returns the local file path for the image. This path is used to
	// store the image on disk and serve it over HTTP.
	ImagePath(imageID string) string

	// IsValidDiffImgName returns true if the given diffImgName is in the correct format.
	IsValidDiffImgName(diffImgName string) bool

	// IsValidImgName returns true if the given imgName is in the correct format.
	IsValidImgName(imgName string) bool
}

// GoldIDPathMapper implements the IDPathMapper interface. The format of an
// imageID is an MD5 digest.
type GoldIDPathMapper struct {}

// Returns a sorted, colon-separated concatenation of two digests.
func (g GoldIDPathMapper) DiffID(leftImgID, rightImgID string) string {
	if rightImgID < leftImgID {
		leftImgID, rightImgID = rightImgID, leftImgID
	}
	return leftImgID + ":" + rightImgID
}

// Splits two colon-separated digests and returns them.
func (g GoldIDPathMapper) SplitDiffID(diffID string) (string, string) {
	images := strings.Split(diffID, ":")
	return images[0], images[1]
}

// Compares the two digests to get the unique image name and then calls
// TwoLevelRadixPath to create the local diff image file path.
func (g GoldIDPathMapper) DiffPath(leftImgID, rightImgID string) string {
	var imageName string
	if leftImgID < rightImgID {
		imageName = fmt.Sprintf("%s-%s", leftImgID, rightImgID)
	} else {
		imageName = fmt.Sprintf("%s-%s", rightImgID, leftImgID)
	}
	imagePath := fmt.Sprintf("%s.%s", imageName, IMG_EXTENSION)
	return fileutil.TwoLevelRadixPath(imagePath)
}

// Calls TwoLevelRadixPath to create the local image file path.
func (g GoldIDPathMapper) ImagePath(imageID string) string {
	imagePath := fmt.Sprintf("%s.%s", imageID, IMG_EXTENSION)
	return fileutil.TwoLevelRadixPath(imagePath)
}

// Ensures that the diffImgName consists of two valid MD5 digests. Should have the
// format leftDigest-rightDigest after the image extension is trimmed.
func (g GoldIDPathMapper) IsValidDiffImgName(diffImgName string) bool {
	digests := strings.Split(diffImgName, "-")
	if len(digests) != 2 {
		return false
	}
	return validation.IsValidDigest(digests[0]) && validation.IsValidDigest(digests[1])
}

// Ensures that the imgName is a valid MD5 digest after the image extension is
// trimmed.
func (g GoldIDPathMapper) IsValidImgName(imgName string) bool {
	return validation.IsValidDigest(imgName)
}

// PixelDiffIDPathMapper implements the IDPathMapper interface. The format
// of an imageID is: YYYY/MM/DD/runID/{nopatch/withpatch}/rank/URLfilename.
// A runID has the format userID-timeStamp.
type PixelDiffIDPathMapper struct {}

// Returns a string containing the common runID, rank and URL of the two image
// paths.
func (p PixelDiffIDPathMapper) DiffID(leftImgID, rightImgID string) string {
	leftPath := strings.Split(leftImgID, "/")
	return leftPath[3] + ":" + leftPath[5] + ":" + leftPath[6]
}

// Returns strings specifying the nopatch and withpatch image paths using the
// given diffID.
func (p PixelDiffIDPathMapper) SplitDiffID(diffID string) (string, string) {
	path := strings.Split(diffID, ":")
	runID := strings.Split(path[0], "-")
	timeStamp := runID[1]
	yearMonthDay := timeStamp[0:4] + "/" + timeStamp[4:6] + "/" + timeStamp[6:8]
	return yearMonthDay + "/" + path[0] + "/nopatch/" + path[1] + "/" + path[2],
				 yearMonthDay + "/" + path[0] + "/withpatch/" + path[1] + "/" + path[2]
}

// Creates the local diff image filepath with the common runID and URL.
func (p PixelDiffIDPathMapper) DiffPath(leftImgID, rightImgID string) string {
	leftPath := strings.Split(leftImgID, "/")
	imageName := leftPath[3] + "/" + leftPath[6]
	return fmt.Sprintf("%s.%s", imageName, IMG_EXTENSION)
}

// Creates the local image filepath.
func (p PixelDiffIDPathMapper) ImagePath(imageID string) string {
	return fmt.Sprintf("%s.%s", imageID, IMG_EXTENSION)
}

// Ensures that diffImgName has the proper number of components in its path. Should
// have the format runID/URLfilename after the image extension is trimmed.
func (p PixelDiffIDPathMapper) IsValidDiffImgName(diffImgName string) bool {
	path := strings.Split(diffImgName, "/")
	return len(path) == 2
}

// Ensures that imgName has the proper number of components in its path. Should
// have the format YYYY/MM/DD/runID/{nopatch/withpatch}/rank/URLfilename
// after the image extension is trimmed.
func (p PixelDiffIDPathMapper) IsValidImgName(imgName string) bool {
	path := strings.Split(imgName, "/")
	return len(path) == 7
}

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

	// mapper contains various functions for creating image IDs and paths.
	mapper IDPathMapper
}

// NewMemDiffStore returns a new instance of MemDiffStore.
// 'gigs' is the approximate number of gigs to use for caching. This is not the
// exact amount memory that will be used, but a tuning parameter to increase
// or decrease memory used. If 'gigs' is 0 nothing will be cached in memory.
// If mapper is not specified, GoldIDPathMapper will be used.
func NewMemDiffStore(client *http.Client, baseDir string, gsBucketNames []string, gsBaseDirs string, gigs int, mapper IDPathMapper) (diff.DiffStore, error) {
	imageCacheCount, diffCacheCount := getCacheCounts(gigs)

	// Set up image retrieval, caching and serving.
	imgDir := fileutil.Must(fileutil.EnsureDirExists(filepath.Join(baseDir, DEFAULT_IMG_DIR_NAME)))

	// Default to GoldIDPathMapper if mapper is not specified
	if mapper == nil {
		mapper = GoldIDPathMapper{}
	}

	imgLoader, err := newImgLoader(client, baseDir, imgDir, gsBucketNames, gsBaseDirs, imageCacheCount, mapper)
	if err != err {
		return nil, err
	}
	mStore, err := newMetricStore(baseDir)
	if err != nil {
		return nil, err
	}

	ret := &MemDiffStore{
		baseDir:         baseDir,
		localDiffDir:    fileutil.Must(fileutil.EnsureDirExists(filepath.Join(baseDir, DEFAULT_DIFFIMG_DIR_NAME))),
		imgLoader:       imgLoader,
		metricsStore:    mStore,
		mapper:          mapper,
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
	diffIDs := d.getDiffIds(leftDigests, rightDigests)
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
func (d *MemDiffStore) Get(priority int64, mainDigest string, rightDigests []string) (map[string]*diff.DiffMetrics, error) {
	if mainDigest == "" {
		return nil, fmt.Errorf("Received empty dMain digest.")
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
				id := d.mapper.DiffID(mainDigest, right)
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
		d1, d2 := m.mapper.SplitDiffID(key)
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

		// Get the file that was requested and trim the image extension.
		file := path[idx+1:]
		file = file[:len(file)-4]

		if dir == DEFAULT_IMG_DIR_NAME {
			// Validate the requested image file.
			if !m.mapper.IsValidImgName(file) {
				http.NotFound(w, r)
				return
			}

			// Make sure the file exists. If not fetch it. Should be the exception.
			if !m.imgLoader.IsOnDisk(file) {
				if _, err = m.imgLoader.Get(diff.PRIORITY_NOW, []string{file}); err != nil {
					sklog.Errorf("Errorf retrieving digests: %s", file)
					http.NotFound(w, r)
					return
				}
			}
		} else {
			// Validate the requested diff file.
			if !m.mapper.IsValidDiffImgName(file) {
				http.NotFound(w, r)
				return
			}
		}

		// Trim the image extension from path and rewrite the path to include
		// the mapper's custom path construction format.
		path = path[:len(path)-4]
		r.URL.Path = m.mapper.ImagePath(path)

		// Cache images for 12 hours.
		w.Header().Set("Cache-control", "public, max-age=43200")
		fileServer.ServeHTTP(w, r)
	}

	// The above function relies on the URL prefix being stripped.
	return http.StripPrefix(urlPrefix, http.HandlerFunc(handlerFunc)), nil
}

// diffMetricsWorker calculates the diff if it's not in the cache.
func (d *MemDiffStore) diffMetricsWorker(priority int64, id string) (interface{}, error) {
	leftDigest, rightDigest := d.mapper.SplitDiffID(id)

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
		if err := d.metricsStore.saveDiffMetric(diffID, dr); err != nil {
			sklog.Errorf("Error saving diff metric: %s", err)
		}
	}()

	go func() {
		defer d.wg.Done()
		// Get the local file path using the mapper and save the diff image there.
		localDiffPath := d.mapper.DiffPath(leftDigest, rightDigest)
		if err := saveFilePath(filepath.Join(d.localDiffDir, localDiffPath), bytes.NewBuffer(imgBytes)); err != nil {
			sklog.Error(err)
		}
	}()
}

// Returns all combinations of leftDigests and rightDigests using the given
// DiffID function of the DiffStore's mapper.
func (d *MemDiffStore) getDiffIds(leftDigests, rightDigests []string) []string {
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
