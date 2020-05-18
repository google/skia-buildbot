package version_file_common

import (
	"context"
	"strings"

	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/skerr"
)

// VersionFileConfig provides configuration for a file in a git repository which
// pins the version of a dependency.
type VersionFileConfig struct {
	// ID of the dependency to be rolled, eg. a repo URL.
	ID string `json:"id"`

	// Path is the path within the repo to the file which pins the
	// dependency.
	Path string `json:"path"`
}

// See documentation for util.Validator interface.
func (c VersionFileConfig) Validate() error {
	if c.ID == "" {
		return skerr.Fmt("ID is required")
	}
	if c.Path == "" {
		return skerr.Fmt("Path is required")
	}
	return nil
}

// TransitiveDepConfig provides configuration for a single transitive
// dependency.
type TransitiveDepConfig struct {
	Child  *VersionFileConfig `json:"child"`
	Parent *VersionFileConfig `json:"parent"`
}

// See documentation for util.Validator interface.
func (c *TransitiveDepConfig) Validate() error {
	if c.Child == nil {
		return skerr.Fmt("Child is required")
	}
	if err := c.Child.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.Parent == nil {
		return skerr.Fmt("Parent is required")
	}
	if err := c.Parent.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// DependencyConfig provides configuration for a dependency whose version is
// pinned in a file and which may have transitive dependencies.
type DependencyConfig struct {
	// Primary dependency.
	VersionFileConfig
	// Transitive dependencies.
	TransitiveDeps TransitiveDepConfigs
}

// See documentation for util.Validator interface.
func (c DependencyConfig) Validate() error {
	if err := c.VersionFileConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if err := c.TransitiveDeps.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// TransitiveDepConfigs provide configuration for multiple transitive
// dependencies.
type TransitiveDepConfigs []*TransitiveDepConfig

// See documentation for util.Validator interface.
func (c TransitiveDepConfigs) Validate() error {
	for _, elem := range c {
		if err := elem.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// GetPinnedRev reads the given file contents to find the pinned revision.
func GetPinnedRev(dep VersionFileConfig, contents string) (string, error) {
	if dep.Path == deps_parser.DepsFileName {
		depsEntry, err := deps_parser.GetDep(contents, dep.ID)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		return depsEntry.Version, nil
	} else {
		return strings.TrimSpace(contents), nil
	}
}

// GetPinnedRevs reads files using the given GetFileFunc to retrieve the given
// pinned revisions. File retrievals are cached for efficiency.
func GetPinnedRevs(ctx context.Context, deps []*VersionFileConfig, getFile GetFileFunc) (map[string]string, error) {
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
		version, err := GetPinnedRev(*dep, contents)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv[dep.ID] = version
	}
	return rv, nil
}

// SetPinnedRev updates the given dependency pin in the given file, returning
// the new contents.
func SetPinnedRev(dep VersionFileConfig, newVersion, oldContents string) (string, error) {
	if dep.Path == deps_parser.DepsFileName {
		newContents, err := deps_parser.SetDep(oldContents, dep.ID, newVersion)
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
func updateSingleDep(ctx context.Context, dep VersionFileConfig, newVersion string, changes map[string]string, getFile GetFileFunc) error {
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
func UpdateDep(ctx context.Context, primaryDep DependencyConfig, rev *revision.Revision, getFile GetFileFunc) (map[string]string, error) {
	// Update the primary dependency.
	changes := make(map[string]string, 1+len(primaryDep.TransitiveDeps))
	if err := updateSingleDep(ctx, primaryDep.VersionFileConfig, rev.Id, changes, getFile); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Handle transitive dependencies.
	if len(primaryDep.TransitiveDeps) > 0 {
		for _, dep := range primaryDep.TransitiveDeps {
			// Find the new revision.
			newVersion, ok := rev.Dependencies[dep.Child.ID]
			if !ok {
				return nil, skerr.Fmt("Could not find transitive dependency %q in %#v", dep.Child.ID, rev)
			}
			// Update.
			if err := updateSingleDep(ctx, *dep.Parent, newVersion, changes, getFile); err != nil {
				return nil, skerr.Wrap(err)
			}
		}
	}

	return changes, nil
}
