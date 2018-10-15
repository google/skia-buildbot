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
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	bb_api "go.chromium.org/luci/common/api/buildbucket/buildbucket/v1"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/tryjobstore"
	"golang.org/x/sync/errgroup"
)

// BuildBucketDBSync fetches issue and build information from the relevant services.
// This defines the interfaces of BuildBucketState and is used to mock it in tests.
type BuildIssueSync interface {
	// SyncIssueTryjob forces a synchronous fetch of the issue information from
	// Gerrit and the Tryjob information from the build system.
	SyncIssueTryjob(issueID, buildBucketID int64) (*tryjobstore.Issue, *tryjobstore.Tryjob, error)
}

const (
	// DefaultSkiaBucketName is the name for the Skia BuildBucket.
	DefaultSkiaBucketName = "skia.primary"

	// DefaultSkiaBuildBucketURL is the default URL for the BuildBucketService used by Skia.
	DefaultSkiaBuildBucketURL = "https://cr-buildbucket.appspot.com/api/buildbucket/v1/"

	// DefaultTimeWindow is the time window we track in build bucket.
	DefaultTimeWindow = 5 * time.Hour // Time we look back at the build bucket

	// DefaultPollInterval is the interval at which we poll BuildBucket.
	DefaultPollInterval = 5 * time.Minute // Interval at which we poll buildbucket

	// DefaultTestBuilderRegex is the default regexp that identifies tryjobs that
	// run tests/produce images. This will probably only work for Skia.
	DefaultTestBuilderRegex = `^Test-`

	// maxConcurrentWrites controls how many updates written concurrently to the
	// underlying TryjobStore.
	maxConcurrentWrites = 1000

	// buildWatcherPollInterval is the interval at which builds, that are in a
	// (pre)running state, are polled.
	buildWatcherPollInterval = 10 * time.Second
)

// Config defines the configuration options for BuildBucketState.
type Config struct {
	// BuildBucketURL is the URL of the BuildBucket instance to poll for tryjobs.
	BuildBucketURL string

	// BuildBucketName is the name of the BuildBucket to poll.
	BuildBucketName string

	// Client is an authenticated http client used to connect to BuildBucket.
	Client *http.Client

	// TryjobStore stores tryjob related data.
	TryjobStore tryjobstore.TryjobStore

	// GerritClient is used to query Gerrit for issue details.
	GerritClient *gerrit.Gerrit

	// PollInterval is the interval at which to poll BuildBucket.
	PollInterval time.Duration

	// TimeWindow is the time delta that defines how far back in time BuildBucket is queried.
	TimeWindow time.Duration

	// BuilderRegexp is the regular expression that has to match for a builder to be included.
	BuilderRegexp string
}

// BuildBucketState captures all tryjobs that are being run by BuildBucket.
// These are the tryjobs that were run within a time windows.
// It polls the BuildBucket and retrieves Tryjobs as they arrived. As needed
// the corresponding Gerrit issues are loaded. This information is then
// stored in a TryjobStore.
type BuildBucketState struct {
	// service client to access buildbucket.
	service       *bb_api.Service
	bucketName    string
	tryjobStore   tryjobstore.TryjobStore
	gerritAPI     *gerrit.Gerrit
	builderRegExp *regexp.Regexp

	// currentBuilds keeps track of currently running builds that we are tracking.
	// mapping: currentBuilds[build_bucket_id] => status
	currentBuilds map[int64]tryjobstore.TryjobStatus

	// watchMutex protects currentBuilds
	watchMutex sync.Mutex
}

// NewBuildBucketState creates a new instance of BuildBucketState.
// bbURL is the URL of the target BuildBucket instance and client is an
// authenticated http client.
func NewBuildBucketState(config *Config) (BuildIssueSync, error) {
	service, err := bb_api.New(config.Client)
	if err != nil {
		return nil, err
	}

	// compile the regular expression to filter builders that should be ingested.
	builderRegExp, err := regexp.Compile(config.BuilderRegexp)
	if err != nil {
		return nil, err
	}

	service.BasePath = config.BuildBucketURL
	ret := &BuildBucketState{
		service:       service,
		bucketName:    config.BuildBucketName,
		tryjobStore:   config.TryjobStore,
		gerritAPI:     config.GerritClient,
		builderRegExp: builderRegExp,
		currentBuilds: map[int64]tryjobstore.TryjobStatus{},
	}
	if err := ret.startPollers(config.PollInterval, config.TimeWindow); err != nil {
		return nil, err
	}
	return ret, nil
}

// SyncIssueTryjob implements the BuildIssueSync interface.
func (b *BuildBucketState) SyncIssueTryjob(issueID, buildBucketID int64) (*tryjobstore.Issue, *tryjobstore.Tryjob, error) {
	// Fetch the build information from BuildBucket and convert it to a Tryjob.
	tryjob, err := b.fetchBuild(buildBucketID)
	if err != nil {
		return nil, nil, err
	}

	// The referenced tryjob doesn't exist.
	if tryjob == nil {
		return nil, nil, fmt.Errorf("Tryjob with BuildBucket id %d for issue %d does not exist.", buildBucketID, issueID)
	}

	// Update the tryjob information.
	if err := b.updateTryjobState(tryjob); err != nil {
		return nil, nil, fmt.Errorf("Error adding build info to tryjob store. \n%s\nError: %s", spew.Sdump(tryjob), err)
	}

	if tryjob.IssueID != issueID {
		return nil, nil, fmt.Errorf("Issue %d is not referenced by tryjob %d.", issueID, buildBucketID)
	}

	issue, err := b.tryjobStore.GetIssue(issueID, false)
	if err != nil {
		return nil, nil, err
	}

	if issue == nil {
		return nil, nil, fmt.Errorf("Issue %d does not exist.", issueID)
	}

	return issue, tryjob, nil
}

// startPollers watches builds that are assumed running (according to the database)
// until they reach a completed state.
// It also starts a poller that continuously queries BuildBucket for newly
// started builds and then watches them the same way.
func (b *BuildBucketState) startPollers(pollInterval, timeWindow time.Duration) error {
	// Fetch all tryjobs that we know as running and watch them for completion.
	tryjobs, err := b.tryjobStore.RunningTryjobs()
	if err != nil {
		return sklog.FmtErrorf("Error retrieving running tryjobs: %s", err)
	}

	// Watch these tryjobs to make sure we catch when they are finished.
	for _, tryjob := range tryjobs {
		b.watchBuild(tryjob.BuildBucketID)
	}

	// Create the channel that will receive the list of running tryjobs.
	buildsCh := make(chan *bb_api.ApiCommonBuildMessage)
	workPermissions := make(chan bool, maxConcurrentWrites)

	// Process the new builds discovered by the poller.
	go func() {
		for build := range buildsCh {
			// Get work permission.
			workPermissions <- true

			go func(build *bb_api.ApiCommonBuildMessage) {
				// Give up work permission at the end.
				defer func() { <-workPermissions }()

				// Only consider builds that have not completed.
				switch build.Status {
				case buildbucket.STATUS_SCHEDULED:
					fallthrough
				case buildbucket.STATUS_STARTED:
					b.watchBuild(build.Id)
				}
			}(build)
		}
	}()

	// Start the poller.
	if err := b.startSearchPoller(buildsCh, pollInterval, timeWindow); err != nil {
		return err
	}

	return nil
}

// GetWatchedBuilds returns the builds that are currently being tracked until
// they complete. Return value:  map[build_bucket_id]status
func (b *BuildBucketState) GetWatchedBuilds() map[int64]tryjobstore.TryjobStatus {
	b.watchMutex.Lock()
	defer b.watchMutex.Unlock()
	ret := make(map[int64]tryjobstore.TryjobStatus, len(b.currentBuilds))
	for k, v := range b.currentBuilds {
		ret[k] = v
	}
	return ret
}

// watchBuild ensures that the build with the given BuildBucket id is being watched.
// If the build is not being watched a new go-routine is started that follows its
// state until it completes.
func (b *BuildBucketState) watchBuild(buildBucketID int64) {
	// Check if we are already watching this build.
	b.watchMutex.Lock()
	defer b.watchMutex.Unlock()
	if _, ok := b.currentBuilds[buildBucketID]; !ok {
		b.currentBuilds[buildBucketID] = tryjobstore.TRYJOB_UNKNOWN
		go b.watchOneBuild(buildBucketID)
	}
}

// watchOneBuild tracks the build that corresponds to the given BuildBucket id.
// It assumes it is being run in its own go-routine.
func (b *BuildBucketState) watchOneBuild(buildBucketID int64) {
	for {
		// fetch the build info
		if tryjob, err := b.fetchBuild(buildBucketID); err != nil {
			sklog.Errorf("Error fetching build from BuildBucket: %s", err)
		} else {
			// If the tryjob is ignored or not available we are done.
			if tryjob == nil {
				return
			}

			// Get the last known status for this id.
			b.watchMutex.Lock()
			lastKnownStatus := b.currentBuilds[buildBucketID]
			b.watchMutex.Unlock()

			// If it has changed since then we need to update it.
			if tryjob.Status != lastKnownStatus {
				if err := b.updateTryjobState(tryjob); err != nil {
					sklog.Errorf("Error updating tryjob state: %s", err)
				} else {
					// If the tryjob has finished we are done watching it.
					if tryjob.Status >= tryjobstore.TRYJOB_COMPLETE {
						b.watchMutex.Lock()
						delete(b.currentBuilds, buildBucketID)
						b.watchMutex.Unlock()
						return
					}
					b.watchMutex.Lock()
					b.currentBuilds[buildBucketID] = tryjob.Status
					b.watchMutex.Unlock()
				}
			}
		}

		// Wait for a little bit and poll the status again.
		time.Sleep(buildWatcherPollInterval)
	}
}

// fetchBuild retrieves the build that corresponds to the given BuildBucket id
// and extracts the information into an instance of Tryjob.
// The first return value being nil, indicates that the build does not exist
// of was ignored for some reason.
func (b *BuildBucketState) fetchBuild(buildBucketID int64) (*tryjobstore.Tryjob, error) {
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

// updateTryjobState adds the provided tryjob information to the TryjobStore.
func (b *BuildBucketState) updateTryjobState(tryjob *tryjobstore.Tryjob) error {
	// Find the existing issue in the tryjob store.
	issue, err := b.tryjobStore.GetIssue(tryjob.IssueID, false)
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
	if err := b.tryjobStore.UpdateTryjob(0, tryjob, nil); err != nil {
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
func (b *BuildBucketState) syncGerritIssue(issueID, patchsetID int64, issue *tryjobstore.Issue) (*tryjobstore.Issue, error) {
	// If 'issue' is nil, we need to see if we can find it in Gerrit.
	if issue == nil {
		var err error
		issue, err = b.updateGerritIssue(issueID, issue)
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
			var err error
			if issue, err = b.updateGerritIssue(issueID, issue); err != nil {
				return nil, fmt.Errorf("Error updating issue %d: %s", issueID, err)
			}
		}
	}

	// Write the update issues to the store.
	if err := b.tryjobStore.UpdateIssue(issue, nil); err != nil {
		return nil, err
	}
	sklog.Infof("Added information for issue %d", issueID)

	return issue, nil
}

// updateGerritIssue fetches issue details from Gerrit and merges them into the
// internal representation of the Gerrit issues. If issue is nil a new instance
// will be allocated and returned.
func (b *BuildBucketState) updateGerritIssue(issueID int64, issue *tryjobstore.Issue) (*tryjobstore.Issue, error) {
	changeInfo, err := b.gerritAPI.GetIssueProperties(issueID)
	if err != nil {
		return nil, err
	}

	if issue == nil {
		issue = &tryjobstore.Issue{}
	}

	commitInfos := make([]*gerrit.CommitInfo, len(changeInfo.Revisions))
	var egroup errgroup.Group
	for idx, rev := range changeInfo.Patchsets {
		func(idx int, rev *gerrit.Revision) {
			egroup.Go(func() error {
				var err error
				commitInfos[idx], err = b.gerritAPI.GetCommit(issueID, rev.ID)
				return err
			})
		}(idx, rev)
	}

	if err := egroup.Wait(); err != nil {
		return nil, sklog.FmtErrorf("Error retrieving commit info for issue %d: %s", issueID, err)
	}

	b.setIssueDetails(issueID, changeInfo, commitInfos, issue)
	return issue, nil
}

// setIssueDetails set the properties of the given IssueDetails from the values
// in the Gerrit ChangeInfo.
func (b *BuildBucketState) setIssueDetails(issueID int64, changeInfo *gerrit.ChangeInfo, commitInfos []*gerrit.CommitInfo, issue *tryjobstore.Issue) {
	issue.ID = issueID
	issue.Subject = changeInfo.Subject
	issue.Owner = changeInfo.Owner.Email
	issue.Updated = changeInfo.Updated
	issue.URL = b.gerritAPI.Url(issueID)
	issue.Status = changeInfo.Status

	// extract the patchset detail.
	psDetails := make([]*tryjobstore.PatchsetDetail, 0, len(changeInfo.Patchsets))
	for idx, revision := range changeInfo.Patchsets {
		psDetails = append(psDetails, &tryjobstore.PatchsetDetail{
			ID:           revision.Number,
			Commit:       revision.ID,
			ParentCommit: getParentCommit(commitInfos[idx]),
		})
	}
	issue.UpdatePatchsets(psDetails)
}

// getParentCommit robustly returns the parent commit from the given CommitInfo instance. If that's
// not possible, the empty string is returned.
func getParentCommit(commitInfo *gerrit.CommitInfo) string {
	if commitInfo == nil || len(commitInfo.Parents) == 0 {
		return ""
	}

	return commitInfo.Parents[0].Commit
}

// pollBuildBucket queries the BuildBucket instance from (now - timeWindow) to now.
func (b *BuildBucketState) searchForNewBuilds(buildsCh chan<- *bb_api.ApiCommonBuildMessage, timeWindow time.Duration) error {
	// Search over a specific time window.
	searchCall := b.service.Search()

	timeWindowStart := time.Now().Add(-timeWindow).UnixNano() / int64(time.Microsecond)
	searchCall.Bucket(b.bucketName).CreationTsLow(timeWindowStart)

	if _, err := searchCall.Run(buildsCh, 0, nil); err != nil {
		return fmt.Errorf("Error querying build bucket: %s", err)
	}
	return nil
}

// ignoreBuild is the central place to determine whether a build from
// BuildBucket should be ignored. For example, BuildBucket can contain build jobs
// that produce no test output.
func (b *BuildBucketState) ignoreBuild(build *bb_api.ApiCommonBuildMessage, params *tryjobstore.Parameters) bool {
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

// startSearchPoller polls the BuildBucket immediately and starts a poller at the
// given interval with the given time windows. All results are written to buildCh.
// If the first poll fails, an error is returned.
func (b *BuildBucketState) startSearchPoller(buildsCh chan<- *bb_api.ApiCommonBuildMessage, interval, timeWindow time.Duration) error {
	if err := b.searchForNewBuilds(buildsCh, timeWindow); err != nil {
		return err
	}
	go func() {
		for range time.Tick(interval) {
			if err := b.searchForNewBuilds(buildsCh, timeWindow); err != nil {
				sklog.Errorf("Error polling BuildBucket: %s", err)
			}
		}
	}()
	return nil
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
