package repo_manager

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/proto"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/skerr"
)

// NewFreeTypeRepoManager returns a RepoManager instance which rolls FreeType
// in DEPS and updates header files and README.chromium.
func NewFreeTypeRepoManager(ctx context.Context, c *proto.FreeTypeRepoManagerConfig, reg *config_vars.Registry, workdir, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	parentRM, err := parent.NewFreeTypeParent(ctx, c.GetParent(), reg, workdir, client, serverURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	childRM, err := child.NewGitiles(ctx, c.GetChild(), reg, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &parentChildRepoManager{
		Child:  childRM,
		Parent: parentRM,
	}, nil
}
