// trybot implements routines to retrieve trybot results from the tracedb data store.
//
package trybot

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/goldingestion"
)

var (
	TIME_FRAME = time.Hour * time.Duration(24*14)
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
	CommitIDs []*tracedb.CommitIDLong `json:"-"`
}

// TrybotResults manages everything related to aggregating information about trybot results.
type TrybotResults struct {
	tileBuilder tracedb.BranchTileBuilder
	reviewURL   string
}

func NewTrybotResults(tileBuilder tracedb.BranchTileBuilder, rietveldAPI *rietveld.Rietveld) *TrybotResults {
	ret := &TrybotResults{
		tileBuilder: tileBuilder,
		reviewURL:   rietveldAPI.Url(),
	}
	return ret
}

// ListTrybotIssues returns all the issues that have recently seen trybot updates. The given
// offset and size return a subset of the list. Aside from the issues we return also the
// total number of current issues to allow pagination.
func (t *TrybotResults) ListTrybotIssues(offset, size int) ([]*Issue, int, error) {
	end := time.Now()
	begin := end.Add(-TIME_FRAME)

	commits, issueIDs, err := t.getCommits(begin, end, t.reviewURL)
	if err != nil {
		return nil, 0, err
	}

	issuesList := t.getIssuesFromCommits(commits, issueIDs)
	maxIdx := util.MaxInt(0, len(issuesList)-1)
	offset = util.MinInt(util.MaxInt(offset, 0), maxIdx)
	size = util.MaxInt(size, 0)
	startIdx := offset
	endIdx := util.MinInt(offset+size, maxIdx)
	return issuesList[startIdx:endIdx], len(issuesList), nil
}

// GetIssue returns the information about a specific issue. It returns the a superset
// of the issue information including the commit ids that make up the issue.
// The commmit ids align with the tile that is returned.
func (t *TrybotResults) GetIssue(issueID string) (*IssueDetails, *tiling.Tile, error) {
	end := time.Now()
	begin := end.Add(-TIME_FRAME)
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

	return &IssueDetails{
		Issue:     issue,
		CommitIDs: commits,
	}, tile, nil
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
