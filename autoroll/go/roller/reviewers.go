package roller

import (
	"fmt"
	"net/url"
	"strings"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/rotations"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// GetReviewers retrieves the current reviewers list.
func GetReviewers(metricsName string, reviewersSources, backupReviewers []string) ([]string, error) {
	tags := map[string]string{
		"roller": metricsName,
	}
	m := metrics2.GetInt64Metric("autoroll_get_reviewers_success", tags)
	success := int64(1)
	allEmails := util.StringSet{}
	for _, s := range reviewersSources {
		emails, err := getReviewersHelper(s)
		if err != nil {
			sklog.Errorf("Failed to retrieve reviewer(s) from %s: %s", s, err)
			success = 0
			emails = backupReviewers
		}
		allEmails.AddLists(emails)
	}
	m.Update(success)
	return allEmails.Keys(), nil
}

// getReviewersHelper interprets a single reviewer as either an email address or
// a URL; if the latter, it loads the reviewers list from the URL.
func getReviewersHelper(reviewer string) ([]string, error) {
	// If the passed-in reviewersConfig doesn't look like a URL, it's probably
	// an email address. Use it directly.
	if _, err := url.ParseRequestURI(reviewer); err != nil {
		if strings.Count(reviewer, "@") == 1 {
			return []string{reviewer}, nil
		} else {
			return nil, fmt.Errorf("Reviewer must be an email address or a valid URL; %q doesn't look like either.", reviewer)
		}
	}
	return rotations.FromURL(httputils.NewTimeoutClient(), reviewer)
}
