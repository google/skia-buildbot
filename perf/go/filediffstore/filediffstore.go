package filediffstore

import (
	"code.google.com/p/goauth2/compute/serviceaccount"
	"code.google.com/p/google-api-go-client/storage/v1"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/perf/go/auth"
	"skia.googlesource.com/buildbot.git/perf/go/diff"
	"skia.googlesource.com/buildbot.git/perf/go/gs"
	"sync"
)

const (
	DEFAULT_IMG_DIR_NAME         = "images"
	DEFAULT_DIFF_DIR_NAME        = "diffs"
	DEFAULT_DIFFMETRICS_DIR_NAME = "diffmetrics"
	DEFAULT_GS_IMG_DIR_NAME      = "dm-images-v1"
	IMG_EXTENSION                = "png"
	DIFF_EXTENSION               = "diff"
	DIFFMETRICS_EXTENSION        = "json"
)

// TODO(rmistry): Record if fetching from google storage failed, keep a map with a
// counter and increment it every time a file fails.
type FileDiffStore struct {
	// The client used to connect to Google Storage.
	client *http.Client

	// The local directory where image digests should be written to.
	localImgDir string

	// The local directory where images diffs should be stored in.
	localDiffDir string

	// The local directory where DiffMetrics should be serialized in.
	localDiffMetricsDir string

	// Cache for all digests in the localBaseDir.
	digestCache map[string]int

	// Cache for recently used diffmetrics, eviction based on LFU.
	diffCache []*diff.DiffMetrics

	// The GS bucket where images are stored.
	gsBucketName string

	// The complete GS URL where images are stored.
	storageBaseDir string

	// Mutex for ensuring safe access to the caches.
	lock sync.Mutex
}

// GetAuthClient is a helper function that runs through the OAuth flow if doOauth
// is true, else it tries to auth using a service account. Intended to be used by
// some clients and passed into NewFileDiffStore.
func GetAuthClient(doOauth bool) (*http.Client, error) {
	var client *http.Client
	var err error
	if doOauth {
		client, err = auth.RunFlow()
		if err != nil {
			return nil, fmt.Errorf("Failed to auth: %s", err)
		}
	} else {
		client, err = serviceaccount.NewClient(nil)
		if err != nil {
			return nil, fmt.Errorf("Failed to auth using a service account: %s", err)
		}
	}
	return client, nil
}

// NewFileDiffStore intializes and returns a file based implementation of
// DiffStore. The optional http.Client is used to make HTTP requests to Google
// Storage. If nil is supplied then a default client is used. The baseDir is the
// local base directory where the DEFAULT_IMG_DIR_NAME, DEFAULT_DIFF_DIR_NAME and
// the DEFAULT_DIFFMETRICS_DIR_NAME directories exist. gsBucketName is the bucket
// images will be downloaded from.
func NewFileDiffStore(client *http.Client, baseDir, gsBucketName string) diff.DiffStore {
	if client == nil {
		client = util.NewTimeoutClient()
	}
	return &FileDiffStore{
		client:              client,
		localImgDir:         filepath.Join(baseDir, DEFAULT_IMG_DIR_NAME),
		localDiffDir:        filepath.Join(baseDir, DEFAULT_DIFF_DIR_NAME),
		localDiffMetricsDir: filepath.Join(baseDir, DEFAULT_DIFFMETRICS_DIR_NAME),
		gsBucketName:        gsBucketName,
		storageBaseDir:      DEFAULT_GS_IMG_DIR_NAME,
		lock:                sync.Mutex{},
	}
}

// Get uses the following algorithm:
// 1. Look for the DiffMetrics of the requested digests in the local cache.
// If found:
//     2. Return the DiffMetrics.
// Else:
//     3. Check to see if the digests exist in the local cache.
// If do not exist locally:
//     4. Download from Google Storage.
// 5. Calculate DiffMetrics.
// 6. Write DiffMetrics to the local cache and return.
func (fs *FileDiffStore) Get(d1, d2 string) (*diff.DiffMetrics, error) {

	// 1. Check if the DiffMetrics exists in the local cache.
	diffMetrics, err := fs.getDiffMetricsFromCache(d1, d2)
	if err != nil {
		return nil, err
	}
	if diffMetrics != nil {
		// 2. The DiffMetrics exists locally, return it.
		return diffMetrics, nil
	}

	// 3. Check to see if the digests exist in the local cache.
	for _, d := range [2]string{d1, d2} {
		exists, err := fs.isDigestInCache(d)
		if err != nil {
			return nil, err
		}
		if !exists {
			// 4. Digest does not exist locally, get it from Google Storage.
			if err := fs.cacheImageFromGS(d); err != nil {
				return nil, err
			}
		}
	}

	// 5. Calculate DiffMetrics.
	diffMetrics, err = fs.diff(d1, d2)
	if err != nil {
		return nil, err
	}
	// 6. Write DiffMetrics to the local cache and return.
	diffMetricsFilePath := filepath.Join(
		fs.localDiffMetricsDir,
		fmt.Sprintf("%s.%s", getDiffBasename(d1, d2), DIFFMETRICS_EXTENSION))
	if err := writeDiffMetrics(diffMetricsFilePath, diffMetrics); err != nil {
		return nil, err
	}
	return diffMetrics, nil
}

func openDiffMetrics(filepath string) (*diff.DiffMetrics, error) {
	f, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("Failed to open DiffMetrics %s for reading: %s", filepath, err)
	}
	diffMetrics := &diff.DiffMetrics{}
	if err := json.Unmarshal(f, diffMetrics); err != nil {
		return nil, fmt.Errorf("Failed to decode diffmetrics: %s", err)
	}
	return diffMetrics, nil
}

func writeDiffMetrics(filepath string, diffMetrics *diff.DiffMetrics) error {
	f, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("Unable to create file %s: %s", filepath, err)
	}
	defer f.Close()

	d, err := json.Marshal(diffMetrics)
	if err != nil {
		return fmt.Errorf("Failed to encode to JSON: %s", err)
	}
	f.Write(d)
	return nil
}

// Returns the file basename to use for the specified digests.
// Eg: Returns 111-222 since 111 < 222 when 111 and 222 are specified as inputs
// regardless of the order.
func getDiffBasename(d1, d2 string) string {
	if d1 < d2 {
		return fmt.Sprintf("%s-%s", d1, d2)
	}
	return fmt.Sprintf("%s-%s", d2, d1)
}

// This method looks for and returns DiffMetrics of the specified digests from the
// local diffmetrics dir. It is thread safe because it locks the diff store's
// mutex before accessing the digest cache.
func (fs *FileDiffStore) getDiffMetricsFromCache(d1, d2 string) (*diff.DiffMetrics, error) {
	filename := fmt.Sprintf("%s.%s", getDiffBasename(d1, d2), DIFFMETRICS_EXTENSION)
	diffMetricsFilePath := filepath.Join(fs.localDiffMetricsDir, filename)
	// Lock the mutex before reading from the local diff directory.
	fs.lock.Lock()
	defer fs.lock.Unlock()
	if _, err := os.Stat(diffMetricsFilePath); err != nil {
		if os.IsNotExist(err) {
			// File does not exist.
			return nil, nil
		} else {
			// There was some other error.
			return nil, err
		}
	}

	diffMetrics, err := openDiffMetrics(diffMetricsFilePath)
	if err != nil {
		return nil, err
	}
	return diffMetrics, nil
}

// This method looks for the specified digest from the local image dir. It is
// thread safe because it locks the diff store's mutext before accessing the digest
// cache.
func (fs *FileDiffStore) isDigestInCache(d string) (bool, error) {
	digestFilePath := filepath.Join(fs.localImgDir, fmt.Sprintf("%s.%s", d, IMG_EXTENSION))
	// Lock the mutex before reading from the local digest directory.
	fs.lock.Lock()
	defer fs.lock.Unlock()
	if _, err := os.Stat(digestFilePath); err != nil {
		if os.IsNotExist(err) {
			// File does not exist.
			return false, nil
		} else {
			// There was some other error.
			return false, err
		}
	}
	return true, nil
}

// Downloads image file from Google Storage and caches it in a local directory. It
// is thread safe because it locks the diff store's mutext before accessing the
// digest cache.
// TODO(rmistry): Try multiple times to get a file from GS, moving to 4 attempts
// per file seems to work. Eg: https://github.com/google/skia-buildbot/blob/master/perf/go/ingester/ingester.go#L268
func (fs *FileDiffStore) cacheImageFromGS(d string) error {
	storage, err := storage.New(fs.client)
	if err != nil {
		return fmt.Errorf("Failed to create interace to Google Storage: %s\n", err)
	}

	objLocation := filepath.Join(fs.storageBaseDir, fmt.Sprintf("%s.%s", d, IMG_EXTENSION))
	res, err := storage.Objects.Get(fs.gsBucketName, objLocation).Do()
	if err != nil {
		return err
	}
	request, err := gs.RequestForStorageURL(res.MediaLink)
	if err != nil {
		return fmt.Errorf("Unable to create Storage MediaURI request: %s\n", err)
	}

	resp, err := fs.client.Do(request)
	if err != nil {
		return fmt.Errorf("Unable to retrieve Storage MediaURI: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("Failed to retrieve: %d  %s", resp.StatusCode, resp.Status)
	}

	outputFile := filepath.Join(fs.localImgDir, fmt.Sprintf("%s.png", d))
	// Lock the mutex before writing to the local image directory.
	fs.lock.Lock()
	defer fs.lock.Unlock()
	out, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("Unable to create file %s: %s", outputFile, err)
	}
	defer out.Close()
	if _, err = io.Copy(out, resp.Body); err != nil {
		return err
	}

	return nil
}

// Calculate the DiffMetrics for the provided digests.
func (fs *FileDiffStore) diff(d1, d2 string) (*diff.DiffMetrics, error) {
	img1, err := diff.OpenImage(filepath.Join(fs.localImgDir, fmt.Sprintf("%s.%s", d1, IMG_EXTENSION)))
	if err != nil {
		return nil, err
	}
	img2, err := diff.OpenImage(filepath.Join(fs.localImgDir, fmt.Sprintf("%s.%s", d2, IMG_EXTENSION)))
	if err != nil {
		return nil, err
	}
	diffFilename := fmt.Sprintf("%s.%s", getDiffBasename(d1, d2), DIFF_EXTENSION)
	diffFilePath := filepath.Join(fs.localDiffDir, diffFilename)
	return diff.Diff(img1, img2, diffFilePath)
}
