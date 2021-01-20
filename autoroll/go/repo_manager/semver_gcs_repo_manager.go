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
	"go.skia.org/infra/go/skerr"
)

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

func (c *SemVerGCSRepoManagerConfig) Validate() error {
	// Set some unused variables on the embedded RepoManager.
	childBranch, err := config_vars.NewTemplate("N/A")
	if err != nil {
		return err
	}
	c.ChildBranch = childBranch
	c.ChildPath = "N/A"
	if err := c.NoCheckoutRepoManagerConfig.Validate(); err != nil {
		return err
	}
	// Unset the unused variables.
	c.ChildBranch = nil
	c.ChildPath = ""

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
func (c SemVerGCSRepoManagerConfig) splitParentChild() (parent.GitilesConfig, child.SemVerGCSConfig, error) {
	parentCfg := parent.GitilesConfig{
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
	}
	if err := parentCfg.Validate(); err != nil {
		return parent.GitilesConfig{}, child.SemVerGCSConfig{}, skerr.Wrapf(err, "generated parent config is invalid")
	}
	childCfg := child.SemVerGCSConfig{
		GCSConfig: child.GCSConfig{
			GCSBucket: c.GCSBucket,
			GCSPath:   c.GCSPath,
		},
		ShortRevRegex: c.ShortRevRegex,
		VersionRegex:  c.VersionRegex,
	}
	if err := childCfg.Validate(); err != nil {
		return parent.GitilesConfig{}, child.SemVerGCSConfig{}, skerr.Wrapf(err, "generated child config is invalid")
	}
	return parentCfg, childCfg, nil
}

// NewSemVerGCSRepoManager returns a gcsRepoManager which uses semantic
// versioning to compare object versions.
func NewSemVerGCSRepoManager(ctx context.Context, c *SemVerGCSRepoManagerConfig, reg *config_vars.Registry, workdir string, g gerrit.GerritInterface, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	parentCfg, childCfg, err := c.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	parentRM, err := parent.NewGitilesFile(ctx, parentCfg, reg, client, serverURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	childRM, err := child.NewSemVerGCS(ctx, childCfg, reg, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM, nil)
}
