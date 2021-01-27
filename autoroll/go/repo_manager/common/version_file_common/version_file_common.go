package version_file_common

import (
	"context"
	"strings"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/skerr"
)

// GetPinnedRev reads the given file contents to find the pinned revision.
func GetPinnedRev(dep *config.VersionFileConfig, contents string) (string, error) {
	if dep.Path == deps_parser.DepsFileName {
		depsEntry, err := deps_parser.GetDep(contents, dep.Id)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		return depsEntry.Version, nil
	}
	return strings.TrimSpace(contents), nil
}

// GetPinnedRevs reads files using the given GetFileFunc to retrieve the given
// pinned revisions. File retrievals are cached for efficiency.
func GetPinnedRevs(ctx context.Context, deps []*config.VersionFileConfig, getFile GetFileFunc) (map[string]string, error) {
	rv := make(map[string]string, len(deps))
	// Cache files in case multiple dependencies are versioned in
	// the same file, eg. DEPS.
	cache := map[string]string{}
	for _, dep := range deps {
		contents, ok := cache[dep.Path]
		if !ok {
			var err error
			contents, err = getFile(ctx, dep.Path)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			cache[dep.Path] = contents
		}
		version, err := GetPinnedRev(dep, contents)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv[dep.Id] = version
	}
	return rv, nil
}

// SetPinnedRev updates the given dependency pin in the given file, returning
// the new contents.
func SetPinnedRev(dep *config.VersionFileConfig, newVersion, oldContents string) (string, error) {
	if dep.Path == deps_parser.DepsFileName {
		newContents, err := deps_parser.SetDep(oldContents, dep.Id, newVersion)
		return newContents, skerr.Wrap(err)
	}
	// Various tools expect a newline at the end of the file.
	// TODO(borenet): This should probably be configurable.
	return newVersion + "\n", nil
}

// GetFileFunc is a function which retrieves the contents of a file.
type GetFileFunc func(ctx context.Context, path string) (string, error)

// updateSingleDep updates the dependency in the given file, writing the new
// contents into the changes map and returning the previous version.
func updateSingleDep(ctx context.Context, dep *config.VersionFileConfig, newVersion string, changes map[string]string, getFile GetFileFunc) error {
	// Look up the path in our changes map to prevent overwriting
	// modifications we've already made.
	oldContents, ok := changes[dep.Path]
	if !ok {
		var err error
		oldContents, err = getFile(ctx, dep.Path)
		if err != nil {
			return skerr.Wrap(err)
		}
	}

	// Find the currently-pinned revision.
	oldVersion, err := GetPinnedRev(dep, oldContents)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Create the new file content.
	if newVersion != oldVersion {
		newContents, err := SetPinnedRev(dep, newVersion, oldContents)
		if err != nil {
			return skerr.Wrap(err)
		}
		changes[dep.Path] = newContents
	}
	return nil
}

// UpdateDep updates the given dependency to the given revision, also updating
// any transitive dependencies to the revisions specified in the new revision of
// the primary dependency. Returns a map whose keys are file names to update and
// values are their updated contents.
func UpdateDep(ctx context.Context, primaryDep *config.DependencyConfig, rev *revision.Revision, getFile GetFileFunc) (map[string]string, error) {
	// Update the primary dependency.
	changes := make(map[string]string, 1+len(primaryDep.Transitive))
	if err := updateSingleDep(ctx, primaryDep.Primary, rev.Id, changes, getFile); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Handle transitive dependencies.
	if len(primaryDep.Transitive) > 0 {
		for _, dep := range primaryDep.Transitive {
			// Find the new revision.
			newVersion, ok := rev.Dependencies[dep.Child.Id]
			if !ok {
				return nil, skerr.Fmt("Could not find transitive dependency %q in %#v", dep.Child.Id, rev)
			}
			// Update.
			if err := updateSingleDep(ctx, dep.Parent, newVersion, changes, getFile); err != nil {
				return nil, skerr.Wrap(err)
			}
		}
	}

	return changes, nil
}
