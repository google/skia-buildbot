// trybot implements routines to retrieve trybot results from the tracedb data store.
//
package trybot

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	cache "github.com/patrickmn/go-cache"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/goldingestion"
)

var (
	TIME_FRAME = time.Hour * time.Duration(24*14)
)

const (
	TRYJOB_SCHEDULED = "scheduled"
	TRYJOB_RUNNING   = "running"
	TRYJOB_COMPLETE  = "complete"
	TRYJOB_INGESTED  = "ingested"
	TRYJOB_FAILED    = "failed"

	PATCHSET_CACHE_EXPIRATION       = 5 * time.Minute
	PATCHSET_CACHE_CLEANUP_INTERVAL = 30 * time.Second
)

// Issue captures information about a single Rietveld issued.
type Issue struct {
	ID        int64   `json:"id"`
	Subject   string  `json:"subject"`
	Owner     string  `json:"owner"`
	Updated   int64   `json:"updated"`
	URL       string  `json:"url"`
	Patchsets []int64 `json:"patchsets"`
}

// IssueDetails extends issue with the commit ideas for the issue.
type IssueDetails struct {
	*Issue
	PatchsetDetails map[int64]*PatchsetDetail `json:"-"`
	CommitIDs       []*tracedb.CommitIDLong   `json:"-"`
	TargetPatchsets []int64                   `json:"-"`
}

type PatchsetDetail struct {
	ID       int64     `json:"id"`
	Tryjobs  []*Tryjob `json:"tryjobs"`
	JobTotal int64     `json:"jobTotal"`
	JobDone  int64     `json:"jobDone"`
	Digests  int64     `json:"digests"`
	InMaster int64     `json:"inMaster"`
	Url      string    `json:"url"`
}

type Tryjob struct {
	Builder     string `json:"builder"`
	Buildnumber string `json:"buildnumber"`
	Status      string `json:"status"`
}

// TrybotResults manages everything related to aggregating information about trybot results.
type TrybotResults struct {
	tileBuilder           tracedb.BranchTileBuilder
	rietveldAPI           *rietveld.Rietveld
	gerritAPI             *gerrit.Gerrit
	ingestionStore        *goldingestion.IngestionStore
	timeFrame             time.Duration
	rietveldPatchsetCache *cache.Cache
}

func NewTrybotResults(tileBuilder tracedb.BranchTileBuilder, rietveldAPI *rietveld.Rietveld, gerritAPI *gerrit.Gerrit, ingestionStore *goldingestion.IngestionStore) *TrybotResults {
	ret := &TrybotResults{
		tileBuilder:           tileBuilder,
		rietveldAPI:           rietveldAPI,
		gerritAPI:             gerritAPI,
		ingestionStore:        ingestionStore,
		timeFrame:             TIME_FRAME,
		rietveldPatchsetCache: cache.New(PATCHSET_CACHE_EXPIRATION, PATCHSET_CACHE_CLEANUP_INTERVAL),
	}
	return ret
}

// ListTrybotIssues returns all the issues that have recently seen trybot updates. The given
// offset and size return a subset of the list. Aside from the issues we return also the
// total number of current issues to allow pagination.
func (t *TrybotResults) ListTrybotIssues(ctx context.Context, offset, size int) ([]*Issue, int, error) {
	end := time.Now()
	begin := end.Add(-t.timeFrame)

	// Get all issues from Rietveld.
	commits, issueIDs, err := t.getCommits(ctx, begin, end, t.rietveldAPI.Url(0), false)
	if err != nil {
		return nil, 0, err
	}
	issuesList := t.getIssuesFromCommits(commits, issueIDs, false)

	// Get all issues from Gerrit.
	commits, issueIDs, err = t.getCommits(ctx, begin, end, t.gerritAPI.Url(0), true)
	if err != nil {
		return nil, 0, err
	}
	issuesList = append(issuesList, t.getIssuesFromCommits(commits, issueIDs, true)...)
	if len(issuesList) == 0 {
		return []*Issue{}, 0, nil
	}

	sort.Sort(IssuesSlice(issuesList))
	maxIdx := util.MaxInt(0, len(issuesList)-1)
	offset = util.MinInt(util.MaxInt(offset, 0), maxIdx)
	size = util.MaxInt(size, 0)
	startIdx := offset
	endIdx := util.MinInt(offset+size, maxIdx)
	return issuesList[startIdx : endIdx+1], len(issuesList), nil
}

// GetIssue returns the information about a specific issue. It returns the a superset
// of the issue information including the commit ids that make up the issue.
// The commmit ids align with the tile that is returned.
func (t *TrybotResults) GetIssue(ctx context.Context, numIssueID int64, targetPatchsets []int64) (*IssueDetails, *tiling.Tile, error) {
	end := time.Now()
	begin := end.Add(-t.timeFrame)
	prefix, isGerrit := t.getPrefix(numIssueID)
	commits, issueIDs, err := t.getCommits(ctx, begin, end, prefix, isGerrit)

	if err != nil {
		return nil, nil, err
	}

	if len(commits) == 0 {
		return nil, nil, nil
	}

	issue := t.getIssuesFromCommits(commits, issueIDs, isGerrit)[0]
	tile, err := t.tileBuilder.CachedTileFromCommits(tracedb.ShortFromLong(commits))
	if err != nil {
		return nil, nil, fmt.Errorf("Error retrieving tile: %s", err)
	}

	// get the patchsets we want and make sure they are probable.
	targetPatchsets, err = t.getTargetPatchsets(issue, targetPatchsets)
	if err != nil {
		return nil, nil, err
	}

	// Retrieve all the patchset results for this commit.
	patchsetDetails, err := t.getPatchsetDetails(issue, isGerrit)
	if err != nil {
		return nil, nil, err
	}

	return &IssueDetails{
		Issue:           issue,
		CommitIDs:       commits,
		PatchsetDetails: patchsetDetails,
		TargetPatchsets: targetPatchsets,
	}, tile, nil
}

// getPrefix returns the URL prefix for the given issue ID and whether it's
// a Gerrit issue or not.
func (t *TrybotResults) getPrefix(issueID int64) (string, bool) {
	// This uses a heuristic to distinguish between Gerrit and Rietveld issues.
	// This is a hack and will be obsolete once Rietveld support is removed.
	if issueID < 1000000 {
		return t.gerritAPI.Url(issueID), true
	}
	return t.rietveldAPI.Url(issueID), false
}

func (t *TrybotResults) getPatchsetDetails(issue *Issue, isGerrit bool) (map[int64]*PatchsetDetail, error) {
	// Select the extraction method.
	extractPatchsetDetails := t.extractRietveldPatchsetDetails
	if isGerrit {
		extractPatchsetDetails = t.extractGerritPatchsetDetails
	}

	ret := make(map[int64]*PatchsetDetail, len(issue.Patchsets))
	var wg sync.WaitGroup
	var mutex sync.Mutex
	for _, pid := range issue.Patchsets {
		wg.Add(1)
		go func(pid int64) {
			defer wg.Done()
			pSet, err := extractPatchsetDetails(issue.ID, pid)
			if err != nil {
				sklog.Errorf("Error retrieving patchset %d: %s", pid, err)
				return
			}

			mutex.Lock()
			defer mutex.Unlock()
			ret[pid] = pSet
		}(pid)
	}
	wg.Wait()
	return ret, nil
}

func (t *TrybotResults) extractRietveldPatchsetDetails(issueID, patchsetID int64) (*PatchsetDetail, error) {
	crPatchSet, err := t.getCachedRietveldPatchset(issueID, patchsetID)
	if err != nil {
		return nil, err
	}

	nTryjobs := len(crPatchSet.TryjobResults)
	tryjobs := make([]*Tryjob, 0, nTryjobs)
	var tjIngested int64 = 0
	for _, tj := range crPatchSet.TryjobResults {
		// Filter out tryjobs we want to ignore. This includes compile bots, since we'll never
		// ingest any results from them. We count the bot as ingested.
		if filterTryjob(tj.Builder) {
			tjIngested++
			continue
		}

		var status string
		checkIngested := false
		switch tj.Result {
		// scheduled but not yet started.
		case 6:
			status = TRYJOB_SCHEDULED
		// currently running.
		case -1:
			status = TRYJOB_RUNNING
			checkIngested = true
			// Finished.
		case 0:
			status = TRYJOB_COMPLETE
			checkIngested = true
		// failed.
		case 2:
			status = TRYJOB_FAILED
		default:
			status = TRYJOB_FAILED
		}

		// Check if the job has been ingested if the job has at least been started.
		if checkIngested && t.ingestionStore.IsIngested(config.CONSTRUCTOR_GOLD, tj.Master, tj.Builder, tj.BuildNumber) {
			status = TRYJOB_INGESTED
			tjIngested++
		}

		tryjobs = append(tryjobs, &Tryjob{
			Builder: tj.Builder,
			Status:  status,
		})
	}

	return &PatchsetDetail{
		ID:       patchsetID,
		Tryjobs:  tryjobs,
		JobDone:  tjIngested,
		JobTotal: int64(nTryjobs),
	}, nil
}

func (t *TrybotResults) getCachedRietveldPatchset(intIssueID, pid int64) (*rietveld.Patchset, error) {
	key := strconv.FormatInt(intIssueID, 10) + ":" + strconv.FormatInt(pid, 10)

	// Check for the key.
	if val, ok := t.rietveldPatchsetCache.Get(key); ok {
		return val.(*rietveld.Patchset), nil
	}

	val, err := t.rietveldAPI.GetPatchset(intIssueID, pid)
	if err != nil {
		return nil, err
	}

	t.rietveldPatchsetCache.Set(key, val, 0)
	return val, nil
}

func (t *TrybotResults) extractGerritPatchsetDetails(issueID, patchsetID int64) (*PatchsetDetail, error) {
	builds, err := t.gerritAPI.GetTrybotResults(issueID, patchsetID)
	if err != nil {
		return nil, err
	}

	nTryjobs := len(builds)
	tryjobs := make([]*Tryjob, 0, nTryjobs)
	var tjIngested int64 = 0
	for _, build := range builds {
		params := build.Parameters
		if (params == nil) || (params.BuilderName == "") || (params.Properties.Master == "") {
			sklog.Errorf("Unable to find builder name or master for a build: %s", build.Id)
			continue
		}
		builderName := build.Parameters.BuilderName

		// Filter out tryjobs we want to ignore. This includes compile bots, since we'll never
		// ingest any results from them. We count the bot as ingested.
		if filterTryjob(build.Parameters.BuilderName) {
			tjIngested++
			continue
		}

		status := TRYJOB_FAILED
		switch build.Status {
		// scheduled but not yet started.
		case buildbucket.STATUS_SCHEDULED:
			status = TRYJOB_SCHEDULED
		// currently running.
		case buildbucket.STATUS_STARTED:
			status = TRYJOB_RUNNING

		case buildbucket.STATUS_COMPLETED:
			if build.Result == buildbucket.RESULT_SUCCESS {
				status = TRYJOB_COMPLETE
			}
		}

		// TODO(stephana): Set the status based on whether the requested tryjobs
		// have already been ingested or not.
		// This depends on buildbucket or task scheduler having an API retrieve
		// the results of a tryjob. We further needs a unique identifier for each
		// tryjob which will be an argument for the call to IsIngested  i.e.:
		// if checkIngested && t.ingestionStore.IsIngested(config.CONSTRUCTOR_GOLD, tj.Master, tj.Builder, tj.BuildNumber) {
		// 	status = TRYJOB_INGESTED
		// 	tjIngested++
		// }

		tryjobs = append(tryjobs, &Tryjob{
			Builder: builderName,
			Status:  status,
		})
	}

	return &PatchsetDetail{
		ID:       patchsetID,
		Tryjobs:  tryjobs,
		JobDone:  tjIngested,
		JobTotal: int64(nTryjobs),
	}, nil
}

func (t *TrybotResults) getTargetPatchsets(issue *Issue, targetPatchsets []int64) ([]int64, error) {
	// if no patchset was given, use the last one.
	if len(targetPatchsets) == 0 {
		pset := issue.Patchsets[len(issue.Patchsets)-1]
		targetPatchsets = []int64{int64(pset)}
	}
	return targetPatchsets, nil
}

// filterTryjob returns true if the given tryjob should be ignored.
func filterTryjob(builder string) bool {
	return !strings.HasPrefix(builder, "Test")
}

// getCommits retrieves the commits within the given time range and prefix.
// isGerrit is a convenience flag indicating whether the Gerrit api should be queried.
func (t *TrybotResults) getCommits(ctx context.Context, startTime, endTime time.Time, prefix string, isGerrit bool) ([]*tracedb.CommitIDLong, map[string]bool, error) {
	return nil, nil, nil
}

// getIssuesFromCommits returns instances of Issue based on the provided commits and the set
// of issue ids. It is assumed that issueIDs only contains issues that have Rietveld details
// attached to them. Any commit that is not in issueIDs will be ommitted.
func (t *TrybotResults) getIssuesFromCommits(commits []*tracedb.CommitIDLong, issueIDs map[string]bool, isGerrit bool) []*Issue {
	return nil
}

// uniqueIssues returns the set of all issues contained in the given list of commit ids. If an issue does not
// have Rietveld information associated with it (i.e. the Details file is nil) it will be ommitted from the
// returned list of commit ids and the set of commit issue ids.
func (t *TrybotResults) uniqueIssues(commitIDs []*tracedb.CommitIDLong, isGerrit bool) ([]*tracedb.CommitIDLong, map[string]bool, time.Time) {
	return nil, nil, time.Time{}
}

// getIssueCreateTime extracts the create time from the details of the details
// field of the given CommitIDLong.
func getIssueCreateTime(cid *tracedb.CommitIDLong, isGerrit bool) time.Time {
	if isGerrit {
		return cid.Details.(*gerrit.ChangeInfo).Created
	}
	return cid.Details.(*rietveld.Issue).Created
}

// getIssuePatchSetsAndModified returns the patchsets for the given issue and
// the timestamp when it was last modified.
func getIssuePatchSetsAndModified(cid *tracedb.CommitIDLong, isGerrit bool) ([]int64, int64) {
	if isGerrit {
		changeInfo := cid.Details.(*gerrit.ChangeInfo)
		return changeInfo.GetPatchsetIDs(), changeInfo.Updated.Unix()
	}
	issue := cid.Details.(*rietveld.Issue)
	return issue.Patchsets, issue.Modified.Unix()
}

type IssuesSlice []*Issue

func (is IssuesSlice) Len() int           { return len(is) }
func (is IssuesSlice) Less(i, j int) bool { return is[i].Updated > is[j].Updated }
func (is IssuesSlice) Swap(i, j int)      { is[i], is[j] = is[j], is[i] }
