package codereview

import (
	"errors"

	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/util"
)

// CodeReviewConfig provides generalized configuration information for a code
// review service.
type CodeReviewConfig interface {
	util.Validator

	// GetIssueUrlBase returns a base URL which can be used to construct
	// URLs for individual issues.
	GetIssueUrlBase() string
}

// GerritConfig provides configuration for Gerrit.
type GerritConfig struct {
	// Gerrit host URL.
	URL string `json:"url"`

	// Project name for uploaded CLs.
	Project string `json:"project"`

	// Labels and values to set for Commit Queue.
	CommitQueueLabels map[string]int `json:"commitQueueLabels"`

	// Labels and values to set for Commit Queue dry runs.
	CommitQueueDryRunLabels map[string]int `json:"commitQueueDryRunLabels"`
}

// See documentation for util.Validator interface.
func (c *GerritConfig) Validate() error {
	if c.URL == "" {
		return errors.New("URL is required.")
	}
	if c.Project == "" {
		return errors.New("Project is required.")
	}
	if len(c.CommitQueueLabels) == 0 {
		return errors.New("CommitQueueLabels is required.")
	}
	if len(c.CommitQueueDryRunLabels) == 0 {
		return errors.New("CommitQueueDryRunLabels is required.")
	}
	return nil
}

// See documentation for CodeReviewConfig interface.
func (c *GerritConfig) GetIssueUrlBase() string {
	return c.URL + "/c/"
}

// GetLabels returns the labels needed for a given CL.
func (c *GerritConfig) GetLabels(dryRun bool) map[string]interface{} {
	labels := c.CommitQueueLabels
	if dryRun {
		labels = c.CommitQueueDryRunLabels
	}
	rv := make(map[string]interface{}, len(labels))
	for k, v := range labels {
		rv[k] = v
	}
	return rv
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
