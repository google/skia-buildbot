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

package github

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v29/github"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
)

const (
	GITHUB_TOKEN_METADATA_KEY = "github_token"
	GITHUB_TOKEN_FILENAME     = "github_token"
	GITHUB_TOKEN_SERVER_PATH  = "/var/secrets/github-token"
	SSH_KEY_FILENAME          = "id_rsa"
	SSH_KEY_SERVER_PATH       = "/var/secrets/ssh-key"

	MERGE_METHOD_SQUASH = "squash"
	MERGE_METHOD_REBASE = "rebase"

	MERGEABLE_STATE_DIRTY    = "dirty"    // Merge conflict.
	MERGEABLE_STATE_CLEAN    = "clean"    // No conflicts.
	MERGEABLE_STATE_UNKNOWN  = "unknown"  // Mergeablility was not checked yet.
	MERGEABLE_STATE_UNSTABLE = "unstable" // Failing or pending commit status.

	WAITING_FOR_GREEN_TREE_LABEL = "waiting for tree to go green"

	CHECK_STATE_SUCCESS         = "success"
	CHECK_STATE_CANCELLED       = "cancelled"
	CHECK_STATE_FAILURE         = "failure"
	CHECK_STATE_NEUTRAL         = "neutral"
	CHECK_STATE_TIMED_OUT       = "timed_out"
	CHECK_STATE_ACTION_REQUIRED = "action_required"
	CHECK_STATE_ERROR           = "error"
	CHECK_STATE_PENDING         = "pending"

	// Known checks.
	CLA_CHECK             = "cla/google"
	IMPORT_COPYBARA_CHECK = "import/copybara"
)

var (
	OPEN_STATE   = "open"
	CLOSED_STATE = "closed"
)

// Check encapsulates the different Github checks (Cirrus/Travis/etc).
type Check struct {
	ID        int64
	Name      string
	State     string
	StartedAt time.Time
	HTMLURL   string
}

// GitHub is used for iteracting with the GitHub API.
type GitHub struct {
	RepoOwner string
	RepoName  string

	client     *github.Client
	httpClient *http.Client
	ctx        context.Context
}

// NewGitHub returns a new GitHub instance.
func NewGitHub(ctx context.Context, repoOwner, repoName string, httpClient *http.Client) (*GitHub, error) {
	client := github.NewClient(httpClient)
	return &GitHub{
		RepoOwner:  repoOwner,
		RepoName:   repoName,
		client:     client,
		httpClient: httpClient,
		ctx:        ctx,
	}, nil
}

// AddToKnownHosts adds github.com to .ssh/known_hosts. Without this,
// interactions with github do not work.
func AddToKnownHosts(ctx context.Context) {
	// From https://serverfault.com/questions/132970/can-i-automatically-add-a-new-host-to-known-hosts
	// Not throwing error below because github does not provide shell access
	// and thus always returns an error.
	_, err := exec.RunCwd(ctx, "", "ssh", "-T", "git@github.com", "-oStrictHostKeyChecking=no")
	sklog.Info(err)
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

// See https://developer.github.com/v3/pulls/#list-pull-requests
// for the API documentation.
func (g *GitHub) ListOpenPullRequests() ([]*github.PullRequest, error) {
	opts := &github.PullRequestListOptions{
		State: OPEN_STATE,
	}
	pullRequests, resp, err := g.client.PullRequests.List(g.ctx, g.RepoOwner, g.RepoName, opts)
	if err != nil {
		return nil, fmt.Errorf("Failed doing pullrequests.list: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected status code %d from pullrequests.list.", resp.StatusCode)
	}
	return pullRequests, nil
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

// See https://developer.github.com/v3/git/refs/#get-a-reference
// for the API documentation.
func (g *GitHub) GetReference(repoOwner, repoName, ref string) (*github.Reference, error) {
	r, resp, err := g.client.Git.GetRef(g.ctx, repoOwner, repoName, ref)
	if err != nil {
		return nil, fmt.Errorf("Failed getting reference %s : %s", ref, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected status code %d from getting reference.", resp.StatusCode)
	}
	return r, nil
}

// See https://developer.github.com/v3/git/refs/#list-matching-references
// for the API documentation.
func (g *GitHub) ListMatchingReferences(repoOwner, repoName, ref string) ([]*github.Reference, error) {
	references, resp, err := g.client.Git.GetRefs(g.ctx, repoOwner, repoName, ref)
	if err != nil {
		return nil, fmt.Errorf("Failed listing references for %s : %s", ref, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected status code %d from listing references.", resp.StatusCode)
	}
	return references, nil
}

// See https://developer.github.com/v3/git/refs/#delete-a-reference
// for the API documentation.
func (g *GitHub) DeleteReference(repoOwner, repoName, ref string) error {
	resp, err := g.client.Git.DeleteRef(g.ctx, repoOwner, repoName, ref)
	if err != nil {
		return fmt.Errorf("Failed deleting reference %s : %s", ref, err)
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Unexpected status code %d from deleting reference.", resp.StatusCode)
	}
	return nil
}

// See https://developer.github.com/v3/git/refs/#create-a-reference
// for the API documentation.
func (g *GitHub) CreateReference(repoOwner, repoName, ref, sha string) error {
	githubRef := &github.Reference{
		Ref: &ref,
		Object: &github.GitObject{
			SHA: &sha,
		},
	}
	_, resp, err := g.client.Git.CreateRef(g.ctx, repoOwner, repoName, githubRef)
	if err != nil {
		return fmt.Errorf("Failed creating reference %s with SHA %s : %s", ref, sha, err)
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("Unexpected status code %d from creating reference.", resp.StatusCode)
	}
	return nil
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

// See https://developer.github.com/v3/issues/#list-repository-issues
// for the API documentation.
func (g *GitHub) GetIssues(open bool, labels []string, maxResults int) ([]*github.Issue, error) {
	opts := &github.IssueListByRepoOptions{
		Labels: labels,
		ListOptions: github.ListOptions{
			PerPage: maxResults, // Default seems to be 30 per page.
		},
	}
	if open {
		opts.State = OPEN_STATE
	}
	issues, resp, err := g.client.Issues.ListByRepo(g.ctx, g.RepoOwner, g.RepoName, opts)
	if err != nil {
		return nil, fmt.Errorf("Failed doing issues.list: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected status code %d from issues.list.", resp.StatusCode)
	}
	return issues, nil
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
func (g *GitHub) RemoveLabel(pullRequestNum int, oldLabel string) error {
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

// See https://developer.github.com/v3/checks/suites/#list-check-suites-for-a-specific-ref
// and https://developer.github.com/v3/checks/suites/#rerequest-check-suite
// for the API documentation.
func (g *GitHub) ReRequestLatestCheckSuite(ref string) error {
	results, resp, err := g.client.Checks.ListCheckSuitesForRef(g.ctx, g.RepoOwner, g.RepoName, ref, nil)
	if err != nil {
		return fmt.Errorf("Failed doing repos.get: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Unexpected status code %d from repos.get.", resp.StatusCode)
	}
	if *results.Total == 0 {
		sklog.Infof("No check suites found to rerequest for %s", ref)
		return nil
	}
	sklog.Infof("Found %d check suites for %s:", *results.Total, ref)

	targetCheckSuite := results.CheckSuites[*results.Total-1]
	checkSuiteId := *targetCheckSuite.ID
	sklog.Infof("Rerequesting %d with status %d", checkSuiteId, targetCheckSuite.Status)
	reRequestResp, err := g.client.Checks.ReRequestCheckSuite(g.ctx, g.RepoOwner, g.RepoName, checkSuiteId)
	if err != nil {
		return fmt.Errorf("Failed doing repos.get: %s", err)
	}
	if reRequestResp.StatusCode != http.StatusCreated {
		return fmt.Errorf("Unexpected status code %d from repos.get. Expected %d.", reRequestResp.StatusCode, http.StatusCreated)
	}
	return nil
}

// See https://developer.github.com/v3/checks/runs/#list-check-runs-for-a-specific-ref
// and https://developer.github.com/v3/repos/commits/#get-a-single-commit
// for the API documentation.
// Note: This combines checks from both ListCheckRunsForRef and GetCombinedStatus.
//       For flutter/engine on 12/2/19 GetCombinedStatus returned luci-engine and sign-cla.
// TODO(rmistry): Use only Checks API when Flutter is moved completely to it.
func (g *GitHub) GetChecks(ref string) ([]*Check, error) {
	totalChecks := []*Check{}

	// Call Checks API.
	checkRuns, resp, err := g.client.Checks.ListCheckRunsForRef(g.ctx, g.RepoOwner, g.RepoName, ref, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed doing repos.get: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected status code %d from repos.get.", resp.StatusCode)
	}
	for _, cr := range checkRuns.CheckRuns {
		check := &Check{
			ID:        *cr.ID,
			Name:      *cr.Name,
			StartedAt: cr.StartedAt.Time,
		}
		if cr.Conclusion != nil {
			check.State = *cr.Conclusion
		}
		if cr.HTMLURL != nil {
			check.HTMLURL = *cr.HTMLURL
		}
		totalChecks = append(totalChecks, check)
	}

	// Call CombinedStatus API.
	combinedStatus, resp, err := g.client.Repositories.GetCombinedStatus(g.ctx, g.RepoOwner, g.RepoName, ref, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed doing repos.get: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected status code %d from repos.get.", resp.StatusCode)
	}
	for _, st := range combinedStatus.Statuses {
		check := &Check{
			ID:        *st.ID,
			Name:      *st.Context,
			State:     *st.State,
			StartedAt: st.GetCreatedAt(),
		}
		if st.TargetURL != nil {
			check.HTMLURL = *st.TargetURL
		}
		totalChecks = append(totalChecks, check)
	}

	return totalChecks, nil
}

// See https://developer.github.com/v3/issues/#get-a-single-issue
// for the API documentation.
func (g *GitHub) GetDescription(pullRequestNum int) (string, error) {
	issue, resp, err := g.client.Issues.Get(g.ctx, g.RepoOwner, g.RepoName, pullRequestNum)
	if err != nil {
		return "", fmt.Errorf("Failed doing issues.get: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Unexpected status code %d from issues.get.", resp.StatusCode)
	}
	return issue.GetBody(), nil
}

func (g *GitHub) ReadRawFile(branch, filePath string) (string, error) {
	githubContentURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", g.RepoOwner, g.RepoName, branch, filePath)
	resp, err := g.httpClient.Get(githubContentURL)
	if err != nil {
		return "", fmt.Errorf("Error when hitting %s: %s", githubContentURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Unexpected status code %d from %s", resp.StatusCode, githubContentURL)
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Could not read from %s: %s", githubContentURL, err)
	}
	return string(bodyBytes), nil
}

func (g *GitHub) GetFullHistoryUrl(userEmail string) string {
	user := strings.Split(userEmail, "@")[0]
	return fmt.Sprintf("https://github.com/%s/%s/pulls/%s", g.RepoOwner, g.RepoName, user)
}

func (g *GitHub) GetPullRequestUrlBase() string {
	return fmt.Sprintf("https://github.com/%s/%s/pull/", g.RepoOwner, g.RepoName)
}

func (g *GitHub) GetIssueUrlBase() string {
	return fmt.Sprintf("https://github.com/%s/%s/issues/", g.RepoOwner, g.RepoName)
}
