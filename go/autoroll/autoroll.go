package autoroll

/*
	Convenience functions for retrieving AutoRoll CLs.
*/

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/golang/protobuf/ptypes"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/comment"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
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
	Closed         bool               `json:"closed"`
	Comments       []*comment.Comment `json:"comments"`
	Committed      bool               `json:"committed"`
	Created        time.Time          `json:"created"`
	IsDryRun       bool               `json:"isDryRun"`
	DryRunFinished bool               `json:"dryRunFinished"`
	DryRunSuccess  bool               `json:"dryRunSuccess"`
	CqFinished     bool               `json:"cqFinished"`
	CqSuccess      bool               `json:"cqSuccess"`
	Issue          int64              `json:"issue"`
	Modified       time.Time          `json:"modified"`
	Patchsets      []int64            `json:"patchSets"`
	Result         string             `json:"result"`
	RollingFrom    string             `json:"rollingFrom"`
	RollingTo      string             `json:"rollingTo"`
	Subject        string             `json:"subject"`
	TryResults     []*TryResult       `json:"tryResults"`
}

// Validate returns an error iff there is some problem with the issue.
func (i *AutoRollIssue) Validate() error {
	if i.Closed {
		if i.Result == ROLL_RESULT_DRY_RUN_IN_PROGRESS || i.Result == ROLL_RESULT_IN_PROGRESS {
			return fmt.Errorf("AutoRollIssue cannot have a Result of %q if it is Closed.", i.Result)
		}
	} else {
		if i.Committed {
			return errors.New("AutoRollIssue cannot be Committed without being Closed.")
		}
	}
	if i.DryRunFinished && !i.IsDryRun {
		return errors.New("DryRunFinished cannot be true unless the roll is a dry run.")
	}
	if i.CqSuccess && !i.CqFinished {
		return errors.New("CqSuccess cannot be true if CqFinished is not true.")
	}
	if i.DryRunSuccess && !i.DryRunFinished {
		return errors.New("DryRunSuccess cannot be true if DryRunFinished is not true.")
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
		Closed:         i.Closed,
		Comments:       commentsCpy,
		Committed:      i.Committed,
		Created:        i.Created,
		CqFinished:     i.CqFinished,
		CqSuccess:      i.CqSuccess,
		DryRunFinished: i.DryRunFinished,
		DryRunSuccess:  i.DryRunSuccess,
		IsDryRun:       i.IsDryRun,
		Issue:          i.Issue,
		Modified:       i.Modified,
		Patchsets:      patchsetsCpy,
		Result:         i.Result,
		RollingFrom:    i.RollingFrom,
		RollingTo:      i.RollingTo,
		Subject:        i.Subject,
		TryResults:     tryResultsCpy,
	}
}

// RollResult derives a result string for the roll.
func RollResult(roll *AutoRollIssue) string {
	if roll.IsDryRun {
		if roll.DryRunFinished {
			if roll.DryRunSuccess {
				return ROLL_RESULT_DRY_RUN_SUCCESS
			} else {
				return ROLL_RESULT_DRY_RUN_FAILURE
			}
		} else {
			return ROLL_RESULT_DRY_RUN_IN_PROGRESS
		}
	}
	if roll.CqFinished {
		if roll.CqSuccess {
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
	sklog.Infof("AllTrybotsSucceeded? %d results. DryRunFinished: %t. CqFinished: %t", len(bots), a.DryRunFinished, a.CqFinished)
	for _, t := range bots {
		sklog.Infof("  %s: %s (%s)", t.Builder, t.Result, t.Category)
		if t.Category != TRYBOT_CATEGORY_CQ {
			sklog.Infof("    ...skipping, not a CQ bot (category %q not %q)", t.Category, TRYBOT_CATEGORY_CQ)
			continue
		}
		if !t.Succeeded() {
			if t.Finished() {
				sklog.Infof("    ...failed")
			}
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
func TryResultFromBuildbucket(b *buildbucketpb.Build) (*TryResult, error) {
	isExperimental := false
	triggeredByCQ := false
	for _, tag := range b.Tags {
		if tag.Key == "user_agent" && tag.Value == "cq" {
			triggeredByCQ = true
		}
		if tag.Key == "cq_experimental" && tag.Value == "true" {
			isExperimental = true
		}
	}
	category := ""
	if triggeredByCQ {
		category = "cq"
		if isExperimental {
			category = "cq_experimental"
		}
	}

	status := TRYBOT_STATUS_SCHEDULED
	result := ""
	switch b.Status {
	case buildbucketpb.Status_STARTED:
		status = TRYBOT_STATUS_STARTED
	case buildbucketpb.Status_SUCCESS:
		status = TRYBOT_STATUS_COMPLETED
		result = TRYBOT_RESULT_SUCCESS
	case buildbucketpb.Status_FAILURE:
		status = TRYBOT_STATUS_COMPLETED
		result = TRYBOT_RESULT_FAILURE
	case buildbucketpb.Status_INFRA_FAILURE:
		status = TRYBOT_STATUS_COMPLETED
		result = TRYBOT_RESULT_FAILURE
	case buildbucketpb.Status_CANCELED:
		status = TRYBOT_STATUS_COMPLETED
		result = TRYBOT_RESULT_CANCELED
	}
	createTime, err := ptypes.Timestamp(b.CreateTime)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to convert timestamp for %d", b.Id)
	}
	createTime = createTime.UTC()
	return &TryResult{
		Builder:  b.Builder.Builder,
		Category: category,
		Created:  createTime,
		Result:   result,
		Status:   status,
		Url:      fmt.Sprintf(buildbucket.BUILD_URL_TMPL, buildbucket.DEFAULT_HOST, b.Id),
	}, nil
}

// TryResultsFromBuildbucket returns a slice of TryResults based on a slice of
// buildbucket.Builds.
func TryResultsFromBuildbucket(tries []*buildbucketpb.Build) ([]*TryResult, error) {
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

func TryResultsFromGithubChecks(checks []*github.Check, checksWaitFor []string) []*TryResult {
	tryResults := []*TryResult{}
	for _, check := range checks {
		sklog.Infof("Looking at check %+v", check)
		if check.ID != 0 {
			testStatus := TRYBOT_STATUS_STARTED
			testResult := ""
			switch check.State {
			case github.CHECK_STATE_PENDING:
				// Still pending.
			case github.CHECK_STATE_ERROR:
				// Fallthrough to the failure state below.
				fallthrough
			case github.CHECK_STATE_FAILURE:
				if util.In(check.Name, checksWaitFor) {
					sklog.Infof("%s has state %s. Waiting for it to succeed.", check.Name, github.CHECK_STATE_FAILURE)
				} else {
					testStatus = TRYBOT_STATUS_COMPLETED
					testResult = TRYBOT_RESULT_FAILURE
				}
			case github.CHECK_STATE_CANCELLED:
				testStatus = TRYBOT_STATUS_COMPLETED
				testResult = TRYBOT_RESULT_FAILURE
			case github.CHECK_STATE_TIMED_OUT:
				testStatus = TRYBOT_STATUS_COMPLETED
				testResult = TRYBOT_RESULT_FAILURE
			case github.CHECK_STATE_ACTION_REQUIRED:
				testStatus = TRYBOT_STATUS_COMPLETED
				testResult = TRYBOT_RESULT_FAILURE
			case github.CHECK_STATE_SUCCESS:
				testStatus = TRYBOT_STATUS_COMPLETED
				testResult = TRYBOT_RESULT_SUCCESS
			case github.CHECK_STATE_NEUTRAL:
				// Skipped tests show up as neutral so we can consider them successful.
				testStatus = TRYBOT_STATUS_COMPLETED
				testResult = TRYBOT_RESULT_SUCCESS
			}
			tryResult := &TryResult{
				Builder:  fmt.Sprintf("%s #%d", check.Name, check.ID),
				Category: TRYBOT_CATEGORY_CQ,
				Created:  check.StartedAt,
				Result:   testResult,
				Status:   testStatus,
			}
			if check.HTMLURL != "" {
				tryResult.Url = check.HTMLURL
			}
			tryResults = append(tryResults, tryResult)
		}
	}
	return tryResults
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
