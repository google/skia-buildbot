package parent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

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

// gitilesDEPSGetLastRollRevFunc returns a gitilesGetLastRollRevFunc which reads
// DEPS to find the last-rolled revision.
func gitilesDEPSGetLastRollRevFunc(dep string) gitilesGetLastRollRevFunc {
	return func(ctx context.Context, repo *gitiles_common.GitilesRepo, baseCommit string) (string, error) {
		depsEntries, err := repo.ParseDEPS(ctx, baseCommit)
		if err != nil {
			return "", skerr.Wrapf(err, "Failed to retrieve DEPS file")
		}
		entry := depsEntries.Get(dep)
		if entry == nil {
			b, err := json.MarshalIndent(depsEntries, "", "  ")
			if err == nil {
				return "", skerr.Fmt("Unable to find %q in DEPS! Entries:\n%s", dep, string(b))
			} else {
				return "", skerr.Fmt("Unable to find %q in DEPS! Failed to encode DEPS entries: %s", dep, err)
			}
		}
		return entry.Version, nil
	}
}

// gitilesDEPSGetChangesForRollFunc returns a gitilesGetChangesForRollFunc which
// update the DEPS file.
// TODO(borenet): Make the primary dependency flexible (DEPS vs other) like the
// TransitiveDeps, and fold this into GitilesParent.
func gitilesDEPSGetChangesForRollFunc(dep string, transitiveDeps map[string]string) gitilesGetChangesForRollFunc {
	return func(ctx context.Context, repo *gitiles_common.GitilesRepo, baseCommit string, from, to *revision.Revision, rolling []*revision.Revision) (map[string]string, []*TransitiveDep, error) {
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
				oldVersion := entries.Get(dep)
				if oldVersion == nil {
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
	getLastRollRev := gitilesDEPSGetLastRollRevFunc(c.Dep)
	getChangesForRoll := gitilesDEPSGetChangesForRollFunc(c.Dep, c.TransitiveDeps)
	return newGitiles(ctx, c.GitilesConfig, reg, client, serverURL, getLastRollRev, getChangesForRoll)
}
