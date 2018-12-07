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

type GoldClient interface {
	Test(name string, imgFileName string) (bool, error)
}

// tCloudClient implements the GoldClient interface for a remote Gold server.
type tCloudClient struct {
	workDir     string
	resultState *tResultState
	httpClient  *http.Client
	freshState  bool
	ready       bool
}

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

	ret := &tCloudClient{
		workDir:    workDir,
		httpClient: httputils.DefaultClientConfig().Client(),
	}

	if err := ret.setResultState(instanceID, passFailStep, goldResult); err != nil {
		return nil, skerr.Fmt("Error initializing result in cloud GoldClient: %s", err)
	}

	return ret, nil
}

func (c *tCloudClient) Test(name string, imgFileName string) (bool, error) {
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
		if err := gsUtilCopyToGCS(imgFileName, gcsImagePath); err != nil {
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

// setResultState assembles the information that needs to be uploaded based on previous calls
// to the function and new arguments.
func (c *tCloudClient) setResultState(instanceID string, passFailStep bool, goldResult *jsonio.GoldResults) error {
	// Load the state from the workdir.
	var err error
	c.resultState, err = loadStateFromJson(c.getResultStateFile())
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

func (c *tCloudClient) isReady() bool {
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

func (c *tCloudClient) getResultStateFile() string {
	return filepath.Join(c.workDir, resultStateFile)
}

func (c *tCloudClient) addResult(name, imgHash string) {
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

// tResultState is an internal container for all information to upload results
// to Gold, including the jsonio.GoldResult structure itself.
type tResultState struct {
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

func newResultState(goldResult *jsonio.GoldResults, passFailStep bool, instanceID, workDir string, httpClient *http.Client) (*tResultState, error) {

	// TODO(stephana): Move deriving the URLs and the bucket to a central place in the backend
	// or get rid of the bucket entirely and expose an upload URL (requires authentication)

	ret := &tResultState{
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

func loadStateFromJson(fileName string) (*tResultState, error) {
	if !fileutil.FileExists(fileName) {
		return nil, nil
	}

	jsonBytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	ret := &tResultState{}
	if err := json.Unmarshal(jsonBytes, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *tResultState) save(fileName string) error {
	return nil
}

func (r *tResultState) readyForTests() bool {
	// Make sure the GoldResult instance is set up correctly.
	if _, err := r.GoldResults.Validate(true); err != nil {
		return false
		//		return skerr.Fmt("Invalid GoldResults set. Missing fields: %s", err)
	}

	return true
}

func (r *tResultState) loadKnownHashes() error {
	gcsKnownHashesPath := fmt.Sprintf("%s/%s", r.Bucket, hashFilePath)
	localFileName := filepath.Join(r.workDir, fetchTempFileName)
	if err := gsUtilCpFromGCS(gcsKnownHashesPath, localFileName); err != nil {
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

func (r *tResultState) loadExpectations() error {
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

func (r *tResultState) getResultFilePath() string {
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

func (r *tResultState) getImagePath(imgHash string) string {
	return fmt.Sprintf("%s/%s/%s.png", r.Bucket, imagePrefix, imgHash)
}
