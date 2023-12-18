package backends

import (
	"context"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"

	"golang.org/x/oauth2/google"
)

const (
	DEFAULT_GITILES_SCOPE = "https://www.googleapis.com/auth/gerritcodereview"
)

// createGitilesClient creates an authenticated client to communicate with Gitiles.
// This is primarily a helper function to CreateRepository.
func createGitilesClient(ctx context.Context, repositoryUrl string) (*gitiles.Repo, error) {
	token, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly, DEFAULT_GITILES_SCOPE)
	if err != nil {
		return nil, err
	}

	// Any non-200 response will be rejected.
	// Note that Gitiles will return non 200 for ~1 minute if we hit the API rate limit.
	// TODO(jeffyoon@): utilize httpNewConfiguredBackOffTransport to create a backoff transport.
	c := httputils.DefaultClientConfig().WithTokenSource(token).With2xxOnly().Client()
	repo := gitiles.NewRepo(repositoryUrl, c)

	return repo, nil
}

// isRepositoryUrlValid checks validity of the repository url.
func isRepositoryUrlValid(repositoryUrl string) error {
	if repositoryUrl == "" {
		return skerr.Fmt("The repository URL is a required parameter and must be defined.")
	}

	return nil
}

// CreateRepository returns an authenticated client to communicate with Gitiles.
func CreateRepository(ctx context.Context, repositoryUrl string) (*gitiles.Repo, error) {
	if err := isRepositoryUrlValid(repositoryUrl); err != nil {
		return nil, err
	}

	gc, err := createGitilesClient(ctx, repositoryUrl)
	if err != nil {
		return nil, err
	}

	return gc, nil
}
