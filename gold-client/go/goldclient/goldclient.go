package goldclient

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"image/png"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/types"
)

const (
	resultPrefix       = "dm-json-v1"
	imagePrefix        = "dm-images-v1"
	resultFileNameTmpl = "dm-%s.json"
	hashFilePath       = "hash_files/known-hashes.txt"
	expsIssueTmpl      = "expectations/issue/%d?patchset=%d"
	expsTmpl           = "expectations/master/%s"

	resultStateFile   = "result-state.json"
	jsonTempFileName  = "dm.json"
	fetchTempFileName = "hashes.txt"
)

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
	workDir     string
	resultState *resultState
	httpClient  *http.Client
	freshState  bool
	ready       bool
}

// NewCloudClient returns an implementation of the GoldClient that relies on the Gold service.
// Arguments:
//    - workDir : is a temporary work directory that needs to be available during the entire
//                testrun (over multiple calls to 'Test')
//    - instanceID: is the id of the Gold instance.
//    - passFailStep: indicates whether each individual call to Test needs to return pass fail.
//    - goldResults: A populated instance of jsonio.GoldResults that contains configuration
//                   shared by all tests.
//
// If a new instance is created for each call to Test, the arguments of the first call are
// preserved. They are cached in a JSON file in the work directory.
func NewCloudClient(workDir, instanceID string, passFailStep bool, goldResult *jsonio.GoldResults) (GoldClient, error) {
	// Make sure the workdir was given and exists.
	if workDir == "" {
		return nil, skerr.Fmt("No 'workDir' provided to NewCloudClient")
	}

	workDir, err := filepath.Abs(workDir)
	if err != nil {
		return nil, err
	}

	if !fileutil.FileExists(workDir) {
		return nil, fmt.Errorf("Workdir path %q does not exist", workDir)
	}

	ret := &cloudClient{
		workDir:    workDir,
		httpClient: httputils.DefaultClientConfig().Client(),
	}

	if err := ret.seresultState(instanceID, passFailStep, goldResult); err != nil {
		return nil, skerr.Fmt("Error initializing result in cloud GoldClient: %s", err)
	}

	return ret, nil
}

// Test implements the GoldClient interface.
func (c *cloudClient) Test(name string, imgFileName string) (bool, error) {
	if !c.isReady() {
		return false, skerr.Fmt("Unable to process test result. Cloud Gold Client uninitialized. Call SetConfig before this call.")
	}

	// Load the PNG from disk and hash it.
	_, imgHash, err := loadAndHashImage(imgFileName)
	if err != nil {
		return false, err
	}

	// Check against known hashes and upload if needed.
	if !c.resultState.KnownHashes[imgHash] {
		gcsImagePath := c.resultState.getImagePath(imgHash)
		if err := gsutilCopy(imgFileName, prefixGS(gcsImagePath)); err != nil {
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

// seresultState assembles the information that needs to be uploaded based on previous calls
// to the function and new arguments.
func (c *cloudClient) seresultState(instanceID string, passFailStep bool, goldResult *jsonio.GoldResults) error {
	// Load the state from the workdir.
	var err error
	c.resultState, err = loadStateFromJson(c.geresultStateFile())
	if err != nil {
		return err
	}

	// If we are ready because we loaded the state, there is nothing to do here.
	if c.isReady() {
		return nil
	}

	// Make sure we have an instand of UploadResults.
	c.resultState, err = newResultState(goldResult, passFailStep, instanceID, c.workDir, c.httpClient)
	if err != nil {
		return err
	}

	c.freshState = true
	return nil
}

// isReady returns true if the instance is ready to accept test results (all necessary info has been
// configured)
func (c *cloudClient) isReady() bool {
	if c.ready {
		return true
	}

	// if resultState hasn't been set yet, then we are simply not ready.
	if c.resultState == nil {
		return false
	}

	c.ready = c.resultState.readyForTests()
	return c.ready
}

// geresultStateFile returns the name of the temporary file that contains
func (c *cloudClient) geresultStateFile() string {
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
func newResultState(goldResult *jsonio.GoldResults, passFailStep bool, instanceID, workDir string, httpClient *http.Client) (*resultState, error) {

	// TODO(stephana): Move deriving the URLs and the bucket to a central place in the backend
	// or get rid of the bucket entirely and expose an upload URL (requires authentication)

	ret := &resultState{
		GoldResults:     goldResult,
		PerTestPassFail: passFailStep,
		InstanceID:      instanceID,
		GoldURL:         fmt.Sprintf("https://%s-gold.skia.org", instanceID),
		Bucket:          fmt.Sprintf("skia-gold-%s", instanceID),
		workDir:         workDir,
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

// readyForTest resturns true if the resultState will be able to upload a valid result file
// once a test is added.
func (r *resultState) readyForTests() bool {
	// Make sure the GoldResult instance is set up correctly.
	if _, err := r.GoldResults.Validate(true); err != nil {
		return false
		//		return skerr.Fmt("Invalid GoldResults set. Missing fields: %s", err)
	}

	return true
}

// loadKnownHashes loads the list of known hashes from the Gold instance.
func (r *resultState) loadKnownHashes() error {
	gcsKnownHashesPath := fmt.Sprintf("%s/%s", r.Bucket, hashFilePath)
	localFileName := filepath.Join(r.workDir, fetchTempFileName)
	if err := gsutilCopy(prefixGS(gcsKnownHashesPath), localFileName); err != nil {
		return err
	}

	lines, err := fileutil.ReadLines(localFileName)
	if err != nil {
		return err
	}

	r.KnownHashes = make(util.StringSet, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			r.KnownHashes[line] = true
		}
	}
	return nil
}

// loadExpecations fetches the expectations from Gold to compare to tests.
func (r *resultState) loadExpectations() error {
	var url string
	if r.GoldResults.Issue > 0 {
		url = fmt.Sprintf("%s/"+expsIssueTmpl, r.GoldURL, r.GoldResults.Issue, r.GoldResults.Patchset)
	} else {
		url = fmt.Sprintf("%s/"+expsTmpl, r.GoldURL, r.GoldResults.GitHash)
	}

	resp, err := r.httpClient.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	return json.NewDecoder(resp.Body).Decode(&r.Expectations)
}

// getResultFilePath returns that path in GCS where the result file should be stored.
func (r *resultState) getResultFilePath() string {
	now := time.Now().UTC()
	year, month, day := now.Date()
	hour := now.Hour()
	fileName := fmt.Sprintf(resultFileNameTmpl, strconv.FormatInt(now.UnixNano()/int64(time.Millisecond), 10))
	path := fmt.Sprintf("%s/%04d/%02d/%02d/%02d/%s", resultPrefix, year, month, day, hour, fileName)

	if r.GoldResults.Issue > 0 {
		path = "trybot/" + path
	}

	return fmt.Sprintf("%s/%s", r.Bucket, path)
}

// getImagePath returns the path in GCS where the image with the given hash should be stored.
func (r *resultState) getImagePath(imgHash string) string {
	return fmt.Sprintf("%s/%s/%s.png", r.Bucket, imagePrefix, imgHash)
}
