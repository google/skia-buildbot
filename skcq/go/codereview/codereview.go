package codereview

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"go.skia.org/infra/go/skerr"

	"go.skia.org/infra/go/gerrit"
)

// Defines a generic interface used by the different code-review frameworks.

// After this is done look at autoroller codereview framework as well.
type CodeReview interface {
	Search(ctx context.Context) (string, error)

	// Have this return something like CodeReviewChange and then can pass this around below.
	GetDetails(cl int) string

	AddComment(cl int, comment string) string

	UpdateLabel(cl int) string

	Submit() string
}

// Extract this into it's own module under codereview called gerrit (also a mock one?)

// NewGerrit returns a gerritCodeReview instance.
func NewGerrit(httpClient *http.Client, config *gerrit.Config, gerritURL string, supportedRepos []string) (CodeReview, error) {

	g, err := gerrit.NewGerritWithConfig(config, gerritURL, httpClient)
	if err != nil {
		return nil, err
	}
	return &gerritCodeReview{
		gerritClient:   g,
		supportedRepos: supportedRepos,
	}, nil
}

type gerritCodeReview struct {
	gerritClient   gerrit.GerritInterface
	supportedRepos []string
}

func (gc *gerritCodeReview) Search(ctx context.Context) (string, error) {
	// Construct search labels from the provided config.
	// Do one search for CQ and one for dry-runs.
	searchTermsCQ := []*gerrit.SearchTerm{}
	for label, val := range gc.gerritClient.Config().SelfApproveLabels {
		searchTermsCQ = append(searchTermsCQ, gerrit.SearchLabel(label, strconv.Itoa(val)))
	}
	for label, val := range gc.gerritClient.Config().SetCqLabels {
		searchTermsCQ = append(searchTermsCQ, gerrit.SearchLabel(label, strconv.Itoa(val)))
	}
	changesCQ, err := gc.gerritClient.Search(ctx, 100, true, searchTermsCQ...)
	if err != nil {
		return "", skerr.Fmt("Could not search for CQ issues: %s", err)
	}

	searchTermsDryRun := []*gerrit.SearchTerm{}
	for label, val := range gc.gerritClient.Config().SelfApproveLabels {
		searchTermsDryRun = append(searchTermsDryRun, gerrit.SearchLabel(label, strconv.Itoa(val)))
	}
	for label, val := range gc.gerritClient.Config().SetDryRunLabels {
		searchTermsDryRun = append(searchTermsDryRun, gerrit.SearchLabel(label, strconv.Itoa(val)))
	}
	changesDryRun, err := gc.gerritClient.Search(ctx, 100, true, searchTermsDryRun...)
	if err != nil {
		return "", skerr.Fmt("Could not search for dry-run issues: %s", err)
	}

	fmt.Println("WE GOT THIS!")
	// Start debug one to see what is happening.
	fmt.Println(changesCQ)
	fmt.Println(changesDryRun)
	for _, ci := range changesCQ {
		fmt.Println(ci.Issue)
	}

	return "", nil
}

func (gc *gerritCodeReview) GetDetails(cl int) string {
	return ""
}

func (gc *gerritCodeReview) AddComment(cl int, comment string) string {
	return ""
}

func (gc *gerritCodeReview) UpdateLabel(cl int) string {
	return ""
}

func (gc *gerritCodeReview) Submit() string {
	return ""
}
