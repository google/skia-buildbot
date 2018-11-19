package goldclient

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"image/png"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/types"
)

const (
	resultPrefix       = "dm-json-v1"
	imagePrefix        = "dm-images-v1"
	resultFileNameTmpl = "dm-%s.json"
	tempFileName       = "dm.json"
)

type GoldClient interface {
	SetConfig(config interface{}) error
	Test(name string, imgFileName string) (bool, error)
}

type UploadResults struct {
	// Extend the GoldResults struct with some meta data about uploading.
	results *jsonio.GoldResults

	perTestPassFail bool
	instanceID      string
	workDir         string
}

func NewUploadResults(results *jsonio.GoldResults, instanceID string, perTestPassFail bool, workDir string) (*UploadResults, error) {
	ret := &UploadResults{
		results:         results,
		perTestPassFail: perTestPassFail,
		instanceID:      instanceID,
		workDir:         workDir,
	}

	return ret, nil
}

func (u *UploadResults) merge(right *UploadResults) {
	if u.results == nil {
		u.results = right.results
	}
	if u.instanceID == "" {
		u.instanceID = right.instanceID
	}
	if !u.perTestPassFail {
		u.perTestPassFail = right.perTestPassFail
	}
}

func (u *UploadResults) getResultFilePath() string {
	now := time.Now().UTC()
	year, month, day := now.Date()
	hour := now.Hour()
	fileName := fmt.Sprintf(resultFileNameTmpl, strconv.FormatInt(now.UnixNano()/int64(time.Millisecond), 10))
	path := fmt.Sprintf("%s/%04d/%02d/%02d/%02d/%s", resultPrefix, year, month, day, hour, fileName)

	if u.results.Issue > 0 {
		path = "trybot/" + path
	}

	return path
}

func (u *UploadResults) getImagePath(imgHash string) string {
	return fmt.Sprintf("%s/%s.png", imagePrefix, imgHash)
}

// Implement the GoldClient interface for a remote Gold server.
type cloudClient struct {
	uploadResults *UploadResults
	ready         bool
	goldURL       string
	bucket        string
	knownHashes   util.StringSet
	expectations  types.TestExp
}

func NewCloudClient(results *UploadResults) (GoldClient, error) {
	ret := &cloudClient{
		uploadResults: &UploadResults{},
	}
	if err := ret.SetConfig(results); err != nil {
		return nil, sklog.FmtErrorf("Error initializing result in Cloud GoldClient: %s", err)
	}

	return ret, nil
}

func (c *cloudClient) SetConfig(config interface{}) error {
	// If we are ready, there is nothing todo here.
	if c.ready {
		return nil
	}

	// Make sure we have an instand of UploadResults.
	resultConf, ok := config.(*UploadResults)
	if !ok {
		return sklog.FmtErrorf("Provided config is not an instance of *UploadResults")
	}
	c.uploadResults.merge(resultConf)

	// From the instance ID load Derive the Gold URL and the bucket from the instance ID.
	if err := c.processInstanceID(resultConf.instanceID); err != nil {
		return err
	}

	// TODO:  Make sure the GoldResult instance is set up correctly.
	if _, err := c.uploadResults.results.Validate(true); err != nil {
		return sklog.FmtErrorf("Invalid GoldResults set. Missing fields: %s", err)
	}

	c.ready = true
	return nil
}

func (c *cloudClient) processInstanceID(instanceID string) error {
	// TODO(stephana): Move the URLs and deriving the bucket to a central place in the backend
	// or get rid of the bucket entirely and expose an upload URL (requires authentication)

	// Derive and set the GoldURL and the upload bucket.
	c.goldURL = fmt.Sprintf("https://%s-gold.skia.org", instanceID)
	c.bucket = fmt.Sprintf("skia-gold-%s", instanceID)

	// TODO(stephana): Fetch the known hashes (may be empty, but should not fail).
	c.knownHashes = util.StringSet{}

	// TODO(stephana): Fetch the baseline (may be empty but should not fail).
	c.expectations = types.TestExp{}

	return nil
}

func (c *cloudClient) Test(name string, imgFileName string) (bool, error) {
	if !c.ready {
		return false, sklog.FmtErrorf("Unable to process test result. Cloud Gold Client uninitialized. Call SetConfig before this call.")
	}

	// Load the PNG from disk and hash it.
	_, imgHash, err := loadAndHashFile(imgFileName)
	if err != nil {
		return false, err
	}

	// Check against known hashes and upload if needed.
	if !c.knownHashes[imgHash] {
		if err := c.gsUtilUploadImage(imgFileName, imgHash); err != nil {
			return false, sklog.FmtErrorf("Error uploading image: %s", err)
		}
	}

	// Upload the result of this test.
	c.addResult(name, imgHash)
	if err := c.gsUtilUploadResultsFile(); err != nil {
		return false, sklog.FmtErrorf("Error uploading result file: %s", err)
	}

	// If we do per test pass/fail then compare to the baseline and return accordingly
	if c.uploadResults.perTestPassFail {
		// Check if this is positive in the expectations.
		// TODO(stephana): Better define semantics of expecations.
		return c.expectations[name][imgHash] == types.POSITIVE, nil
	}

	// If we don't do per-test pass/fail then return true.
	return true, nil
}

func (c *cloudClient) addResult(name, imgHash string) {
	// Add the result to the overall results.
	newResult := &jsonio.Result{
		Digest: imgHash,
		Key:    map[string]string{types.PRIMARY_KEY_FIELD: name},

		// TODO(stephana): check if the backend still relies on this. s
		Options: map[string]string{"ext": "png"},
	}

	// TODO(stephana): Make the corpus field an option.
	if _, ok := c.uploadResults.results.Key[types.CORPUS_FIELD]; !ok {
		newResult.Key[types.CORPUS_FIELD] = c.uploadResults.instanceID
	}
	c.uploadResults.results.Results = append(c.uploadResults.results.Results, newResult)
}

func (c *cloudClient) gsUtilUploadResultsFile() error {
	localFileName := filepath.Join(c.uploadResults.workDir, tempFileName)
	jsonBytes, err := json.MarshalIndent(c.uploadResults.results, "", "  ")
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(localFileName, jsonBytes, 0600); err != nil {
		return err
	}

	// Get the path to upload to
	objPath := c.uploadResults.getResultFilePath()
	return c.gsUtilCopy(localFileName, objPath)
}

func (c *cloudClient) gsUtilCopy(srcFile, targetPath string) error {
	srcFile, err := filepath.Abs(srcFile)
	if err != nil {
		return err
	}

	// runCmd := &exec.Command{
	// 	Name:           "gsutil",
	// 	Args:           []string{"cp", srcFile, fmt.Sprintf("gs://%s/%s", c.bucket, targetPath)},
	// 	Timeout:        5 * time.Second,
	// 	CombinedOutput: &combinedBuf,
	// 	Verbose:        exec.Silent,
	// }

	runCmd := exec.Command("gsutil", "cp", srcFile, fmt.Sprintf("gs://%s/%s", c.bucket, targetPath))
	outBytes, err := runCmd.CombinedOutput()
	if err != nil {
		return sklog.FmtErrorf("Error running gsutil. Got output \n%s\n and error: %s", outBytes, err)
	}

	return nil
}

func (c *cloudClient) gsUtilUploadImage(imagePath string, imgHash string) error {
	objPath := c.uploadResults.getImagePath(imgHash)
	return c.gsUtilCopy(imagePath, objPath)
}

func loadAndHashFile(fileName string) ([]byte, string, error) {
	// Load the image
	reader, err := os.Open(fileName)
	if err != nil {
		return nil, "", err
	}
	defer util.Close(reader)

	imgBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, "", sklog.FmtErrorf("Error loading file %s: %s", fileName, err)
	}

	img, err := png.Decode(bytes.NewBuffer(imgBytes))
	if err != nil {
		return nil, "", sklog.FmtErrorf("Error decoding PNG in file %s: %s", fileName, err)
	}
	nrgbaImg := diff.GetNRGBA(img)
	md5Hash := fmt.Sprintf("%x", md5.Sum(nrgbaImg.Pix))
	return imgBytes, md5Hash, nil
}
