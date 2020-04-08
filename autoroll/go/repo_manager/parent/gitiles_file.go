package parent

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// GitilesFileConfig provides configuration for a Parent which pins dependencies
// using a file checked into the repo. The revision ID of the dependency makes
// up the full contents of the file, unless the file is "DEPS", which is a
// special case.
type GitilesFileConfig struct {
	GitilesConfig

	// Dep is the ID of the dependency to be rolled, eg. a repo URL.
	Dep string `json:"dep"`

	// Path is the path within the repo to the file which pins the
	// dependency.
	Path string `json:"path"`

	// TransitiveDeps is an optional mapping of dependency ID (eg. repo URL)
	// to the location within the repo where its version is pinned (eg. DEPS).
	// The Child must have a dependencies with the same IDs. When rolling
	// the Child to a new revision, these will be updated to match the
	// versions which are pinned by the Child at the target revision.
	TransitiveDeps map[string]string `json:"transitiveDeps,omitempty"`
}

// See documentation for util.Validator interface.
func (c GitilesFileConfig) Validate() error {
	if err := c.GitilesConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.Dep == "" {
		return skerr.Fmt("Dep is required")
	}
	if c.Path == "" {
		return skerr.Fmt("Path is required")
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
	TransitiveDeps map[string]string `json:"transitiveDeps,omitempty"`
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
func gitilesFileGetLastRollRevFunc(path, dep string) gitilesGetLastRollRevFunc {
	return func(ctx context.Context, repo *gitiles_common.GitilesRepo, baseCommit string) (string, error) {
		contents, err := repo.GetFile(ctx, path, baseCommit)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		return gitiles_common.GetPinnedRev(path, dep, contents)
	}
}

// gitilesFileGetChangesForRollFunc returns a gitilesGetChangesForRollFunc which
// update the given file.
func gitilesFileGetChangesForRollFunc(path, dep string, transitiveDeps map[string]string) gitilesGetChangesForRollFunc {
	return func(ctx context.Context, repo *gitiles_common.GitilesRepo, baseCommit string, from, to *revision.Revision, rolling []*revision.Revision) (map[string]string, []*TransitiveDep, error) {
		// Update the primary dependency.
		changes := make(map[string]string, 1+len(transitiveDeps))
		if _, err := repo.UpdateDep(ctx, baseCommit, path, dep, to.Id, changes); err != nil {
			return nil, nil, skerr.Wrap(err)
		}

		// Handle transitive dependencies.
		var td []*TransitiveDep
		if len(transitiveDeps) > 0 {
			td = make([]*TransitiveDep, 0, len(transitiveDeps))
			for dep, path := range transitiveDeps {
				// Find the new revision.
				newVersion, ok := to.Dependencies[dep]
				if !ok {
					sklog.Errorf("Could not find transitive dependency %q in %+v", dep, to)
					continue
				}
				// Update.
				oldVersion, err := repo.UpdateDep(ctx, baseCommit, path, dep, newVersion, changes)
				if err != nil {
					return nil, nil, skerr.Wrap(err)
				}
				// Add the transitive dep to the list.
				td = append(td, &TransitiveDep{
					ParentPath:  dep,
					RollingFrom: oldVersion,
					RollingTo:   newVersion,
				})
			}
		}

		return changes, td, nil
	}
}

// NewGitilesFile returns a Parent implementation which uses Gitiles to roll
// a dependency.
func NewGitilesFile(ctx context.Context, c GitilesFileConfig, reg *config_vars.Registry, client *http.Client, serverURL string) (*gitilesParent, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	getLastRollRev := gitilesFileGetLastRollRevFunc(c.Path, c.Dep)
	getChangesForRoll := gitilesFileGetChangesForRollFunc(c.Path, c.Dep, c.TransitiveDeps)
	return newGitiles(ctx, c.GitilesConfig, reg, client, serverURL, getLastRollRev, getChangesForRoll)
}

// NewGitilesDEPS returns a Parent implementation which uses Gitiles to roll
// DEPS.
func NewGitilesDEPS(ctx context.Context, c GitilesDEPSConfig, reg *config_vars.Registry, client *http.Client, serverURL string) (*gitilesParent, error) {
	return NewGitilesFile(ctx, GitilesFileConfig{
		GitilesConfig:  c.GitilesConfig,
		Dep:            c.Dep,
		Path:           deps_parser.DepsFileName,
		TransitiveDeps: c.TransitiveDeps,
	}, reg, client, serverURL)
}
