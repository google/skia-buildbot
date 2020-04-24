package parent

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/skerr"
)

// GitilesFileConfig provides configuration for a Parent which pins dependencies
// using a file checked into the repo. The revision ID of the dependency makes
// up the full contents of the file, unless the file is "DEPS", which is a
// special case.
type GitilesFileConfig struct {
	GitilesConfig
	version_file_common.DependencyConfig
}

// See documentation for util.Validator interface.
func (c GitilesFileConfig) Validate() error {
	if err := c.GitilesConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if err := c.DependencyConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// GitilesDEPSConfig provides configuration for a Parent which pins dependencies
// using DEPS.
type GitilesDEPSConfig struct {
	GitilesConfig

	// Dep is the ID of the dependency to be rolled, eg. a repo URL.
	Dep string `json:"dep"`

	// TransitiveDeps is an optional mapping of dependency ID (eg. repo URL)
	// to the location within the repo where its version is pinned (eg. DEPS).
	// The Child must have a dependencies with the same IDs. When rolling
	// the Child to a new revision, these will be updated to match the
	// versions which are pinned by the Child at the target revision.
	TransitiveDeps []*version_file_common.VersionFileConfig `json:"transitiveDeps,omitempty"`
}

// See documentation for util.Validator interface.
func (c GitilesDEPSConfig) Validate() error {
	if err := c.GitilesConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.Dep == "" {
		return skerr.Fmt("Dep is required")
	}
	return nil
}

// gitilesFileGetLastRollRevFunc returns a gitilesGetLastRollRevFunc which reads
// the given file to find the last-rolled revision.
func gitilesFileGetLastRollRevFunc(dep version_file_common.VersionFileConfig) gitilesGetLastRollRevFunc {
	return func(ctx context.Context, repo *gitiles_common.GitilesRepo, baseCommit string) (string, error) {
		contents, err := repo.GetFile(ctx, dep.Path, baseCommit)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		return version_file_common.GetPinnedRev(dep, contents)
	}
}

// gitilesFileGetChangesForRollFunc returns a gitilesGetChangesForRollFunc which
// update the given file.
func gitilesFileGetChangesForRollFunc(dep version_file_common.DependencyConfig) gitilesGetChangesForRollFunc {
	return func(ctx context.Context, repo *gitiles_common.GitilesRepo, baseCommit string, from, to *revision.Revision, rolling []*revision.Revision) (map[string]string, []*version_file_common.TransitiveDepUpdate, error) {
		getFile := func(ctx context.Context, path string) (string, error) {
			return repo.GetFile(ctx, path, baseCommit)
		}
		return version_file_common.UpdateDep(ctx, dep, to, getFile)
	}
}

// NewGitilesFile returns a Parent implementation which uses Gitiles to roll
// a dependency.
func NewGitilesFile(ctx context.Context, c GitilesFileConfig, reg *config_vars.Registry, client *http.Client, serverURL string) (*gitilesParent, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	getLastRollRev := gitilesFileGetLastRollRevFunc(c.VersionFileConfig)
	getChangesForRoll := gitilesFileGetChangesForRollFunc(c.DependencyConfig)
	return newGitiles(ctx, c.GitilesConfig, reg, client, serverURL, getLastRollRev, getChangesForRoll)
}

// NewGitilesDEPS returns a Parent implementation which uses Gitiles to roll
// DEPS.
func NewGitilesDEPS(ctx context.Context, c GitilesDEPSConfig, reg *config_vars.Registry, client *http.Client, serverURL string) (*gitilesParent, error) {
	return NewGitilesFile(ctx, GitilesFileConfig{
		GitilesConfig: c.GitilesConfig,
		DependencyConfig: version_file_common.DependencyConfig{
			VersionFileConfig: version_file_common.VersionFileConfig{
				ID:   c.Dep,
				Path: deps_parser.DepsFileName,
			},
			TransitiveDeps: c.TransitiveDeps,
		},
	}, reg, client, serverURL)
}
