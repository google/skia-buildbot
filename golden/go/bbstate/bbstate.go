// The bbstate package tracks the state of tryjobs in BuildBucket and loads
// issues from Gerrit to maintain consistent tryjob information in an instance
// of tryjobstore.TryjobStore.
package bbstate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	bb_api "go.chromium.org/luci/common/api/buildbucket/buildbucket/v1"

	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/tryjobstore"
)

const (
	// DefaultSkiaBuildBucketURL is the default URL for the BuildBucketService used by Skia.
	DefaultSkiaBuildBucketURL = "https://cr-buildbucket.appspot.com/api/buildbucket/v1/"

	// DefaultTimeWindow is the time window we track in build bucket.
	DefaultTimeWindow = 12 * time.Hour // Time we look back at the build bucket

	// DefaultPollInterval is the interval at which we poll BuildBucket.
	DefaultPollInterval = 5 * time.Minute // Interval at which we poll buildbucket

	// TestBuilderPrefix is the prefix that identifies tryjobs that run tests/produce images.
	TestBuilderPrefix = "Test-"

	// maxConcurrentWrites controls how many updates written concurrently to the
	// underlying TryjobStore.
	maxConcurrentWrites = 1000
)

// BuildBucketState captures all tryjobs that are being run by BuildBucket.
// These are the tryjobs that were run within a time windows.
// It polls the BuildBucket and retrieves Tryjobs as they arrived. As needed
// the corresponding Gerrit issues are loaded. This information is then
// stored in a TryjobStore.
type BuildBucketState struct {
	// service client to access buildbucket.
	service     *bb_api.Service
	tryjobStore tryjobstore.TryjobStore
	gerritAPI   *gerrit.Gerrit
}

// NewBuildBucketState creates a new instance of BuildBucketState.
// bbURL is the URL of the target BuildBucket instance and client is an
// authenticated http client.
func NewBuildBucketState(bbURL string, client *http.Client, tryjobStore tryjobstore.TryjobStore, gerritAPI *gerrit.Gerrit) (*BuildBucketState, error) {
	service, err := bb_api.New(client)
	if err != nil {
		return nil, err
	}
	service.BasePath = bbURL
	ret := &BuildBucketState{
		service:     service,
		tryjobStore: tryjobStore,
		gerritAPI:   gerritAPI,
	}
	if err := ret.start(); err != nil {
		return nil, err
	}
	return ret, nil
}

// LoadGerritIssue loads the given Gerrit issue and store it in the TryjobStore.
func (b *BuildBucketState) LoadGerritIssue(issueID int64) (bool, error) {
	issue, err := b.syncGerritIssue(issueID, -1, nil)
	return issue != nil, err
}

// start continuously processes data it gets from buildbucket by polling.
func (b *BuildBucketState) start() error {
	// Create the channel that will receive the buildbot results.
	buildsCh := make(chan *bb_api.ApiCommonBuildMessage)
	workPermissions := make(chan bool, maxConcurrentWrites)

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
				if b.ignoreBuild(build, params) {
					return
				}

				// Extract the tryjob info.
				tryjob, err := getTryjobInfo(build, params)
				if err != nil {
					sklog.Errorf("Error extracting tryjob info: %s", err)
					return
				}

				if err := b.updateTryjobState(params, tryjob); err != nil {
					sklog.Errorf("Error adding build info to tryjob store: %s", err)
					return
				}
			}(build)
		}
	}()

	// Start the poller.
	if err := b.startBuildPoller(buildsCh, DefaultPollInterval, DefaultTimeWindow); err != nil {
		return err
	}

	return nil
}

// updateTryjobState adds the provided tryjob information to the TryjobStore.
func (b *BuildBucketState) updateTryjobState(params *tryjobstore.Parameters, tryjob *tryjobstore.Tryjob) error {
	// Find the existing issue in the tryjob store.
	issue, err := b.tryjobStore.GetIssue(tryjob.IssueID, false, nil)
	if err != nil {
		return err
	}

	if !issue.HasPatchset(tryjob.PatchsetID) {
		// Make sure we have an up to date issue. Note: 'issue' might be nil
		// if we didn't find it in the issue store.
		if issue, err = b.syncGerritIssue(tryjob.IssueID, tryjob.PatchsetID, issue); err != nil {
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
	if err := b.tryjobStore.UpdateTryjob(tryjob.IssueID, tryjob); err != nil {
		return fmt.Errorf("Error updating issue and tryjob (%d, %d). Got error: %s", tryjob.IssueID, tryjob.BuildBucketID, err)
	}
	return nil
}

// syncGerritIssue makes sure the data between Gerrit and the TryjobStore are
// consistent.
// If the the given 'issue' is nil, it will try and fetch it from Gerrit and
// create a new entry in the TryjobStore.
// If the issue identified by 'issueID' does not exist in Gerrit it will
// return nil.
func (b *BuildBucketState) syncGerritIssue(issueID, patchsetID int64, issue *tryjobstore.IssueDetails) (*tryjobstore.IssueDetails, error) {
	// If 'issue' is nil, we need to see if we can find it in Gerrit.
	if issue == nil {
		var err error
		issue, err = b.fetchGerritIssue(issueID)
		if err != nil {
			// We didn't find the issue in Gerrit.
			if err == gerrit.ErrNotFound {
				return nil, nil
			}
			return nil, fmt.Errorf("Error fetching issue %d: %s", issueID, err)
		}
	} else {
		// Check if the issue is up to date.
		if !issue.HasPatchset(patchsetID) {
			if err := b.updateGerritIssue(issueID, issue); err != nil {
				return nil, fmt.Errorf("Error updating issue %d: %s", issueID, err)
			}
		}
	}

	// Write the update issues to the store.
	if err := b.tryjobStore.UpdateIssue(issue); err != nil {
		return nil, err
	}

	return issue, nil
}

// fetchGerritIssue retrieves the given issue from Gerrit.
func (b *BuildBucketState) fetchGerritIssue(issueID int64) (*tryjobstore.IssueDetails, error) {
	changeInfo, err := b.gerritAPI.GetIssueProperties(issueID)
	if err != nil {
		return nil, err
	}

	ret := &tryjobstore.IssueDetails{Issue: &tryjobstore.Issue{}}
	b.setIssueDetails(issueID, changeInfo, ret)
	return ret, nil
}

// updateGerritIssue fetches issue detaisl from Gerrit and merges them into the
// internal representation of the Gerrit issues.
func (b *BuildBucketState) updateGerritIssue(issueID int64, issue *tryjobstore.IssueDetails) error {
	changeInfo, err := b.gerritAPI.GetIssueProperties(issueID)
	if err != nil {
		return err
	}
	b.setIssueDetails(issueID, changeInfo, issue)
	return nil
}

// setIssueDetails set the properties of the given IssueDetails from the values
// in the Gerrit ChangeInfo.
func (b *BuildBucketState) setIssueDetails(issueID int64, changeInfo *gerrit.ChangeInfo, issue *tryjobstore.IssueDetails) {
	issue.Issue.ID = issueID
	issue.Issue.Subject = changeInfo.Subject
	issue.Issue.Owner = changeInfo.Owner.Email
	issue.Issue.Updated = changeInfo.Updated
	issue.Issue.URL = b.gerritAPI.Url(issueID)
	issue.Status = changeInfo.Status
	issue.UpdatePatchsets(extractPatchsetDetails(changeInfo))
}

// pollBuildBucket queries the BuildBucket instance from (now - timeWindow) to now.
func (b *BuildBucketState) pollBuildBucket(buildsCh chan<- *bb_api.ApiCommonBuildMessage, timeWindow time.Duration) error {
	sklog.Infof("Starting search of buildbucket.")

	// Search over a specific time window.
	searchCall := b.service.Search()

	timeWindowStart := time.Now().Add(-timeWindow).UnixNano() / int64(time.Microsecond)
	searchCall.Bucket("skia.primary").CreationTsLow(timeWindowStart)

	if err := searchCall.Run(buildsCh, 0, nil); err != nil {
		return fmt.Errorf("Error querying build bucket: %s", err)
	}
	sklog.Infof("Done. Successfully searched buildbucket.")
	return nil
}

// ignoreBuild is the central place to determine whether a build from
// BuildBucket should be ignored. For example, BuildBucket can contain build jobs
// that produce no test output.
func (b *BuildBucketState) ignoreBuild(build *bb_api.ApiCommonBuildMessage, params *tryjobstore.Parameters) bool {
	return !strings.HasPrefix(params.BuilderName, TestBuilderPrefix)
}

// startBuildPoller polls the BuildBucket immediatedly and starts a poller at the
// given interval with the given time windows. All results are written to buildCh.
// If the first poll fails, an error is returned.
func (b *BuildBucketState) startBuildPoller(buildsCh chan<- *bb_api.ApiCommonBuildMessage, interval, timeWindow time.Duration) error {
	if err := b.pollBuildBucket(buildsCh, timeWindow); err != nil {
		return err
	}
	go func() {
		for range time.Tick(interval) {
			if err := b.pollBuildBucket(buildsCh, timeWindow); err != nil {
				sklog.Errorf("Error polling BuildBucket: %s", err)
			}
		}
	}()
	return nil
}

// extractPatchsetDetails produces instances of PatchsetDetail from the change
// info retrieved from Gerrit.
func extractPatchsetDetails(changeInfo *gerrit.ChangeInfo) []*tryjobstore.PatchsetDetail {
	ret := make([]*tryjobstore.PatchsetDetail, 0, len(changeInfo.Patchsets))
	for _, revision := range changeInfo.Patchsets {
		ret = append(ret, &tryjobstore.PatchsetDetail{ID: revision.Number})
	}
	return ret
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
