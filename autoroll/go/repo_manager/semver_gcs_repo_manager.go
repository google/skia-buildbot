package repo_manager

import (
	"context"
	"errors"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
)

// SemVerGCSRepoManagerConfig provides configuration for a RepoManager which
// rolls a file in GCS which is versioned using Semantic Versioning.
type SemVerGCSRepoManagerConfig struct {
	NoCheckoutRepoManagerConfig
	Gerrit *codereview.GerritConfig `json:"gerrit,omitempty"`

	// GCS bucket used for finding child revisions.
	GCSBucket string `json:"gcsBucket"`

	// Path within the GCS bucket which contains child revisions.
	GCSPath string `json:"gcsPath"`

	// File to update in the parent repo.
	VersionFile string `json:"versionFile"`

	// ShortRevRegex is a regular expression string which indicates
	// what part of the revision ID string should be used as the shortened
	// ID for display. If not specified, the full ID string is used.
	ShortRevRegex *config_vars.Template `json:"shortRevRegex,omitempty"`

	// VersionRegex is a regular expression string containing one or more
	// integer capture groups. The integers matched by the capture groups
	// are compared, in order, when comparing two revisions.
	VersionRegex *config_vars.Template `json:"versionRegex"`
}

// Validate implements the util.Validator interface.
func (c *SemVerGCSRepoManagerConfig) Validate() error {
	if err := c.NoCheckoutRepoManagerConfig.Validate(); err != nil {
		return err
	}
	if c.VersionRegex == nil {
		return errors.New("VersionRegex is required.")
	}
	if err := c.VersionRegex.Validate(); err != nil {
		return err
	}
	if c.ShortRevRegex != nil {
		if err := c.ShortRevRegex.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// splitParentChild splits the SemVerGCSRepoManagerConfig into a
// parent.GitilesConfig and a child.SemVerGCSConfig.
// TODO(borenet): Update the config format to directly define the parent
// and child. We shouldn't need most of the New.*RepoManager functions.
func (c SemVerGCSRepoManagerConfig) splitParentChild() (*ParentChildConfig, error) {
	cfg := &ParentChildConfig{
		Parent: parent.Config{
			GitilesFile: &parent.GitilesConfig{
				GitilesConfig: gitiles_common.GitilesConfig{
					Branch:  c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ParentBranch,
					RepoURL: c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ParentRepo,
				},
				Gerrit: c.Gerrit,
				DependencyConfig: version_file_common.DependencyConfig{
					VersionFileConfig: version_file_common.VersionFileConfig{
						ID:   c.GCSPath, // TODO
						Path: c.VersionFile,
					},
				},
			},
		},
		Child: child.Config{
			SemVerGCS: &child.SemVerGCSConfig{
				GCSConfig: child.GCSConfig{
					GCSBucket: c.GCSBucket,
					GCSPath:   c.GCSPath,
				},
				ShortRevRegex: c.ShortRevRegex,
				VersionRegex:  c.VersionRegex,
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		return nil, skerr.Wrapf(err, "generated config is invalid")
	}
	return cfg, nil
}

// NewSemVerGCSRepoManager returns a gcsRepoManager which uses semantic
// versioning to compare object versions.
func NewSemVerGCSRepoManager(ctx context.Context, c *SemVerGCSRepoManagerConfig, reg *config_vars.Registry, workdir, rollerName string, gerritClient *gerrit.Gerrit, githubClient *github.GitHub, recipeCfgFile, serverURL string, httpClient *http.Client, cr codereview.CodeReview, local bool) (*ParentChildRepoManager, error) {
	cfg, err := c.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildFromConfig(ctx, cfg, reg, httpClient, gerritClient, githubClient, serverURL, workdir, rollerName, cr.UserName(), cr.UserEmail(), recipeCfgFile)
}
