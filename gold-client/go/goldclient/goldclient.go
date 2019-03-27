package goldclient

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	gstorage "cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/types"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
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
	jsonTempFile = "gsutil_dm.json"

	// authOpt is the file in the work directory where the auth options are cached.
	authOptFile = "auth_opt.json"
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

	// SetAuthOpt sets the authentication method for interacting with GCS and the Gold backend.
	// Use any of the functions that return AuthOpt instance to generate this, e.g. LUCIAuthOpt.
	SetAuthOpt(opt *AuthOpt) error
}

// TODO(stephana): Change AuthOpt so the internal fields are hidden, but we can still
// serialize it JSON easily and clients are force to rely on using the *AuthOpt functions.

// AuthOpt encapsulates the authentication option to be used by SetAuthOpt.
type AuthOpt struct {
	Luci           bool
	ServiceAccount string
}

// validate returns a nil error if the AuthOpt object is valid.
func (a *AuthOpt) validate() error {
	if !a.Luci && a.ServiceAccount == "" {
		return skerr.Fmt("No valid authentication method provided.")
	}
	return nil
}

// ServiceAccountAuthOpt returns an AuthOpt instance that configures a service account file
// to use to generate a TokenSource for authentication with GCP.
func ServiceAccountAuthOpt(svcAccountFile string) *AuthOpt {
	return &AuthOpt{ServiceAccount: svcAccountFile}
}

// LUCIAuthOpt returns an AuthOpt instance to get auth information from the LUCI context.
func LUCIAuthOpt() *AuthOpt { return &AuthOpt{Luci: true} }

// cloudUploader implementations provide functions to upload to GCS.
type cloudUploader interface {
	// copy copies from a local file to GCS. The dst string is assumed to have a gs:// prefix.
	// Currently only uploading from a local file to GCS is supported.
	uploadBytesOrFile(data []byte, fileName, dst string) error

	// uploadJson serializes the given data to JSON and uploads the result to GCS.
	// An implementation can use tempFileName for temporary storage of JSON data.
	uploadJson(data interface{}, tempFileName, gcsObjectPath string) error
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
	ready bool

	// auth stores the authentication method to use.
	auth       *AuthOpt
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

	workDir, err := fileutil.EnsureDirExists(config.WorkDir)
	if err != nil {
		return nil, err
	}

	// TODO(stephana): When we add authentication via a service account this needs to be
	// be triggered by an argument to this function or a config flag of some sort.

	ret := &cloudClient{
		workDir: workDir,
	}
	if err := ret.setHttpClient(); err != nil {
		return nil, skerr.Fmt("Error setting http client: %s", err)
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
		if err := saveJSONFile(c.getResultStateFile(), c.resultState); err != nil {
			return false, err
		}
	}
	return passed, err
}

// getUploader returns a cloudUploader instance. It either uses oauth/http if available or
// shells out to gsutil if no authentication is available.
func (c *cloudClient) getUploader() (cloudUploader, error) {
	if c.auth != nil {
		return newHttpUploader(context.TODO(), c.httpClient)
	}
	return gsutilUploader{}, nil
}

// addTest adds a test to results. If perTestPassFail is true it will also upload the result.
func (c *cloudClient) addTest(name string, imgFileName string) (bool, error) {
	if err := c.isReady(); err != nil {
		return false, skerr.Fmt("Unable to process test result. Cloud Gold Client not ready: %s", err)
	}

	// Get an uploader. This is either based on an authenticated client or on gsutils.
	uploader, err := c.getUploader()
	if err != nil {
		return false, skerr.Fmt("Error retrieving uploader: %s", err)
	}

	// Load the PNG from disk and hash it.
	imgBytes, imgHash, err := loadAndHashImage(imgFileName)
	if err != nil {
		return false, err
	}
	fmt.Printf("Given image with hash %s for test %s\n", imgHash, name)
	for expectHash, expectLabel := range c.resultState.Expectations[name] {
		fmt.Printf("Expectation for test: %s (%s)\n", expectHash, expectLabel.String())
	}

	var egroup errgroup.Group
	// Check against known hashes and upload if needed.
	if !c.resultState.KnownHashes[imgHash] {
		egroup.Go(func() error {
			gcsImagePath := c.resultState.getGCSImagePath(imgHash)
			if err := uploader.uploadBytesOrFile(imgBytes, imgFileName, prefixGCS(gcsImagePath)); err != nil {
				return skerr.Fmt("Error uploading image %s to %s. Got: %s", imgFileName, gcsImagePath, err)
			}
			return nil
		})
	}

	// Add the result of this test.
	c.addResult(name, imgHash)

	// At this point the result should be correct for uploading.
	if _, err := c.resultState.GoldResults.Validate(false); err != nil {
		return false, err
	}

	// If we do per test pass/fail then upload the result and compare it to the baseline.
	ret := true
	if c.resultState.PerTestPassFail {
		egroup.Go(func() error {
			localFileName := filepath.Join(c.workDir, jsonTempFile)
			resultFilePath := c.resultState.getResultFilePath()
			if err := uploader.uploadJson(c.resultState.GoldResults, localFileName, resultFilePath); err != nil {
				return skerr.Fmt("Error uploading JSON file to GCS path %s: %s", resultFilePath, err)
			}
			return nil
		})
		ret = c.resultState.Expectations[name][imgHash] == types.POSITIVE
	}

	if err := egroup.Wait(); err != nil {
		return false, err
	}
	return ret, nil
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

	if err := c.loadAuthOpt(); err != nil {
		return skerr.Fmt("Error loading auth information: %s", err)
	}

	// If we are ready that means we have loaded the resultState from the temporary directory.
	if err := c.isReady(); err == nil {
		return nil
	}

	// If we have enough information we create an instance of the result state. Sometimes we
	// might create an instance with minimal information to, e.g. add auth information.
	if config != nil && config.InstanceID != "" {
		c.resultState, err = newResultState(goldResult, config, c.workDir, c.httpClient)
		if err != nil {
			return err
		}

		// Setting freshState to true indicates that this needs
		// to be stored to disk once a test has been added successfully.
		c.freshState = true
	}
	return nil
}

// SetAuthOpt implements the GoldClient interface.
func (c *cloudClient) SetAuthOpt(authOpt *AuthOpt) error {
	if err := authOpt.validate(); err != nil {
		return err
	}
	c.auth = authOpt
	if err := c.saveAuthOpt(); err != nil {
		return err
	}

	// Instantiate a HTTP client to make sure the credentials were valid.
	return c.setHttpClient()
}

// loadAuthOpt loads the auth options that have been configure earlier from disk.
func (c *cloudClient) loadAuthOpt() error {
	inFile := filepath.Join(c.workDir, authOptFile)
	ret := &AuthOpt{}
	found, err := loadJSONFile(inFile, &ret)
	if err != nil {
		return err
	}

	if found {
		c.auth = ret
	}
	return c.setHttpClient()
}

// setHttpClient sets authenticated httpClient, if authentication was configured via SetAuthConfig.
// It also retrieves a token of the configured source to make sure it works.
func (c *cloudClient) setHttpClient() error {
	// If no auth option was set, we return an unauthenticated client.
	if c.auth == nil {
		c.httpClient = httputils.DefaultClientConfig().Client()
		return nil
	}

	var tokenSrc oauth2.TokenSource
	var err error
	if c.auth.Luci {
		tokenSrc, err = auth.NewLUCIContextTokenSource(gstorage.ScopeFullControl)
	} else {
		tokenSrc, err = auth.NewJWTServiceAccountTokenSource("#bogus", c.auth.ServiceAccount, gstorage.ScopeFullControl)
	}
	if err != nil {
		return skerr.Fmt("Unable to instantiate auth token source: %s", err)
	}

	// Retrieve a token to make sure we can retrieve a token. We assume this is cached inside tokenSrc.
	if _, err := tokenSrc.Token(); err != nil {
		return skerr.Fmt("Error retrieving initial auth token: %s", err)
	}
	c.httpClient = httputils.DefaultClientConfig().WithTokenSource(tokenSrc).Client()
	return nil
}

// saveAuthOpt assumes that auth has been set. It saves it to the work directory for retrieval
// during later calls.
func (c *cloudClient) saveAuthOpt() error {
	outFile := filepath.Join(c.workDir, authOptFile)
	return saveJSONFile(outFile, c.auth)
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

	// Check whether we have some means of uploading results
	if c.auth == nil && !gsutilAvailable() {
		return skerr.Fmt("Unable to find 'gsutil' on the PATH and no authentication information provided")
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
	ret := &resultState{}
	exists, err := loadJSONFile(fileName, ret)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return ret, nil
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

// loadExpectations fetches the expectations from Gold to compare to tests.
func (r *resultState) loadExpectations() error {
	urlPath := strings.Replace(shared.EXPECTATIONS_ROUTE, "{commit_hash}", r.GoldResults.GitHash, 1)
	if r.GoldResults.Issue > 0 {
		urlPath = fmt.Sprintf("%s?issue=%d", urlPath, r.GoldResults.Issue)
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

// loadJSONFile loads and parses the JSON in 'fileName'. If the file doesn't exist it returns
// (false, nil). If the first return value is true, 'data' contains the parse JSON data.
func loadJSONFile(fileName string, data interface{}) (bool, error) {
	if !fileutil.FileExists(fileName) {
		return false, nil
	}

	err := util.WithReadFile(fileName, func(r io.Reader) error {
		return json.NewDecoder(r).Decode(data)
	})
	if err != nil {
		return false, skerr.Fmt("Error reading/parsing JSON file: %s", err)
	}

	return true, nil
}

// saveJSONFile stores the given 'data' in a file with the given name
func saveJSONFile(fileName string, data interface{}) error {
	err := util.WithWriteFile(fileName, func(w io.Writer) error {
		return json.NewEncoder(w).Encode(data)
	})
	if err != nil {
		return skerr.Fmt("Error writing/serializing to JSON: %s", err)
	}
	return nil
}
