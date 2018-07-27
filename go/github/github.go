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
	"io/ioutil"
	"net/http"
	"strings"

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

	CHECK_STATE_PENDING = "pending"
	CHECK_STATE_SUCCESS = "success"
	CHECK_STATE_ERROR   = "error"
	CHECK_STATE_FAILURE = "failure"
)

var (
	CLOSED_STATE = "closed"
)

// GitHub is used for iteracting with the GitHub API.
type GitHub struct {
	RepoOwner string
	RepoName  string
	TravisCi  *travisci.TravisCI

	client     *github.Client
	httpClient *http.Client
	ctx        context.Context
}

// NewGitHub returns a new GitHub instance.
func NewGitHub(ctx context.Context, repoOwner, repoName string, httpClient *http.Client, travisAccessToken string) (*GitHub, error) {
	travisCi, err := travisci.NewTravisCI(ctx, repoOwner, repoName, travisAccessToken)
	if err != nil {
		return nil, err
	}

	client := github.NewClient(httpClient)
	return &GitHub{
		RepoOwner:  repoOwner,
		RepoName:   repoName,
		TravisCi:   travisCi,
		client:     client,
		httpClient: httpClient,
		ctx:        ctx,
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

func (g *GitHub) GetIssueUrlBase() string {
	return fmt.Sprintf("https://github.com/%s/%s/pull/", g.RepoOwner, g.RepoName)
}

// CREATE A TEST AND GET THIS WORKING BEFORE YOU DO ANYTHING ELSE!!!!!!

// See https://developer.github.com/v3/checks/runs/#list-check-runs-for-a-specific-ref
// for the API documentation.
func (g *GitHub) GetChecks(ref string) ([]github.RepoStatus, error) {
	// https://api.github.com/repos/flutter/flutter/commits/f5b5ac1c8115dfca50c4ca143f288383f569e623/statuses
	// target_url is unique I think, will have to dedupall this
	combinedStatus, resp, err := g.client.Repositories.GetCombinedStatus(g.ctx, g.RepoOwner, g.RepoName, ref, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed doing repos.get: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected status code %d from repos.get.", resp.StatusCode)
	}

	/*
			tryResults := []*autoroll.TryResult{}
		for _, travisBuild := range travisBuilds {
			if travisBuild.Id != 0 {
				testStatus := autoroll.TRYBOT_STATUS_STARTED
				testResult := ""
				switch travisBuild.State {
				case travisci.BUILD_STATE_CREATED:
					// Build is not completely ready yet. It will not have a
					// startedAt yet.
					continue
				case travisci.BUILD_STATE_FAILED:
					testStatus = autoroll.TRYBOT_STATUS_COMPLETED
					testResult = autoroll.TRYBOT_RESULT_FAILURE
				case travisci.BUILD_STATE_PASSED:
					testStatus = autoroll.TRYBOT_STATUS_COMPLETED
					testResult = autoroll.TRYBOT_RESULT_SUCCESS
				}
				buildStartedAt, err := time.Parse(time.RFC3339, travisBuild.StartedAt)
				if err != nil {
					return nil, nil, fmt.Errorf("Failed to parse %s: %s", travisBuild.StartedAt, err)
				}
				tryResults = append(tryResults,
					&autoroll.TryResult{
						Builder:  fmt.Sprintf("TravisCI Build #%d", travisBuild.Id),
						Category: autoroll.TRYBOT_CATEGORY_CQ,
						Created:  buildStartedAt,
						Result:   testResult,
						Status:   testStatus,
						Url:      t.GetBuildURL(travisBuild.Id),
					})
			}
		}
	*/

	fmt.Println("-----------------------------")
	fmt.Println(combinedStatus.GetTotalCount())
	fmt.Println(combinedStatus.Statuses)
	return combinedStatus.Statuses, nil
	//for _, check := range combinedStatus.Statuses {
	//	check.ID
	//	check.State
	//	check.GetCreatedAt()
	//	if check.ID != 0 {
	//		testStatus := autoroll.TRYBOT_STATUS_STARTED
	//		testResult := ""
	//		switch check.State {
	//		case CHECK_STATE_PENDING:
	//			// Build is till pending. Should still have a created?
	//			fmt.Println("PENDING PENDING")
	//			fmt.Println("Not exactly sure what to do here.. still add it if everything is there?")
	//			fmt.Println(check.GetCreatedAt())
	//			continue
	//		case CHECK_STATE_FAILURE:
	//			testStatus = autoroll.TRYBOT_STATUS_COMPLETED
	//			testResult = autoroll.TRYBOT_RESULT_FAILURE
	//		case CHECK_STATE_ERROR:
	//			testStatus = autoroll.TRYBOT_STATUS_COMPLETED
	//			testResult = autoroll.TRYBOT_RESULT_FAILURE
	//		case CHECK_STATE_SUCCESS:
	//			testStatus = autoroll.TRYBOT_STATUS_COMPLETED
	//			testResult = autoroll.TRYBOT_RESULT_SUCCESS
	//		}
	//		fmt.Println("Createdat is same as build started at ????")
	//		fmt.Println(check.GetCreatedAt())
	//		buildStartedAt := check.GetCreatedAt()
	//		// Need two things here!
	//		// Builder name
	//		// Builder URL
	//		tryResults = append(tryResults,
	//			&autoroll.TryResult{
	//				Builder:  fmt.Sprintf("%s #%d", check.Context, check.ID),
	//				Category: autoroll.TRYBOT_CATEGORY_CQ,
	//				Created:  buildStartedAt,
	//				Result:   testResult,
	//				Status:   testStatus,
	//				Url:      check.TargetURL,
	//			})
	//	}
	//}
}

/*
[github.RepoStatus{ID:5267670598, URL:"https://api.github.com/repos/flutter/flutter/statuses/f5b5ac1c8115dfca50c4ca143f288383f569e623",
State:"success",
TargetURL:"https://github.com/apps/wip",
Description:"ready for review",
Context:"WIP",
CreatedAt:time.Time{wall:, ext:},
UpdatedAt:time.Time{wall:, ext:}}

github.RepoStatus{ID:5267670835, URL:"https://api.github.com/repos/flutter/flutter/statuses/f5b5ac1c8115dfca50c4ca143f288383f569e623", State:"success", Description:"All necessary CLAs are signed", Context:"cla/google", CreatedAt:time.Time{wall:, ext:}, UpdatedAt:time.Time{wall:, ext:}}


github.RepoStatus{ID:5267673892, URL:"https://api.github.com/repos/flutter/flutter/statuses/f5b5ac1c8115dfca50c4ca143f288383f569e623",
State:"success",
Context:"flutter-build",
CreatedAt:time.Time{wall:, ext:},
UpdatedAt:time.Time{wall:, ext:}} github.RepoStatus{ID:5267678909,
URL:"https://api.github.com/repos/flutter/flutter/statuses/f5b5ac1c8115dfca50c4ca143f288383f569e623", State:"failure", TargetURL:"https://cirrus-ci.com/task/5725433001672704", Context:"docs", CreatedAt:time.Time{wall:, ext:}, UpdatedAt:time.Time{wall:, ext:}} github.RepoStatus{ID:5267681214, URL:"https://api.github.com/repos/flutter/flutter/statuses/f5b5ac1c8115dfca50c4ca143f288383f569e623", State:"failure", TargetURL:"https://cirrus-ci.com/task/5162483048251392", Context:"analyze", CreatedAt:time.Time{wall:, ext:}, UpdatedAt:time.Time{wall:, ext:}} github.RepoStatus{ID:5267681227, URL:"https://api.github.com/repos/flutter/flutter/statuses/f5b5ac1c8115dfca50c4ca143f288383f569e623", State:"failure", TargetURL:"https://cirrus-ci.com/task/4881008071540736", Context:"tool_tests-linux", CreatedAt:time.Time{wall:, ext:}, UpdatedAt:time.Time{wall:, ext:}} github.RepoStatus{ID:5267681253, URL:"https://api.github.com/repos/flutter/flutter/statuses/f5b5ac1c8115dfca50c4ca143f288383f569e623", State:"pending", TargetURL:"https://cirrus-ci.com/task/6006907978383360", Context:"tests-windows", CreatedAt:time.Time{wall:, ext:}, UpdatedAt:time.Time{wall:, ext:}} github.RepoStatus{ID:5267681276, URL:"https://api.github.com/repos/flutter/flutter/statuses/f5b5ac1c8115dfca50c4ca143f288383f569e623", State:"pending", TargetURL:"https://cirrus-ci.com/task/5443958024962048", Context:"tool_tests-windows", CreatedAt:time.Time{wall:, ext:}, UpdatedAt:time.Time{wall:, ext:}} github.RepoStatus{ID:5267690974, URL:"https://api.github.com/repos/flutter/flutter/statuses/f5b5ac1c8115dfca50c4ca143f288383f569e623", State:"failure", TargetURL:"https://cirrus-ci.com/task/4740270583185408", Context:"tool_tests-macos", CreatedAt:time.Time{wall:, ext:}, UpdatedAt:time.Time{wall:, ext:}} github.RepoStatus{ID:5267694814, URL:"https://api.github.com/repos/flutter/flutter/statuses/f5b5ac1c8115dfca50c4ca143f288383f569e623", State:"failure", TargetURL:"https://cirrus-ci.com/task/6288382955094016", Context:"tests-linux", CreatedAt:time.Time{wall:, ext:}, UpdatedAt:time.Time{wall:, ext:}} github.RepoStatus{ID:5267695567, URL:"https://api.github.com/repos/flutter/flutter/statuses/f5b5ac1c8115dfca50c4ca143f288383f569e623", State:"error", TargetURL:"https://travis-ci.org/flutter/flutter/builds/408635945?utm_source=github_status&utm_medium=notification", Description:"The Travis CI build could not complete due to an error", Context:"continuous-integration/travis-ci/pr", CreatedAt:time.Time{wall:, ext:}, UpdatedAt:time.Time{wall:, ext:}} github.RepoStatus{ID:5267699873, URL:"https://api.github.com/repos/flutter/flutter/statuses/f5b5ac1c8115dfca50c4ca143f288383f569e623", State:"failure", TargetURL:"https://cirrus-ci.com/task/6569857931804672", Context:"tests-macos", CreatedAt:time.Time{wall:, ext:}, UpdatedAt:time.Time{wall:, ext:}}]
*/
