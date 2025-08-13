package goldclient

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/jsonutils"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/gold-client/go/imgmatching"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/web/frontend"
)

const (
	// stateFile is the name of the file that holds the state in the work directory
	// between calls
	stateFile = "result-state.json"

	// jsonTempFile is the temporary file that is created to upload results via gsutil.
	jsonTempFile = "dm.json"

	// digestsDirectory is the directory inside the work directory in which digests downloaded from
	// GCS will be cached.
	digestsDirectory = "digests"

	// CombineInexactMatches is a key that can be passed to Test() as an optional key. If its
	// value is "1", and if an inexact match is found, the digest of the closest image will be
	// reported to Gold instead of the digest of the new image. This is to avoid adding many
	// unique images to the corpus for tests that are known to be flaky.
	CombineInexactMatches = "combine-inexact-matches"
)

// GoldClient is the uniform interface to communicate with the Gold service.
type GoldClient interface {
	// SetSharedConfig populates the config with details that will be shared
	// with all tests. This is safe to be called more than once, although
	// new settings will overwrite the old ones. This will cause the
	// baseline and known hashes to be (re-)downloaded from Gold.
	SetSharedConfig(ctx context.Context, sharedConfig jsonio.GoldResults, skipValidation bool) error

	// Test adds a test result to the current test run.
	//
	// The provided image will be uploaded to GCS if the hash of its pixels has not been seen before.
	// Using auth.SetDryRun(true) can prevent this. If imgDigest is provided, that hash will
	// override the built-in hashing. If imgDigest is provided and imgFileName is not, it is assumed
	// that the image has already been uploaded to GCS.
	//
	// additionalKeys is an optional set of key/value pairs that apply to only this test. This is
	// typically a small amount of data (and can be nil). If there are many keys, they are likely
	// shared between tests and should be added in SetSharedConfig.
	//
	// optionalKeys can be used to specify a non-exact image matching algorithm and its parameters,
	// and any other optional keys specific to this test. This is optional and can be nil.
	// TODO(lovisolo): Explicitly mention key used to specify a non-exact image matching algorithm.
	//
	// If the GoldClient is configured for pass/fail mode, a JSON file will be uploaded to GCS with
	// the test results, and the returned boolean will indicate whether the test passed the
	// comparison against the baseline using the specified non-exact image matching algorithm, or
	// exact matching if none is specified.
	//
	// If the GoldClient is *not* configured for pass/fail mode, no JSON will be uploaded, and Test
	// will return true.
	//
	// An error is only returned if there was a technical problem in processing the test.
	Test(ctx context.Context, name types.TestName, imgFileName string, imgDigest types.Digest, additionalKeys, optionalKeys map[string]string) (bool, error)

	// Check operates similarly to Test, except it does not persist anything about the call. That is,
	// the image will not be uploaded to Gold, only compared against the baseline.
	//
	// Argument optionalKeys is used to specify a non-exact image matching algorithm and its
	// parameters. TODO(lovisolo): Explicitly mention the optional key used for this.
	//
	// Argument keys is required if a non-exact image matching algorithm is specified. The test keys
	// are used to compute the ID of the trace from which to retrieve the most recent positive image
	// to be used as the basis of the non-exact image comparison.
	//
	// If both keys and optionalKeys are empty, an exact comparison will be carried out against the
	// baseline.
	Check(ctx context.Context, name types.TestName, imgFileName string, keys, optionalKeys map[string]string) (bool, error)

	// Diff computes a diff of the closest image to the given image file and puts it into outDir,
	// along with the closest image file itself.
	Diff(ctx context.Context, grouping paramtools.Params, imgFileName, outDir string) error

	// Finalize uploads the JSON file for all Test() calls previously seen.
	// A no-op if configured for pass/fail mode, since the JSON would have been uploaded
	// on the calls to Test().
	Finalize(ctx context.Context) error

	// Whoami makes a request to Gold's /json/v1/whoami endpoint and returns the email address in the
	// response. For debugging purposes only.
	Whoami(ctx context.Context) (string, error)

	// TriageAsPositive triages the given digest for the given test as positive by making a request
	// to Gold's /json/v2/triage endpoint. The image matching algorithm name will be used as the author
	// of the triage operation.
	TriageAsPositive(ctx context.Context, testName types.TestName, digest types.Digest, algorithmName string) error

	// MostRecentPositiveDigest retrieves the most recent positive digest for the given trace via
	// Gold's /json/v2/latestpositivedigest/{traceId} endpoint.
	MostRecentPositiveDigest(ctx context.Context, traceId tiling.TraceIDV2) (types.Digest, error)
}

// GoldClientDebug contains some "optional" methods that can assist
// in debugging.
type GoldClientDebug interface {
	// DumpBaseline returns a human-readable representation of the baseline as a string.
	// This is a set of test names that each have a set of image
	// digests that each have exactly one types.Label.
	DumpBaseline() (string, error)
	// DumpKnownHashes returns a human-readable representation of the known image digests
	// which is a list of hashes.
	DumpKnownHashes() (string, error)
}

// CloudClient implements the GoldClient interface for the remote Gold service.
type CloudClient struct {
	// workDir is a temporary directory that has to exist between related calls
	workDir string

	// resultState keeps track of the all the information to generate and upload a valid result.
	resultState *resultState

	// these functions are overwritable by tests
	loadAndHashImage func(path string) ([]byte, types.Digest, error)

	// groupingParamKeysByCorpus are the param keys by which digests on each corpus are grouped.
	groupingParamKeysByCorpus map[string][]string
}

// GoldClientConfig is a config structure to configure GoldClient instances
type GoldClientConfig struct {
	// WorkDir is a temporary directory that caches data for one run with multiple calls to GoldClient
	WorkDir string

	// InstanceID is the id of the backend Gold instance
	InstanceID string

	// PassFailStep indicates whether each call to Test(...) should return a pass/fail value.
	PassFailStep bool

	// FailureFile is a file on disk that will contain newline-separated links to triage
	// any failures. Only written to if PassFailStep is true
	FailureFile string

	// OverrideGoldURL is optional and overrides the formulaic GoldURL (typically legacy instances).
	OverrideGoldURL string

	// OverrideBucket is optional and overrides the formulaic GCS Bucket.
	OverrideBucket string

	// UploadOnly is a mode where we don't check expectations against the server - i.e.
	// we just operate in upload mode.
	UploadOnly bool
}

// NewCloudClient returns an implementation of the GoldClient that relies on the Gold service.
// If a new instance is created for each call to Test, the arguments of the first call are
// preserved. They are cached in a JSON file in the work directory.
func NewCloudClient(config GoldClientConfig) (*CloudClient, error) {
	// Make sure the workdir was given and exists.
	if config.WorkDir == "" {
		return nil, skerr.Fmt("no 'workDir' provided to NewCloudClient")
	}

	workDir, err := fileutil.EnsureDirExists(config.WorkDir)
	if err != nil {
		return nil, skerr.Wrapf(err, "setting up workdir %q", config.WorkDir)
	}

	if config.InstanceID == "" {
		return nil, skerr.Fmt("empty config passed into NewCloudClient")
	}

	ret := CloudClient{
		workDir:          workDir,
		loadAndHashImage: loadAndHashImage,
		resultState:      newResultState(jsonio.GoldResults{}, &config),
	}

	// write it to disk
	if err := saveJSONFile(ret.getResultStatePath(), ret.resultState); err != nil {
		return nil, skerr.Wrapf(err, "writing the state to disk")
	}

	return &ret, nil
}

// LoadCloudClient returns a GoldClient that has previously been stored to disk
// in the path given by workDir.
func LoadCloudClient(workDir string) (*CloudClient, error) {
	// Make sure the workdir was given and exists.
	if workDir == "" {
		return nil, skerr.Fmt("No 'workDir' provided to LoadCloudClient")
	}
	ret := CloudClient{
		workDir:          workDir,
		loadAndHashImage: loadAndHashImage,
	}
	var err error
	ret.resultState, err = loadStateFromJSON(ret.getResultStatePath())
	if err != nil {
		return nil, skerr.Wrapf(err, "loading state from disk")
	}

	return &ret, nil
}

// loadAndHashImage loads an image from disk and hashes the internal Pixel buffer. It returns
// the bytes of the encoded image and the MD5 hash of the pixels as hex encoded string.
func loadAndHashImage(fileName string) ([]byte, types.Digest, error) {
	// Load the image and save the bytes because we need to return them.
	imgBytes, err := os.ReadFile(fileName)
	if err != nil {
		return nil, "", skerr.Wrapf(err, "loading file %s", fileName)
	}
	img, err := png.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return nil, "", skerr.Wrapf(err, "decoding PNG in file %s", fileName)
	}
	nrgbaImg := diff.GetNRGBA(img)
	// hash it
	s := md5.Sum(nrgbaImg.Pix)
	md5Hash := hex.EncodeToString(s[:])
	return imgBytes, types.Digest(md5Hash), nil
}

// SetSharedConfig implements the GoldClient interface.
func (c *CloudClient) SetSharedConfig(ctx context.Context, sharedConfig jsonio.GoldResults, skipValidation bool) error {
	if !skipValidation {
		if err := sharedConfig.Validate(); err != nil {
			return skerr.Wrapf(err, "invalid configuration")
		}
	}

	existingConfig := GoldClientConfig{
		WorkDir: c.workDir,
	}
	if c.resultState != nil {
		existingConfig.FailureFile = c.resultState.FailureFile
		existingConfig.InstanceID = c.resultState.InstanceID
		existingConfig.OverrideBucket = c.resultState.Bucket
		existingConfig.OverrideGoldURL = c.resultState.GoldURL
		existingConfig.PassFailStep = c.resultState.PerTestPassFail
		existingConfig.UploadOnly = c.resultState.UploadOnly
	}
	c.resultState = newResultState(sharedConfig, &existingConfig)

	if !c.resultState.UploadOnly {
		// The GitHash may have changed (or been set for the first time),
		// So we can now load the baseline. We can also download the hashes
		// at this time, although we could have done it at any time before since
		// that does not depend on the GitHash we have.
		if err := c.downloadHashesAndBaselineFromGold(ctx); err != nil {
			return skerr.Wrapf(err, "downloading from Gold")
		}
	}

	return saveJSONFile(c.getResultStatePath(), c.resultState)
}

// Test implements the GoldClient interface.
func (c *CloudClient) Test(ctx context.Context, name types.TestName, imgFileName string, imgDigest types.Digest, additionalKeys, optionalKeys map[string]string) (bool, error) {
	passes, err := c.addTest(ctx, name, imgFileName, imgDigest, additionalKeys, optionalKeys)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	// In pass-fail (aka streaming mode), we want to make sure we don't upload these same results
	// in the next upload state. As such, we delete them and don't persist them to disk.
	if c.resultState.PerTestPassFail {
		c.resultState.SharedConfig.Results = nil
	}
	return passes, saveJSONFile(c.getResultStatePath(), c.resultState)
}

// addTest adds a test to results. If perTestPassFail is true it will also upload the result.
// Returns true if the test was added (and maybe uploaded) successfully.
func (c *CloudClient) addTest(
	ctx context.Context,
	name types.TestName,
	imgFileName string,
	imgDigest types.Digest,
	additionalKeys,
	optionalKeys map[string]string) (bool, error) {
	// Get an uploader. This is either based on an authenticated client or on gsutils.
	uploader := extractGCSUploader(ctx)

	var imgBytes []byte
	if imgFileName != "" {
		// Load the PNG from disk and hash it.
		b, imgHash, err := c.loadAndHashImage(imgFileName)
		if err != nil {
			return false, skerr.Wrap(err)
		}
		imgBytes = b
		// If a digest has been supplied, we'll use that. Otherwise, we'll use the hash we computed
		// ourselves when loading the image.
		if imgDigest == "" {
			imgDigest = imgHash
		}
	}

	// Add the result of this test.
	traceParams, traceID := c.addResult(name, imgDigest, additionalKeys, optionalKeys)

	// Check that the trace params include the keys needed by the corpus' grouping, and fail early
	// if they do not.
	//
	// We will use the returned grouping to generate links to the digest details page if necessary.
	grouping, err := c.groupingForTrace(ctx, traceParams)
	if err != nil {
		return false, skerr.Wrapf(err, "computing grouping for test %q", name)
	}

	// At this point the result should be correct for uploading.
	if err := c.resultState.SharedConfig.Validate(); err != nil {
		return false, skerr.Wrapf(err, "invalid test config")
	}

	infof(ctx, "Given image with hash %s for test %s\n", imgDigest, name)
	for expectHash, expectLabel := range c.resultState.Expectations[name] {
		infof(ctx, "Expectation for test: %s (%s)\n", expectHash, expectLabel)
	}

	var egroup errgroup.Group
	// Check against known hashes and upload if needed.
	if !c.resultState.KnownHashes[imgDigest] && imgBytes != nil {
		egroup.Go(func() error {
			gcsImagePath := c.resultState.getGCSImagePath(imgDigest)
			if err := uploader.UploadBytes(ctx, imgBytes, imgFileName, gcsImagePath); err != nil {
				return skerr.Fmt("Error uploading image %s to %s. Got: %s", imgFileName, gcsImagePath, err)
			}
			return nil
		})
	}

	// If we do per test pass/fail then upload the result and compare it to the baseline.
	ret := true
	if c.resultState.PerTestPassFail {
		ret, err = c.handlePassFailComparison(ctx, name, traceID, &imgBytes, imgDigest, optionalKeys, grouping)
	}

	if err := egroup.Wait(); err != nil {
		return false, err
	}
	return ret, nil
}

// handlePassFailComparison is a helper to handle the addTest logic when PerTestPassFail is set.
// Returns true if the image comparison was successful.
func (c *CloudClient) handlePassFailComparison(
	ctx context.Context,
	name types.TestName,
	traceID tiling.TraceIDV2,
	imgBytes *[]byte,
	imgDigest types.Digest,
	optionalKeys map[string]string,
	grouping paramtools.Params) (bool, error) {

	algorithmName, _, err := imgmatching.MakeMatcher(optionalKeys)
	if err != nil {
		return false, skerr.Wrapf(err, "parsing image matching algorithm from optional keys")
	}

	use_inexact_matching := algorithmName != imgmatching.ExactMatching
	combine, ok := optionalKeys[CombineInexactMatches]
	combine_results := ok && combine == "1"

	if use_inexact_matching && combine_results {
		return c.handleInexactComparisonWithCombinedMatches(ctx, name, traceID, imgBytes, imgDigest, optionalKeys, grouping)
	}
	return c.handleStandardComparison(ctx, name, traceID, imgBytes, imgDigest, optionalKeys, grouping)
}

// handleStandardComparison is a helper to handle the handlePassFailComparison logic that is used
// in the vast majority of comparisons. The result is uploaded, and if inexact matching is used,
// successful matches will cause the new image to be triaged as positive. Returns true if the
// image comparison was successful.
func (c *CloudClient) handleStandardComparison(
	ctx context.Context,
	name types.TestName,
	traceID tiling.TraceIDV2,
	imgBytes *[]byte,
	imgDigest types.Digest,
	optionalKeys map[string]string,
	grouping paramtools.Params) (bool, error) {

	var egroup errgroup.Group
	egroup.Go(func() error {
		return c.uploadResultJSON(ctx)
	})

	ret := true
	egroup.Go(func() error {
		match, algorithmName, _, err := c.matchImageAgainstBaseline(ctx, name, traceID, *imgBytes, imgDigest, optionalKeys)
		if err != nil {
			return skerr.Wrapf(err, "matching image against baseline")
		}
		ret = match

		// If the image is untriaged, but matches the latest positive digest in its baseline via the
		// specified non-exact image matching algorithm, then triage the image as positive.
		if match && algorithmName != imgmatching.ExactMatching {
			infof(ctx, "Triaging digest %q for test %q as positive (algorithm name: %q)\n", imgDigest, name, algorithmName)
			err = c.TriageAsPositive(ctx, name, imgDigest, string(algorithmName))
			if err != nil {
				return skerr.Wrapf(err, "triaging image as positive, image hash %q, test name %q, algorithm name %q", imgDigest, name, algorithmName)
			}
		}

		if !match {
			err = c.reportTriageLink(ctx, imgDigest, grouping)
			if err != nil {
				return skerr.Wrapf(err, "reporting triage link")
			}
		}

		return nil
	})

	if err := egroup.Wait(); err != nil {
		return false, err
	}

	return ret, nil
}

// handleInexactComparisonWithCombinedMatches is a helper to handle the handlePassFailComparison
// logic that is used when inexact matching is used with the CombineInexactMatches optional key set
// to 1. This behaves similarly to handleStandardComparison, but in the event of a successful
// inexact comparison, the recent positive image that was compared against will be reported instead
// of the image that was actually produced. Returns true if the image comparison was successful.
func (c *CloudClient) handleInexactComparisonWithCombinedMatches(
	ctx context.Context,
	name types.TestName,
	traceID tiling.TraceIDV2,
	imgBytes *[]byte,
	imgDigest types.Digest,
	optionalKeys map[string]string,
	grouping paramtools.Params) (bool, error) {

	// We cannot do work asynchronously from the start in this code path like we
	// can with other comparisons since the result we report will be dependent on
	// whether the inexact comparison succeeds or not.

	match, algorithmName, mostRecentPositiveDigest, err := c.matchImageAgainstBaseline(ctx, name, traceID, *imgBytes, imgDigest, optionalKeys)
	if err != nil {
		wrapped_err := skerr.Wrapf(err, "matching image against baseline")
		err = c.uploadResultJSON(ctx)
		if err != nil {
			wrapped_err = skerr.Wrapf(wrapped_err, "uploading result json")
		}
		return false, wrapped_err
	}

	// If the image matched and we didn't use exact matching, then we report that
	// the image we compared against was produced.
	if match && algorithmName != imgmatching.ExactMatching {
		infof(ctx, "Reporting digest %q instead of %q because %q is enabled\n", mostRecentPositiveDigest, imgDigest, CombineInexactMatches)
		index := len(c.resultState.SharedConfig.Results) - 1
		c.resultState.SharedConfig.Results[index].Digest = mostRecentPositiveDigest
	}

	// At this point we can safely upload asynchronously.
	var egroup errgroup.Group
	egroup.Go(func() error {
		return c.uploadResultJSON(ctx)
	})

	if !match {
		egroup.Go(func() error {
			err = c.reportTriageLink(ctx, imgDigest, grouping)
			if err != nil {
				return skerr.Wrapf(err, "reporting triage link")
			}
			return nil
		})
	}

	if err := egroup.Wait(); err != nil {
		return false, err
	}

	return match, nil
}

// reportTriageLink constructs and reports a triage link for the given digest and grouping. This
// link is reported via both logging and the failure file.
func (c *CloudClient) reportTriageLink(
	ctx context.Context,
	imgDigest types.Digest,
	grouping paramtools.Params) error {

	link := fmt.Sprintf("%s/detail?grouping=%s&digest=%s", c.resultState.GoldURL, url.QueryEscape(urlEncode(grouping)), imgDigest)
	if c.resultState.SharedConfig.ChangelistID != "" {
		link += "&changelist_id=" + c.resultState.SharedConfig.ChangelistID
		link += "&crs=" + c.resultState.SharedConfig.CodeReviewSystem
	}
	link += "\n"
	infof(ctx, "Untriaged or negative image: %s\n", link)
	ff := c.resultState.FailureFile
	if ff != "" {
		f, err := os.OpenFile(ff, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return skerr.Fmt("could not open failure file %s: %s", ff, err)
		}
		if _, err := f.WriteString(link); err != nil {
			_ = f.Close() // Write error is more important.
			return skerr.Fmt("could not write to failure file %s: %s", ff, err)
		}
		if err := f.Close(); err != nil {
			return skerr.Fmt("could not close failure file %s: %s", ff, err)
		}
	}
	return nil
}

// groupingForTrace returns the grouping for the given trace. It fails if the trace params do not
// include all the keys needed by the corpus' grouping.
func (c *CloudClient) groupingForTrace(ctx context.Context, traceParams paramtools.Params) (paramtools.Params, error) {
	corpus, ok := traceParams[types.CorpusField]
	if !ok {
		return nil, skerr.Fmt("trace params must include key %q, got: %v", types.CorpusField, traceParams)
	}
	groupings, err := c.getGroupings(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "retrieving groupings")
	}
	groupingParams, ok := groupings[corpus]
	if !ok {
		return nil, skerr.Fmt("grouping params for corpus %q are unknown; known grouping params: %v", corpus, groupings)
	}
	grouping := paramtools.Params{}
	for _, param := range groupingParams {
		grouping[param], ok = traceParams[param]
		if !ok {
			return nil, skerr.Fmt("trace params must include key %q, got: %v", param, traceParams)
		}
	}
	return grouping, nil
}

// getGroupings returns the param keys by which digests on each corpus are grouped.
func (c *CloudClient) getGroupings(ctx context.Context) (map[string][]string, error) {
	if c.groupingParamKeysByCorpus == nil {
		endpointUrl := c.resultState.GoldURL + "/json/v1/groupings"

		jsonBytes, err := getWithRetries(ctx, endpointUrl)
		if err != nil {
			return nil, skerr.Wrapf(err, "making request to %s", endpointUrl)
		}

		groupingsResponse := frontend.GroupingsResponse{}
		if err := json.Unmarshal(jsonBytes, &groupingsResponse); err != nil {
			return nil, skerr.Wrapf(err, "unmarshalling JSON response from %s", endpointUrl)
		}

		c.groupingParamKeysByCorpus = groupingsResponse.GroupingParamKeysByCorpus
	}

	return c.groupingParamKeysByCorpus, nil
}

// Check implements the GoldClient interface.
func (c *CloudClient) Check(ctx context.Context, name types.TestName, imgFileName string, keys, optionalKeys map[string]string) (bool, error) {
	if len(c.resultState.Expectations) == 0 {
		if err := c.downloadHashesAndBaselineFromGold(ctx); err != nil {
			return false, skerr.Wrapf(err, "fetching baseline")
		}
		if err := saveJSONFile(c.getResultStatePath(), c.resultState); err != nil {
			return false, skerr.Wrapf(err, "writing the expectations to disk")
		}
	}
	if len(c.resultState.Expectations) == 0 {
		return false, skerr.Fmt("Expectations are empty, despite re-loading them from Gold")
	}

	// Load the PNG from disk and hash it.
	imgBytes, imgHash, err := c.loadAndHashImage(imgFileName)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	infof(ctx, "Given image with hash %s for test %s\n", imgHash, name)
	for expectHash, expectLabel := range c.resultState.Expectations[name] {
		infof(ctx, "Expectation for test: %s (%s)\n", expectHash, expectLabel)
	}

	_, _, traceID := c.makeResultKeyAndTraceParamsAndID(name, keys)
	match, _, _, err := c.matchImageAgainstBaseline(ctx, name, traceID, imgBytes, imgHash, optionalKeys)
	return match, skerr.Wrap(err)
}

// matchImageAgainstBaseline matches the given image against the baseline. A non-exact image
// matching algorithm will be used if one is specified via the optionalKeys; otherwise exact
// matching will be used (i.e. the image's MD5 hash will be matched against the hashes of the
// baseline images labeled as positive).
//
// It assumes that the baseline has already been downloaded from Gold.
//
// Returns true if the image matches the baseline, or false otherwise.
//
// Returns the algorithm name used to determine the match. If the image's digest was labeled as
// either positive or negative in the baseline, imgmatching.ExactMatching will be returned
// regardless of whether a non-exact image matching algorithm was specified via the optionalKeys.
//
// If the algorithm is actually run, the digest of the image that was used for comparison is also
// returned.
//
// A non-nil error is returned if there are any problems parsing or instantiating the specified
// image matching algorithm, for example if there are any missing parameters.
func (c *CloudClient) matchImageAgainstBaseline(
	ctx context.Context,
	testName types.TestName,
	traceId tiling.TraceIDV2,
	imageBytes []byte,
	imageHash types.Digest,
	optionalKeys map[string]string) (bool, imgmatching.AlgorithmName, types.Digest, error) {
	// First we check whether the digest is a known positive or negative, regardless of the specified
	// image matching algorithm.
	if c.resultState.Expectations[testName][imageHash] == expectations.Positive {
		return true, imgmatching.ExactMatching, "", nil
	}
	if c.resultState.Expectations[testName][imageHash] == expectations.Negative {
		return false, imgmatching.ExactMatching, "", nil
	}

	// Extract the specified image matching algorithm from the optionalKeys (defaulting to exact
	// matching if none is specified) and obtain an instance of the imgmatching.Matcher if the
	// algorithm requires one (i.e. all but exact matching).
	algorithmName, matcher, err := imgmatching.MakeMatcher(optionalKeys)
	if err != nil {
		return false, "", "", skerr.Wrapf(err, "parsing image matching algorithm from optional keys")
	}

	// Nothing else to do if performing exact matching: we've already checked whether the image is a
	// known positive.
	if algorithmName == imgmatching.ExactMatching {
		return false, algorithmName, "", nil
	}

	// This can happen if a user supplied just the hash and not the image itself.
	if len(imageBytes) == 0 {
		return false, "", "", skerr.Fmt("Must supply the image if using a non-exact matching algorithm")
	}

	// Decode test output PNG image.
	img, err := png.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return false, "", "", skerr.Wrapf(err, "decoding PNG image")
	}

	// Fetch the most recent positive digest.
	infof(ctx, "Fetching most recent positive digest for trace with ID %q.\n", traceId)
	mostRecentPositiveDigest, err := c.MostRecentPositiveDigest(ctx, traceId)
	if err != nil {
		return false, "", "", skerr.Wrapf(err, "retrieving most recent positive image")
	}

	// Download from GCS the image corresponding to the most recent positive digest.
	var mostRecentPositiveImage image.Image // Will be nil if no existing positive image is found.
	if mostRecentPositiveDigest == tiling.MissingDigest {
		infof(ctx, "No recent positive digests for trace with ID %q. This probably means that the test was newly added.\n", traceId)
	} else {
		mostRecentPositiveImage, _, err = c.getDigestFromCacheOrGCS(ctx, mostRecentPositiveDigest)
		if err != nil {
			return false, "", "", skerr.Wrapf(err, "downloading most recent positive image from GCS")
		}
	}

	// Return algorithm's output.
	infof(ctx, "Non-exact image comparison using algorithm %q against most recent positive digest %q.\n", algorithmName, mostRecentPositiveDigest)
	return matcher.Match(mostRecentPositiveImage, img), algorithmName, mostRecentPositiveDigest, nil
}

// Finalize implements the GoldClient interface.
func (c *CloudClient) Finalize(ctx context.Context) error {
	return c.uploadResultJSON(ctx)
}

// uploadResultJSON uploads the results (which live in SharedConfig, specifically
// SharedConfig.Results), to GCS.
func (c *CloudClient) uploadResultJSON(ctx context.Context) error {
	localFileName := filepath.Join(c.workDir, jsonTempFile)
	resultFilePath := c.resultState.getResultFilePath(ctx)
	uploader := extractGCSUploader(ctx)
	if err := uploader.UploadJSON(ctx, c.resultState.SharedConfig, localFileName, resultFilePath); err != nil {
		return skerr.Fmt("Error uploading JSON file to GCS path %s: %s", resultFilePath, err)
	}
	return nil
}

// getResultStatePath returns the path of the temporary file where the state is cached as JSON
func (c *CloudClient) getResultStatePath() string {
	return filepath.Join(c.workDir, stateFile)
}

// addResult adds the given test to the overall results and returns the params and ID of the
// affected trace.
func (c *CloudClient) addResult(name types.TestName, imgHash types.Digest, additionalKeys, optionalKeys map[string]string) (paramtools.Params, tiling.TraceIDV2) {
	resultKey, traceParams, traceID := c.makeResultKeyAndTraceParamsAndID(name, additionalKeys)

	newResult := jsonio.Result{
		Digest: imgHash,
		Key:    resultKey,

		// We need to specify this is a png, otherwise the backend will refuse
		// to ingest it.
		Options: map[string]string{"ext": "png"},
	}
	for k, v := range optionalKeys {
		newResult.Options[k] = v
	}

	c.resultState.SharedConfig.Results = append(c.resultState.SharedConfig.Results, newResult)

	return traceParams, traceID
}

// makeResultKeyAndTraceParamsAndID computes the key for a jsonio.Result and the params and ID of
// the affected trace.
func (c *CloudClient) makeResultKeyAndTraceParamsAndID(name types.TestName, additionalKeys map[string]string) (map[string]string, paramtools.Params, tiling.TraceIDV2) {
	// Retrieve the shared keys given the via the "imgtest init" command with the --keys-file flag.
	// If said command was not previously invoked (e.g. when calling "imgtest check") the shared
	// config will be nil, in which case we initialize the shared keys as an empty map.
	sharedKeys := map[string]string{}
	if len(c.resultState.SharedConfig.Key) != 0 {
		sharedKeys = c.resultState.SharedConfig.Key
	}

	// Populate the result key with the test name and any test-specific keys.
	resultKey := map[string]string{types.PrimaryKeyField: string(name)}
	for k, v := range additionalKeys {
		resultKey[k] = v
	}

	// Set the "source_type" field to the instance ID if it's not specified either via the
	// test-specific keys or the shared keys.
	if resultKey[types.CorpusField] == "" && sharedKeys[types.CorpusField] == "" {
		resultKey[types.CorpusField] = c.resultState.InstanceID
	}

	// Compute trace ID from shared and test-specific keys. The latter overwrites the former in the
	// rare case of a conflict.
	traceParams := paramtools.Params{}
	traceParams.Add(sharedKeys, resultKey)

	return resultKey, traceParams, createTraceIDV2(traceParams)
}

// createTraceIDV2 encodes the params to JSON and then hex encodes it to create a traceIDV2.
// We do not use the code in the sql package to avoid bringing a lot of extra dependencies into this
// executable (but the fact that they match is covered by unit tests).
func createTraceIDV2(keys paramtools.Params) tiling.TraceIDV2 {
	h := md5.Sum(jsonutils.MarshalStringMap(keys))
	return tiling.TraceIDV2(hex.EncodeToString(h[:]))
}

// downloadHashesAndBaselineFromGold downloads the hashes and baselines
// and stores them to resultState.
func (c *CloudClient) downloadHashesAndBaselineFromGold(ctx context.Context) error {
	// What hashes have we seen already (to avoid uploading them again).
	if err := c.resultState.loadKnownHashes(ctx); err != nil {
		return skerr.Wrap(err)
	}

	infof(ctx, "Loaded %d known hashes\n", len(c.resultState.KnownHashes))

	// Fetch the baseline
	if err := c.resultState.loadExpectations(ctx); err != nil {
		return skerr.Wrap(err)
	}
	infof(ctx, "Loaded %d tests from the baseline\n", len(c.resultState.Expectations))

	return nil
}

// Diff fulfills the GoldClient interface.
func (c *CloudClient) Diff(ctx context.Context, grouping paramtools.Params, imgFileName, outDir string) error {
	if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
		return skerr.Wrapf(err, "creating outdir %s", outDir)
	}

	// 1) Read in file, write it to outdir/ using correct hash.
	b, inputDigest, err := c.loadAndHashImage(imgFileName)
	if err != nil {
		return skerr.Wrapf(err, "reading input %s", imgFileName)
	}

	origFilePath := filepath.Join(outDir, fmt.Sprintf("input-%s.png", inputDigest))
	if err := os.WriteFile(origFilePath, b, 0644); err != nil {
		return skerr.Wrapf(err, "writing to %s", origFilePath)
	}

	leftImg, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return skerr.Wrapf(err, "reading %s as png", origFilePath)
	}

	// 2) Check JSON endpoint digests to download
	encodedGrouping := urlEncode(grouping)
	u := fmt.Sprintf("%s/json/v2/digests?grouping=%s", c.resultState.GoldURL, url.QueryEscape(encodedGrouping))
	jb, err := getWithRetries(ctx, u)
	if err != nil {
		return skerr.Wrapf(err, "reading images for grouping %#v from gold (url: %s)", grouping, u)
	}

	var dlr frontend.DigestListResponse
	if err := json.Unmarshal(jb, &dlr); err != nil {
		return skerr.Wrapf(err, "invalid JSON from digests served from %s: %s", u, string(jb))
	}
	if len(dlr.Digests) == 0 {
		errorf(ctx, "Gold doesn't know of any digests that grouping %#v\n", grouping)
		return nil
	}

	infof(ctx, "Going to compare %s.png against %d other images\n", inputDigest, len(dlr.Digests))

	// 3a) Download those from bucket (or use from working directory cache). We download them with
	//    the same credentials that let us upload them.
	smallestCombined := float32(math.MaxFloat32)
	var closestRightDigest types.Digest

	// we have to keep the raw bytes (i.e. we cannot simply re-encoded closestDiffImg to disk)
	// because golang does not support fancy image things like color spaces, so re-encoding it
	// would lose that data.
	var closestRightImg []byte
	var closestDiffImg image.Image
	for _, d := range dlr.Digests {
		rightImg, db, err := c.getDigestFromCacheOrGCS(ctx, d)
		if err != nil {
			return skerr.Wrap(err)
		}

		// 3b) Compare each of the images to the given image, looking for the smallest combined
		//     diff metric.
		dm, diffImg := diff.PixelDiff(leftImg, rightImg)
		combined := diff.CombinedDiffMetric(dm.MaxRGBADiffs, dm.PixelDiffPercent)
		if combined < smallestCombined {
			smallestCombined = combined
			closestDiffImg = diffImg
			closestRightImg = db
			closestRightDigest = d
		}
	}
	infof(ctx, "Digest %s was closest (combined metric of %f)\n", closestRightDigest, smallestCombined)

	// 4) Write closest image and the diff to that image to the output directory.
	o := filepath.Join(outDir, fmt.Sprintf("closest-%s.png", closestRightDigest))
	if err := os.WriteFile(o, closestRightImg, 0644); err != nil {
		return skerr.Wrapf(err, "writing closest image to %s", o)
	}

	o = filepath.Join(outDir, "diff.png")
	diffFile, err := os.Create(o)
	if err != nil {
		return skerr.Wrapf(err, "opening %s for writing", o)
	}
	if err := png.Encode(diffFile, closestDiffImg); err != nil {
		return skerr.Wrapf(err, "encoding diff image")
	}

	return skerr.Wrap(diffFile.Close())
}

// urlEncode returns the map as a url-encoded string.
func urlEncode(p map[string]string) string {
	values := make(url.Values, len(p))
	for key, value := range p {
		values[key] = []string{value}
	}
	return values.Encode()
}

// getDigestFromCacheOrGCS downloads from GCS the PNG file corresponding to the given digest, and
// returns the decoded image.Image and raw PNG file as a byte slice.
//
// The downloaded image is cached on disk. Subsequent calls for the same digest will load the
// cached image from disk.
func (c *CloudClient) getDigestFromCacheOrGCS(ctx context.Context, digest types.Digest) (image.Image, []byte, error) {
	// Make sure the local cache directory exists.
	cachePath := filepath.Join(c.workDir, digestsDirectory)
	if err := os.MkdirAll(cachePath, os.ModePerm); err != nil {
		return nil, nil, skerr.Wrapf(err, "creating digests directory %s", cachePath)
	}

	// Path where the digest should be cached.
	digestPath := filepath.Join(cachePath, string(digest)+".png")

	// Try to read digest from the local cache.
	digestBytes, err := os.ReadFile(digestPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, skerr.Wrapf(err, "reading file %s", digestPath)
	}

	// Download and cache digest if it wasn't found on the cache.
	if os.IsNotExist(err) {
		downloader := extractImageDownloader(ctx)

		// Download digest.
		digestGcsPath := c.resultState.getGCSImagePath(digest)
		digestBytes, err = downloader.DownloadImage(ctx, c.resultState.GoldURL, digest)
		if err != nil {
			return nil, nil, skerr.Wrapf(err, "downloading %s", digestGcsPath)
		}

		// Cache digest.
		if err := os.WriteFile(digestPath, digestBytes, 0644); err != nil {
			return nil, nil, skerr.Wrapf(err, "caching to %s", digestPath)
		}
	}

	// Decode PNG file.
	img, err := png.Decode(bytes.NewReader(digestBytes))
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "decoding PNG file at %s", digestPath)
	}

	return img, digestBytes, nil
}

// Whoami fulfills the GoldClient interface.
func (c *CloudClient) Whoami(ctx context.Context) (string, error) {
	jsonBytes, err := getWithRetries(ctx, c.resultState.GoldURL+"/json/v1/whoami")
	if err != nil {
		return "", skerr.Wrapf(err, "making request to %s/json/v1/whoami", c.resultState.GoldURL)
	}

	whoami := map[string]string{}
	if err := json.Unmarshal(jsonBytes, &whoami); err != nil {
		return "", skerr.Wrapf(err, "parsing JSON response from %s/json/v1/whoami", c.resultState.GoldURL)
	}

	email, ok := whoami["whoami"]
	if !ok {
		return "", skerr.Wrapf(err, `JSON response from %s/json/v1/whoami does not contain key "whoami"`, c.resultState.GoldURL)
	}

	return email, nil
}

// TriageAsPositive fulfills the GoldClient interface.
func (c *CloudClient) TriageAsPositive(ctx context.Context, testName types.TestName, digest types.Digest, algorithmName string) error {
	// Build TriageRequest struct and encode it into JSON.
	triageRequest := &frontend.TriageRequestV2{
		TestDigestStatus:       map[types.TestName]map[types.Digest]expectations.Label{testName: {digest: expectations.Positive}},
		CodeReviewSystem:       c.resultState.SharedConfig.CodeReviewSystem,
		ChangelistID:           c.resultState.SharedConfig.ChangelistID,
		ImageMatchingAlgorithm: algorithmName,
	}
	jsonTriageRequest, err := json.Marshal(triageRequest)
	if err != nil {
		return skerr.Wrapf(err, `encoding frontend.TriageRequest into JSON for test %q, digest %q, algorithm %q and CL %q`, testName, digest, algorithmName, c.resultState.SharedConfig.ChangelistID)
	}

	// Make /json/v1/triage request. Response is always empty.
	_, err = post(ctx, c.resultState.GoldURL+"/json/v2/triage", "application/json", bytes.NewReader(jsonTriageRequest))
	if err != nil {
		return skerr.Wrapf(err, `making POST request to %s/json/v2/triage for test %q, digest %q, algorithm %q and CL %q`, c.resultState.GoldURL, testName, digest, algorithmName, c.resultState.SharedConfig.ChangelistID)
	}

	return nil
}

// MostRecentPositiveDigest fulfills the GoldClient interface.
func (c *CloudClient) MostRecentPositiveDigest(ctx context.Context, id tiling.TraceIDV2) (types.Digest, error) {
	endpointUrl := c.resultState.GoldURL + "/json/v2/latestpositivedigest/" + string(id)

	jsonBytes, err := getWithRetries(ctx, endpointUrl)
	if err != nil {
		return "", skerr.Wrapf(err, "making request to %s", endpointUrl)
	}

	mostRecentPositiveDigest := frontend.MostRecentPositiveDigestResponse{}
	if err := json.Unmarshal(jsonBytes, &mostRecentPositiveDigest); err != nil {
		return "", skerr.Wrapf(err, "unmarshalling JSON response from %s", endpointUrl)
	}

	return mostRecentPositiveDigest.Digest, nil
}

// DumpBaseline fulfills the GoldClientDebug interface
func (c *CloudClient) DumpBaseline() (string, error) {
	if c.resultState == nil || c.resultState.Expectations == nil {
		return "", errors.New("Not instantiated - call init?")
	}
	return stringifyBaseline(c.resultState.Expectations), nil
}

func stringifyBaseline(b map[types.TestName]map[types.Digest]expectations.Label) string {
	names := make([]string, 0, len(b))
	for testName := range b {
		names = append(names, string(testName))
	}
	sort.Strings(names)
	s := strings.Builder{}
	for _, testName := range names {
		digestMap := b[types.TestName(testName)]
		digests := make([]string, 0, len(digestMap))
		for d := range digestMap {
			digests = append(digests, string(d))
		}
		sort.Strings(digests)
		_, _ = fmt.Fprintf(&s, "%s:\n", testName)
		for _, d := range digests {
			_, _ = fmt.Fprintf(&s, "\t%s : %s\n", d, digestMap[types.Digest(d)])
		}
	}
	return s.String()
}

// DumpKnownHashes fulfills the GoldClientDebug interface
func (c *CloudClient) DumpKnownHashes() (string, error) {
	if c.resultState == nil || c.resultState.KnownHashes == nil {
		return "", errors.New("Not instantiated - call init?")
	}
	var hashes []string
	for h := range c.resultState.KnownHashes {
		hashes = append(hashes, string(h))
	}
	sort.Strings(hashes)
	s := strings.Builder{}
	_, _ = s.WriteString("Hashes:\n\t")
	_, _ = s.WriteString(strings.Join(hashes, "\n\t"))
	_, _ = s.WriteString("\n")
	return s.String(), nil
}

func infof(ctx context.Context, fmtStr string, args ...interface{}) {
	w := extractLogWriter(ctx)
	_, _ = fmt.Fprintf(w, fmtStr, args...)
}

func errorf(ctx context.Context, fmtStr string, args ...interface{}) {
	w := extractErrorWriter(ctx)
	_, _ = fmt.Fprintf(w, fmtStr, args...)
}

// Make sure CloudClient fulfills the GoldClient interface
var _ GoldClient = (*CloudClient)(nil)

// Make sure CloudClient fulfills the GoldClientDebug interface
var _ GoldClientDebug = (*CloudClient)(nil)
