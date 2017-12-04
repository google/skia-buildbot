package bbstate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "go.chromium.org/luci/buildbucket"
	bb_api "go.chromium.org/luci/common/api/buildbucket/buildbucket/v1"

	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/tryjobstore"
)

const (
	// DEFAULT_TIME_WINDOW = 45 * 24 * time.Hour // Time we look back at the build bucket
	DEFAULT_TIME_WINDOW   = 12 * time.Hour  // Time we look back at the build bucket
	POLL_INTERVAL         = 5 * time.Minute // Interval at which we poll buildbucket
	TEST_BUILDER_PREFIX   = "Test-"         // Prefix that identifies tryjobs that run tests/produce images.
	MAX_CONCURRENT_WRITES = 1000
)

// BuildBucketState captures all tryjobs we are currently interested in.
// These are the tryjobs that were run within the time windows and some that
// have been explicitly requested by users.
type BuildBucketState struct {
	service     *bb_api.Service
	tryjobStore tryjobstore.TryjobStore
	gerritAPI   *gerrit.Gerrit
}

func NewBuildBucketState(client *http.Client, tryjobStore tryjobstore.TryjobStore, gerritAPI *gerrit.Gerrit) (*BuildBucketState, error) {
	service, err := bb_api.New(client)
	if err != nil {
		return nil, err
	}
	service.BasePath = "https://cr-buildbucket.appspot.com/api/buildbucket/v1/"
	ret := &BuildBucketState{
		service:     service,
		tryjobStore: tryjobStore,
		gerritAPI:   gerritAPI,
	}
	ret.start()

	return ret, nil
}

// start continuously processes data it gets from buildbucket by polling.
func (t *BuildBucketState) start() error {
	// Create the channel that will receive the buildbot results.
	buildsCh := make(chan *bb_api.ApiCommonBuildMessage)
	workPermissions := make(chan bool, MAX_CONCURRENT_WRITES)

	// Process the builds produced by the poller.
	go func() {
		for build := range buildsCh {
			// Get work permission.
			workPermissions <- true

			go func(build *bb_api.ApiCommonBuildMessage) {
				// Give up work permission at the end.
				defer func() { <-workPermissions }()

				// Parse the parameters encoded in the ParametersJson field.
				params := &tryjobstore.Parameters{}
				if err := json.Unmarshal([]byte(build.ParametersJson), params); err != nil {
					sklog.Errorf("Error unmarshalling params: %s", err)
					return
				}

				// Check if this is a builder we can ignore.
				if t.ignoreBuild(build, params) {
					// sklog.Infof("Ignoring build: %d", build.Id)
					return
				}

				// Extract the tryjob info.
				tryjob, err := getTryJobInfo(build, params)
				if err != nil {
					sklog.Errorf("Error extracting tryjob info: %s", err)
					return
				}
				//			sklog.Infof("Got tryjob info: %d / %d:  %s", issueID, patchsetID, tryjob.String())

				if err := t.updateTryjobState(params, tryjob); err != nil {
					sklog.Errorf("Error adding build info to tryjob store: %s", err)
					return
				} else {
					// TODO remove commented out log statements
					sklog.Infof("Updated issue and tryjob (%d, %d)", tryjob.IssueID, tryjob.BuildBucketID)
					// sklog.Infof("Received build: %s, %s", build.Bucket, build.CreatedBy)
				}
			}(build)
		}
	}()

	// Start the poller.
	if err := t.startBuildPoller(buildsCh, POLL_INTERVAL, DEFAULT_TIME_WINDOW); err != nil {
		return err
	}

	return nil
}

// Loads the Gerrit issue identified by the issueID into the tryjob store.
func (t *BuildBucketState) LoadGerritIssue(issueID int64) (bool, error) {
	issue, err := t.syncGerritIssue(issueID, -1, nil)
	return issue != nil, err
}

func (t *BuildBucketState) updateTryjobState(params *tryjobstore.Parameters, tryjob *tryjobstore.Tryjob) error {
	// Find the existing issue in the tryjob store.
	issue, err := t.tryjobStore.GetIssue(tryjob.IssueID, false, nil)
	if err != nil {
		return err
	}

	if !issue.HasPatchset(tryjob.PatchsetID) {
		// Make sure we have an up to date issue. Note: 'issue' might be nil
		// if we didn't find it in the issue store.
		if issue, err = t.syncGerritIssue(tryjob.IssueID, tryjob.PatchsetID, issue); err != nil {
			return err
		}

		// If the issue was not found we have a problem.
		if issue == nil {
			return fmt.Errorf("Issue %d was not found.", tryjob.IssueID)
		}

		// Make sure the patchset detail is present.
		if !issue.HasPatchset(tryjob.PatchsetID) {
			return fmt.Errorf("Found issue %d, but could not find patchset detail %d", tryjob.IssueID, tryjob.PatchsetID)
		}
	}

	// Add the Tryjob.
	if err := t.tryjobStore.UpdateTryjob(tryjob.IssueID, tryjob); err != nil {
		return fmt.Errorf("Error updating issue and tryjob (%d, %d). Got error: %s", tryjob.IssueID, tryjob.BuildBucketID, err)
	}
	return nil
}

func (t *BuildBucketState) syncGerritIssue(issueID, patchsetID int64, issue *tryjobstore.IssueDetails) (*tryjobstore.IssueDetails, error) {
	// If we didn't find the issue then create a new one.
	var err error
	if issue == nil {
		issue, err = t.fetchGerritIssue(issueID)
		if err != nil {
			if err == gerrit.ErrNotFound {
				return nil, nil
			}
			return nil, err
		}
	} else {
		// Check if the issue is up to date.
		// sklog.Infof("Found gerrit issue: %d", issueID)
		if !issue.HasPatchset(patchsetID) {
			err = t.updateGerritIssue(issueID, issue)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("Error fetching or updating issue %d: %s", issueID, err)
	}

	// Write the update issues to the store.
	if err := t.tryjobStore.UpdateIssue(issue); err != nil {
		return nil, err
	}

	return issue, nil
}

func (t *BuildBucketState) fetchGerritIssue(issueID int64) (*tryjobstore.IssueDetails, error) {
	changeInfo, err := t.gerritAPI.GetIssueProperties(issueID)
	if err != nil {
		return nil, err
	}

	ret := &tryjobstore.IssueDetails{Issue: &tryjobstore.Issue{}}
	t.setIssueDetails(issueID, changeInfo, ret)
	return ret, nil
}

func (t *BuildBucketState) updateGerritIssue(issueID int64, issue *tryjobstore.IssueDetails) error {
	changeInfo, err := t.gerritAPI.GetIssueProperties(issueID)
	if err != nil {
		return err
	}
	t.setIssueDetails(issueID, changeInfo, issue)
	return nil
}

func (t *BuildBucketState) setIssueDetails(issueID int64, changeInfo *gerrit.ChangeInfo, issue *tryjobstore.IssueDetails) {
	issue.Issue.ID = issueID
	issue.Issue.Subject = changeInfo.Subject
	issue.Issue.Owner = changeInfo.Owner.Email
	issue.Issue.Updated = changeInfo.Updated
	issue.Issue.URL = t.gerritAPI.Url(issueID)
	issue.Status = changeInfo.Status
	issue.UpdatePatchsets(extractPatchsetDetails(changeInfo))
}

func extractPatchsetDetails(changeInfo *gerrit.ChangeInfo) []*tryjobstore.PatchsetDetail {
	ret := make([]*tryjobstore.PatchsetDetail, 0, len(changeInfo.Patchsets))
	for _, revision := range changeInfo.Patchsets {
		ret = append(ret, &tryjobstore.PatchsetDetail{ID: revision.Number})
	}
	return ret
}

var extractPatchsetRegex = regexp.MustCompile(`^refs\/changes\/[0-9]*\/[0-9]*\/(.*)$`)

func getTryJobInfo(build *bb_api.ApiCommonBuildMessage, params *tryjobstore.Parameters) (*tryjobstore.Tryjob, error) {
	matchedGroups := extractPatchsetRegex.FindStringSubmatch(params.Properties.GerritPatchset)
	if len(matchedGroups) != 2 {
		return nil, fmt.Errorf("Unable to extract patchset info from '%s'", params.Properties.GerritPatchset)
	}

	// sklog.Infof("Gerrit patchset Info: %s", matchedGroups[1])
	patchsetID, err := strconv.ParseInt(matchedGroups[1], 10, 64)
	if err != nil {
		return nil, err
	}

	issueID := int64(params.Properties.GerritIssue)

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

	ret := &tryjobstore.Tryjob{
		IssueID:       issueID,
		PatchsetID:    patchsetID,
		Builder:       params.BuilderName,
		BuildBucketID: build.Id,
		Status:        status,
	}

	return ret, nil
}

func (t *BuildBucketState) startBuildPoller(buildsCh chan<- *bb_api.ApiCommonBuildMessage, interval, timeWindow time.Duration) error {
	if err := t.pollBuildBucket(buildsCh, timeWindow); err != nil {
		return err
	}
	go func() {
		for range time.Tick(interval) {
			if err := t.pollBuildBucket(buildsCh, timeWindow); err != nil {
				sklog.Errorf("Error polling BuildBucket: %s", err)
			}
		}
	}()
	return nil
}

func (t *BuildBucketState) pollBuildBucket(buildsCh chan<- *bb_api.ApiCommonBuildMessage, timeWindow time.Duration) error {
	sklog.Infof("Starting search of buildbucket.")
	// Search over a specific time window.
	searchCall := t.service.Search()

	timeWindowStart := time.Now().Add(-timeWindow).UnixNano() / int64(time.Microsecond)
	searchCall.Bucket("skia.primary").CreationTsLow(timeWindowStart)
	// sklog.Infof("Starting window: %d", timeWindowStart)

	if err := searchCall.Run(buildsCh, 0, nil); err != nil {
		return fmt.Errorf("Error querying build bucket: %s", err)
	}
	sklog.Infof("Done. Successfully searched buildbucket.")
	return nil
}

func (t *BuildBucketState) ignoreBuild(build *bb_api.ApiCommonBuildMessage, params *tryjobstore.Parameters) bool {
	return !strings.HasPrefix(params.BuilderName, TEST_BUILDER_PREFIX)
}
