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

// NewGitilesDEPS returns a Parent implementation which uses Gitiles to roll
// DEPS.
func NewGitilesDEPS(ctx context.Context, c GitilesDEPSConfig, reg *config_vars.Registry, client *http.Client, serverURL string) (Parent, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	update := func(ctx context.Context, repo *gitiles.Repo, baseCommit string) (string, error) {
		depsEntries, err := deps_parser.FromGitiles(ctx, repo, baseCommit)
		if err != nil {
			return "", skerr.Wrapf(err, "Failed to retrieve DEPS file")
		}
		entry, ok := depsEntries[c.Dep]
		if !ok {
			return "", skerr.Fmt("Unable to find %q in DEPS!", c.Dep)
		}
		return entry.Version, nil
	}

	getChangesForRoll := func(ctx context.Context, repo *gitiles.Repo, baseCommit string, from, to *revision.Revision, rolling []*revision.Revision) (map[string]string, []*TransitiveDep, error) {
		// Download the DEPS file from the parent repo.
		var buf bytes.Buffer
		if err := repo.ReadFileAtRef(ctx, deps_parser.DepsFileName, baseCommit, &buf); err != nil {
			return nil, nil, skerr.Wrapf(err, "Failed to retrieve DEPS file")
		}

		// Write the new DEPS content.
		depsContent, err := deps_parser.SetDep(buf.String(), c.Dep, to.Id)
		if err != nil {
			return nil, nil, skerr.Wrapf(err, "Failed to set new revision")
		}
		// We'll add the new DEPS content to the changes for the roll
		// after we handle transitive DEPS.
		changes := map[string]string{}

		// Handle transitive DEPS.
		var transitiveDeps []*TransitiveDep
		if len(c.TransitiveDeps) > 0 {
			entries, err := deps_parser.ParseDeps(depsContent)
			if err != nil {
				return nil, nil, skerr.Wrap(err)
			}
			transitiveDeps = make([]*TransitiveDep, 0, len(c.TransitiveDeps))
			for dep, path := range c.TransitiveDeps {
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
				transitiveDeps = append(transitiveDeps, &TransitiveDep{
					ParentPath:  oldVersion.Path,
					RollingFrom: oldVersion.Version,
					RollingTo:   newVersion,
				})
			}
		}

		changes[deps_parser.DepsFileName] = depsContent
		return changes, transitiveDeps, nil
	}
	return newGitiles(ctx, c.GitilesConfig, reg, client, serverURL, update, getChangesForRoll)
}
