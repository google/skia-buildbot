package repo_manager

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
)

// NewFreeTypeRepoManager returns a RepoManager instance which rolls FreeType
// in DEPS and updates header files and README.chromium.
func NewFreeTypeRepoManager(ctx context.Context, c *config.FreeTypeRepoManagerConfig, workdir, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	parentRepo := gitiles.NewRepo(c.Parent.Gitiles.Gitiles.RepoUrl, client)
	gc, ok := codereview.GerritConfigs[c.Parent.Gitiles.Gerrit.Config]
	if !ok {
		return nil, skerr.Fmt("Unknown Gerrit config %s", c.Parent.Gitiles.Gerrit.Config)
	}
	g, err := gerrit.NewGerritWithConfig(gc, c.Parent.Gitiles.Gerrit.Url, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	parentRM, err := parent.NewFreeTypeParent(ctx, c.GetParent(), workdir, parentRepo, g, serverURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	childRepo := gitiles.NewRepo(c.Child.Gitiles.RepoUrl, client)
	childRM, err := child.NewGitiles(ctx, c.GetChild(), childRepo)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &parentChildRepoManager{
		Child:  childRM,
		Parent: parentRM,
	}, nil
}
