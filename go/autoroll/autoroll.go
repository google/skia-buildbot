package autoroll

/*
	Convenience functions for retrieving AutoRoll CLs.
*/

import (
	"regexp"
	"sort"
	"time"

	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/util"
)

const (
	ROLL_AUTHOR        = "skia-deps-roller@chromium.org"
	POLLER_ROLLS_LIMIT = 10
	RECENT_ROLLS_LIMIT = 200
	RIETVELD_URL       = "https://codereview.chromium.org"
)

var (
	r              = rietveld.New(RIETVELD_URL, nil)
	ROLL_REV_REGEX = regexp.MustCompile("Roll .+ [0-9a-zA-Z]+\\.\\.([0-9a-zA-Z]+) \\(\\d+ commit.*\\)\\.")
)

// AutoRollIssue is a trimmed-down rietveld.Issue containing just the
// fields we care about for AutoRoll CLs.
type AutoRollIssue struct {
	Closed      bool         `json:"closed"`
	Committed   bool         `json:"committed"`
	CommitQueue bool         `json:"commitQueue"`
	Issue       int64        `json:"issue"`
	Modified    time.Time    `json:"modified"`
	Patchsets   []int64      `json:"patchSets"`
	Result      string       `json:"result"`
	Subject     string       `json:"subject"`
	TryResults  []*TryResult `json:"tryResults"`
}

// autoRollIssue returns an AutoRollIssue instance based on the given
// rietveld.Issue.
func autoRollIssue(i *rietveld.Issue) *AutoRollIssue {
	roll := &AutoRollIssue{
		Closed:      i.Closed,
		Committed:   i.Committed,
		CommitQueue: i.CommitQueue,
		Issue:       i.Issue,
		Modified:    i.Modified,
		Patchsets:   i.Patchsets,
		Subject:     i.Subject,
	}
	roll.Result = rollResult(roll)
	return roll
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

// TryResult is a struct which contains trybot result details.
type TryResult struct {
	Builder string `json:"builder"`
	Result  string `json:"result"`
	Status  string `json:"status"`
	Url     string `json:"url"`
}

type autoRollIssueSlice []*AutoRollIssue

func (s autoRollIssueSlice) Len() int           { return len(s) }
func (s autoRollIssueSlice) Less(i, j int) bool { return s[i].Modified.After(s[j].Modified) }
func (s autoRollIssueSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type tryResultSlice []*TryResult

func (s tryResultSlice) Len() int           { return len(s) }
func (s tryResultSlice) Less(i, j int) bool { return s[i].Builder < s[j].Builder }
func (s tryResultSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// search queries Rietveld for issues matching the known DEPS roll format.
func search(r *rietveld.Rietveld, limit int, terms ...*rietveld.SearchTerm) ([]*AutoRollIssue, error) {
	terms = append(terms, rietveld.SearchOwner(ROLL_AUTHOR))
	res, err := r.Search(limit, terms...)
	if err != nil {
		return nil, err
	}
	rv := make([]*AutoRollIssue, 0, len(res))
	for _, i := range res {
		if ROLL_REV_REGEX.FindString(i.Subject) != "" {
			rv = append(rv, autoRollIssue(i))
		}
	}
	sort.Sort(autoRollIssueSlice(rv))
	return rv, nil
}

// GetRecentRolls returns any DEPS rolls modified after the given Time, with a
// limit of RECENT_ROLLS_LIMIT.
func GetRecentRolls(modifiedAfter time.Time) ([]*AutoRollIssue, error) {
	issues, err := search(r, RECENT_ROLLS_LIMIT, rietveld.SearchModifiedAfter(modifiedAfter))
	if err != nil {
		return nil, err
	}
	return issues, nil
}

// GetLastNRolls returns the last N DEPS rolls.
func GetLastNRolls(n int) ([]*AutoRollIssue, error) {
	issues, err := search(r, n)
	if err != nil {
		return nil, err
	}
	if len(issues) <= n {
		return issues, nil
	}
	return issues[:n], nil
}

// AutoRollStatusPoller is a PollingStatus which periodically loads the recent
// DEPS rolls.
func AutoRollStatusPoller() (*util.PollingStatus, error) {
	return util.NewPollingStatus(func() (interface{}, error) {
		return GetLastNRolls(POLLER_ROLLS_LIMIT)
	}, 1*time.Minute)
}
