package goldingestion

import (
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/rietveld"
	tracedb "go.skia.org/infra/go/trace/db"
)

// TODO(stephana): Remove all Rietveld related code and function below as soon
// as the trybot package and old search is removed.

const (
	CONFIG_RIETVELD_CODE_REVIEW_URL = "RietveldCodeReviewURL"
)

// ExtractIssueInfo returns the issue id and the patchset id for a given commitID.
func ExtractIssueInfo(commitID *tracedb.CommitID, rietveldReview *rietveld.Rietveld, gerritReview *gerrit.Gerrit) (string, string) {
	issue, ok := gerritReview.ExtractIssue(commitID.Source)
	if ok {
		return issue, commitID.ID
	}
	issue, _ = rietveldReview.ExtractIssue(commitID.Source)
	return issue, commitID.ID
}
