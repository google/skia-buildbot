package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

var (
	CLOSED_STATE = "closed"
)

// GitHub is an object used for iteracting with the GitHub V3 API.
type GitHub struct {
	client    *github.Client
	ctx       context.Context
	repoOwner string
	repoName  string
}

// NewGitHub returns a new GitHub instance. If accessToken is empty then
// unauthenticated API calls are made.
func NewGitHub(ctx context.Context, repoOwner, repoName, accessToken string) (*GitHub, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})
	oauth2Client := oauth2.NewClient(ctx, ts)
	client := github.NewClient(oauth2Client)
	return &GitHub{
		client:    client,
		ctx:       ctx,
		repoOwner: repoOwner,
		repoName:  repoName,
	}, nil
}

// See https://developer.github.com/v3/pulls/#create-a-pull-request
// for the API documentation.
func (g *GitHub) CreatePullRequest(title, baseBranch, headBranch string) (*github.PullRequest, error) {
	newPullRequest := &github.NewPullRequest{
		Title: &title,
		Base:  &baseBranch,
		Head:  &headBranch,
	}
	pullRequest, resp, err := g.client.PullRequests.Create(g.ctx, g.repoOwner, g.repoName, newPullRequest)
	if err != nil {
		return nil, fmt.Errorf("Failed doing pullrequests.create: %s", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("Unexpected status code %d from pullrequests.create.", resp.StatusCode)
	}
	return pullRequest, nil
}

// See https://developer.github.com/v3/pulls/#update-a-pull-request
// for the API documentation.
func (g *GitHub) ClosePullRequest(pullRequestNum int) (*github.PullRequest, error) {
	editPullRequest := &github.PullRequest{
		State: &CLOSED_STATE,
	}
	edittedPullRequest, resp, err := g.client.PullRequests.Edit(g.ctx, g.repoOwner, g.repoName, pullRequestNum, editPullRequest)
	if err != nil {
		return nil, fmt.Errorf("Failed doing pullrequests.edit: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected status code %d from pullrequests.edit.", resp.StatusCode)
	}
	if edittedPullRequest.GetState() != CLOSED_STATE {
		return nil, fmt.Errorf("Tried to close pull request %d but the state is %s", pullRequestNum, edittedPullRequest.GetState())
	}
	return edittedPullRequest, nil
}
