package parent

import (
	"bytes"
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

type GitilesDEPSConfig struct {
	GitilesConfig
	Dep string `json:"dep"`

	// TransitiveDeps is an optional mapping of dependency ID (eg. repo URL)
	// to the location within the repo where its version is pinned (eg. DEPS).
	// The Child must have a dependencies with the same IDs. When rolling
	// the Child to a new revision, these will be updated to match the
	// versions which are pinned by the Child at the target revision.
	TransitiveDeps map[string]string `json:"transitiveDeps,omitempty"`
}

func (c GitilesDEPSConfig) Validate() error {
	if err := c.GitilesConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.Dep == "" {
		return skerr.Fmt("Dep is required")
	}
	return nil
}

// gitilesDEPSUpdateFunc returns a gitilesUpdateFunc which reads DEPS to find
// the last-rolled revision.
func gitilesDEPSUpdateFunc(dep string) gitilesUpdateFunc {
	return func(ctx context.Context, repo *gitiles.Repo, baseCommit string) (string, error) {
		depsEntries, err := deps_parser.FromGitiles(ctx, repo, baseCommit)
		if err != nil {
			return "", skerr.Wrapf(err, "Failed to retrieve DEPS file")
		}
		entry, ok := depsEntries[dep]
		if !ok {
			return "", skerr.Fmt("Unable to find %q in DEPS!", dep)
		}
		return entry.Version, nil
	}
}

// gitilesDEPSGetChangesForRollFunc returns a gitilesGetChangesForRollFunc which
// update the DEPS file.
func gitilesDEPSGetChangesForRollFunc(dep string, transitiveDeps map[string]string) gitilesGetChangesForRollFunc {
	return func(ctx context.Context, repo *gitiles.Repo, baseCommit string, from, to *revision.Revision, rolling []*revision.Revision) (map[string]string, []*TransitiveDep, error) {
		// Download the DEPS file from the parent repo.
		var buf bytes.Buffer
		if err := repo.ReadFileAtRef(ctx, deps_parser.DepsFileName, baseCommit, &buf); err != nil {
			return nil, nil, skerr.Wrapf(err, "Failed to retrieve DEPS file")
		}

		// Write the new DEPS content.
		depsContent, err := deps_parser.SetDep(buf.String(), dep, to.Id)
		if err != nil {
			return nil, nil, skerr.Wrapf(err, "Failed to set new revision")
		}
		// We'll add the new DEPS content to the changes for the roll
		// after we handle transitive DEPS.
		changes := map[string]string{}

		// Handle transitive DEPS.
		var td []*TransitiveDep
		if len(transitiveDeps) > 0 {
			entries, err := deps_parser.ParseDeps(depsContent)
			if err != nil {
				return nil, nil, skerr.Wrap(err)
			}
			td = make([]*TransitiveDep, 0, len(transitiveDeps))
			for dep, path := range transitiveDeps {
				oldVersion, ok := entries[dep]
				if !ok {
					sklog.Errorf("Could not find transitive dependency %q in %+v", dep, entries)
					continue
				}
				newVersion, ok := to.Dependencies[dep]
				if !ok {
					sklog.Errorf("Could not find transitive dependency %q in %+v", dep, to)
					continue
				}
				if path == deps_parser.DepsFileName {
					depsContent, err = deps_parser.SetDep(depsContent, dep, newVersion)
					if err != nil {
						return nil, nil, skerr.Wrapf(err, "Failed to set new version for transitive dependency %q", dep)
					}
				} else {
					changes[path] = newVersion
				}
				td = append(td, &TransitiveDep{
					ParentPath:  oldVersion.Path,
					RollingFrom: oldVersion.Version,
					RollingTo:   newVersion,
				})
			}
		}

		changes[deps_parser.DepsFileName] = depsContent
		return changes, td, nil
	}
}

// NewGitilesDEPS returns a Parent implementation which uses Gitiles to roll
// DEPS.
func NewGitilesDEPS(ctx context.Context, c GitilesDEPSConfig, reg *config_vars.Registry, client *http.Client, serverURL string) (*gitilesParent, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	update := gitilesDEPSUpdateFunc(c.Dep)
	getChangesForRoll := gitilesDEPSGetChangesForRollFunc(c.Dep, c.TransitiveDeps)
	return newGitiles(ctx, c.GitilesConfig, reg, client, serverURL, update, getChangesForRoll)
}
