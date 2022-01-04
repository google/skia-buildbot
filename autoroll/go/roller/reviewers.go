package roller

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/rotations"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	measurementGetReviewersSuccess  = "autoroll_get_reviewers_success"
	measurementGetReviewersNonEmpty = "autoroll_get_reviewers_nonempty"
)

// GetReviewers retrieves the current reviewers list. Does not return errors; if
// retrieval of the reviewers list fails, the provided backup reviewers are
// returned.
func GetReviewers(c *http.Client, metricsName string, reviewersSources, backupReviewers []string) []string {
	allEmails := util.StringSet{}
	for _, s := range reviewersSources {
		success := int64(1)
		emails, err := getReviewersHelper(c, s)
		if err != nil {
			sklog.Errorf("Failed to retrieve reviewer(s) from %s: %s", s, err)
			success = 0
		} else {
			allEmails.AddLists(emails)
		}

		metrics2.GetInt64Metric("autoroll_get_reviewers_success", map[string]string{
			"roller":          metricsName,
			"reviewer_source": s,
		}).Update(success)
	}

	nonEmpty := int64(1)
	if len(allEmails) == 0 {
		allEmails.AddLists(backupReviewers)
		nonEmpty = 0
	}
	metrics2.GetInt64Metric("autoroll_get_reviewers_nonempty", map[string]string{
		"roller": metricsName,
	}).Update(nonEmpty)

	// Sort for consistency in testing.
	rv := allEmails.Keys()
	sort.Strings(rv)
	return rv
}

// getReviewersHelper interprets a single reviewer as either an email address or
// a URL; if the latter, it loads the reviewers list from the URL.
func getReviewersHelper(c *http.Client, reviewer string) ([]string, error) {
	// If the passed-in reviewersConfig doesn't look like a URL, it's probably
	// an email address. Use it directly.
	if _, err := url.ParseRequestURI(reviewer); err != nil {
		if strings.Count(reviewer, "@") == 1 {
			return []string{reviewer}, nil
		} else {
			return nil, fmt.Errorf("Reviewer must be an email address or a valid URL; %q doesn't look like either.", reviewer)
		}
	}
	return rotations.FromURL(c, reviewer)
}
