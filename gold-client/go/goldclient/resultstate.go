package goldclient

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/types"
)

const (
	// jsonPrefix is the path prefix in the GCS bucket that holds JSON result files
	jsonPrefix = "dm-json-v1"

	// imagePrefix is the path prefix in the GCS bucket that holds images.
	imagePrefix = "dm-images-v1"

	// goldHostTemplate constructs the URL of the Gold instance from the instance id
	goldHostTemplate = "https://%s-gold.skia.org"

	// bucketTemplate constructs the name of the ingestion bucket from the instance id
	bucketTemplate = "skia-gold-%s"

	// Skia's naming conventions are old and don't follow the patterns that
	// newer clients do. One day, it might be nice to align the skia names
	// to match the rest.
	bucketSkiaLegacy     = "skia-infra-gm"
	hostSkiaLegacy       = "https://gold.skia.org"
	instanceIDSkiaLegacy = "skia-legacy"

	hostFuchsiaCorp   = "https://fuchsia-gold.corp.goog"
	instanceIDFuchsia = "fuchsia"
)

// md5Regexp is used to check whether strings are MD5 hashes.
var md5Regexp = regexp.MustCompile(`^[a-f0-9]{32}$`)

// resultState is an internal container for all information to upload results
// to Gold, including the jsonio.GoldResult structure itself.
type resultState struct {
	// SharedConfig is all the data that is common test to test, for example, the
	// keys about this machine (e.g. GPU, OS).
	SharedConfig    *jsonio.GoldResults
	PerTestPassFail bool
	FailureFile     string
	UploadOnly      bool
	InstanceID      string
	GoldURL         string
	Bucket          string
	KnownHashes     types.DigestSet
	Expectations    expectations.Baseline
}

// newResultState creates a new instance of resultState
func newResultState(sharedConfig *jsonio.GoldResults, config *GoldClientConfig) *resultState {
	goldURL := config.OverrideGoldURL
	if goldURL == "" {
		goldURL = GetGoldInstanceURL(config.InstanceID)
	}

	ret := &resultState{
		SharedConfig:    sharedConfig,
		PerTestPassFail: config.PassFailStep,
		FailureFile:     config.FailureFile,
		InstanceID:      config.InstanceID,
		UploadOnly:      config.UploadOnly,
		GoldURL:         goldURL,
		Bucket:          getBucket(config.InstanceID),
	}

	return ret
}

// getBucket returns the bucket name for a given instance id.
// This is usually a formulaic transform, but there are some special cases.
func getBucket(instanceID string) string {
	if instanceID == instanceIDSkiaLegacy {
		return bucketSkiaLegacy
	}
	return fmt.Sprintf(bucketTemplate, instanceID)
}

// loadKnownHashes loads the list of known hashes from the Gold instance.
func (r *resultState) loadKnownHashes(httpClient HTTPClient) error {
	r.KnownHashes = types.DigestSet{}

	// Fetch the known hashes via http
	hashesURL := r.GoldURL + shared.KnownHashesRouteV1
	body, err := getWithRetries(httpClient, hashesURL)
	if err != nil {
		return skerr.Wrapf(err, "getting known hashes from %s (with retries)", hashesURL)
	}

	scanner := bufio.NewScanner(bytes.NewBuffer(body))
	for scanner.Scan() {
		// Ignore empty lines and lines that are not valid MD5 hashes
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) > 0 && md5Regexp.Match(line) {
			r.KnownHashes[types.Digest(line)] = true
		}
	}
	if err := scanner.Err(); err != nil {
		return skerr.Wrapf(err, "scanning response of HTTP request")
	}
	return nil
}

// loadExpectations fetches the expectations from Gold to compare to tests.
func (r *resultState) loadExpectations(httpClient HTTPClient) error {
	urlPath := shared.ExpectationsRouteV1
	if r.SharedConfig != nil && r.SharedConfig.ChangeListID != "" {
		urlPath = fmt.Sprintf("%s?issue=%s&crs=%s", urlPath, url.QueryEscape(r.SharedConfig.ChangeListID), url.QueryEscape(r.SharedConfig.CodeReviewSystem))
	}

	u := r.GoldURL + urlPath
	jsonBytes, err := getWithRetries(httpClient, u)
	if err != nil {
		return skerr.Wrapf(err, "getting expectations from %s (with retries)", u)
	}

	exp := &baseline.Baseline{}

	if err := json.Unmarshal(jsonBytes, exp); err != nil {
		fmt.Printf("Fetched from %s\n", u)
		if len(jsonBytes) > 200 {
			fmt.Printf(`Invalid JSON: "%s..."`, string(jsonBytes[0:200]))
		} else {
			fmt.Printf(`Invalid JSON: "%s"`, string(jsonBytes))
		}
		return skerr.Wrapf(err, "parsing JSON; this sometimes means auth issues")
	}

	r.Expectations = exp.Expectations
	return nil
}

// getResultFilePath returns that path in GCS where the result file should be stored.
//
// The path follows the path described here:
//    https://github.com/google/skia-buildbot/blob/master/golden/docs/INGESTION.md
// The file name of the path also contains a timestamp to make it unique since all
// calls within the same test run are written to the same output path.
func (r *resultState) getResultFilePath(now time.Time) string {
	year, month, day := now.Date()
	hour := now.Hour()

	// Assemble a path that looks like this:
	// <path_prefix>/YYYY/MM/DD/HH/<git_hash_or_cl>/<job_id>/<per_run_file_name>.json
	// The first segments up to 'HH' are required so the Gold ingester can scan these prefixes for
	// new files. The later segments are necessary to make the path unique within the runs of one
	// hour and increase readability of the paths for troubleshooting.
	// It is vital that the time segments of the path are based on UTC location.
	fileName := fmt.Sprintf("dm-%d.json", now.UnixNano())
	jobID := r.SharedConfig.TryJobID
	if jobID == "" {
		jobID = r.SharedConfig.Builder
		if jobID == "" {
			jobID = "waterfall"
		}
	}
	gitHashOrCL := r.SharedConfig.GitHash
	if r.SharedConfig.ChangeListID != "" {
		gitHashOrCL = fmt.Sprintf("%s_%s_%d", r.SharedConfig.ChangeListID, r.SharedConfig.PatchSetID, r.SharedConfig.PatchSetOrder)
	}
	segments := []interface{}{
		jsonPrefix,
		year,
		month,
		day,
		hour,
		gitHashOrCL,
		jobID,
		fileName}
	path := fmt.Sprintf("%s/%04d/%02d/%02d/%02d/%s/%s/%s", segments...)

	if r.SharedConfig.ChangeListID != "" {
		path = "trybot/" + path
	}
	return fmt.Sprintf("%s/%s", r.Bucket, path)
}

// getGCSImagePath returns the path in GCS where the image with the given hash should be stored.
func (r *resultState) getGCSImagePath(imgHash types.Digest) string {
	return fmt.Sprintf("%s%s/%s/%s.png", gcsPrefix, r.Bucket, imagePrefix, imgHash)
}

// loadStateFromJSON loads a serialization of a resultState instance that was previously written
// via the save method.
func loadStateFromJSON(fileName string) (*resultState, error) {
	ret := &resultState{}
	exists, err := loadJSONFile(fileName, ret)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, skerr.Fmt("The state file %q doesn't exist.", fileName)
	}
	return ret, nil
}
