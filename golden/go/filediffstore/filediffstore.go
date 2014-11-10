package filediffstore

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"code.google.com/p/goauth2/compute/serviceaccount"
	"code.google.com/p/google-api-go-client/storage/v1"
	"github.com/golang/glog"
	"github.com/rcrowley/go-metrics"
	"skia.googlesource.com/buildbot.git/go/auth"
	"skia.googlesource.com/buildbot.git/go/gs"
	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/golden/go/diff"
)

const (
	DEFAULT_IMG_DIR_NAME         = "images"
	DEFAULT_DIFF_DIR_NAME        = "diffs"
	DEFAULT_DIFFMETRICS_DIR_NAME = "diffmetrics"
	DEFAULT_GS_IMG_DIR_NAME      = "dm-images-v1"
	IMG_EXTENSION                = "png"
	DIFF_EXTENSION               = "png"
	DIFFMETRICS_EXTENSION        = "json"
	RECOMMENDED_WORKER_POOL_SIZE = 1000

	// Limit the number of times diffstore tries to get a file before giving up.
	MAX_URI_GET_TRIES = 4
)

var (
	// Contains the number of times digests were successfully downloaded from
	// Google Storage.
	downloadSuccessCount metrics.Counter
	// Contains the number of times digests failed to download from
	// Google Storage.
	downloadFailureCount metrics.Counter
)

// Init initializes the module.
func Init() {
	downloadSuccessCount = metrics.NewRegisteredCounter("golden.gsdownload.success", metrics.DefaultRegistry)
	downloadFailureCount = metrics.NewRegisteredCounter("golden.gsdownload.failiure", metrics.DefaultRegistry)
}

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

	// The channels workers pick up tasks from.
	absPathCh chan *WorkerReq
	getCh     chan *WorkerReq

	// Mutexes for ensuring safe access to the different local caches.
	diffDirLock   sync.Mutex
	digestDirLock sync.Mutex
}

// GetAuthClient is a helper function that runs through the OAuth flow if doOauth
// is true, else it tries to auth using a service account. Intended to be used by
// some clients and passed into NewFileDiffStore.
func GetAuthClient(doOauth bool) (*http.Client, error) {
	var client *http.Client
	var err error
	if doOauth {
		client, err = auth.RunFlow(auth.DefaultOAuthConfig)
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
// images will be downloaded from. workerPoolSize is the max number of
// simultaneous goroutines that will be created when running Get or AbsPath.
// Use RECOMMENDED_WORKER_POOL_SIZE if unsure what this value should be.
func NewFileDiffStore(client *http.Client, baseDir, gsBucketName string, workerPoolSize int) diff.DiffStore {
	if client == nil {
		client = util.NewTimeoutClient()
	}
	fs := &FileDiffStore{
		client:              client,
		localImgDir:         ensureDir(filepath.Join(baseDir, DEFAULT_IMG_DIR_NAME)),
		localDiffDir:        ensureDir(filepath.Join(baseDir, DEFAULT_DIFF_DIR_NAME)),
		localDiffMetricsDir: ensureDir(filepath.Join(baseDir, DEFAULT_DIFFMETRICS_DIR_NAME)),
		gsBucketName:        gsBucketName,
		storageBaseDir:      DEFAULT_GS_IMG_DIR_NAME,
	}
	fs.activateWorkers(workerPoolSize)
	return fs
}

type WorkerReq struct {
	id     interface{}
	respCh chan<- *WorkerResp
}

type WorkerResp struct {
	id  interface{}
	val interface{}
}

type GetId struct {
	dMain  string
	dOther string
}

func (fs *FileDiffStore) activateWorkers(workerPoolSize int) {
	fs.absPathCh = make(chan *WorkerReq, workerPoolSize)
	fs.getCh = make(chan *WorkerReq, workerPoolSize)

	for i := 0; i < workerPoolSize; i++ {
		go func() {
			for {
				select {
				case req := <-fs.absPathCh:
					req.respCh <- &WorkerResp{id: req.id, val: fs.absPathOne(req.id.(string))}
				case req := <-fs.getCh:
					gid := req.id.(GetId)
					req.respCh <- &WorkerResp{id: req.id, val: fs.getOne(gid.dMain, gid.dOther)}
				}
			}
		}()
	}
}

// getOne uses the following algorithm:
// 1. Look for the DiffMetrics of the digests in the local cache.
// If found:
//     2. Return the DiffMetrics.
// Else:
//     3. Make sure the digests exist in the local cache. Download it from
//        Google Storage if necessary.
//     4. Calculate DiffMetrics.
//     5. Write DiffMetrics to the cache and return.
func (fs *FileDiffStore) getOne(dMain, dOther string) interface{} {
	// 1. Check if the DiffMetrics exists in the local cache.
	diffMetrics, err := fs.getDiffMetricsFromCache(dMain, dOther)
	if err != nil {
		glog.Errorf("Failed to getDiffMetricsFromCache for digest %s and digest %s: %s", dMain, dOther, err)
		return nil
	}
	if diffMetrics != nil {
		// 2. The DiffMetrics exists locally return it.
		return diffMetrics
	}

	// 3. Make sure the digests exist in the local cache. Download it from
	//    Google Storage if necessary.
	if err = fs.ensureDigestInCache(dOther); err != nil {
		glog.Errorf("Failed to ensureDigestInCache for digest %s: %s", dOther, err)
		return nil
	}

	// 4. Calculate DiffMetrics.
	diffMetrics, err = fs.diff(dMain, dOther)
	if err != nil {
		glog.Errorf("Failed to calculate DiffMetrics for digest %s and digest %s: %s", dMain, dOther, err)
		return nil
	}
	glog.Infof("Calculated DiffMetrics for %s and %s\n", dMain, dOther)
	// 5. Write DiffMetrics to the local cache and return it.
	diffMetricsFilePath := filepath.Join(
		fs.localDiffMetricsDir,
		fmt.Sprintf("%s.%s", getDiffBasename(dMain, dOther), DIFFMETRICS_EXTENSION))
	if err := fs.writeDiffMetrics(diffMetricsFilePath, diffMetrics); err != nil {
		glog.Errorf("Failed to writeDiffMetrics for digest %s and digest %s: %s", dMain, dOther, err)
		return nil
	}
	return diffMetrics
}

// absPathOne uses the following algorithm:
// 1. Make sure the digests exist in the local cache. Download it from
//    Google Storage if necessary.
// 2. Find and return the absolute path to the digest.
func (fs *FileDiffStore) absPathOne(digest string) interface{} {
	// 1. Make sure we have a local copy of the digest and
	//    download it if necessary. Note: Downloading should
	//    be the exception.
	if err := fs.ensureDigestInCache(digest); err != nil {
		glog.Errorf("Failed to ensureDigestInCache for digest %s: %s", digest, err)
		return nil
	}
	// 2. Find and return the absolute path to the digest.
	return fs.getDigestImagePath(digest)
}

// Get documentation is found in the diff.DiffStore interface.
// This implementation of Get uses the following algorithm:
// 1. Look for the main digest in local cache else download from Google
//    Storage.
// 2. Create map of digests to their DiffMetrics. This map will be
//    populated and returned.
// 3. Create the channel where responses from workers will be received in.
// 4. Send requests to the request channel.
// The workers will then call getOne with the requests.
// 5. Return map of digests to DiffMetrics once all requests have been
//     processed by the workers.
func (fs *FileDiffStore) Get(dMain string, dRest []string) (map[string]*diff.DiffMetrics, error) {
	// 1. Look for main digest in local cache else download from GS.
	if err := fs.ensureDigestInCache(dMain); err != nil {
		// We cannot compute any DiffMetrics without the main digest.
		// Therefore, fail immediately if the main digest cannot be
		// retrieved.
		return nil, fmt.Errorf("Failed to ensureDigestInCache for main digest %s: %s", dMain, err)
	}

	// 2. Create map of digests to their DiffMetrics. This map will be
	//    populated and returned.
	digestsToDiffMetrics := make(map[string]*diff.DiffMetrics, len(dRest))

	// If the input is empty then we are done. We are doing this here, because if the call to 
	// ensureDigestInCache fails we likely have a programming error and we want to catch that.
        if len(dRest) == 0 {
		return digestsToDiffMetrics, nil
	}


	// 3. Create the channel where responses from workers will be received in.
	respCh := make(chan *WorkerResp, len(dRest))
	// 4. Send requests to the request channel.
	for dIndex := range dRest {
		dOther := dRest[dIndex]
		fs.getCh <- &WorkerReq{id: GetId{dMain: dMain, dOther: dOther}, respCh: respCh}
	}
	digestErrors := 0
	for {
		select {
		case resp := <-respCh:
			gid := resp.id.(GetId)
			if val, ok := resp.val.(*diff.DiffMetrics); ok {
				digestsToDiffMetrics[gid.dOther] = val
			} else {
				// This block will be reached when the DiffMetrics could
				// not be calculated due to failures.
				digestErrors++
			}
			if (len(digestsToDiffMetrics) + digestErrors) == len(dRest) {
				// 5. Return map of digests to paths once all requests have
				//    been processed by the workers.
				return digestsToDiffMetrics, nil
			}
		}
	}
}

// AbsPath documentation is found in the diff.DiffStore interface.
// This implementation of AbsPath uses the following algorithm:
// 1. Create map of digests to paths map. This map will be populated and
//    returned.
// 2. Create the channel where responses from workers will be received in.
// 3. Send requests to the request channel.
// The workers will then call absPathOne with the requests.
// 4. Return map of digests to paths once all requests have been processed
//    by the workers.
func (fs *FileDiffStore) AbsPath(digests []string) map[string]string {
	// 1. Create map of digests to their paths. This map will be populated
	//    and returned.
	digestsToPaths := make(map[string]string, len(digests))

        // If the input is empty then we are done. 
        if len(digests) == 0 {
                return digestsToPaths
        }

	// 2. Create the channel where responses from workers will be received
	//    in.
	respCh := make(chan *WorkerResp, len(digests))
	// 3. Send requests to the request channel.
	for dIndex := range digests {
		digest := digests[dIndex]
		fs.absPathCh <- &WorkerReq{id: digest, respCh: respCh}
	}
	digestErrors := 0
	for {
		select {
		case resp := <-respCh:
			digest, _ := resp.id.(string)
			if val, ok := resp.val.(string); ok {
				digestsToPaths[digest] = val
			} else {
				// This block will be reached when the path could not be
				// calculated due to failures.
				digestErrors++
			}
			if (len(digestsToPaths) + digestErrors) == len(digests) {
				// 4. Return map of digests to paths once all requests have
				//    been processed.
				return digestsToPaths
			}
		}
	}
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

func (fs *FileDiffStore) writeDiffMetrics(filepath string, diffMetrics *diff.DiffMetrics) error {
	// Lock the mutex before writing to the local diff directory.
	fs.diffDirLock.Lock()
	defer fs.diffDirLock.Unlock()
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
	fs.diffDirLock.Lock()
	defer fs.diffDirLock.Unlock()
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

// ensureDigestInCache checks if the image corresponding to digest is cached
// localy. If not it will download it from GS.
func (fs *FileDiffStore) ensureDigestInCache(d string) error {
	exists, err := fs.isDigestInCache(d)
	if err != nil {
		return err
	}
	if !exists {
		// Digest does not exist locally, get it from Google Storage.
		if err := fs.cacheImageFromGS(d); err != nil {
			return err
		}
	}
	return nil
}

// This method looks for the specified digest from the local image dir. It is
// thread safe because it locks the diff store's mutext before accessing the digest
// cache.
func (fs *FileDiffStore) isDigestInCache(d string) (bool, error) {
	digestFilePath := fs.getDigestImagePath(d)
	// Lock the mutex before reading from the local digest directory.
	fs.digestDirLock.Lock()
	defer fs.digestDirLock.Unlock()
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
// digest cache. If the provided digest does not exist in Google Storage then
// downloadFailureCount is incremented.
func (fs *FileDiffStore) cacheImageFromGS(d string) error {
	storage, err := storage.New(fs.client)
	if err != nil {
		return fmt.Errorf("Failed to create interface to Google Storage: %s\n", err)
	}

	objLocation := filepath.Join(fs.storageBaseDir, fmt.Sprintf("%s.%s", d, IMG_EXTENSION))
	res, err := storage.Objects.Get(fs.gsBucketName, objLocation).Do()
	if err != nil {
		downloadFailureCount.Inc(1)
		return err
	}
	respBody, err := fs.getRespBody(res)
	defer respBody.Close()
	if err != nil {
		downloadFailureCount.Inc(1)
		return err
	}

	outputFile := filepath.Join(fs.localImgDir, fmt.Sprintf("%s.png", d))
	// Lock the mutex before writing to the local digest directory.
	fs.digestDirLock.Lock()
	defer fs.digestDirLock.Unlock()
	out, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("Unable to create file %s: %s", outputFile, err)
	}
	defer out.Close()
	if _, err = io.Copy(out, respBody); err != nil {
		return err
	}

	glog.Infof("Downloaded %s to %s", objLocation, outputFile)
	downloadSuccessCount.Inc(1)
	return nil
}

// Returns the response body of the specified GS object. Tries MAX_URI_GET_TRIES
// times if download is unsuccessful. Client must close the response body when
// finished with it.
func (fs *FileDiffStore) getRespBody(res *storage.Object) (io.ReadCloser, error) {
	for i := 0; i < MAX_URI_GET_TRIES; i++ {
		request, err := gs.RequestForStorageURL(res.MediaLink)
		if err != nil {
			glog.Warningf("Unable to create Storage MediaURI request: %s\n", err)
			continue
		}

		resp, err := fs.client.Do(request)
		if err != nil {
			glog.Warningf("Unable to retrieve Storage MediaURI: %s", err)
			continue
		}
		if resp.StatusCode != 200 {
			glog.Warningf("Failed to retrieve: %d  %s", resp.StatusCode, resp.Status)
			resp.Body.Close()
			continue
		}
		return resp.Body, nil
	}
	return nil, fmt.Errorf("Failed fetching file after %d attempts", MAX_URI_GET_TRIES)
}

// Calculate the DiffMetrics for the provided digests.
func (fs *FileDiffStore) diff(d1, d2 string) (*diff.DiffMetrics, error) {
	img1, err := diff.OpenImage(fs.getDigestImagePath(d1))
	if err != nil {
		return nil, err
	}
	img2, err := diff.OpenImage(fs.getDigestImagePath(d2))
	if err != nil {
		return nil, err
	}
	diffFilename := fmt.Sprintf("%s.%s", getDiffBasename(d1, d2), DIFF_EXTENSION)
	diffFilePath := filepath.Join(fs.localDiffDir, diffFilename)
	return diff.Diff(img1, img2, diffFilePath)
}

// getDigestPath returns the filepath where the image corresponding to the
// give digests should be stored.
func (fs *FileDiffStore) getDigestImagePath(digest string) string {
	return filepath.Join(fs.localImgDir, fmt.Sprintf("%s.%s", digest, IMG_EXTENSION))
}

// ensureDir checks whether the given path to a directory exits and creates it
// if necessary. Returns the absolute path that corresponds to the input path.
func ensureDir(dirPath string) string {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		glog.Fatalf("Unable to get absolute path of %s. Got error: %s", dirPath, err.Error())
	}

	if err := os.MkdirAll(absPath, 0700); err != nil {
		glog.Fatalf("Unable to create or verify directory %s. Got error: %s", absPath, err.Error())
	}
	return absPath
}
