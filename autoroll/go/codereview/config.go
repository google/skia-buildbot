package codereview

import (
	"errors"
	"fmt"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/util"
)

const (
	// GERRIT_CONFIG_ANDROID is a Gerrit server configuration used by
	// Android and related projects.
	GERRIT_CONFIG_ANDROID = "android"

	// GERIT_CONFIG_CHROMIUM is a Gerrit server configuration used by
	// Chromium and related projects.
	GERRIT_CONFIG_CHROMIUM = "chromium"
)

var (
	// GERRIT_LABELS indicates which labels should be set on Gerrit CLs in
	// normal and dry run modes, for each Gerrit configuration.
	GERRIT_LABELS = map[string]map[bool]map[string]interface{}{
		GERRIT_CONFIG_ANDROID: map[bool]map[string]interface{}{
			// Normal mode.
			false: map[string]interface{}{
				gerrit.CODEREVIEW_LABEL:      "2",
				gerrit.PRESUBMIT_READY_LABEL: "1",
				gerrit.AUTOSUBMIT_LABEL:      gerrit.AUTOSUBMIT_LABEL_SUBMIT,
			},
			// Dry run mode.
			true: map[string]interface{}{
				gerrit.CODEREVIEW_LABEL:      "2",
				gerrit.PRESUBMIT_READY_LABEL: "1",
				gerrit.AUTOSUBMIT_LABEL:      gerrit.AUTOSUBMIT_LABEL_NONE,
			},
		},
		GERRIT_CONFIG_CHROMIUM: map[bool]map[string]interface{}{
			// Normal mode.
			false: map[string]interface{}{
				gerrit.CODEREVIEW_LABEL:  gerrit.CODEREVIEW_LABEL_APPROVE,
				gerrit.COMMITQUEUE_LABEL: gerrit.COMMITQUEUE_LABEL_SUBMIT,
			},
			// Dry run mode.
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

	// Init creates a CodeReview object based on this CodeReviewConfig.
	Init(gerrit.GerritInterface, *github.GitHub) (CodeReview, error)
}

// GerritConfig provides configuration for Gerrit.
type GerritConfig struct {
	// Gerrit host URL.
	URL string `json:"url"`

	// Project name for uploaded CLs.
	Project string `json:"project"`

	// Gerrit instance configuration. This informs the roller which labels
	// to set, among other things. The value should be one of the
	// GERRIT_CONFIG_ constants in this package.
	Config string `json:"config"`
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
func (c *GerritConfig) Init(gerritClient gerrit.GerritInterface, githubClient *github.GitHub) (CodeReview, error) {
	return newGerritCodeReview(c, gerritClient)
}

// GetLabels returns the labels needed for a given CL.
func (c *GerritConfig) GetLabels(dryRun bool) map[string]interface{} {
	return GERRIT_LABELS[c.Config][dryRun]
}

// GithubConfig provides configuration for Github.
type GithubConfig struct {
	RepoOwner      string   `json:"repoOwner,omitempty"`
	RepoName       string   `json:"repoName,omitempty"`
	ChecksNum      int      `json:"checksNum,omitempty"`
	ChecksWaitFor  []string `json:"checksWaitFor,omitempty"`
	MergeMethodURL string   `json:"mergeMethodURL,omitempty"`
}

// See documentation for util.Validator interface.
func (c *GithubConfig) Validate() error {
	if c.RepoOwner == "" {
		return errors.New("RepoOwner is required.")
	}
	if c.RepoName == "" {
		return errors.New("RepoName is required.")
	}
	if c.ChecksNum == 0 {
		return errors.New("At least one check is required.")
	}
	return nil
}

// See documentation for CodeReviewConfig interface.
func (c *GithubConfig) Init(gerritClient gerrit.GerritInterface, githubClient *github.GitHub) (CodeReview, error) {
	return newGithubCodeReview(c, githubClient)
}

// Google3 config is an empty configuration object for Google3.
type Google3Config struct{}

// See documentation for util.Validator interface.
func (c *Google3Config) Validate() error {
	return nil
}

// See documentation for CodeReviewConfig interface.
func (c *Google3Config) Init(gerrit.GerritInterface, *github.GitHub) (CodeReview, error) {
	return nil, errors.New("Init not implemented for Google3Config.")
}
