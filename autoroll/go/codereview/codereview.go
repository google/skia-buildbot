package codereview

import (
	"context"
	"errors"
	"fmt"

	"go.skia.org/infra/autoroll/go/recent_rolls"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/util"
)

const (
	GERRIT_CONFIG_ANDROID  = "android"
	GERRIT_CONFIG_CHROMIUM = "chromium"
)

var (
	GERRIT_LABELS = map[string]map[bool]map[string]interface{}{
		GERRIT_CONFIG_ANDROID: map[bool]map[string]interface{}{
			false: map[string]interface{}{
				gerrit.CODEREVIEW_LABEL:      gerrit.CODEREVIEW_LABEL_APPROVE,
				gerrit.PRESUBMIT_READY_LABEL: 1,
				gerrit.AUTOSUBMIT_LABEL:      gerrit.AUTOSUBMIT_LABEL_SUBMIT,
			},
			true: map[string]interface{}{
				gerrit.CODEREVIEW_LABEL:      gerrit.CODEREVIEW_LABEL_APPROVE,
				gerrit.PRESUBMIT_READY_LABEL: 1,
				gerrit.AUTOSUBMIT_LABEL:      gerrit.AUTOSUBMIT_LABEL_NONE,
			},
		},
		GERRIT_CONFIG_CHROMIUM: map[bool]map[string]interface{}{
			false: map[string]interface{}{
				gerrit.CODEREVIEW_LABEL:  gerrit.CODEREVIEW_LABEL_APPROVE,
				gerrit.COMMITQUEUE_LABEL: gerrit.COMMITQUEUE_LABEL_SUBMIT,
			},
			true: map[string]interface{}{
				gerrit.CODEREVIEW_LABEL:  gerrit.CODEREVIEW_LABEL_APPROVE,
				gerrit.COMMITQUEUE_LABEL: gerrit.COMMITQUEUE_LABEL_DRY_RUN,
			},
		},
	}
)

// CodeReviewConfig provides generalized configuration information for a code
// review service.
type CodeReviewConfig interface {
	util.Validator

	// GetIssueUrlBase returns a base URL which can be used to construct
	// URLs for individual issues.
	GetIssueUrlBase() string

	// RetrieveRoll retrieves a RollImpl corresponding to the given issue.
	RetrieveRoll(context.Context, gerrit.GerritInterface, *github.GitHub, autoroll.FullHashFn, *recent_rolls.RecentRolls, int64, func(context.Context, RollImpl) error) (RollImpl, error)
}

// GerritConfig provides configuration for Gerrit.
type GerritConfig struct {
	// Gerrit host URL.
	URL string `json:"url"`

	// Project name for uploaded CLs.
	Project string `json:"project"`

	// Gerrit instance configuration.
	Config string `json:"labelConfig"`
}

// See documentation for util.Validator interface.
func (c *GerritConfig) Validate() error {
	if c.URL == "" {
		return errors.New("URL is required.")
	}
	if c.Project == "" {
		return errors.New("Project is required.")
	}
	if c.Config != GERRIT_CONFIG_ANDROID && c.Config != GERRIT_CONFIG_CHROMIUM {
		return fmt.Errorf("Config must be one of: [%s, %s]", GERRIT_CONFIG_ANDROID, GERRIT_CONFIG_CHROMIUM)
	}
	return nil
}

// See documentation for CodeReviewConfig interface.
func (c *GerritConfig) GetIssueUrlBase() string {
	return c.URL + "/c/"
}

// See documentation for CodeReviewConfig interface.
func (c *GerritConfig) RetrieveRoll(ctx context.Context, gerritClient gerrit.GerritInterface, githubClient *github.GitHub, fullHashFn autoroll.FullHashFn, recent *recent_rolls.RecentRolls, issue int64, cb func(context.Context, RollImpl) error) (RollImpl, error) {
	if c.Config == GERRIT_CONFIG_ANDROID {
		return NewGerritAndroidRoll(ctx, gerritClient, fullHashFn, recent, issue, c, cb)
	}
	return NewGerritRoll(ctx, gerritClient, fullHashFn, recent, issue, c, cb)
}

// GetLabels returns the labels needed for a given CL.
func (c *GerritConfig) GetLabels(dryRun bool) map[string]interface{} {
	return GERRIT_LABELS[c.Config][dryRun]
}

// GithubConfig provides configuration for Github.
type GithubConfig struct {
	RepoOwner      string   `json:"repoOwner,omitempty"`
	RepoName       string   `json:"repoName,omitempty"`
	ChecksNum      int      `json:"checksNumb,omitempty"`
	ChecksWaitFor  []string `json:"checksWaitFor,omitempty"`
	MergeMethodURL string   `json:"mergeMethodURL,omitempty"`
}

// See documentation for util.Validator interface.
func (c *GithubConfig) Validate() error {
	// TODO(borenet): Which fields are required?
	if c.RepoOwner == "" {
		return errors.New("RepoOwner is required.")
	}
	if c.RepoName == "" {
		return errors.New("RepoName is required.")
	}
	return nil
}

// See documentation for CodeReviewConfig interface.
func (c *GithubConfig) GetIssueUrlBase() string {
	return (&github.GitHub{
		RepoOwner: c.RepoOwner,
		RepoName:  c.RepoName,
	}).GetIssueUrlBase()
}

// See documentation for CodeReviewConfig interface.
func (c *GithubConfig) RetrieveRoll(ctx context.Context, gerritClient gerrit.GerritInterface, githubClient *github.GitHub, fullHashFn autoroll.FullHashFn, recent *recent_rolls.RecentRolls, issue int64, cb func(context.Context, RollImpl) error) (RollImpl, error) {
	return NewGithubRoll(ctx, githubClient, fullHashFn, recent, issue, c, cb)
}

// Google3 config is an empty configuration object for Google3.
type Google3Config struct{}

// See documentation for util.Validator interface.
func (c *Google3Config) Validate() error {
	return nil
}

// See documentation for CodeReviewConfig interface.
func (c *Google3Config) GetIssueUrlBase() string {
	return ""
}

// See documentation for CodeReviewConfig interface.
func (c *Google3Config) RetrieveRoll(ctx context.Context, gerritClient gerrit.GerritInterface, githubClient *github.GitHub, fullHashFn autoroll.FullHashFn, recent *recent_rolls.RecentRolls, issue int64, cb func(context.Context, RollImpl) error) (RollImpl, error) {
	return nil, errors.New("RetrieveRoll not implemented for Google3Config.")
}
