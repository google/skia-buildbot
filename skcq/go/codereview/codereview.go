package codereview

import (
	"context"
	"net/http"
	"strconv"

	"go.skia.org/infra/go/skerr"

	"go.skia.org/infra/go/gerrit"
)

// Defines a generic interface used by the different code-review frameworks.

// After this is done look at autoroller codereview framework as well.
type CodeReview interface {
	// TODO(rmistry): Will definitely need an abstraction for ChangeInfo after you know all the things you really want.
	Search(ctx context.Context) ([]*gerrit.ChangeInfo, error)

	// Have this return something like CodeReviewChange and then can pass this around below.
	GetDetails(cl int) string

	AddComment(cl int, comment string) string

	// Url returns the url of the issue identified by issueID or the
	// base URL of the Gerrit instance if issueID is 0.
	Url(issueID int64) string

	IsDryRun(ctx context.Context, ci *gerrit.ChangeInfo) bool

	IsCQ(ctx context.Context, ci *gerrit.ChangeInfo) bool

	RemoveFromCQ(ctx context.Context, ci *gerrit.ChangeInfo, reason string) error

	GetFileNames(ctx context.Context, issueID, patchsetID int64) ([]string, error)

	// Returns list of changes that will be submitted at the same time as the given change.
	// Note: The specified change will not be included in the returned slice of changes.
	GetSubmittedTogether(ctx context.Context, ci *gerrit.ChangeInfo) ([]*gerrit.ChangeInfo, error)

	Submit(ctx context.Context, ci *gerrit.ChangeInfo) error
}

// Extract this into it's own module under codereview called gerrit (also a mock one?)

// NewGerrit returns a gerritCodeReview instance.
func NewGerrit(httpClient *http.Client, config *gerrit.Config, gerritURL string) (CodeReview, error) {

	g, err := gerrit.NewGerritWithConfig(config, gerritURL, httpClient)
	if err != nil {
		return nil, err
	}
	return &gerritCodeReview{
		gerritClient: g,
		config:       config,
	}, nil
}

type gerritCodeReview struct {
	gerritClient gerrit.GerritInterface
	config       *gerrit.Config
}

func (gc *gerritCodeReview) Url(issueID int64) string {
	return gc.gerritClient.Url(issueID)
}

func (gc *gerritCodeReview) IsDryRun(ctx context.Context, ci *gerrit.ChangeInfo) bool {
	return gc.config.DryRunRunning(ci)
}

func (gc *gerritCodeReview) IsCQ(ctx context.Context, ci *gerrit.ChangeInfo) bool {
	return gc.config.CqRunning(ci)
}

func (gc *gerritCodeReview) GetFileNames(ctx context.Context, issueID, patchsetID int64) ([]string, error) {
	return gc.GetFileNames(ctx, issueID, patchsetID)
}

func (gc *gerritCodeReview) Search(ctx context.Context) ([]*gerrit.ChangeInfo, error) {
	openSearchTerm := gerrit.SearchStatus("OPEN")
	// Construct search labels from the provided config.
	// Do one search for CQ and one for dry-runs.

	// Below will need some better refactorings...

	searchTermsCQ := []*gerrit.SearchTerm{openSearchTerm}
	// DO not search for already approved because we do not want it to match. Let it fail with that verifier.
	// for label, val := range gc.gerritClient.Config().SelfApproveLabels {
	// 	searchTermsCQ = append(searchTermsCQ, gerrit.SearchLabel(label, strconv.Itoa(val)))
	// }
	for label, val := range gc.gerritClient.Config().SetCqLabels {
		searchTermsCQ = append(searchTermsCQ, gerrit.SearchLabel(label, strconv.Itoa(val)))
	}
	changesCQ, err := gc.gerritClient.Search(ctx, 100, true, searchTermsCQ...)
	if err != nil {
		return nil, skerr.Fmt("Could not search for CQ issues: %s", err)
	}

	searchTermsDryRun := []*gerrit.SearchTerm{openSearchTerm}
	for label, val := range gc.gerritClient.Config().SetDryRunLabels {
		searchTermsDryRun = append(searchTermsDryRun, gerrit.SearchLabel(label, strconv.Itoa(val)))
	}
	changesDryRun, err := gc.gerritClient.Search(ctx, 100, true, searchTermsDryRun...)
	if err != nil {
		return nil, skerr.Fmt("Could not search for dry-run issues: %s", err)
	}

	matchingChanges := append(changesCQ, changesDryRun...)

	// Convert matching changes to fully filled-in ChangeInfo objects.
	fullMatchingChanges := []*gerrit.ChangeInfo{}
	for _, ci := range matchingChanges {
		fullCI, err := gc.gerritClient.GetIssueProperties(ctx, ci.Issue)
		if err != nil {
			return nil, skerr.Fmt("Could not get full issue properties of %d", ci.Issue)
		}
		fullMatchingChanges = append(fullMatchingChanges, fullCI)
	}

	return fullMatchingChanges, nil
}

func (gc *gerritCodeReview) GetDetails(cl int) string {
	return ""
}

func (gc *gerritCodeReview) AddComment(cl int, comment string) string {
	return ""
}

func (gc *gerritCodeReview) RemoveFromCQ(ctx context.Context, ci *gerrit.ChangeInfo, reason string) error {
	return gc.gerritClient.RemoveFromCQ(ctx, ci, reason)
}

func (gc *gerritCodeReview) GetSubmittedTogether(ctx context.Context, ci *gerrit.ChangeInfo) ([]*gerrit.ChangeInfo, error) {
	changes, nonVisibleChanges, err := gc.gerritClient.SubmittedTogether(ctx, ci)
	if err != nil {
		return nil, skerr.Fmt("Could not get the list of submitted together changes: %s", err)
	}
	if nonVisibleChanges > 0 {
		return nil, skerr.Fmt("The SkCQ service account does not have access to view some submitted together changes of %d", ci.Issue)
	}
	// Filter out the specified ChangeInfo and return fully filled-in ChangeInfo objects.
	fullFilteredChanges := []*gerrit.ChangeInfo{}
	for _, c := range changes {
		if c.Id != ci.Id {
			fullCI, err := gc.gerritClient.GetIssueProperties(ctx, c.Issue)
			if err != nil {
				return nil, skerr.Fmt("Could not get full issue properties of %d", c.Issue)
			}
			fullFilteredChanges = append(fullFilteredChanges, fullCI)
		}
	}
	return fullFilteredChanges, nil
}

func (gc *gerritCodeReview) Submit(ctx context.Context, ci *gerrit.ChangeInfo) error {
	return gc.gerritClient.Submit(ctx, ci)
}
