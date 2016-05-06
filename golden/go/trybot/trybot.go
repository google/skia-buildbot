// trybot implements routines to retrieve trybot results from the tracedb data store.
//
package trybot

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	cache "github.com/patrickmn/go-cache"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/rietveld"
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
	ID        string  `json:"id"`
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
	TargetPatchsets []string                  `json:"-"`
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
	tileBuilder    tracedb.BranchTileBuilder
	reviewURL      string
	rietveldAPI    *rietveld.Rietveld
	ingestionStore *goldingestion.IngestionStore
	timeFrame      time.Duration
	patchsetCache  *cache.Cache
}

func NewTrybotResults(tileBuilder tracedb.BranchTileBuilder, rietveldAPI *rietveld.Rietveld, ingestionStore *goldingestion.IngestionStore) *TrybotResults {
	ret := &TrybotResults{
		tileBuilder:    tileBuilder,
		reviewURL:      rietveldAPI.Url(),
		rietveldAPI:    rietveldAPI,
		ingestionStore: ingestionStore,
		timeFrame:      TIME_FRAME,
		patchsetCache:  cache.New(PATCHSET_CACHE_EXPIRATION, PATCHSET_CACHE_CLEANUP_INTERVAL),
	}
	return ret
}

// ListTrybotIssues returns all the issues that have recently seen trybot updates. The given
// offset and size return a subset of the list. Aside from the issues we return also the
// total number of current issues to allow pagination.
func (t *TrybotResults) ListTrybotIssues(offset, size int) ([]*Issue, int, error) {
	end := time.Now()
	begin := end.Add(-t.timeFrame)

	commits, issueIDs, err := t.getCommits(begin, end, t.reviewURL)
	if err != nil {
		return nil, 0, err
	}

	issuesList := t.getIssuesFromCommits(commits, issueIDs)
	if len(issuesList) == 0 {
		return []*Issue{}, 0, nil
	}

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
func (t *TrybotResults) GetIssue(issueID string, targetPatchsets []string) (*IssueDetails, *tiling.Tile, error) {
	end := time.Now()
	begin := end.Add(-t.timeFrame)
	prefix := goldingestion.GetPrefix(issueID, t.reviewURL)
	commits, issueIDs, err := t.getCommits(begin, end, prefix)

	if err != nil {
		return nil, nil, err
	}

	if len(commits) == 0 {
		return nil, nil, nil
	}

	issue := t.getIssuesFromCommits(commits, issueIDs)[0]
	tile, err := t.tileBuilder.CachedTileFromCommits(tracedb.ShortFromLong(commits))
	if err != nil {
		return nil, nil, fmt.Errorf("Error retrieving tile: %s", err)
	}

	// Retrieve all the patchset results for this commit.
	patchsetDetails, targetPatchsets, err := t.getPatchsetDetails(issue, targetPatchsets)
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

func (t *TrybotResults) getPatchsetDetails(issue *Issue, targetPatchsets []string) (map[int64]*PatchsetDetail, []string, error) {
	// Get the target patchsets.
	var int64PidMap map[int64]bool = nil

	// if no patchset was given, use the last one.
	if len(targetPatchsets) == 0 {
		pset := issue.Patchsets[len(issue.Patchsets)-1]
		targetPatchsets = []string{strconv.Itoa(int(pset))}
		int64PidMap = map[int64]bool{pset: true}
	} else {
		int64PidMap = make(map[int64]bool, len(targetPatchsets))
		for _, k := range targetPatchsets {
			convKey, err := strconv.ParseInt(k, 10, 64)
			if err != nil {
				return nil, nil, fmt.Errorf("Invalid patchset id (%s): %s", k, err)
			}
			int64PidMap[convKey] = true
		}
	}

	ret := make(map[int64]*PatchsetDetail, len(issue.Patchsets))
	var wg sync.WaitGroup
	var mutex sync.Mutex

	intIssueID, err := strconv.ParseInt(issue.ID, 10, 64)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to parse issue id %s. Got error: %s", issue.ID, err)
	}

	for _, pid := range issue.Patchsets {
		wg.Add(1)
		go func(pid int64) {
			defer wg.Done()
			crPatchSet, err := t.getCachedPatchset(intIssueID, pid, int64PidMap)
			if err != nil {
				glog.Errorf("Error retrieving patchset %d: %s", pid, err)
				return
			}

			nTryjobs := len(crPatchSet.TryjobResults)
			tryjobs := make([]*Tryjob, 0, nTryjobs)
			var tjIngested int64 = 0
			for _, tj := range crPatchSet.TryjobResults {
				// Filter out tryjobs we want to ignore. This includes compile bots, since we'll never
				// ingest any results from them. We count the bot as ingested.
				if filterTryjob(tj) {
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

			pSet := &PatchsetDetail{
				ID:       pid,
				Tryjobs:  tryjobs,
				JobDone:  tjIngested,
				JobTotal: int64(nTryjobs),
			}

			mutex.Lock()
			defer mutex.Unlock()
			ret[pid] = pSet
		}(pid)
	}
	wg.Wait()
	return ret, targetPatchsets, nil
}

func (t *TrybotResults) getCachedPatchset(intIssueID, pid int64, targetPatchsets map[int64]bool) (*rietveld.Patchset, error) {
	key := strconv.FormatInt(intIssueID, 10) + ":" + strconv.FormatInt(pid, 10)

	// Check for the key.
	if val, ok := t.patchsetCache.Get(key); ok {
		return val.(*rietveld.Patchset), nil
	}

	val, err := t.rietveldAPI.GetPatchset(intIssueID, pid)
	if err != nil {
		return nil, err
	}

	t.patchsetCache.Set(key, val, 0)
	return val, nil
}

// filterTryjob returns true if the given tryjob should be ignored.
func filterTryjob(tj *rietveld.TryjobResult) bool {
	return !strings.HasPrefix(tj.Builder, "Test")
}

func (t *TrybotResults) getCommits(startTime, endTime time.Time, prefix string) ([]*tracedb.CommitIDLong, map[string]bool, error) {
	commits, err := t.tileBuilder.ListLong(startTime, endTime, prefix)
	if err != nil {
		return nil, nil, fmt.Errorf("Error retrieving commits in the range %s - %s. Got error: %s", startTime, endTime, err)
	}

	commits, issueIDs, newBegin := t.uniqueIssues(commits)

	// Retrieve any commitIDs we have not retrieved for the issues of interest.
	earlierCommits, err := t.tileBuilder.ListLong(newBegin, startTime, t.reviewURL)
	if err != nil {
		return nil, nil, fmt.Errorf("Error retrieving commits in the range %s - %s. Got error: %s", newBegin, startTime, err)
	}

	// Only get the commitIDs we are interested in.
	temp := make([]*tracedb.CommitIDLong, 0, len(earlierCommits))
	for _, cid := range earlierCommits {
		iid, _ := goldingestion.ExtractIssueInfo(cid.CommitID, t.reviewURL)
		if issueIDs[iid] {
			temp = append(temp, cid)
		}
	}
	commits = append(temp, commits...)
	return commits, issueIDs, nil
}

// getIssuesFromCommits returns instances of Issue based on the provided commits and the set
// of issue ids. It is assumed that issueIDs only contains issues that have Rietveld details
// attached to them. Any commit that is not in issueIDs will be ommitted.
func (t *TrybotResults) getIssuesFromCommits(commits []*tracedb.CommitIDLong, issueIDs map[string]bool) []*Issue {
	codeReviewURL := strings.TrimSuffix(t.reviewURL, "/")
	issueMap := make(map[string]*Issue, len(issueIDs))
	for _, cid := range commits {
		iid, _ := goldingestion.ExtractIssueInfo(cid.CommitID, t.reviewURL)

		// Ignore issues that are not in issueIDs
		if !issueIDs[iid] {
			continue
		}

		issue, ok := issueMap[iid]
		if !ok {
			details := cid.Details.(*rietveld.Issue)
			issue = &Issue{
				ID:        iid,
				Subject:   cid.Desc,
				Owner:     cid.Author,
				Updated:   details.Modified.Unix(),
				URL:       fmt.Sprintf("%s/%s", codeReviewURL, iid),
				Patchsets: details.Patchsets,
			}
			issueMap[iid] = issue
		}
	}

	ret := make([]*Issue, 0, len(issueMap))
	for _, issue := range issueMap {
		ret = append(ret, issue)
	}

	sort.Sort(IssuesSlice(ret))
	return ret
}

// uniqueIssues returns the set of all issues contained in the given list of commit ids. If an issue does not
// have Rietveld information associated with it (i.e. the Details file is nil) it will be ommitted from the
// returned list of commit ids and the set of commit issue ids.
func (t *TrybotResults) uniqueIssues(commitIDs []*tracedb.CommitIDLong) ([]*tracedb.CommitIDLong, map[string]bool, time.Time) {
	minTime := time.Now()
	issueIDs := map[string]bool{}
	ret := make([]*tracedb.CommitIDLong, 0, len(commitIDs))

	for _, cid := range commitIDs {
		if cid.Details == nil {
			continue
		}
		ret = append(ret, cid)
		iid, _ := goldingestion.ExtractIssueInfo(cid.CommitID, t.reviewURL)
		issueIDs[iid] = true
		rIssue := cid.Details.(*rietveld.Issue)
		if minTime.After(rIssue.Created) {
			minTime = rIssue.Created
		}
	}
	return ret, issueIDs, minTime
}

type IssuesSlice []*Issue

func (is IssuesSlice) Len() int           { return len(is) }
func (is IssuesSlice) Less(i, j int) bool { return is[i].Updated > is[j].Updated }
func (is IssuesSlice) Swap(i, j int)      { is[i], is[j] = is[j], is[i] }
