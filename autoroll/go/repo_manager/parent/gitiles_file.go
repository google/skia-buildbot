package parent

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/proto"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
)

// gitilesFileGetChangesForRollFunc returns a gitilesGetChangesForRollFunc which
// update the given file.
func gitilesFileGetChangesForRollFunc(dep *proto.DependencyConfig) gitilesGetChangesForRollFunc {
	return func(ctx context.Context, repo *gitiles_common.GitilesRepo, baseCommit string, from, to *revision.Revision, rolling []*revision.Revision) (map[string]string, error) {
		getFile := func(ctx context.Context, path string) (string, error) {
			return repo.GetFile(ctx, path, baseCommit)
		}
		return version_file_common.UpdateDep(ctx, dep, to, getFile)
	}
}

// NewGitilesFile returns a Parent implementation which uses Gitiles to roll
// a dependency.
func NewGitilesFile(ctx context.Context, c *proto.GitilesParentConfig, reg *config_vars.Registry, client *http.Client, serverURL string) (*gitilesParent, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	getChangesForRoll := gitilesFileGetChangesForRollFunc(c.Dep)
	return newGitiles(ctx, c, reg, client, serverURL, getChangesForRoll)
}
