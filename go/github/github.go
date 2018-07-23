// package github provides a library for interacting with Github via it's API:
// https://developer.github.com/v3/
//
// This library assumes that the http.Client provided in NewGitHub
// contains the appropriate authentication.
// One way to authenticate is to use a personal access token as described in
// https://developer.github.com/v3/auth/. Apps can retreive this from metadata.
// Other way to authenticate is to use client_id and client_secret as described
// in https://developer.github.com/v3/oauth_authorizations/. That can also be
// retreived by apps via metadata.
//
// We would rather use service accounts but Github only supports service
// accounts in Github apps:
// https://developer.github.com/apps/differences-between-apps/
//
// For information on the travis access token see the travisci package.

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

	GITHUB_TOKEN_LOCAL_FILENAME   = "github_token"
	TRAVISCI_TOKEN_LOCAL_FILENAME = "travisci_token"

	MERGE_METHOD_SQUASH = "squash"
	MERGE_METHOD_REBASE = "rebase"

	MERGEABLE_STATE_DIRTY    = "dirty"    // Merge conflict.
	MERGEABLE_STATE_CLEAN    = "clean"    // No conflicts.
	MERGEABLE_STATE_UNKNOWN  = "unknown"  // Mergeablility was not checked yet.
	MERGEABLE_STATE_UNSTABLE = "unstable" // Failing or pending commit status.

	COMMIT_LABEL = "autoroller: commit"
	DRYRUN_LABEL = "autoroller: dyrun"
)

var (
	CLOSED_STATE = "closed"
)

// GitHub is used for iteracting with the GitHub API.
type GitHub struct {
	RepoOwner string
	RepoName  string
	TravisCi  *travisci.TravisCI

	client *github.Client
	ctx    context.Context
}

// NewGitHub returns a new GitHub instance.
func NewGitHub(ctx context.Context, repoOwner, repoName string, httpClient *http.Client, travisAccessToken string) (*GitHub, error) {
	travisCi, err := travisci.NewTravisCI(ctx, repoOwner, repoName, travisAccessToken)
	if err != nil {
		return nil, err
	}

	client := github.NewClient(httpClient)
	return &GitHub{
		RepoOwner: repoOwner,
		RepoName:  repoName,
		TravisCi:  travisCi,
		client:    client,
		ctx:       ctx,
	}, nil
}

// See https://developer.github.com/v3/issues/comments/#create-a-comment
// for the API documentation.
func (g *GitHub) AddComment(pullRequestNum int, msg string) error {
	comment := &github.IssueComment{
		Body: &msg,
	}
	_, resp, err := g.client.Issues.CreateComment(g.ctx, g.RepoOwner, g.RepoName, pullRequestNum, comment)
	if err != nil {
		return fmt.Errorf("Failed doing issues.createcomment: %s", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("Unexpected status code %d from issues.createcomment.", resp.StatusCode)
	}
	return nil
}

// See https://developer.github.com/v3/users/#get-the-authenticated-user
// for the API documentation.
func (g *GitHub) GetAuthenticatedUser() (*github.User, error) {
	user, resp, err := g.client.Users.Get(g.ctx, "")
	if err != nil {
		return nil, fmt.Errorf("Failed doing users.get: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected status code %d from users.get.", resp.StatusCode)
	}
	return user, nil
}

// See https://developer.github.com/v3/pulls/#get-a-single-pull-request
// for the API documentation.
func (g *GitHub) GetPullRequest(pullRequestNum int) (*github.PullRequest, error) {
	pullRequest, resp, err := g.client.PullRequests.Get(g.ctx, g.RepoOwner, g.RepoName, pullRequestNum)
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
func (g *GitHub) CreatePullRequest(title, baseBranch, headBranch, body string) (*github.PullRequest, error) {
	newPullRequest := &github.NewPullRequest{
		Title: &title,
		Base:  &baseBranch,
		Head:  &headBranch,
		Body:  &body,
	}
	pullRequest, resp, err := g.client.PullRequests.Create(g.ctx, g.RepoOwner, g.RepoName, newPullRequest)
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
func (g *GitHub) MergePullRequest(pullRequestNum int, msg, mergeMethod string) error {
	options := &github.PullRequestOptions{
		MergeMethod: mergeMethod,
	}
	_, resp, err := g.client.PullRequests.Merge(g.ctx, g.RepoOwner, g.RepoName, pullRequestNum, msg, options)
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
	edittedPullRequest, resp, err := g.client.PullRequests.Edit(g.ctx, g.RepoOwner, g.RepoName, pullRequestNum, editPullRequest)
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

func getLabelNames(labels []github.Label) []string {
	labelNames := []string{}
	for _, l := range labels {
		labelNames = append(labelNames, l.GetName())
	}
	return labelNames
}

// See https://developer.github.com/v3/issues/#get-a-single-issue
// for the API documentation.
func (g *GitHub) GetLabels(pullRequestNum int) ([]string, error) {
	pullRequest, resp, err := g.client.Issues.Get(g.ctx, g.RepoOwner, g.RepoName, pullRequestNum)
	if err != nil {
		return nil, fmt.Errorf("Failed doing issues.get: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected status code %d from issues.get.", resp.StatusCode)
	}
	return getLabelNames(pullRequest.Labels), nil
}

// See https://developer.github.com/v3/issues/#edit-an-issue
// for the API documentation.
func (g *GitHub) AddLabel(pullRequestNum int, newLabel string) error {
	// Get all existing labels on the PR.
	labels, err := g.GetLabels(pullRequestNum)
	if err != nil {
		return fmt.Errorf("Error when getting labels for %d: %s", pullRequestNum, err)
	}
	// Add the new labels.
	labels = append(labels, newLabel)

	req := &github.IssueRequest{
		Labels: &labels,
	}
	_, resp, err := g.client.Issues.Edit(g.ctx, g.RepoOwner, g.RepoName, pullRequestNum, req)
	if err != nil {
		return fmt.Errorf("Failed doing issues.edit: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Unexpected status code %d from issues.edit.", resp.StatusCode)
	}
	return nil
}

// See https://developer.github.com/v3/issues/#edit-an-issue
// for the API documentation.
// Note: This adds the newLabel even if the oldLabel is not found.
func (g *GitHub) ReplaceLabel(pullRequestNum int, oldLabel, newLabel string) error {
	// Get all existing labels on the PR.
	existingLabels, err := g.GetLabels(pullRequestNum)
	if err != nil {
		return fmt.Errorf("Error when getting labels for %d: %s", pullRequestNum, err)
	}
	// Remove the specified label.
	newLabels := []string{}
	for _, l := range existingLabels {
		if l != oldLabel {
			newLabels = append(newLabels, l)
		}
	}
	// Add the new label.
	newLabels = append(newLabels, newLabel)

	req := &github.IssueRequest{
		Labels: &newLabels,
	}
	_, resp, err := g.client.Issues.Edit(g.ctx, g.RepoOwner, g.RepoName, pullRequestNum, req)
	if err != nil {
		return fmt.Errorf("Failed doing issues.edit: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Unexpected status code %d from issues.edit.", resp.StatusCode)
	}
	return nil
}
