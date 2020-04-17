package version_file_common

import (
	"context"
	"strings"

	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// VersionFileConfig provides configuration for a file in a git repository which
// pins the version of a dependency.
type VersionFileConfig struct {
	// Dep is the ID of the dependency to be rolled, eg. a repo URL.
	Dep string `json:"dep"`

	// Path is the path within the repo to the file which pins the
	// dependency.
	Path string `json:"path"`
}

// See documentation for util.Validator interface.
func (c VersionFileConfig) Validate() error {
	if c.Dep == "" {
		return skerr.Fmt("Dep is required")
	}
	if c.Path == "" {
		return skerr.Fmt("Path is required")
	}
	return nil
}

// DependencyConfig provides configuration for a dependency whose version is
// pinned in a file and which may have transitive dependencies.
type DependencyConfig struct {
	// Primary dependency.
	VersionFileConfig
	// Transitive dependencies.
	TransitiveDeps []*VersionFileConfig
}

// See documentation for util.Validator interface.
func (c DependencyConfig) Validate() error {
	if err := c.VersionFileConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	for _, td := range c.TransitiveDeps {
		if err := td.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// GetPinnedRev reads the given file contents to find the pinned revision.
func GetPinnedRev(path, dep, contents string) (string, error) {
	if path == deps_parser.DepsFileName {
		depsEntry, err := deps_parser.GetDep(contents, dep)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		return depsEntry.Version, nil
	} else {
		return strings.TrimSpace(contents), nil
	}
}

// SetPinnedRev updates the given dependency pin in the given file, returning
// the new contents.
func SetPinnedRev(path, dep, newVersion, oldContents string) (string, error) {
	if path == deps_parser.DepsFileName {
		newContents, err := deps_parser.SetDep(oldContents, dep, newVersion)
		return newContents, skerr.Wrap(err)
	}
	return newVersion, nil
}

type GetFileFunc func(ctx context.Context, path string) (string, error)

// updateSingleDep updates the dependency in the given file, writing the new
// contents into the changes map and returning the previous version.
func updateSingleDep(ctx context.Context, path, dep, newVersion string, changes map[string]string, getFile GetFileFunc) (string, error) {
	// Look up the path in our changes map to prevent overwriting
	// modifications we've already made.
	oldContents, ok := changes[path]
	if !ok {
		var err error
		oldContents, err = getFile(ctx, path)
		if err != nil {
			return "", skerr.Wrap(err)
		}
	}

	// Find the currently-pinned revision.
	oldVersion, err := GetPinnedRev(path, dep, oldContents)
	if err != nil {
		return "", skerr.Wrap(err)
	}

	// Create the new file content.
	if newVersion != oldVersion {
		newContents, err := SetPinnedRev(path, dep, newVersion, oldContents)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		changes[path] = newContents
	}
	return oldVersion, nil
}

// UpdateDep updates the given dependency to the given revision, also updating
// any transitive dependencies to the revisions specified in the new revision of
// the primary dependency. Returns a map whose keys are file names to update and
// values are their updated contents.
func UpdateDep(ctx context.Context, primaryDep DependencyConfig, rev *revision.Revision, getFile GetFileFunc) (map[string]string, error) {
	// Update the primary dependency.
	changes := make(map[string]string, 1+len(primaryDep.TransitiveDeps))
	if _, err := updateSingleDep(ctx, primaryDep.Path, primaryDep.Dep, rev.Id, changes, getFile); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Handle transitive dependencies.
	//var td []*TransitiveDep
	if len(primaryDep.TransitiveDeps) > 0 {
		//td = make([]*TransitiveDep, 0, len(transitiveDeps))
		for _, dep := range primaryDep.TransitiveDeps {
			// Find the new revision.
			newVersion, ok := rev.Dependencies[dep.Dep]
			if !ok {
				sklog.Errorf("Could not find transitive dependency %q in %+v", dep.Dep, rev)
				continue
			}
			// Update.
			/*oldVersion*/
			_, err := updateSingleDep(ctx, dep.Path, dep.Dep, newVersion, changes, getFile)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			// Add the transitive dep to the list.
			//td = append(td, &TransitiveDep{
			//	ParentPath:  dep,
			//	RollingFrom: oldVersion,
			//	RollingTo:   newVersion,
			//})
		}
	}

	return changes /*td,*/, nil
}
