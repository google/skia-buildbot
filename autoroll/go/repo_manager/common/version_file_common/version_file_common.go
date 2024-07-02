package version_file_common

import (
	"context"
	"regexp"
	"strings"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/common/pyl"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/bazel"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

func getUsingRegex(dep *config.VersionFileConfig, contents string) (string, string, error) {
	re, err := regexp.Compile(dep.Regex)
	if err != nil {
		// We check the regex when we validate the config, so in theory we
		// shouldn't run into this.
		return "", "", skerr.Wrap(err)
	}
	match := re.FindStringSubmatch(contents)
	if match == nil {
		return "", "", skerr.Fmt("no match found for regex `%s` in:\n%s", dep.Regex, contents)
	}
	// We expect the regex to contain exactly one capture group, so the
	// match slice should contain two elements: entire matched text, and the
	// contents of the capture group.
	if len(match) != 2 {
		return "", "", skerr.Fmt("wrong number of matches found for regex `%s`; expected two; Contents:\n%s", dep.Regex, contents)
	}
	return match[0], match[1], nil
}

// GetPinnedRev reads the given file contents to find the pinned revision.
func GetPinnedRev(dep *config.VersionFileConfig, contents string) (string, error) {
	if dep.Regex != "" {
		_, revision, err := getUsingRegex(dep, contents)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		return revision, nil
	} else if dep.Path == deps_parser.DepsFileName {
		depsEntry, err := deps_parser.GetDep(contents, dep.Id)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		return depsEntry.Version, nil
	} else if strings.HasSuffix(dep.Path, ".pyl") {
		return pyl.Get(contents, dep.Id)
	} else if bazel.IsBazelFile(dep.Path) {
		entry, err := bazel.GetDep(contents, bazel.DependencyID(dep.Id))
		if err != nil {
			return "", skerr.Wrap(err)
		}
		return entry.Version, nil
	} else {
		return strings.TrimSpace(contents), nil
	}
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
func SetPinnedRev(dep *config.VersionFileConfig, newRev *revision.Revision, oldContents string) (string, error) {
	if dep.Regex != "" {
		fullMatch, oldVersion, err := getUsingRegex(dep, oldContents)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		// Replace the full string matched by the regex instead of just the
		// revision ID itself, in case the same string appears more than once in
		// the file.
		repl := strings.Replace(fullMatch, oldVersion, newRev.Id, 1)
		newContents := strings.Replace(oldContents, fullMatch, repl, 1)
		return newContents, nil
	} else if dep.Path == deps_parser.DepsFileName {
		newContents, err := deps_parser.SetDep(oldContents, dep.Id, newRev.Id)
		return newContents, skerr.Wrap(err)
	} else if strings.HasSuffix(dep.Path, ".pyl") {
		return pyl.Set(oldContents, dep.Id, newRev.Id)
	} else if bazel.IsBazelFile(dep.Path) {
		newContents, err := bazel.SetDep(oldContents, bazel.DependencyID(dep.Id), newRev.Id, newRev.Checksum)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		return newContents, nil
	} else {
		// Various tools expect a newline at the end of the file.
		// TODO(borenet): This should probably be configurable.
		return newRev.Id + "\n", nil
	}
}

// GetFileFunc is a function which retrieves the contents of a file.
type GetFileFunc func(ctx context.Context, path string) (string, error)

// updateSingleDep updates the dependency in the given file, writing the new
// contents into the changes map and returning the previous version.
func updateSingleDep(ctx context.Context, dep *config.VersionFileConfig, newRev *revision.Revision, changes map[string]string, getFile GetFileFunc) (string, error) {
	// Look up the path in our changes map to prevent overwriting
	// modifications we've already made.
	oldContents, ok := changes[dep.Path]
	if !ok {
		var err error
		oldContents, err = getFile(ctx, dep.Path)
		if err != nil {
			return "", skerr.Wrap(err)
		}
	}

	// Find the currently-pinned revision.
	oldVersion, err := GetPinnedRev(dep, oldContents)
	if err != nil {
		return "", skerr.Wrap(err)
	}

	// Create the new file content.
	if newRev.Id != oldVersion {
		newContents, err := SetPinnedRev(dep, newRev, oldContents)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		if newContents == oldContents {
			return "", skerr.Fmt("Failed to update dependency %s from %s to %s in %s; new contents identical to old contents:\n%s", dep.Id, oldVersion, newRev.Id, dep.Path, oldContents)
		}
		changes[dep.Path] = newContents

		// Update gitlink if it exists.
		if dep.Path == deps_parser.DepsFileName {
			hasSubmodules, err := deps_parser.HasGitSubmodules(oldContents)
			if err != nil {
				return "", skerr.Wrap(err)
			}
			if hasSubmodules {
				depsEntry, err := deps_parser.GetDep(oldContents, dep.Id)
				if err != nil {
					return "", skerr.Wrap(err)
				}
				oldContents, err = getFile(ctx, depsEntry.Path)
				if err == nil {
					// Path found
					newContents = strings.ReplaceAll(oldContents, oldVersion, newRev.Id)
					if oldContents != newContents {
						changes[depsEntry.Path] = newContents
					}
				}
			}
		}

	}
	return oldVersion, nil
}

// UpdateDep updates the given dependency to the given revision, also updating
// any transitive dependencies to the revisions specified in the new revision of
// the primary dependency. Returns a map whose keys are file names to update and
// values are their updated contents.
func UpdateDep(ctx context.Context, primaryDep *config.DependencyConfig, rev *revision.Revision, getFile GetFileFunc) (map[string]string, error) {
	// Update the primary dependency.
	changes := make(map[string]string, 1+len(primaryDep.Transitive))
	oldRev, err := updateSingleDep(ctx, primaryDep.Primary, rev, changes, getFile)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	replacements := map[string]string{
		oldRev: rev.Id,
	}

	// Handle transitive dependencies.
	if len(primaryDep.Transitive) > 0 {
		for _, dep := range primaryDep.Transitive {
			// Find the new revision.
			newRev, ok := rev.Dependencies[dep.Child.Id]
			if !ok {
				return nil, skerr.Fmt("Could not find transitive dependency %q in %#v", dep.Child.Id, rev)
			}
			// Update.
			oldRev, err := updateSingleDep(ctx, dep.Parent, &revision.Revision{Id: newRev}, changes, getFile)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			replacements[oldRev] = newRev
		}
	}

	// Handle find-and-replace.
	sklog.Infof("find and replace count: \"%d\".", len(primaryDep.FindAndReplace))
	for _, f := range primaryDep.FindAndReplace {
		oldContents, ok := changes[f]
		if !ok {
			oldContents, err = getFile(ctx, f)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
		}
		newContents := oldContents
		for oldRev, newRev := range replacements {
			sklog.Infof("replacing \"%s\" with \"%s\" in %s.", oldRev, newRev, f)
			newContents = strings.ReplaceAll(newContents, oldRev, newRev)
		}
		if oldContents != newContents {
			changes[f] = newContents
		} else if _, ok := changes[f]; !ok {
			sklog.Warningf("find-and-replace made no changes to %s", f)
		}
	}

	return changes, nil
}
