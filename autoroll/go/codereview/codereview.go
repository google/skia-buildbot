package codereview

import (
	"context"
	"errors"
	"strings"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/recent_rolls"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
)

// CodeReview outlines the autoroller's interaction with a code review system.
type CodeReview interface {
	// GetIssueUrlBase returns a base URL which can be used to construct
	// URLs for individual issues.
	GetIssueUrlBase() string

	// GetFullHistoryUrl returns a url that contains all changes uploaded by
	// the user.
	GetFullHistoryUrl() string

	// RetrieveRoll retrieves a RollImpl corresponding to the given issue.
	// The passed-in AutoRollIssue becomes owned by the RollImpl; it may
	// modify it, insert it into the RecentRolls DB, etc.
	// TODO(borenet): Consider storing the rollingTo Revision as part of the
	// autoroll.AutoRollIssue struct, to avoid passing it around.
	RetrieveRoll(context.Context, *autoroll.AutoRollIssue, *recent_rolls.RecentRolls, *revision.Revision, func(context.Context, RollImpl) error) (RollImpl, error)

	// UserEmail returns the email address of the authenticated user.
	UserEmail() string

	// UserName returns the name of the authenticated user.
	UserName() string

	// Client returns the API client used to interact with this CodeReview.
	Client() interface{}
}

// gerritCodeReview is a CodeReview backed by Gerrit.
type gerritCodeReview struct {
	cfg            *config.GerritConfig
	fullHistoryUrl string
	gerritClient   gerrit.GerritInterface
	issueUrlBase   string
	userEmail      string
	userName       string
}

// NewGerrit returns a gerritCodeReview instance.
func NewGerrit(cfg *config.GerritConfig, gerritClient gerrit.GerritInterface) (CodeReview, error) {
	userEmail, err := gerritClient.GetUserEmail(context.TODO())
	if err != nil {
		return nil, err
	}
	userName := strings.SplitN(userEmail, "@", 2)[0]
	return &gerritCodeReview{
		cfg:            cfg,
		fullHistoryUrl: cfg.Url + "/q/owner:" + userEmail,
		gerritClient:   gerritClient,
		issueUrlBase:   cfg.Url + "/c/",
		userEmail:      userEmail,
		userName:       userName,
	}, nil
}

// GetIssueUrlBase implements CodeReview.
func (c *gerritCodeReview) GetIssueUrlBase() string {
	return c.issueUrlBase
}

// GetFullHistoryUrl implements CodeReview.
func (c *gerritCodeReview) GetFullHistoryUrl() string {
	return c.fullHistoryUrl
}

// RetrieveRoll implements CodeReview.
func (c *gerritCodeReview) RetrieveRoll(ctx context.Context, issue *autoroll.AutoRollIssue, recent *recent_rolls.RecentRolls, rollingTo *revision.Revision, finishedCallback func(context.Context, RollImpl) error) (RollImpl, error) {
	return newGerritRoll(ctx, c.cfg, issue, c.gerritClient, recent, c.issueUrlBase, rollingTo, finishedCallback)
}

// UserEmail implements CodeReview.
func (c *gerritCodeReview) UserEmail() string {
	return c.userEmail
}

// UserName implements CodeReview.
func (c *gerritCodeReview) UserName() string {
	return c.userName
}

// Client implements CodeReview.
func (c *gerritCodeReview) Client() interface{} {
	return c.gerritClient
}

// githubCodeReview is a CodeReview backed by Github.
type githubCodeReview struct {
	cfg            *config.GitHubConfig
	fullHistoryUrl string
	githubClient   *github.GitHub
	issueUrlBase   string
	userEmail      string
	userName       string
}

// NewGitHub returns a githubCodeReview instance.
func NewGitHub(cfg *config.GitHubConfig, githubClient *github.GitHub) (CodeReview, error) {
	user, err := githubClient.GetAuthenticatedUser()
	if err != nil {
		return nil, err
	}
	userEmail := user.GetEmail()
	if userEmail == "" {
		return nil, errors.New("Found no email address for github user.")
	}
	userName := user.GetLogin()
	if userName == "" {
		return nil, errors.New("Found no login for github user.")
	}
	return &githubCodeReview{
		cfg:            cfg,
		issueUrlBase:   githubClient.GetPullRequestUrlBase(),
		fullHistoryUrl: githubClient.GetFullHistoryUrl(userName),
		githubClient:   githubClient,
		userEmail:      userEmail,
		userName:       userName,
	}, nil
}

// GetIssueUrlBase implements CodeReview.
func (c *githubCodeReview) GetIssueUrlBase() string {
	return c.issueUrlBase
}

// GetFullHistoryUrl implements CodeReview.
func (c *githubCodeReview) GetFullHistoryUrl() string {
	return c.fullHistoryUrl
}

// RetrieveRoll implements CodeReview.
func (c *githubCodeReview) RetrieveRoll(ctx context.Context, issue *autoroll.AutoRollIssue, recent *recent_rolls.RecentRolls, rollingTo *revision.Revision, finishedCallback func(context.Context, RollImpl) error) (RollImpl, error) {
	return newGithubRoll(ctx, issue, c.githubClient, recent, c.issueUrlBase, c.cfg, rollingTo, finishedCallback)
}

// UserEmail implements CodeReview.
func (c *githubCodeReview) UserEmail() string {
	return c.userEmail
}

// UserName implements CodeReview.
func (c *githubCodeReview) UserName() string {
	return c.userName
}

// Client implements CodeReview.
func (c *githubCodeReview) Client() interface{} {
	return c.githubClient
}
