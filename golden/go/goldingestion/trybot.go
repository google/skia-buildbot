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

// TODO: remove
// ExtractIssueInfo returns the issue id and the patchset id for a given commitID.
func ExtractIssueInfo(commitID *tracedb.CommitID, rietveldReview *rietveld.Rietveld, gerritReview *gerrit.Gerrit) (int64, int64) {
	return 0, 0
}
