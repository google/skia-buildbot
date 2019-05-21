package autoroll

/*
	Convenience functions for retrieving AutoRoll CLs.
*/

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"

	github_api "github.com/google/go-github/github"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/comment"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	AUTOROLL_STATUS_URL = "https://autoroll.skia.org/json/status"
	POLLER_ROLLS_LIMIT  = 10
	RECENT_ROLLS_LIMIT  = 200

	ROLL_RESULT_DRY_RUN_SUCCESS     = "dry run succeeded"
	ROLL_RESULT_DRY_RUN_FAILURE     = "dry run failed"
	ROLL_RESULT_DRY_RUN_IN_PROGRESS = "dry run in progress"
	ROLL_RESULT_IN_PROGRESS         = "in progress"
	ROLL_RESULT_SUCCESS             = "succeeded"
	ROLL_RESULT_FAILURE             = "failed"

	TRYBOT_CATEGORY_CQ = "cq"

	TRYBOT_STATUS_STARTED   = "STARTED"
	TRYBOT_STATUS_COMPLETED = "COMPLETED"
	TRYBOT_STATUS_SCHEDULED = "SCHEDULED"

	TRYBOT_RESULT_CANCELED = "CANCELED"
	TRYBOT_RESULT_SUCCESS  = "SUCCESS"
	TRYBOT_RESULT_FAILURE  = "FAILURE"
)

var (
	// "RESTRICT AUTOMERGE: " is from skbug.com/8998
	ROLL_REV_REGEX = regexp.MustCompile(`^(?:(?:\[\S+\] )|(?:RESTRICT AUTOMERGE: ))?Roll \S+(?:\s+\S+)* (?:from )?(\S+)(?:(?:\.\.)|(?: to ))(\S+)(?: \(\d+ commit.*\))?\.?`)

	OPEN_ROLL_VALID_RESULTS = []string{
		ROLL_RESULT_DRY_RUN_FAILURE,
		ROLL_RESULT_DRY_RUN_IN_PROGRESS,
		ROLL_RESULT_DRY_RUN_SUCCESS,
		ROLL_RESULT_IN_PROGRESS,
	}

	DRY_RUN_RESULTS = []string{
		ROLL_RESULT_DRY_RUN_FAILURE,
		ROLL_RESULT_DRY_RUN_IN_PROGRESS,
		ROLL_RESULT_DRY_RUN_SUCCESS,
	}

	FAILURE_RESULTS = []string{
		ROLL_RESULT_DRY_RUN_FAILURE,
		ROLL_RESULT_FAILURE,
	}

	SUCCESS_RESULTS = []string{
		ROLL_RESULT_DRY_RUN_SUCCESS,
		ROLL_RESULT_SUCCESS,
	}
)

// AutoRollIssue is a struct containing the information we care about for
// AutoRoll CLs.
type AutoRollIssue struct {
	Closed            bool               `json:"closed"`
	Comments          []*comment.Comment `json:"comments"`
	Committed         bool               `json:"committed"`
	CommitQueue       bool               `json:"commitQueue"`
	CommitQueueDryRun bool               `json:"cqDryRun"`
	Created           time.Time          `json:"created"`
	Issue             int64              `json:"issue"`
	Modified          time.Time          `json:"modified"`
	Patchsets         []int64            `json:"patchSets"`
	Result            string             `json:"result"`
	RollingFrom       string             `json:"rollingFrom"`
	RollingTo         string             `json:"rollingTo"`
	Subject           string             `json:"subject"`
	TryResults        []*TryResult       `json:"tryResults"`
}

// Validate returns an error iff there is some problem with the issue.
func (i *AutoRollIssue) Validate() error {
	if i.Closed {
		if i.Result == ROLL_RESULT_IN_PROGRESS {
			return fmt.Errorf("AutoRollIssue cannot have a Result of %q if it is Closed.", ROLL_RESULT_IN_PROGRESS)
		}
	} else {
		if i.Committed {
			return errors.New("AutoRollIssue cannot be Committed without being Closed.")
		}
		if !util.In(i.Result, OPEN_ROLL_VALID_RESULTS) {
			return fmt.Errorf("AutoRollIssue which is not Closed must have as a Result one of: %v", OPEN_ROLL_VALID_RESULTS)
		}
	}
	return nil
}

// Copy returns a copy of the AutoRollIssue.
func (i *AutoRollIssue) Copy() *AutoRollIssue {
	var commentsCpy []*comment.Comment
	if i.Comments != nil {
		commentsCpy = make([]*comment.Comment, 0, len(i.Comments))
		for _, c := range i.Comments {
			commentsCpy = append(commentsCpy, c.Copy())
		}
	}
	var patchsetsCpy []int64
	if i.Patchsets != nil {
		patchsetsCpy = make([]int64, len(i.Patchsets))
		copy(patchsetsCpy, i.Patchsets)
	}
	var tryResultsCpy []*TryResult
	if i.TryResults != nil {
		tryResultsCpy = make([]*TryResult, 0, len(i.TryResults))
		for _, t := range i.TryResults {
			tryResultsCpy = append(tryResultsCpy, t.Copy())
		}
	}
	return &AutoRollIssue{
		Closed:            i.Closed,
		Comments:          commentsCpy,
		Committed:         i.Committed,
		CommitQueue:       i.CommitQueue,
		CommitQueueDryRun: i.CommitQueueDryRun,
		Created:           i.Created,
		Issue:             i.Issue,
		Modified:          i.Modified,
		Patchsets:         patchsetsCpy,
		Result:            i.Result,
		RollingFrom:       i.RollingFrom,
		RollingTo:         i.RollingTo,
		Subject:           i.Subject,
		TryResults:        tryResultsCpy,
	}
}

// UpdateFromGitHubPullRequest updates the AutoRollIssue instance based on the
// given PullRequest.
func (i *AutoRollIssue) UpdateFromGitHubPullRequest(ctx context.Context, pullRequest *github_api.PullRequest, g *github.GitHub) error {
	labels, err := g.GetLabels(pullRequest.GetNumber())
	if err != nil {
		return err
	}
	cq := false
	dryRun := false
	// If for some reason both COMMIT and DRYRUN labels are on the PR then
	// give precedence to DRYRUN.
	if util.In(github.DRYRUN_LABEL, labels) {
		dryRun = true
	} else if util.In(github.COMMIT_LABEL, labels) {
		cq = true
	}

	ps := make([]int64, 0, *pullRequest.Commits)
	for i := 1; i <= *pullRequest.Commits; i++ {
		ps = append(ps, int64(i))
	}
	i.Closed = pullRequest.GetState() == github.CLOSED_STATE
	i.Committed = pullRequest.GetMerged()
	i.CommitQueue = cq
	i.CommitQueueDryRun = dryRun
	i.Created = pullRequest.GetCreatedAt()
	i.Issue = int64(pullRequest.GetNumber())
	i.Modified = pullRequest.GetUpdatedAt()
	i.Patchsets = ps
	i.Subject = pullRequest.GetTitle()
	i.Result = rollResult(i)
	return nil
}

// UpdateFromGerritChangeInfo updates the AutoRollIssue instance based on the
// given gerrit.ChangeInfo.
func (i *AutoRollIssue) UpdateFromGerritChangeInfo(ctx context.Context, ci *gerrit.ChangeInfo, rollIntoAndroid bool) error {
	cq := false
	dryRun := false
	if rollIntoAndroid {
		rejected := false
		if _, ok := ci.Labels[gerrit.PRESUBMIT_VERIFIED_LABEL]; ok {
			for _, lb := range ci.Labels[gerrit.PRESUBMIT_VERIFIED_LABEL].All {
				if lb.Value == gerrit.PRESUBMIT_VERIFIED_LABEL_REJECTED {
					rejected = true
					break
				}
			}
		}
		if !rejected {
			if _, ok := ci.Labels[gerrit.AUTOSUBMIT_LABEL]; ok {
				for _, lb := range ci.Labels[gerrit.AUTOSUBMIT_LABEL].All {
					if lb.Value == gerrit.AUTOSUBMIT_LABEL_NONE {
						cq = true
						dryRun = true
					} else if lb.Value == gerrit.AUTOSUBMIT_LABEL_SUBMIT {
						cq = true
						dryRun = false
						break
					}
				}
			}
		}
	} else {
		if _, ok := ci.Labels[gerrit.COMMITQUEUE_LABEL]; ok {
			for _, lb := range ci.Labels[gerrit.COMMITQUEUE_LABEL].All {
				if lb.Value == gerrit.COMMITQUEUE_LABEL_DRY_RUN {
					cq = true
					dryRun = true
				} else if lb.Value == gerrit.COMMITQUEUE_LABEL_SUBMIT {
					cq = true
				}
			}
		}
	}

	ps := make([]int64, 0, len(ci.Patchsets))
	for _, p := range ci.Patchsets {
		ps = append(ps, p.Number)
	}
	i.Closed = ci.IsClosed()
	i.Committed = ci.Committed
	i.CommitQueue = cq
	i.CommitQueueDryRun = dryRun
	i.Created = ci.Created
	i.Issue = ci.Issue
	i.Modified = ci.Updated
	i.Patchsets = ps
	i.Subject = ci.Subject
	i.Result = rollResult(i)
	return nil
}

// rollResult derives a result string for the roll.
func rollResult(roll *AutoRollIssue) string {
	if roll.Closed {
		if roll.Committed {
			return ROLL_RESULT_SUCCESS
		} else {
			return ROLL_RESULT_FAILURE
		}
	}
	return ROLL_RESULT_IN_PROGRESS
}

// AllTrybotsFinished returns true iff all CQ trybots have finished for the
// given issue.
func (a *AutoRollIssue) AllTrybotsFinished() bool {
	for _, t := range a.TryResults {
		if t.Category != TRYBOT_CATEGORY_CQ {
			continue
		}
		if !t.Finished() {
			return false
		}
	}
	return true
}

// AtleastOneTrybotFailure returns true iff there is atleast one trybot that has
// failed for the given issue.
func (a *AutoRollIssue) AtleastOneTrybotFailure() bool {
	// For each trybot, find the most recent result.
	bots := map[string]*TryResult{}
	for _, t := range a.TryResults {
		if prev, ok := bots[t.Builder]; !ok || prev.Created.Before(t.Created) {
			bots[t.Builder] = t
		}
	}
	for _, t := range bots {
		sklog.Infof("  %s: %s (%s)", t.Builder, t.Result, t.Category)
		if t.Category != TRYBOT_CATEGORY_CQ {
			continue
		}
		if t.Failed() {
			return true
		}
	}
	return false
}

// AllTrybotsSucceeded returns true iff all CQ trybots have succeeded for the
// given issue. Note that some trybots may fail and be retried, in which case a
// successful retry counts as a success.
func (a *AutoRollIssue) AllTrybotsSucceeded() bool {
	// For each trybot, find the most recent result.
	bots := map[string]*TryResult{}
	for _, t := range a.TryResults {
		if prev, ok := bots[t.Builder]; !ok || prev.Created.Before(t.Created) {
			bots[t.Builder] = t
		}
	}
	sklog.Infof("AllTrybotsSucceeded? %d results.", len(bots))
	for _, t := range bots {
		sklog.Infof("  %s: %s (%s)", t.Builder, t.Result, t.Category)
		if t.Category != TRYBOT_CATEGORY_CQ {
			sklog.Infof("    ...skipping, not a CQ bot (category %q not %q)", t.Category, TRYBOT_CATEGORY_CQ)
			continue
		}
		if !t.Succeeded() {
			sklog.Infof("    ...failed")
			return false
		}
	}
	return true
}

// Failed returns true iff the roll failed (including dry run failure).
func (a *AutoRollIssue) Failed() bool {
	return util.In(a.Result, FAILURE_RESULTS)
}

// Succeeded returns true iff the roll succeeded (including dry run success).
func (a *AutoRollIssue) Succeeded() bool {
	return util.In(a.Result, SUCCESS_RESULTS)
}

// TryResult is a struct which contains trybot result details.
type TryResult struct {
	Builder  string    `json:"builder"`
	Category string    `json:"category"`
	Created  time.Time `json:"created_ts"`
	Result   string    `json:"result"`
	Status   string    `json:"status"`
	Url      string    `json:"url"`
}

// TryResultFromBuildbucket returns a new TryResult based on a buildbucketpb.Build.
func TryResultFromBuildbucket(b *buildbucket.Build) (*TryResult, error) {
	return &TryResult{
		Builder:  b.Parameters.BuilderName,
		Category: b.Parameters.Properties.Category,
		Created:  time.Time(b.Created),
		Result:   b.Result,
		Status:   b.Status,
		Url:      b.Url,
	}, nil
}

// TryResultsFromBuildbucket returns a slice of TryResults based on a slice of
// buildbucket.Builds.
func TryResultsFromBuildbucket(tries []*buildbucket.Build) ([]*TryResult, error) {
	res := make([]*TryResult, 0, len(tries))
	for _, t := range tries {
		tryResult, err := TryResultFromBuildbucket(t)
		if err != nil {
			return nil, err
		}
		res = append(res, tryResult)
	}
	sort.Sort(tryResultSlice(res))
	return res, nil
}

// Finished returns true iff the trybot is done running.
func (t TryResult) Finished() bool {
	return t.Status == TRYBOT_STATUS_COMPLETED
}

// Failed returns true iff the trybot completed and failed.
func (t TryResult) Failed() bool {
	return t.Finished() && t.Result == TRYBOT_RESULT_FAILURE
}

// Succeeded returns true iff the trybot completed successfully.
func (t TryResult) Succeeded() bool {
	return t.Finished() && t.Result == TRYBOT_RESULT_SUCCESS
}

// Copy returns a copy of the TryResult.
func (t *TryResult) Copy() *TryResult {
	return &TryResult{
		Builder:  t.Builder,
		Category: t.Category,
		Created:  t.Created,
		Result:   t.Result,
		Status:   t.Status,
		Url:      t.Url,
	}
}

type autoRollIssueSlice []*AutoRollIssue

func (s autoRollIssueSlice) Len() int           { return len(s) }
func (s autoRollIssueSlice) Less(i, j int) bool { return s[i].Modified.After(s[j].Modified) }
func (s autoRollIssueSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type tryResultSlice []*TryResult

func (s tryResultSlice) Len() int           { return len(s) }
func (s tryResultSlice) Less(i, j int) bool { return s[i].Builder < s[j].Builder }
func (s tryResultSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
