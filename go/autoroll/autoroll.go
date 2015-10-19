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
	// TODO(borenet): Change this to "skia-deps-roller@chromium.org" once
	// the server is working end-to-end.
	ROLL_AUTHOR        = "borenet@google.com"
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
	Closed      bool      `json:"closed"`
	Committed   bool      `json:"committed"`
	CommitQueue bool      `json:"commitQueue"`
	Issue       int64     `json:"issue"`
	Modified    time.Time `json:"modified"`
	Subject     string    `json:"subject"`
}

type autoRollIssueSlice []*AutoRollIssue

func (s autoRollIssueSlice) Len() int           { return len(s) }
func (s autoRollIssueSlice) Less(i, j int) bool { return s[i].Modified.After(s[j].Modified) }
func (s autoRollIssueSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func search(r *rietveld.Rietveld, limit int, terms ...*rietveld.SearchTerm) ([]*AutoRollIssue, error) {
	terms = append(terms, rietveld.SearchOwner(ROLL_AUTHOR))
	res, err := r.Search(limit, terms...)
	if err != nil {
		return nil, err
	}
	rv := make([]*AutoRollIssue, 0, len(res))
	for _, i := range res {
		if ROLL_REV_REGEX.FindString(i.Subject) != "" {
			rv = append(rv, &AutoRollIssue{
				Closed:      i.Closed,
				Committed:   i.Committed,
				CommitQueue: i.CommitQueue,
				Issue:       i.Issue,
				Modified:    i.Modified,
				Subject:     i.Subject,
			})
		}
	}
	return rv, nil
}

// GetRecentRolls returns any DEPS rolls modified after the given Time, with a
// limit of RECENT_ROLLS_LIMIT.
func GetRecentRolls(modifiedAfter time.Time) ([]*AutoRollIssue, error) {
	issues, err := search(r, RECENT_ROLLS_LIMIT, rietveld.SearchModifiedAfter(modifiedAfter))
	if err != nil {
		return nil, err
	}
	sort.Sort(autoRollIssueSlice(issues))
	return issues, nil
}

// GetLastNRolls returns the last N DEPS rolls.
func GetLastNRolls(n int) ([]*AutoRollIssue, error) {
	issues, err := search(r, n)
	if err != nil {
		return nil, err
	}
	sort.Sort(autoRollIssueSlice(issues))
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
