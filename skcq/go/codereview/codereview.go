package codereview

import (
	"net/http"

	"go.skia.org/infra/go/gerrit"
)

// Defines a generic interface used by the different code-review frameworks.

// After this is done look at autoroller codereview framework as well.
type CodeReview interface {
	Search() string

	// Have this return something like CodeReviewChange and then can pass this around below.
	GetDetails(cl int) string

	AddComment(cl int, comment string) string

	UpdateLabel(cl int) string

	Submit() string
}

// Extract this into it's own module under codereview called gerrit (also a mock one?)

// NewGerrit returns a gerritCodeReview instance.
func NewGerrit(httpClient *http.Client) (CodeReview, error) {

	// Might have to make the config configurable one day.
	g, err := gerrit.NewGerritWithConfig(gerrit.ConfigSkia, gerrit.GerritSkiaURL, httpClient)
	if err != nil {
		return nil, err
	}
	return &gerritCodeReview{
		gerritClient: g,
	}, nil
}

type gerritCodeReview struct {
	gerritClient gerrit.GerritInterface
}

func (g *gerritCodeReview) Search() string {
	return ""
}

func (g *gerritCodeReview) GetDetails(cl int) string {
	return ""
}

func (g *gerritCodeReview) AddComment(cl int, comment string) string {
	return ""
}

func (g *gerritCodeReview) UpdateLabel(cl int) string {
	return ""
}

func (g *gerritCodeReview) Submit() string {
	return ""
}
