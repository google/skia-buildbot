package child

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

// GitilesConfig provides configuration for gitilesChild.
type GitilesConfig struct {
	gitiles_common.GitilesConfig
	// Path indicates one path of the repo to watch for changes; all other
	// commits are ignored. Note that this may produce strange results if the
	// git history for the path is not linear.
	Path string `json:"path"`
}

// GitilesConfigToProto converts a GitilesConfig to a
// config.GitilesChildConfig.
func GitilesConfigToProto(cfg *GitilesConfig) *config.GitilesChildConfig {
	return &config.GitilesChildConfig{
		Gitiles: gitiles_common.GitilesConfigToProto(&cfg.GitilesConfig),
		Path:    cfg.Path,
	}
}

// ProtoToGitilesConfig converts a config.GitilesChildConfig to a
// GitilesConfig.
func ProtoToGitilesConfig(cfg *config.GitilesChildConfig) (*GitilesConfig, error) {
	gc, err := gitiles_common.ProtoToGitilesConfig(cfg.Gitiles)
	if err != nil {
		return nil, err
	}
	return &GitilesConfig{
		GitilesConfig: *gc,
		Path:          cfg.Path,
	}, nil
}

// gitilesChild is an implementation of Child which uses Gitiles rather than a
// local checkout.
type gitilesChild struct {
	*gitiles_common.GitilesRepo
	path string
}

// NewGitiles returns an implementation of Child which uses Gitiles rather
// than a local checkout.
func NewGitiles(ctx context.Context, c GitilesConfig, reg *config_vars.Registry, client *http.Client) (Child, error) {
	g, err := gitiles_common.NewGitilesRepo(ctx, c.GitilesConfig, reg, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &gitilesChild{
		GitilesRepo: g,
		path:        c.Path,
	}, nil
}

// See documentation for Child interface.
func (c *gitilesChild) Update(ctx context.Context, lastRollRev *revision.Revision) (*revision.Revision, []*revision.Revision, error) {
	tipRev, err := c.GetTipRevision(ctx)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "Failed to retrieve tip rev")
	}
	var notRolledCommits []*vcsinfo.LongCommit
	if c.path == "" {
		notRolledCommits, err = c.LogFirstParent(ctx, lastRollRev.Id, tipRev.Id)
	} else {
		notRolledCommits, err = c.Log(ctx, git.LogFromTo(lastRollRev.Id, tipRev.Id), gitiles.LogPath(c.path))
	}
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "failed to retrieve not-rolled revisions")
	}
	notRolledRevs, err := c.ConvertRevisions(ctx, notRolledCommits)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "failed to convert not-rolled revisions")
	}
	if c.path != "" && len(notRolledRevs) > 0 && tipRev.Id != notRolledRevs[0].Id {
		sklog.Info("Tip rev %q does not match first not-rolled rev %q; using not-rolled rev. This is expected for rollers which watch a sub-section of a repository.", tipRev.Id, notRolledRevs[0].Id)
		tipRev = notRolledRevs[0]
	}
	return tipRev, notRolledRevs, nil
}

// See documentation for Child interface.
func (c *gitilesChild) Download(ctx context.Context, rev *revision.Revision, dest string) error {
	return git_common.Clone(ctx, c.URL, dest, rev)
}

// gitilesChild implements Child.
var _ Child = &gitilesChild{}
