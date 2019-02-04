package bbstate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	bb_api "go.chromium.org/luci/common/api/buildbucket/buildbucket/v1"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/tryjobstore"
)

type bbServiceV1 struct {
	service       *bb_api.Service
	bucketName    string
	builderRegExp *regexp.Regexp
}

func newBuildBucketV1(httpClient *http.Client, buildBucketURL, bucketName string, builderRegExp *regexp.Regexp) (iBuildBucketSvc, error) {
	service, err := bb_api.New(httpClient)
	if err != nil {
		return nil, err
	}

	service.BasePath = buildBucketURL
	return &bbServiceV1{
		service:       service,
		bucketName:    bucketName,
		builderRegExp: builderRegExp,
	}, nil
}

// fetchBuild retrieves the build that corresponds to the given BuildBucket id
// and extracts the information into an instance of Tryjob.
// The first return value being nil, indicates that the build does not exist
// of was ignored for some reason.
func (b *bbServiceV1) Get(buildBucketID int64) (*tryjobstore.Tryjob, error) {
	// func (b *BuildBucketState) fetchBuild(buildBucketID int64) (*tryjobstore.Tryjob, error) {
	buildResp, err := b.service.Get(buildBucketID).Do()
	if err != nil {
		return nil, err
	}

	if buildResp == nil {
		return nil, fmt.Errorf("buildResp is nil. No result found.")
	}

	if buildResp.Build == nil {
		return nil, fmt.Errorf("Build information is nil. No result found.")
	}

	if (buildResp.Error != nil) && (buildResp.Error.Message != "") {
		return nil, fmt.Errorf("Unable to retrieve build %d. Got %s", buildBucketID, buildResp.Error.Message)
	}
	build := buildResp.Build

	// Parse the parameters encoded in the ParametersJson field.
	params := &tryjobstore.Parameters{}
	if err := json.Unmarshal([]byte(build.ParametersJson), params); err != nil {
		return nil, fmt.Errorf("Error unmarshalling params: %s", err)
	}

	// Check if this is a builder we can ignore.
	if b.ignoreBuild(build, params) {
		return nil, nil
	}

	// Extract the tryjob info.
	return getTryjobInfo(build, params)
}

func (b *bbServiceV1) Search(resultCh chan<- *tBuildInfo, timeWindow time.Duration) error {
	// // pollBuildBucket queries the BuildBucket instance from (now - timeWindow) to now.
	// func (b *BuildBucketState) searchForNewBuilds(buildsCh chan<- *bb_api.ApiCommonBuildMessage, timeWindow time.Duration) error {
	// Search over a specific time window.
	searchCall := b.service.Search()

	timeWindowStart := time.Now().Add(-timeWindow).UnixNano() / int64(time.Microsecond)
	searchCall.Bucket(b.bucketName).CreationTsLow(timeWindowStart)

	builds, _, err := searchCall.Fetch(0, nil)
	if err != nil {
		return skerr.Fmt("Error querying buildbucket: %s", err)
	}

	for _, build := range builds {
		bi := &tBuildInfo{Id: build.Id}
		switch build.Status {
		case buildbucket.STATUS_SCHEDULED:
			bi.Status = bbs_STATUS_SCHEDULED
		case buildbucket.STATUS_STARTED:
			bi.Status = bbs_STATUS_STARTED
		default:
			bi.Status = bbs_STATUS_OTHER
		}
		resultCh <- bi
	}
	return nil
}

// ignoreBuild is the central place to determine whether a build from
// BuildBucket should be ignored. For example, BuildBucket can contain build jobs
// that produce no test output.
func (b *bbServiceV1) ignoreBuild(build *bb_api.ApiCommonBuildMessage, params *tryjobstore.Parameters) bool {
	// If BuildResultDetails are there, then parse them and see if
	// resultDetails['properties']['skip_test'] exists and is true.
	// This will only apply to some clients, but there should not be any false positives.
	if build.ResultDetailsJson != "" {
		resultDetails := map[string]interface{}{}
		if err := json.Unmarshal([]byte(build.ResultDetailsJson), &resultDetails); err != nil {
			sklog.Errorf("Error unmarshalling generic JSON: %s", err)
		} else if props, ok := resultDetails["properties"].(map[string]interface{}); ok {
			// If skip_test exists and is a bool with value true we need to ignore this build.
			if val, ok := props["skip_test"].(bool); ok && val {
				return true
			}
		}
	}

	// Check whether the builder is ruled out by a regular expression.
	return !b.builderRegExp.Match([]byte(params.BuilderName))
}

// extractPatchsetRegex is used to extract the patchset ID from BuildBucket builds.
var extractPatchsetRegex = regexp.MustCompile(`^refs\/changes\/[0-9]*\/[0-9]*\/(?P<patchset>.+)$`)

// getTryjobInfo extracts tryjob information from the BuildBucket record.
// It translates the status of a BuildBucket build to the status defined for
// Tryjob instances in tryjobstore, which is richer in that it also captures
// whether a tryjob result has been ingested or not.
func getTryjobInfo(build *bb_api.ApiCommonBuildMessage, params *tryjobstore.Parameters) (*tryjobstore.Tryjob, error) {
	matchedGroups := extractPatchsetRegex.FindStringSubmatch(params.Properties.GerritPatchset)
	if len(matchedGroups) != 2 {
		return nil, fmt.Errorf("Unable to extract patchset info from '%s'", params.Properties.GerritPatchset)
	}

	patchsetID, err := strconv.ParseInt(matchedGroups[1], 10, 64)
	if err != nil {
		return nil, err
	}

	issueID := int64(params.Properties.GerritIssue)

	// Make sure the relevant ids are correct.
	if (issueID <= 0) || (patchsetID <= 0) {
		return nil, sklog.FmtErrorf("Invalid issue id (%d) or patchset id (%d).", issueID, patchsetID)
	}

	// Translate the two result fields into one for tryjobs.
	var status tryjobstore.TryjobStatus = tryjobstore.TRYJOB_UNKNOWN
	switch build.Status {
	case buildbucket.STATUS_SCHEDULED:
		status = tryjobstore.TRYJOB_SCHEDULED
	case buildbucket.STATUS_STARTED:
		status = tryjobstore.TRYJOB_RUNNING
	case buildbucket.STATUS_COMPLETED:
		switch build.Result {
		case buildbucket.RESULT_CANCELED:
			fallthrough
		case buildbucket.RESULT_FAILURE:
			status = tryjobstore.TRYJOB_FAILED
		case buildbucket.RESULT_SUCCESS:
			status = tryjobstore.TRYJOB_COMPLETE
		}
	}

	if status == tryjobstore.TRYJOB_UNKNOWN {
		return nil, fmt.Errorf("Unknown tryjob state. Got (status, result): (%s, %s)", build.Status, build.Result)
	}

	// UpdateTs is in micro seconds.
	// Note: Multiplying by time.Microsecond results in the correct number of nanoseconds.
	const microPerSec = int64(time.Second / time.Microsecond)
	updated := time.Unix(build.UpdatedTs/microPerSec, (build.UpdatedTs%microPerSec)*int64(time.Microsecond))
	ret := &tryjobstore.Tryjob{
		IssueID:       issueID,
		PatchsetID:    patchsetID,
		Builder:       params.BuilderName,
		BuildBucketID: build.Id,
		Updated:       updated,
		Status:        status,
	}

	return ret, nil
}
