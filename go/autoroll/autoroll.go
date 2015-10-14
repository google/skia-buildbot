package autoroll

/*
	Convenience functions for retrieving AutoRoll CLs.
*/

import (
	"time"

	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/util"
)

const (
	OWNER              = "skia-deps-roller@chromium.org"
	POLLER_ROLLS_LIMIT = 10
	RECENT_ROLLS_LIMIT = 200
	RIETVELD_URL       = "https://codereview.chromium.org"
)

var r = rietveld.New(RIETVELD_URL, nil)

// AutoRollIssue is a trimmed-down rietveld.Issue containing just the
// fields we care about for AutoRoll CLs.
type AutoRollIssue struct {
	Closed    bool
	Committed bool
	Issue     int
	Modified  time.Time
}

func search(limit int, terms ...*rietveld.SearchTerm) ([]*AutoRollIssue, error) {
	terms = append(terms, rietveld.SearchOwner(OWNER))
	res, err := r.Search(limit, terms...)
	if err != nil {
		return nil, err
	}
	rv := make([]*AutoRollIssue, 0, len(res))
	for _, i := range res {
		rv = append(rv, &AutoRollIssue{
			Closed:    i.Closed,
			Committed: i.Committed,
			Issue:     i.Issue,
			Modified:  i.Modified,
		})
	}
	return rv, nil
}

// GetRecentRolls returns
func GetRecentRolls(modifiedAfter time.Time) ([]*AutoRollIssue, error) {
	return search(RECENT_ROLLS_LIMIT, rietveld.SearchModifiedAfter(modifiedAfter))
}

func GetLastNRolls(n int) ([]*AutoRollIssue, error) {
	issues, err := search(n)
	if err != nil {
		return nil, err
	}
	if len(issues) <= n {
		return issues, nil
	}
	return issues[:n], nil
}

func AutoRollStatusPoller() (*util.PollingStatus, error) {
	return util.NewPollingStatus(func() (interface{}, error) {
		return GetLastNRolls(POLLER_ROLLS_LIMIT)
	}, 1*time.Minute)
}
