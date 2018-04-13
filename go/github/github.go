package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/github"

	"go.skia.org/infra/go/travisci"
)

const (
	GITHUB_TOKEN_METADATA_KEY   = "github_token"
	TRAVISCI_TOKEN_METADATA_KEY = "travisci_token"
)

var (
	CLOSED_STATE = "closed"
)

// GitHub is used for iteracting with the GitHub API.
type GitHub struct {
	client    *github.Client
	ctx       context.Context
	repoOwner string
	repoName  string
	TravisCi  *travisci.TravisCI
}

// NewGitHub returns a new GitHub instance.
func NewGitHub(ctx context.Context, repoOwner, repoName string, httpClient *http.Client, travisAccessToken string) (*GitHub, error) {
	travisCi, err := travisci.NewTravisCI(ctx, repoOwner, repoName, travisAccessToken)
	if err != nil {
		return nil, err
	}

	client := github.NewClient(httpClient)
	return &GitHub{
		client:    client,
		ctx:       ctx,
		repoOwner: repoOwner,
		repoName:  repoName,
		TravisCi:  travisCi,
	}, nil
}

// See https://developer.github.com/v3/issues/comments/#create-a-comment
// for the API documentation.
func (g *GitHub) AddComment(pullRequestNum int, msg string) error {
	comment := &github.IssueComment{
		Body: &msg,
	}
	_, resp, err := g.client.Issues.CreateComment(g.ctx, g.repoOwner, g.repoName, pullRequestNum, comment)
	if err != nil {
		return fmt.Errorf("Failed doing issues.createcomment: %s", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("Unexpected status code %d from issues.createcomment.", resp.StatusCode)
	}
	return nil
}

// See https://developer.github.com/v3/pulls/#get-a-single-pull-request
// for the API documentation.
func (g *GitHub) GetPullRequest(pullRequestNum int) (*github.PullRequest, error) {
	pullRequest, resp, err := g.client.PullRequests.Get(g.ctx, g.repoOwner, g.repoName, pullRequestNum)
	if err != nil {
		return nil, fmt.Errorf("Failed doing pullrequests.get: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected status code %d from pullrequests.get.", resp.StatusCode)
	}
	return pullRequest, nil
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

// See https://developer.github.com/v3/pulls/#merge-a-pull-request-merge-button
// for the API documentation.
func (g *GitHub) MergePullRequest(pullRequestNum int, msg string) error {
	_, resp, err := g.client.PullRequests.Merge(g.ctx, g.repoOwner, g.repoName, pullRequestNum, msg, &github.PullRequestOptions{})
	if err != nil {
		return fmt.Errorf("Failed doing pullrequests.merge: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Unexpected status code %d from pullrequests.merge.", resp.StatusCode)
	}
	return nil
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
