package bbtrybot

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	// REMOVE REMOVE REMOVE

	_ "go.chromium.org/luci/buildbucket"
	bb_api "go.chromium.org/luci/common/api/buildbucket/buildbucket/v1"

	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/trybotstore"
)

const (
	// TIME_WINDOW = 45 * 24 * time.Hour // Time we look back at the build bucket
	TIME_WINDOW              = 12 * time.Hour  // Time we look back at the build bucket
	POLL_INTERVAL            = 5 * time.Minute // Interval at which we poll buildbucket
	TEST_BUILDER_PREFIX      = "Test-"         // Prefix that identifies Trybots that run tests/produce images.
	MAX_CONCURRENT_PROCESSES = 100
)

type TrybotStateInterface interface {
	AddResult()
}

// TryjobState captures all tryjobs we are currently interested in.
// These are the trybots that were run within the time windows and some that
// have been explicitly requested by users.
type TrybotState struct {
	service     *bb_api.Service
	trybotStore trybotstore.TrybotStore
	gerritAPI   *gerrit.Gerrit
}

func NewTrybotState(client *http.Client, trybotStore trybotstore.TrybotStore, gerritAPI *gerrit.Gerrit) (*TrybotState, error) {
	service, err := bb_api.New(client)
	if err != nil {
		return nil, err
	}
	service.BasePath = "https://cr-buildbucket.appspot.com/api/buildbucket/v1/"
	ret := &TrybotState{
		service:     service,
		trybotStore: trybotStore,
		gerritAPI:   gerritAPI,
	}
	ret.start()

	return ret, nil
}

// TODO
func (t *TrybotState) AddResult() {}

// start continuously processes data it gets from buildbucket by polling.
func (t *TrybotState) start() error {
	// Create the channel that will receive the buildbot results.
	buildsCh := make(chan *bb_api.ApiCommonBuildMessage)
	workPermissions := make(chan bool, MAX_CONCURRENT_PROCESSES)

	// Process the builds produced by the poller.
	go func() {
		for build := range buildsCh {
			// Get work permission.
			workPermissions <- true

			go func(build *bb_api.ApiCommonBuildMessage) {
				// Give up work permission at the end.
				defer func() { <-workPermissions }()

				// Parse the parameters encoded in the ParametersJson field.
				params := &trybotstore.Parameters{}
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
				issueID, patchsetID, tryjob, err := getTryJobInfo(build, params)
				if err != nil {
					sklog.Errorf("Error extracting tryjob info: %s", err)
					return
				}
				sklog.Infof("Got tryjob info: %d / %d:  %s", issueID, patchsetID, tryjob.String())

				if err := t.updateTryjobState(issueID, params, patchsetID, tryjob); err != nil {
					sklog.Errorf("Error adding build info to trybot store: %s", err)
					return
				}

				// TODO remove commented out log statements
				sklog.Infof("Received build: %s, %s", build.Bucket, build.CreatedBy)
			}(build)
		}
	}()

	// Start the poller.
	if err := t.startBuildPoller(buildsCh, POLL_INTERVAL, TIME_WINDOW); err != nil {
		return err
	}

	return nil
}

func (t *TrybotState) updateTryjobState(issueID int64, params *trybotstore.Parameters, patchsetID int64, tryjob *trybotstore.Tryjob) error {
	// Find the existing issue in the trybot store.
	issue, err := t.trybotStore.GetIssue(issueID, false, nil)
	if err != nil {
		return err
	}

	if !issue.HasPatchset(patchsetID) {
		// Make sure we have an up to date issue. Note: 'issue' might be nil
		// if we didn't find it in the issue store.
		if issue, err = t.syncGerritIssue(issueID, patchsetID, issue); err != nil {
			return err
		}
	}

	// Add the Tryjob.
	if err := t.trybotStore.UpdateTryjob(issueID, tryjob); err != nil {
		return err
	}
	return nil
}

func (t *TrybotState) syncGerritIssue(issueID, patchsetID int64, issue *trybotstore.IssueDetails) (*trybotstore.IssueDetails, error) {
	// If we didn't find the issue then create a new one.
	var err error
	if issue == nil {
		issue, err = t.fetchGerritIssue(issueID)

		// Do soemthing if the issue doesn't exist.
		if err != nil {
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

	// Make sure the patchset detail is present.
	if !issue.HasPatchset(patchsetID) {
		return nil, fmt.Errorf("Found issue %d, but could not find patchset detail %d", issueID, patchsetID)
	}

	// Write the update issues to the store.
	if err := t.trybotStore.UpdateIssue(issue); err != nil {
		return nil, err
	}

	return issue, nil
}

func (t *TrybotState) fetchGerritIssue(issueID int64) (*trybotstore.IssueDetails, error) {
	changeInfo, err := t.gerritAPI.GetIssueProperties(issueID)
	if err != nil {
		return nil, err
	}

	ret := &trybotstore.IssueDetails{Issue: &trybotstore.Issue{}}
	setIssueDetails(issueID, changeInfo, ret)
	return ret, nil
}

func (t *TrybotState) updateGerritIssue(issueID int64, issue *trybotstore.IssueDetails) error {
	changeInfo, err := t.gerritAPI.GetIssueProperties(issueID)
	if err != nil {
		return err
	}
	setIssueDetails(issueID, changeInfo, issue)
	return nil
}

func setIssueDetails(issueID int64, changeInfo *gerrit.ChangeInfo, issue *trybotstore.IssueDetails) {
	issue.Issue.ID = issueID
	issue.Issue.Subject = changeInfo.Subject
	issue.Issue.Owner = changeInfo.Owner.Email
	issue.Issue.Updated = changeInfo.Updated
	issue.UpdatePatchsets(extractPatchsetDetails(changeInfo))
}

func extractPatchsetDetails(changeInfo *gerrit.ChangeInfo) []*trybotstore.PatchsetDetail {
	ret := make([]*trybotstore.PatchsetDetail, 0, len(changeInfo.Patchsets))
	for _, revision := range changeInfo.Patchsets {
		ret = append(ret, &trybotstore.PatchsetDetail{ID: revision.Number})
	}
	return ret
}

var extractPatchsetRegex = regexp.MustCompile(`^refs\/changes\/[0-9]*\/[0-9]*\/(.*)$`)

func getTryJobInfo(build *bb_api.ApiCommonBuildMessage, params *trybotstore.Parameters) (int64, int64, *trybotstore.Tryjob, error) {
	matchedGroups := extractPatchsetRegex.FindStringSubmatch(params.Properties.GerritPatchset)
	if len(matchedGroups) != 2 {
		return 0, 0, nil, fmt.Errorf("Unable to extract patchset info from '%s'", params.Properties.GerritPatchset)
	}

	// sklog.Infof("Gerrit patchset Info: %s", matchedGroups[1])
	patchsetID, err := strconv.ParseInt(matchedGroups[1], 10, 64)
	if err != nil {
		return 0, 0, nil, err
	}

	// Translate the two result fields into one for tryjobs.
	var status trybotstore.TryjobStatus = trybotstore.TRYJOB_UNKNOWN
	switch build.Status {
	case buildbucket.STATUS_SCHEDULED:
		status = trybotstore.TRYJOB_SCHEDULED
	case buildbucket.STATUS_STARTED:
		status = trybotstore.TRYJOB_RUNNING
	case buildbucket.STATUS_COMPLETED:
		switch build.Result {
		case buildbucket.RESULT_CANCELED:
			fallthrough
		case buildbucket.RESULT_FAILURE:
			status = trybotstore.TRYJOB_FAILED
		case buildbucket.RESULT_SUCCESS:
			status = trybotstore.TRYJOB_COMPLETE
		}
	}

	if status == trybotstore.TRYJOB_UNKNOWN {
		return 0, 0, nil, fmt.Errorf("Unknown trybot state. Got (status, result): (%s, %s)", build.Status, build.Result)
	}

	ret := &trybotstore.Tryjob{
		PatchsetID:  patchsetID,
		Builder:     params.BuilderName,
		Buildnumber: build.Id,
		Status:      status,
	}

	return int64(params.Properties.GerritIssue), patchsetID, ret, nil
}

func (t *TrybotState) startBuildPoller(buildsCh chan<- *bb_api.ApiCommonBuildMessage, interval, timeWindow time.Duration) error {
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

func (t *TrybotState) pollBuildBucket(buildsCh chan<- *bb_api.ApiCommonBuildMessage, timeWindow time.Duration) error {
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

func (t *TrybotState) ignoreBuild(build *bb_api.ApiCommonBuildMessage, params *trybotstore.Parameters) bool {
	return !strings.HasPrefix(params.BuilderName, TEST_BUILDER_PREFIX)
}
