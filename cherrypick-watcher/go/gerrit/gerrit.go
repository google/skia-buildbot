package gerrit

import (
	"context"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	maxGerritSearchResults = 100
)

// FindAllOpenCherrypicks finds all open cherrypicks from the provided
// repo+branch.
func FindAllOpenCherrypicks(ctx context.Context, gerritClient gerrit.GerritInterface, repo, branch string) ([]*gerrit.ChangeInfo, error) {

	// Query gerrit for open changes in the repo+branch combination.
	searchTerms := []*gerrit.SearchTerm{
		gerrit.SearchProject(repo),
		gerrit.SearchBranch(branch),
		gerrit.SearchStatus(gerrit.ChangeStatusOpen),
	}
	changes, err := gerritClient.Search(ctx, maxGerritSearchResults, true, searchTerms...)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not query gerrit for repo %s and branch %s", repo, branch)
	}
	return changes, nil
}

// IsCherrypickIn returns whether the specified cherrypick exists in the
// provided repo+branch.
func IsCherrypickIn(ctx context.Context, gerritClient gerrit.GerritInterface, repo, branch string, cherrypickChange int) (bool, error) {

	searchTerms := []*gerrit.SearchTerm{
		gerrit.SearchProject(repo),
		gerrit.SearchBranch(branch),
		gerrit.SearchCherrypickOf(cherrypickChange),
	}
	changes, err := gerritClient.Search(ctx, maxGerritSearchResults, true, searchTerms...)
	if err != nil {
		return false, skerr.Wrapf(err, "Could not query gerrit for repo %s and branch %s", repo, branch)
	}
	if len(changes) == 0 {
		return false, nil
	}

	// Log the cherrypicked change numbers. Use for debugging.
	changeNums := []int64{}
	for _, c := range changes {
		changeNums = append(changeNums, c.Issue)
	}
	sklog.Infof("The cherrypick %d was found in changes %+v in %s %s", cherrypickChange, changeNums, repo, branch)

	return true, nil
}

// AddReminderComment adds the specified comment to the provided partial ChangeInfo obj.
func AddReminderComment(ctx context.Context, gerritClient gerrit.GerritInterface, partialChange *gerrit.ChangeInfo, comment string) error {
	// A fully populated ChangeInfo obj is required to add comments.
	ci, err := gerritClient.GetIssueProperties(ctx, partialChange.Issue)
	if err != nil {
		return skerr.Wrapf(err, "Could not get issue properties for %d", partialChange.Issue)
	}
	// Publish the provided comment.
	if err := gerritClient.AddComment(ctx, ci, comment); err != nil {
		return skerr.Wrapf(err, "Could not add a comment to %d", ci.Issue)
	}
	return nil
}
