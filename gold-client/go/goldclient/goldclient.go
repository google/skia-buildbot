package goldclient

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"image/png"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/types"
)

const (
	// resultPrefix is the path prefix in the GCS bucket that holds JSON result files
	resultPrefix = "dm-json-v1"

	// imagePrefix is the path prefix in the GCS bucket that holds images.
	imagePrefix = "dm-images-v1"

	// knownHashesURLPath is path on the Gold instance to retrieve the known image hashes that do
	// not need to be uploaded anymore.
	knownHashesURLPath = "json/hashes"

	// goldURLTmp constructs the URL of the Gold instance from the instance id
	goldURLTmpl = "https://%s-gold.skia.org"

	// bucketNameTmpl constructs the name of the ingestion bucket from the instance id
	bucketNameTmpl = "skia-gold-%s"

	// resultStateFile is the name of the file that holds the state in the work directory between calls
	resultStateFile = "result-state.json"

	// jsonTempFileName is the temporary file that is created to upload results via gsutil.
	jsonTempFileName = "gsutil_dm.json"
)

// md5Regexp is used to check whether strings are MD5 hashes.
var md5Regexp = regexp.MustCompile(`^[a-f0-9]{32}$`)

// GoldClient is the uniform interface to communicate with the Gold service.
type GoldClient interface {
	// Test adds a test result to the current testrun. If the GoldClient is configured to
	// return PASS/FAIL for each test, the returned boolean indicates whether the test passed
	// comparison with the expectations. An error is only returned if there was a technical problem
	// in processing the test.
	Test(name string, imgFileName string) (bool, error)
}

// cloudClient implements the GoldClient interface for the remote Gold service.
type cloudClient struct {
	// workDir is a temporary directory that has to exist between related calls
	workDir string

	// resultState keeps track of the all the information to generate and upload a valid result.
	resultState *resultState

	// freshState indicates that the instance was created from scratch, not loaded from disk.
	freshState bool

	// ready caches the result of the isReady call so we avoid duplicate work.
	ready      bool
	httpClient *http.Client
}

// GoldClientConfig is a config structure to configure GoldClient instances
type GoldClientConfig struct {
	// WorkDir is a temporary directory that caches data for one run with multiple calls to GoldClient
	WorkDir string

	// InstanceID is the id of the backend Gold instance
	InstanceID string

	// PassFailStep indicates whether each call to Test(...) should return a pass/fail value.
	PassFailStep bool

	// OverrideGoldURL is optional and allows to override the GoldURL for testing.
	OverrideGoldURL string
}

// NewCloudClient returns an implementation of the GoldClient that relies on the Gold service.
// Arguments:
//    - goldResults: A populated instance of jsonio.GoldResults that contains configuration
//                   shared by all tests.
//
// If a new instance is created for each call to Test, the arguments of the first call are
// preserved. They are cached in a JSON file in the work directory.
func NewCloudClient(config *GoldClientConfig, goldResult *jsonio.GoldResults) (GoldClient, error) {
	// Make sure the workdir was given and exists.
	if config.WorkDir == "" {
		return nil, skerr.Fmt("No 'workDir' provided to NewCloudClient")
	}
	workDir, err := filepath.Abs(config.WorkDir)
	if err != nil {
		return nil, err
	}

	// TODO(stephana): When we add authentication via a service account this needs to be
	// be triggered by an argument to this function or a config flag of some sort.

	// Make sure 'gsutil' is on the PATH.
	if !gsutilAvailable() {
		return nil, skerr.Fmt("Unable to find 'gsutil' on the PATH")
	}

	if !fileutil.FileExists(workDir) {
		return nil, fmt.Errorf("Workdir path %q does not exist", workDir)
	}

	ret := &cloudClient{
		workDir:    workDir,
		httpClient: httputils.DefaultClientConfig().Client(),
	}

	if err := ret.initResultState(config, goldResult); err != nil {
		return nil, skerr.Fmt("Error initializing result in cloud GoldClient: %s", err)
	}

	return ret, nil
}

// Test implements the GoldClient interface.
func (c *cloudClient) Test(name string, imgFileName string) (bool, error) {
	passed, err := c.addTest(name, imgFileName)

	// If there was no error and this is new instance then save the resultState for the next call.
	if err == nil && c.freshState {
		if err := c.resultState.save(c.getResultStateFile()); err != nil {
			return false, err
		}
	}
	return passed, err
}

// addTest adds a test to results. If perTestPassFail is true it will also upload the result.
func (c *cloudClient) addTest(name string, imgFileName string) (bool, error) {
	if err := c.isReady(); err != nil {
		return false, skerr.Fmt("Unable to process test result. Cloud Gold Client not ready: %s", err)
	}

	// Load the PNG from disk and hash it.
	_, imgHash, err := loadAndHashImage(imgFileName)
	if err != nil {
		return false, err
	}

	// Check against known hashes and upload if needed.
	if !c.resultState.KnownHashes[imgHash] {
		gcsImagePath := c.resultState.getGCSImagePath(imgHash)
		if err := gsutilCopy(imgFileName, prefixGCS(gcsImagePath)); err != nil {
			return false, skerr.Fmt("Error uploading image: %s", err)
		}
	}

	// Add the result of this test.
	c.addResult(name, imgHash)

	// At this point the result should be correct for uploading.
	if _, err := c.resultState.GoldResults.Validate(false); err != nil {
		return false, err
	}

	// If we do per test pass/fail then upload the result and compare it to the baseline.
	if c.resultState.PerTestPassFail {
		localFileName := filepath.Join(c.workDir, jsonTempFileName)
		if err := gsUtilUploadJson(c.resultState.GoldResults, localFileName, c.resultState.getResultFilePath()); err != nil {
			return false, err
		}
		return c.resultState.Expectations[name][imgHash] == types.POSITIVE, nil
	}

	// If we don't do per-test pass/fail then return true.
	return true, nil
}

// initResultState assembles the information that needs to be uploaded based on previous calls
// to the function and new arguments.
func (c *cloudClient) initResultState(config *GoldClientConfig, goldResult *jsonio.GoldResults) error {
	// Load the state from the workdir.
	var err error
	c.resultState, err = loadStateFromJson(c.getResultStateFile())
	if err != nil {
		return err
	}

	// If we are ready that means we have loaded the resultState from the temporary directory.
	if err := c.isReady(); err == nil {
		return nil
	}

	// Create a new instance of result state. Setting freshState to true indicates that this needs
	// to be stored to disk once a test has been added successfully.
	c.resultState, err = newResultState(goldResult, config, c.workDir, c.httpClient)
	if err != nil {
		return err
	}

	c.freshState = true
	return nil
}

// isReady returns true if the instance is ready to accept test results (all necessary info has been
// configured)
func (c *cloudClient) isReady() error {
	if c.ready {
		return nil
	}

	// if resultState hasn't been set yet, then we are simply not ready.
	if c.resultState == nil {
		return skerr.Fmt("No result state object available")
	}

	// Check if the GoldResults instance is complete once results are added.
	if _, err := c.resultState.GoldResults.Validate(true); err != nil {
		return skerr.Fmt("Gold results fields invalid: %s", err)
	}

	c.ready = true
	return nil
}

// getResultStateFile returns the name of the temporary file where the state is cached as JSON
func (c *cloudClient) getResultStateFile() string {
	return filepath.Join(c.workDir, resultStateFile)
}

func (c *cloudClient) addResult(name, imgHash string) {
	// Add the result to the overall results.
	newResult := &jsonio.Result{
		Digest: imgHash,
		Key:    map[string]string{types.PRIMARY_KEY_FIELD: name},

		// TODO(stephana): check if the backend still relies on this.
		Options: map[string]string{"ext": "png"},
	}

	// TODO(stephana): Make the corpus field an option.
	if _, ok := c.resultState.GoldResults.Key[types.CORPUS_FIELD]; !ok {
		newResult.Key[types.CORPUS_FIELD] = c.resultState.InstanceID
	}
	c.resultState.GoldResults.Results = append(c.resultState.GoldResults.Results, newResult)
}

// loadAndHashImage loads an image from disk and hashes the internal Pixel buffer. It returns
// the bytes of the encoded image and the MD5 hash as hex encoded string.
func loadAndHashImage(fileName string) ([]byte, string, error) {
	// Load the image
	reader, err := os.Open(fileName)
	if err != nil {
		return nil, "", err
	}
	defer util.Close(reader)

	imgBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, "", skerr.Fmt("Error loading file %s: %s", fileName, err)
	}

	img, err := png.Decode(bytes.NewBuffer(imgBytes))
	if err != nil {
		return nil, "", skerr.Fmt("Error decoding PNG in file %s: %s", fileName, err)
	}
	nrgbaImg := diff.GetNRGBA(img)
	md5Hash := fmt.Sprintf("%x", md5.Sum(nrgbaImg.Pix))
	return imgBytes, md5Hash, nil
}

// resultState is an internal container for all information to upload results
// to Gold, including the jsonio.GoldResult structure itself.
type resultState struct {
	// Extend the GoldResults struct with some meta data about uploading.
	GoldResults     *jsonio.GoldResults
	PerTestPassFail bool
	InstanceID      string
	GoldURL         string
	Bucket          string
	KnownHashes     util.StringSet
	Expectations    types.TestExp

	// not saved as state
	httpClient *http.Client
	workDir    string
}

// newResultState creates a new instance resultState and downloads the relevant files from Gold.
func newResultState(goldResult *jsonio.GoldResults, config *GoldClientConfig, workDir string, httpClient *http.Client) (*resultState, error) {

	// TODO(stephana): Move deriving the URLs and the bucket to a central place in the backend
	// or get rid of the bucket entirely and expose an upload URL (requires authentication)

	goldURL := config.OverrideGoldURL
	if goldURL == "" {
		goldURL = fmt.Sprintf(goldURLTmpl, config.InstanceID)
	}

	ret := &resultState{
		GoldResults:     goldResult,
		PerTestPassFail: config.PassFailStep,
		InstanceID:      config.InstanceID,
		GoldURL:         goldURL,
		Bucket:          fmt.Sprintf(bucketNameTmpl, config.InstanceID),
		workDir:         workDir,
		httpClient:      httpClient,
	}

	if err := ret.loadKnownHashes(); err != nil {
		return nil, err
	}

	// TODO(stephana): Fetch the baseline (may be empty but should not fail).
	if err := ret.loadExpectations(); err != nil {
		return nil, err
	}

	return ret, nil
}

// loadStateFromJson loads a serialization of a resultState instance that was previously written
// via the save method.
func loadStateFromJson(fileName string) (*resultState, error) {
	// If the state is not on disk, we return nil, indicating that a new resultState has to be created
	if !fileutil.FileExists(fileName) {
		return nil, nil
	}

	jsonBytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	ret := &resultState{}
	if err := json.Unmarshal(jsonBytes, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// save serializes this instance to JSON and writes it to the given file.
func (r *resultState) save(fileName string) error {
	jsonBytes, err := json.Marshal(r)
	if err != nil {
		return skerr.Fmt("Error serializing resultState to JSON: %s", err)
	}

	if err := ioutil.WriteFile(fileName, jsonBytes, 0644); err != nil {
		return skerr.Fmt("Error writing resultState to %s: %s", fileName, err)
	}
	return nil
}

// loadKnownHashes loads the list of known hashes from the Gold instance.
func (r *resultState) loadKnownHashes() error {
	r.KnownHashes = util.StringSet{}

	// Fetch the known hashes via http
	hashesURL := fmt.Sprintf("%s/%s", r.GoldURL, knownHashesURLPath)
	resp, err := r.httpClient.Get(hashesURL)
	if err != nil {
		return skerr.Fmt("Error retrieving known hashes file: %s", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	// Retrieve the body and parse the list of known hashes.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return skerr.Fmt("Error reading body of HTTP response: %s", err)
	}
	if err := resp.Body.Close(); err != nil {
		return skerr.Fmt("Error closing HTTP response: %s", err)
	}

	scanner := bufio.NewScanner(bytes.NewBuffer(body))
	for scanner.Scan() {
		// Ignore empty lines and lines that are not valid MD5 hashes
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) > 0 && md5Regexp.Match(line) {
			r.KnownHashes[string(line)] = true
		}
	}
	if err := scanner.Err(); err != nil {
		return skerr.Fmt("Error scanning response of HTTP request: %s", err)
	}
	return nil
}

// loadExpecations fetches the expectations from Gold to compare to tests.
func (r *resultState) loadExpectations() error {
	var urlPath string
	if r.GoldResults.Issue > 0 {
		issueID := strconv.FormatInt(r.GoldResults.Issue, 10)
		urlPath = strings.Replace(shared.EXPECATIONS_ISSUE_ROUTE, "{issue_id}", issueID, 1)
	} else {
		urlPath = strings.Replace(shared.EXPECATIONS_ROUTE, "{commit_hash}", r.GoldResults.GitHash, 1)
	}
	url := fmt.Sprintf("%s/%s", r.GoldURL, strings.TrimLeft(urlPath, "/"))

	resp, err := r.httpClient.Get(url)
	if err != nil {
		return err
	}

	jsonBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return skerr.Fmt("Error reading body of request to %s: %s", url, err)
	}
	if err := resp.Body.Close(); err != nil {
		return skerr.Fmt("Error closing response from request to %s: %s", url, err)
	}

	exp := &baseline.CommitableBaseLine{}

	if err := json.Unmarshal(jsonBytes, exp); err != nil {
		return skerr.Fmt("Error parsing JSON: %s", err)
	}

	r.Expectations = exp.Baseline
	return nil
}

// getResultFilePath returns that path in GCS where the result file should be stored.
//
// The path follows the path described here:
//    https://github.com/google/skia-buildbot/blob/master/golden/docs/INGESTION.md
// The file name of the path also contains a timestamp to make it unique since all
// calls within the same test run are written to the same output path.
func (r *resultState) getResultFilePath() string {
	now := time.Now().UTC()
	year, month, day := now.Date()
	hour := now.Hour()

	// Assemble a path that looks like this:
	// <path_prefix>/YYYY/MM/DD/HH/<git_hash>/<build_id>/<time_stamp>/<per_run_file_name>.json
	// The first segments up to 'HH' are required so the Gold ingester can scan these prefixes for
	// new files. The later segments are necessary to make the path unique within the runs of one
	// hour and increase readability of the paths for troubleshooting.
	// It is vital that the times segments of the path are based on UTC location.
	fileName := fmt.Sprintf("dm-%d.json", now.UnixNano())
	segments := []interface{}{
		resultPrefix,
		year,
		month,
		day,
		hour,
		r.GoldResults.GitHash,
		r.GoldResults.BuildBucketID,
		time.Now().Unix(),
		fileName}
	path := fmt.Sprintf("%s/%04d/%02d/%02d/%02d/%s/%d/%d/%s", segments...)

	if r.GoldResults.Issue > 0 {
		path = "trybot/" + path
	}
	return fmt.Sprintf("%s/%s", r.Bucket, path)
}

// getGCSImagePath returns the path in GCS where the image with the given hash should be stored.
func (r *resultState) getGCSImagePath(imgHash string) string {
	return fmt.Sprintf("%s/%s/%s.png", r.Bucket, imagePrefix, imgHash)
}
